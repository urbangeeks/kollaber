package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

const (
	defaultSearchLimit = 25
	maxSearchLimit     = 100
)

type SearchHandler struct{ q *store.Queries }

func NewSearchHandler(q *store.Queries) *SearchHandler { return &SearchHandler{q} }

type searchComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at"`
}

// searchHit carries the event for both kinds. A comment match without its event
// is a quote with no context — you'd know someone wrote "rolled it back" but
// not what they rolled back or when.
type searchHit struct {
	Kind    string         `json:"kind"`
	Event   eventResponse  `json:"event"`
	Comment *searchComment `json:"comment,omitempty"`
	Rank    float32        `json:"rank"`
}

type searchResponse struct {
	Query   string      `json:"query"`
	Count   int         `json:"count"`
	Results []searchHit `json:"results"`
}

// Search handles GET /search?q=…&environment_id=…&limit=…
func (h *SearchHandler) Search(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	query := strings.TrimSpace(c.QueryParam("q"))
	if query == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "q is required"})
	}

	params := store.SearchParams{
		OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
		Query: query,
		Limit: parseInt32(c.QueryParam("limit"), defaultSearchLimit, maxSearchLimit),
	}

	if envStr := c.QueryParam("environment_id"); envStr != "" {
		envID, err := uuid.Parse(envStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id must be a uuid"})
		}
		pg := pgtype.UUID{Bytes: envID, Valid: true}
		params.EnvironmentID = &pg
	}

	hits, err := h.q.Search(c.Request().Context(), params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "search failed"})
	}

	results := make([]searchHit, 0, len(hits))
	for _, hit := range hits {
		out := searchHit{
			Kind:  hit.Kind,
			Event: toEventResponse(hit.Event),
			Rank:  hit.Rank,
		}
		if hit.Kind == "comment" {
			out.Comment = &searchComment{
				ID:        uuid.UUID(hit.CommentID.Bytes).String(),
				Body:      hit.CommentBody,
				UserID:    uuid.UUID(hit.CommentUserID.Bytes).String(),
				CreatedAt: hit.CommentCreatedAt.Time.Format(time.RFC3339),
			}
		}
		results = append(results, out)
	}

	return c.JSON(http.StatusOK, searchResponse{
		Query:   query,
		Count:   len(results),
		Results: results,
	})
}
