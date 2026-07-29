package api

import (
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/urbangeeks/kollaber/internal/store"
)

func annotationRow(eventType, service, status, envName string, at time.Time, metadata string) store.AnnotationRow {
	return store.AnnotationRow{
		Event: store.Event{
			ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
			Type:      eventType,
			Service:   service,
			Status:    status,
			Timestamp: pgTime(at),
			Metadata:  []byte(metadata),
		},
		EnvironmentName: envName,
	}
}

// Notes are the one type that must not appear on someone's dashboard by
// default, and every other valid type must, so a new event type is visible
// rather than silently missing.
func TestAnnotationTypesExcludesNotesAndNothingElse(t *testing.T) {
	got := annotationTypes()

	if slices.Contains(got, "note") {
		t.Error("notes would render as markers by default")
	}
	for _, valid := range store.ValidEventTypes {
		if valid == "note" {
			continue
		}
		if !slices.Contains(got, valid) {
			t.Errorf("event type %q is missing from the default annotation set", valid)
		}
	}
}

func TestToAnnotation(t *testing.T) {
	at := time.Date(2026, 7, 29, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name      string
		row       store.AnnotationRow
		wantTitle string
		wantText  string
		wantTags  []string
	}{
		{
			name:      "successful deploy",
			row:       annotationRow("deploy", "api", "success", "prod", at, `{"version":"v1.2.3","author":"jerome"}`),
			wantTitle: "deploy api",
			wantText:  "author=jerome, version=v1.2.3",
			wantTags:  []string{"deploy", "prod", "api"},
		},
		{
			// A status worth noticing earns a tag; "success" does not, or the
			// tag would be on almost every marker and filter nothing.
			name:      "failed deploy is tagged",
			row:       annotationRow("deploy", "api", "failure", "prod", at, `{}`),
			wantTitle: "deploy api",
			wantText:  "",
			wantTags:  []string{"deploy", "prod", "api", "failure"},
		},
		{
			name:      "event with no service",
			row:       annotationRow("alert", "", "failure", "staging", at, `{"summary":"5xx spike"}`),
			wantTitle: "alert",
			wantText:  "summary=5xx spike",
			wantTags:  []string{"alert", "staging", "failure"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toAnnotation(tt.row)

			if got.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", got.Title, tt.wantTitle)
			}
			if got.Text != tt.wantText {
				t.Errorf("text = %q, want %q", got.Text, tt.wantText)
			}
			if !slices.Equal(got.Tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", got.Tags, tt.wantTags)
			}
			// Grafana places markers by epoch milliseconds; seconds would put
			// every one of them in January 1970.
			if got.Time != at.UnixMilli() {
				t.Errorf("time = %d, want %d (epoch ms)", got.Time, at.UnixMilli())
			}
		})
	}
}

// Metadata feeds a tooltip here rather than a markdown table, so it must not
// carry the table escaping the postmortem needs.
func TestAnnotationTextIsNotMarkdownEscaped(t *testing.T) {
	row := annotationRow("deploy", "api", "success", "prod", time.Now(), `{"cmd":"sh -c 'a | b'"}`)

	if got := toAnnotation(row).Text; got != "cmd=sh -c 'a | b'" {
		t.Errorf("text = %q, want an unescaped pipe", got)
	}
}

func TestParseAnnotationWindow(t *testing.T) {
	tests := []struct {
		name    string
		from    string
		to      string
		wantErr bool
	}{
		{"both empty defaults to the last day", "", "", false},
		{"rfc3339 range", "2026-07-01T00:00:00Z", "2026-07-02T00:00:00Z", false},
		{"grafana sends fractional seconds", "2026-07-01T00:00:00.070Z", "2026-07-02T00:00:00.000Z", false},
		{"epoch milliseconds", "1751328000000", "1751414400000", false},
		{"mixed formats", "1751328000000", "2026-07-02T00:00:00Z", false},
		{"from after to", "2026-07-02T00:00:00Z", "2026-07-01T00:00:00Z", true},
		{"from equals to", "2026-07-01T00:00:00Z", "2026-07-01T00:00:00Z", true},
		{"malformed from", "yesterday", "", true},
		{"malformed to", "", "tomorrow", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, err := parseAnnotationWindow(tt.from, tt.to)
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

func TestParseAnnotationFilters(t *testing.T) {
	envID := uuid.New()

	tests := []struct {
		name      string
		query     string
		wantErr   bool
		wantTypes []string
		wantSvc   string
		wantEnv   bool
	}{
		{
			name:      "empty query keeps the defaults",
			query:     "",
			wantTypes: annotationTypes(),
		},
		{
			name:      "single type",
			query:     "type=deploy",
			wantTypes: []string{"deploy"},
		},
		{
			name:      "comma separated types with spaces",
			query:     "type=deploy,+rollback",
			wantTypes: []string{"deploy", "rollback"},
		},
		{
			// The exclusion is a default, not a rule about what may be asked
			// for: someone who wants notes on their dashboard can have them.
			name:      "notes can be requested explicitly",
			query:     "type=note",
			wantTypes: []string{"note"},
		},
		{
			name:    "service filter",
			query:   "service=api",
			wantSvc: "api",
		},
		{
			name:    "environment filter",
			query:   "environment_id=" + envID.String(),
			wantEnv: true,
		},
		{
			// A typo must say so rather than quietly producing a panel that is
			// always bare, which reads as "nothing ever happens here".
			name:    "unknown type is an error",
			query:   "type=deploys",
			wantErr: true,
		},
		{
			name:    "malformed environment id",
			query:   "environment_id=not-a-uuid",
			wantErr: true,
		},
		{
			name:    "type present but empty",
			query:   "type=,",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := url.ParseQuery(tt.query)
			if err != nil {
				t.Fatalf("bad test query %q: %v", tt.query, err)
			}

			got, err := parseAnnotationFilters(values)
			if tt.wantErr {
				if err == nil {
					t.Fatal("want error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantTypes != nil && !slices.Equal(got.types, tt.wantTypes) {
				t.Errorf("types = %v, want %v", got.types, tt.wantTypes)
			}
			if got.service != tt.wantSvc {
				t.Errorf("service = %q, want %q", got.service, tt.wantSvc)
			}
			if (got.environmentID != nil) != tt.wantEnv {
				t.Errorf("environmentID set = %v, want %v", got.environmentID != nil, tt.wantEnv)
			}
		})
	}
}

// The free-text box Grafana gives a dashboard author is read as a query string
// so that a POST and a GET filter through the same code. A bare word in that
// box must degrade to the defaults, not to an error the author cannot see.
func TestBareWordInAnnotationQueryFallsBackToDefaults(t *testing.T) {
	values, err := url.ParseQuery("prod")
	if err != nil {
		t.Fatalf("a bare word must not fail to parse: %v", err)
	}

	got, err := parseAnnotationFilters(values)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !slices.Equal(got.types, annotationTypes()) {
		t.Errorf("types = %v, want the defaults", got.types)
	}
}
