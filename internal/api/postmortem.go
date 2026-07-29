package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	kollabai "github.com/urbangeeks/kollaber/internal/ai"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

const (
	// Caps on what one document will pull in. A postmortem over a month of a
	// busy environment is not a document anyone reads, and it would blow the
	// model's context on the narrative step.
	maxPostmortemEvents   = 500
	maxPostmortemComments = 1000
	// Events fed to the model, newest-relevant first. The full set still goes
	// into the rendered timeline; only the narrative works from a sample.
	maxNarrativeEvents = 120
	narrativeMaxTokens = 1500
)

// narrative_status values. The document is always produced; only the AI summary
// section varies, so the caller can explain the difference rather than being
// handed a 402 and nothing else.
const (
	narrativeIncluded     = "included"
	narrativeNotRequested = "not_requested"
	narrativeUpgrade      = "upgrade_required"
	narrativeUnavailable  = "unavailable"
	narrativeFailed       = "failed"
)

type PostmortemHandler struct{ q *store.Queries }

func NewPostmortemHandler(q *store.Queries) *PostmortemHandler { return &PostmortemHandler{q} }

type postmortemRequest struct {
	EnvironmentID uuid.UUID `json:"environment_id"`
	From          string    `json:"from"`
	To            string    `json:"to"`
	// Narrative asks for the AI summary section. Defaults to true; the rest of
	// the document is produced either way.
	Narrative *bool `json:"narrative"`
}

type postmortemResponse struct {
	Markdown        string   `json:"markdown"`
	EnvironmentName string   `json:"environment_name"`
	From            string   `json:"from"`
	To              string   `json:"to"`
	EventCount      int      `json:"event_count"`
	CommentCount    int      `json:"comment_count"`
	Participants    []string `json:"participants"`
	NarrativeStatus string   `json:"narrative_status"`
	Truncated       bool     `json:"truncated"`
}

// Generate handles POST /postmortems — a markdown document covering one
// environment over one time window.
//
// The factual half (timeline, discussion, participants) is assembled from data
// the org already owns and is always returned, on every plan and with or
// without an Anthropic key. Only the narrative section is gated. Withholding
// the user's own timeline behind a plan would be indefensible; withholding a
// model call we pay for is not.
func (h *PostmortemHandler) Generate(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := c.Request().Context()

	var req postmortemRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.EnvironmentID == uuid.Nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id is required"})
	}

	from, to, err := parseWindow(req.From, req.To)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	pgEnvID := pgtype.UUID{Bytes: req.EnvironmentID, Valid: true}
	env, err := h.q.GetEnvironmentByID(ctx, pgEnvID)
	if err != nil || env.OrgID != pgOrgID {
		// Checking org membership here rather than trusting the id: environments
		// are the only thing tying an event to an org.
		return c.JSON(http.StatusNotFound, echo.Map{"error": "environment not found"})
	}

	events, err := h.q.ListEventsFiltered(ctx, store.ListEventsFilterParams{
		OrgID:         pgOrgID,
		EnvironmentID: &pgEnvID,
		After:         &from,
		Before:        &to,
		Limit:         maxPostmortemEvents,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load events"})
	}

	comments, err := h.q.ListCommentsInRange(ctx, store.ListCommentsInRangeParams{
		OrgID:         pgOrgID,
		EnvironmentID: pgEnvID,
		From:          from,
		To:            to,
		Limit:         maxPostmortemComments,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load comments"})
	}

	// ListEventsFiltered returns newest first; a postmortem reads forwards.
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Timestamp.Time.Before(events[j].Timestamp.Time)
	})

	doc := postmortemDoc{
		Environment: env.Name,
		From:        from,
		To:          to,
		Events:      events,
		Comments:    comments,
	}

	status := narrativeNotRequested
	if req.Narrative == nil || *req.Narrative {
		status = h.narrativeFor(c, pgOrgID, &doc)
	}

	return c.JSON(http.StatusOK, postmortemResponse{
		Markdown:        doc.render(),
		EnvironmentName: env.Name,
		From:            from.UTC().Format(time.RFC3339),
		To:              to.UTC().Format(time.RFC3339),
		EventCount:      len(events),
		CommentCount:    len(comments),
		Participants:    doc.participants(),
		NarrativeStatus: status,
		Truncated:       len(events) >= maxPostmortemEvents,
	})
}

// narrativeFor fills doc.Narrative when the org is entitled and the model
// answers, and reports which of those did not hold otherwise. It never fails
// the request: a missing narrative degrades the document, it does not void it.
func (h *PostmortemHandler) narrativeFor(c echo.Context, orgID pgtype.UUID, doc *postmortemDoc) string {
	ctx := c.Request().Context()

	// Every org has a billing row, so a read failure here is a database problem,
	// not a plan problem. Reporting it as upgrade_required would tell a Pro org
	// to buy what it already has.
	b, err := h.q.GetOrgBilling(ctx, orgID)
	if err != nil {
		return narrativeFailed
	}
	if !billing.EntitlementsFor(b.Plan).AIPostmortems {
		return narrativeUpgrade
	}
	if len(doc.Events) == 0 {
		return narrativeNotRequested
	}

	text, err := kollabai.Generate(ctx, doc.narrativePrompt(), narrativeMaxTokens)
	if err != nil {
		if strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
			return narrativeUnavailable
		}
		return narrativeFailed
	}

	doc.Narrative = strings.TrimSpace(text)
	return narrativeIncluded
}

// parseWindow accepts RFC3339 bounds and defaults to the last 24 hours, which
// is the window a postmortem is usually wanted for.
func parseWindow(fromStr, toStr string) (from, to time.Time, err error) {
	to = time.Now()
	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			return from, to, fmt.Errorf("to must be an RFC3339 timestamp")
		}
	}

	from = to.Add(-24 * time.Hour)
	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return from, to, fmt.Errorf("from must be an RFC3339 timestamp")
		}
	}

	if !from.Before(to) {
		return from, to, fmt.Errorf("from must be before to")
	}
	return from, to, nil
}

// postmortemDoc is everything the document is rendered from. Kept separate from
// the handler so rendering can be tested without a database or a model.
type postmortemDoc struct {
	Environment string
	From        time.Time
	To          time.Time
	Events      []store.Event
	Comments    []store.CommentWithAuthor
	Narrative   string
}

func fmtUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04 UTC")
}

// participants returns the distinct comment authors, most talkative first. They
// are who to ask when the document leaves a question open.
func (d *postmortemDoc) participants() []string {
	counts := map[string]int{}
	for _, c := range d.Comments {
		counts[c.AuthorEmail]++
	}
	out := make([]string, 0, len(counts))
	for email := range counts {
		out = append(out, email)
	}
	sort.Slice(out, func(i, j int) bool {
		if counts[out[i]] != counts[out[j]] {
			return counts[out[i]] > counts[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

func (d *postmortemDoc) countsByType() map[string]int {
	counts := map[string]int{}
	for _, e := range d.Events {
		counts[e.Type]++
	}
	return counts
}

func (d *postmortemDoc) render() string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Postmortem — %s\n\n", d.Environment)
	fmt.Fprintf(&sb, "**Window:** %s → %s  \n", fmtUTC(d.From), fmtUTC(d.To))

	counts := d.countsByType()
	types := make([]string, 0, len(counts))
	for t := range counts {
		types = append(types, t)
	}
	sort.Strings(types)
	parts := make([]string, 0, len(types))
	for _, t := range types {
		parts = append(parts, fmt.Sprintf("%d %s", counts[t], t))
	}
	fmt.Fprintf(&sb, "**Events:** %d", len(d.Events))
	if len(parts) > 0 {
		fmt.Fprintf(&sb, " (%s)", strings.Join(parts, ", "))
	}
	sb.WriteString("  \n")

	people := d.participants()
	if len(people) > 0 {
		fmt.Fprintf(&sb, "**Participants:** %s  \n", strings.Join(people, ", "))
	}
	sb.WriteString("\n")

	sb.WriteString("## Summary\n\n")
	if d.Narrative != "" {
		sb.WriteString(d.Narrative)
		sb.WriteString("\n\n")
	} else {
		sb.WriteString("_No AI narrative was generated for this document._\n\n")
	}

	sb.WriteString("## Timeline\n\n")
	if len(d.Events) == 0 {
		sb.WriteString("_No events in this window._\n\n")
	} else {
		sb.WriteString("| Time (UTC) | Type | Service | Status | Detail |\n")
		sb.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, e := range d.Events {
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n",
				e.Timestamp.Time.UTC().Format("15:04:05"),
				e.Type, e.Service, e.Status, metadataSummary(e.Metadata))
		}
		sb.WriteString("\n")
	}

	if len(d.Comments) > 0 {
		sb.WriteString("## Discussion\n\n")
		byEvent := map[pgtype.UUID][]store.CommentWithAuthor{}
		for _, c := range d.Comments {
			byEvent[c.EventID] = append(byEvent[c.EventID], c)
		}
		// Walk events in order so the discussion follows the timeline rather
		// than map iteration order.
		for _, e := range d.Events {
			thread := byEvent[e.ID]
			if len(thread) == 0 {
				continue
			}
			fmt.Fprintf(&sb, "### %s · %s — %s\n\n",
				e.Type, e.Service, e.Timestamp.Time.UTC().Format("15:04:05"))
			for _, c := range thread {
				fmt.Fprintf(&sb, "- **%s** (%s): %s\n",
					c.AuthorEmail, c.CreatedAt.Time.UTC().Format("15:04"), c.Body)
			}
			sb.WriteString("\n")
		}
	}

	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "_Generated by Kollaber on %s._\n", fmtUTC(time.Now()))

	return sb.String()
}

// metadataPairs renders an event's metadata as compact key=value strings, keys
// sorted so the same event always renders identically. Newlines are flattened
// because every surface this feeds — a table cell, a chart tooltip — is one
// line. Shared with the Grafana annotations endpoint.
func metadataPairs(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil || len(meta) == 0 {
		return nil
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		v := strings.ReplaceAll(fmt.Sprintf("%v", meta[k]), "\n", " ")
		pairs = append(pairs, k+"="+v)
	}
	return pairs
}

// metadataSummary renders metadata for a markdown table cell. Pipes are escaped
// here rather than in metadataPairs because they only break markdown — a
// tooltip showing a stray backslash would be the cost of escaping them for
// everyone.
func metadataSummary(raw []byte) string {
	pairs := metadataPairs(raw)
	for i, p := range pairs {
		pairs[i] = strings.ReplaceAll(p, "|", "\\|")
	}
	return strings.Join(pairs, ", ")
}

// narrativePrompt asks for prose only. The timeline and discussion are rendered
// deterministically from the data, so having the model restate them would only
// add a chance of it restating them wrongly.
func (d *postmortemDoc) narrativePrompt() string {
	var sb strings.Builder
	sb.WriteString(`You are an SRE assistant writing the summary section of an incident postmortem.

Write 2-4 short paragraphs covering: what happened, what the impact appears to
have been, and what the likely root cause was. If the data does not support a
conclusion, say so plainly rather than guessing.

Do not write a timeline or list the events back — those are rendered separately.
Do not invent details. Use only what is below. Output prose only, no headers.

`)
	fmt.Fprintf(&sb, "ENVIRONMENT: %s\nWINDOW: %s to %s\n\nEVENTS\n",
		d.Environment, fmtUTC(d.From), fmtUTC(d.To))

	events := d.Events
	if len(events) > maxNarrativeEvents {
		events = events[len(events)-maxNarrativeEvents:]
		sb.WriteString("(earlier events omitted)\n")
	}
	for _, e := range events {
		fmt.Fprintf(&sb, "[%s] %s %s status=%s %s\n",
			e.Timestamp.Time.UTC().Format(time.RFC3339),
			e.Type, e.Service, e.Status, metadataSummary(e.Metadata))
	}

	if len(d.Comments) > 0 {
		sb.WriteString("\nDISCUSSION\n")
		for _, c := range d.Comments {
			fmt.Fprintf(&sb, "- %s: %s\n", c.AuthorEmail, c.Body)
		}
	}

	return sb.String()
}
