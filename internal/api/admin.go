package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type AdminHandler struct{ q *store.Queries }

func NewAdminHandler(q *store.Queries) *AdminHandler { return &AdminHandler{q} }

type orgStatRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	MemberCount int32  `json:"member_count"`
	EnvCount    int32  `json:"env_count"`
	CreatedAt   string `json:"created_at"`
}

func (h *AdminHandler) ListOrgs(c echo.Context) error {
	rows, err := h.q.ListOrgsWithStats(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch orgs"})
	}
	out := make([]orgStatRow, len(rows))
	for i, r := range rows {
		out[i] = orgStatRow{
			ID:          uuid.UUID(r.ID.Bytes).String(),
			Name:        r.Name,
			Slug:        r.Slug,
			MemberCount: r.MemberCount,
			EnvCount:    r.EnvCount,
			CreatedAt:   r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(http.StatusOK, out)
}
