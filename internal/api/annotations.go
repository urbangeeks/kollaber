package api

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

// A panel carrying a thousand vertical lines is a solid bar, so the cap is set
// where the display stops being useful rather than where the query gets slow.
const maxAnnotations = 1000

// Default window when a caller names neither end. Grafana always sends a range;
// this is for a bare curl.
const defaultAnnotationWindow = 24 * time.Hour

// annotationExcludedTypes are the event types that never render as a marker
// unless asked for by name.
//
// A vertical line on someone else's dashboard should mark something that
// happened to the system, not something a person said about it. Everything else
// in store.ValidEventTypes is a change or a firing and qualifies, so the set is
// derived by exclusion — a new event type shows up on dashboards by default
// instead of being silently missing until someone notices.
var annotationExcludedTypes = map[string]bool{"note": true}

type AnnotationsHandler struct{ q *store.Queries }

func NewAnnotationsHandler(q *store.Queries) *AnnotationsHandler { return &AnnotationsHandler{q} }

// grafanaAnnotation is one marker. The field names and the millisecond epoch
// are Grafana's contract, not ours.
type grafanaAnnotation struct {
	Title string   `json:"title"`
	Text  string   `json:"text"`
	Time  int64    `json:"time"`
	Tags  []string `json:"tags"`
}

// grafanaAnnotationRequest is the body the simple-json datasources POST. Only
// the range and the free-text query are ours to act on; the rest of the
// annotation object is Grafana's own bookkeeping.
type grafanaAnnotationRequest struct {
	Range struct {
		From string `json:"from"`
		To   string `json:"to"`
	} `json:"range"`
	Annotation struct {
		Query string `json:"query"`
	} `json:"annotation"`
}

type annotationFilters struct {
	environmentID *pgtype.UUID
	service       string
	types         []string
}

// List handles GET /annotations, which is what the Infinity datasource and any
// curl will use. Filters come from the query string.
func (h *AnnotationsHandler) List(c echo.Context) error {
	params := c.QueryParams()

	from, to, err := parseAnnotationWindow(params.Get("from"), params.Get("to"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	filters, err := parseAnnotationFilters(params)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return h.serve(c, from, to, filters)
}

// Query handles POST /annotations, the contract the simple-json datasources
// speak.
//
// Grafana gives the dashboard author one free-text box per annotation track, so
// its contents are read as a query string. That way a POST filters through
// exactly the same code as a GET and the two cannot drift.
func (h *AnnotationsHandler) Query(c echo.Context) error {
	var req grafanaAnnotationRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	from, to, err := parseAnnotationWindow(req.Range.From, req.Range.To)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	// A bare word in that box parses as a valueless key and simply selects
	// nothing, which leaves the defaults in place. Someone typing a stray note
	// gets every change back rather than an error they cannot see from Grafana.
	values, err := url.ParseQuery(strings.TrimSpace(req.Annotation.Query))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{
			"error": "annotation query must be url-encoded, e.g. type=deploy&service=api",
		})
	}
	filters, err := parseAnnotationFilters(values)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	return h.serve(c, from, to, filters)
}

func (h *AnnotationsHandler) serve(c echo.Context, from, to time.Time, f annotationFilters) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	rows, err := h.q.ListAnnotations(c.Request().Context(), store.ListAnnotationsParams{
		OrgID:         pgtype.UUID{Bytes: orgID, Valid: true},
		EnvironmentID: f.environmentID,
		Types:         f.types,
		Service:       f.service,
		From:          from,
		To:            to,
		Limit:         maxAnnotations,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load events"})
	}

	// Built with make so an empty result marshals to [] — Grafana treats a null
	// body as a datasource error and shows the panel as broken.
	out := make([]grafanaAnnotation, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAnnotation(r))
	}
	return c.JSON(http.StatusOK, out)
}

func toAnnotation(r store.AnnotationRow) grafanaAnnotation {
	title := r.Type
	if r.Service != "" {
		title += " " + r.Service
	}

	tags := []string{r.Type, r.EnvironmentName}
	if r.Service != "" {
		tags = append(tags, r.Service)
	}
	// Only a status worth noticing earns a tag. Tagging every healthy deploy
	// "success" would make the tag useless for filtering, which is the only
	// thing tags are for.
	if r.Status != "" && r.Status != "success" {
		tags = append(tags, r.Status)
	}

	return grafanaAnnotation{
		Title: title,
		Text:  strings.Join(metadataPairs(r.Metadata), ", "),
		Time:  r.Timestamp.Time.UnixMilli(),
		Tags:  tags,
	}
}

// annotationTypes is the default type set: everything valid that is not
// excluded.
func annotationTypes() []string {
	out := make([]string, 0, len(store.ValidEventTypes))
	for _, t := range store.ValidEventTypes {
		if !annotationExcludedTypes[t] {
			out = append(out, t)
		}
	}
	return out
}

// parseAnnotationFilters reads the filters shared by both verbs. An unknown
// type is an error rather than an empty result, so a typo in a dashboard shows
// up as a message instead of a panel that is simply always bare.
func parseAnnotationFilters(v url.Values) (annotationFilters, error) {
	f := annotationFilters{types: annotationTypes()}

	if s := v.Get("environment_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			return f, fmt.Errorf("environment_id must be a uuid")
		}
		pg := pgtype.UUID{Bytes: id, Valid: true}
		f.environmentID = &pg
	}

	f.service = v.Get("service")

	if raw := v.Get("type"); raw != "" {
		var types []string
		for t := range strings.SplitSeq(raw, ",") {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			// Naming an excluded type explicitly is allowed: the exclusion is a
			// default, not a rule about what may be asked for.
			if !store.IsValidEventType(t) {
				return f, fmt.Errorf("unknown event type %q", t)
			}
			types = append(types, t)
		}
		if len(types) == 0 {
			return f, fmt.Errorf("type filter is empty")
		}
		f.types = types
	}

	return f, nil
}

// parseAnnotationWindow accepts either end as RFC3339 or as epoch milliseconds.
// Grafana sends RFC3339; epoch milliseconds are accepted because that is what
// its own annotation payloads carry, and a person copying one across into a
// curl should not have to convert it by hand.
func parseAnnotationWindow(fromStr, toStr string) (from, to time.Time, err error) {
	to = time.Now()
	if toStr != "" {
		if to, err = parseAnnotationTime(toStr); err != nil {
			return from, to, fmt.Errorf("to: %w", err)
		}
	}

	from = to.Add(-defaultAnnotationWindow)
	if fromStr != "" {
		if from, err = parseAnnotationTime(fromStr); err != nil {
			return from, to, fmt.Errorf("from: %w", err)
		}
	}

	if !from.Before(to) {
		return from, to, fmt.Errorf("from must be before to")
	}
	return from, to, nil
}

func parseAnnotationTime(s string) (time.Time, error) {
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.UnixMilli(ms).UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be an RFC3339 timestamp or epoch milliseconds")
	}
	return t, nil
}
