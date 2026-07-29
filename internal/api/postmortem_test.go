package api

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/urbangeeks/kollaber/internal/store"
)

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func docEvent(eventType, service, status string, at time.Time, metadata string) store.Event {
	return store.Event{
		ID:            pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Type:          eventType,
		Service:       service,
		Status:        status,
		EnvironmentID: pgtype.UUID{Bytes: uuid.New(), Valid: true},
		Timestamp:     pgTime(at),
		Metadata:      []byte(metadata),
	}
}

func docComment(eventID pgtype.UUID, email, body string, at time.Time) store.CommentWithAuthor {
	return store.CommentWithAuthor{
		Comment: store.Comment{
			ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
			EventID:   eventID,
			Body:      body,
			CreatedAt: pgTime(at),
		},
		AuthorEmail: email,
	}
}

func sampleDoc() postmortemDoc {
	base := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	deploy := docEvent("deploy", "api", "success", base, `{"version":"v1.2.3","author":"jerome"}`)
	alert := docEvent("alert", "api", "failure", base.Add(5*time.Minute), `{"summary":"5xx spike"}`)

	return postmortemDoc{
		Environment: "prod",
		From:        base.Add(-time.Hour),
		To:          base.Add(time.Hour),
		Events:      []store.Event{deploy, alert},
		Comments: []store.CommentWithAuthor{
			docComment(alert.ID, "alice@example.com", "Rolling back", base.Add(6*time.Minute)),
			docComment(alert.ID, "bob@example.com", "Confirmed recovered", base.Add(9*time.Minute)),
			docComment(alert.ID, "alice@example.com", "Adding a regression test", base.Add(12*time.Minute)),
		},
	}
}

func TestRenderIncludesEveryFactualSection(t *testing.T) {
	doc := sampleDoc()
	md := doc.render()

	for _, want := range []string{
		"# Postmortem — prod",
		"**Events:** 2 (1 alert, 1 deploy)",
		"## Timeline",
		"## Discussion",
		"| 12:00:00 | deploy | api | success | author=jerome, version=v1.2.3 |",
		"| 12:05:00 | alert | api | failure | summary=5xx spike |",
		"**alice@example.com** (12:06): Rolling back",
		"**bob@example.com** (12:09): Confirmed recovered",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered document is missing %q\n---\n%s", want, md)
		}
	}
}

// The factual document must stand on its own. If it only made sense with the
// AI section present, gating that section behind a plan would gate the user's
// own timeline behind a plan too.
func TestRenderWithoutNarrativeStillHasTheFacts(t *testing.T) {
	doc := sampleDoc()
	md := doc.render()

	if !strings.Contains(md, "_No AI narrative was generated") {
		t.Error("expected a placeholder where the narrative would go")
	}
	if !strings.Contains(md, "## Timeline") || !strings.Contains(md, "12:05:00") {
		t.Error("timeline missing from a document with no narrative")
	}
}

func TestRenderIncludesNarrativeWhenPresent(t *testing.T) {
	doc := sampleDoc()
	doc.Narrative = "A bad deploy took down checkout for six minutes."
	md := doc.render()

	if !strings.Contains(md, doc.Narrative) {
		t.Error("narrative missing from rendered document")
	}
	if strings.Contains(md, "_No AI narrative was generated") {
		t.Error("placeholder shown despite a narrative being present")
	}
}

func TestParticipantsOrderedByVolume(t *testing.T) {
	doc := sampleDoc()
	got := doc.participants()

	want := []string{"alice@example.com", "bob@example.com"} // alice wrote 2, bob 1
	if len(got) != len(want) {
		t.Fatalf("want %d participants, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("participant %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRenderEmptyWindow(t *testing.T) {
	doc := postmortemDoc{
		Environment: "staging",
		From:        time.Now().Add(-time.Hour),
		To:          time.Now(),
	}
	md := doc.render()

	if !strings.Contains(md, "_No events in this window._") {
		t.Error("expected an explicit empty-window note")
	}
	if strings.Contains(md, "## Discussion") {
		t.Error("discussion section rendered with no comments")
	}
}

// A pipe in metadata would break out of its table cell and corrupt every column
// to its right.
func TestMetadataSummaryEscapesTableBreakingCharacters(t *testing.T) {
	got := metadataSummary([]byte(`{"cmd":"sh -c 'a | b'","note":"line\nbreak"}`))

	// Strip the escaped pipes; anything left is a live one.
	if strings.Contains(strings.ReplaceAll(got, `\|`, ""), "|") {
		t.Errorf("unescaped pipe would break the markdown table: %q", got)
	}
	if !strings.Contains(got, `\|`) {
		t.Errorf("pipe was dropped rather than escaped: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("newline would break the markdown table: %q", got)
	}
}

func TestMetadataSummaryIsStable(t *testing.T) {
	raw := []byte(`{"zebra":"1","alpha":"2","middle":"3"}`)
	first := metadataSummary(raw)

	// Go map iteration is randomised, so an unsorted implementation passes
	// sometimes. Repeat enough to make that unlikely.
	for range 50 {
		if got := metadataSummary(raw); got != first {
			t.Fatalf("metadata order is not stable: %q then %q", first, got)
		}
	}
	if first != "alpha=2, middle=3, zebra=1" {
		t.Errorf("keys not sorted: %q", first)
	}
}

func TestMetadataSummaryHandlesJunk(t *testing.T) {
	for _, raw := range []string{"", "{}", "null", "not json", `["a"]`} {
		if got := metadataSummary([]byte(raw)); got != "" {
			t.Errorf("metadataSummary(%q) = %q, want empty", raw, got)
		}
	}
}

func TestParseWindow(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"both empty defaults to last 24h", "", "", false},
		{"explicit range", "2026-07-01T00:00:00Z", "2026-07-02T00:00:00Z", false},
		{"from after to", "2026-07-02T00:00:00Z", "2026-07-01T00:00:00Z", true},
		{"from equals to", "2026-07-01T00:00:00Z", "2026-07-01T00:00:00Z", true},
		{"malformed from", "yesterday", "", true},
		{"malformed to", "", "tomorrow", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := parseWindow(tt.from, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !from.Before(to) {
				t.Errorf("from %s is not before to %s", from, to)
			}
		})
	}
}

// The narrative prompt must not ask the model to restate the timeline: the
// timeline is rendered from data, and a restatement can only introduce drift.
func TestNarrativePromptAsksForProseOnly(t *testing.T) {
	doc := sampleDoc()
	prompt := doc.narrativePrompt()

	if !strings.Contains(prompt, "Do not write a timeline") {
		t.Error("prompt does not tell the model to skip the timeline")
	}
	if !strings.Contains(prompt, "5xx spike") {
		t.Error("prompt is missing event metadata the model needs")
	}
	if !strings.Contains(prompt, "alice@example.com: Rolling back") {
		t.Error("prompt is missing the discussion")
	}
}

func TestNarrativePromptCapsEventCount(t *testing.T) {
	base := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	doc := postmortemDoc{Environment: "prod", From: base, To: base.Add(48 * time.Hour)}
	for i := range maxNarrativeEvents + 50 {
		doc.Events = append(doc.Events,
			docEvent("deploy", "api", "success", base.Add(time.Duration(i)*time.Minute), `{}`))
	}

	prompt := doc.narrativePrompt()

	if !strings.Contains(prompt, "(earlier events omitted)") {
		t.Error("oversized event list was not truncated")
	}
	if got := strings.Count(prompt, "] deploy api"); got != maxNarrativeEvents {
		t.Errorf("prompt carries %d events, want %d", got, maxNarrativeEvents)
	}
	// Truncation must keep the events closest to the end of the window, since
	// that is where an incident's resolution lives.
	last := doc.Events[len(doc.Events)-1].Timestamp.Time.UTC().Format(time.RFC3339)
	if !strings.Contains(prompt, last) {
		t.Error("truncation dropped the most recent events")
	}
}
