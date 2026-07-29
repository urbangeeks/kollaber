package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type InventoryHandler struct{ q *store.Queries }

func NewInventoryHandler(q *store.Queries) *InventoryHandler { return &InventoryHandler{q} }

type serviceVersionResponse struct {
	Service    string  `json:"service"`
	Version    *string `json:"version"`
	EventID    string  `json:"event_id"`
	EventType  string  `json:"event_type"`
	DeployedAt string  `json:"deployed_at"`
}

type inventoryResponse struct {
	EnvironmentID   string                   `json:"environment_id"`
	EnvironmentName string                   `json:"environment_name"`
	At              string                   `json:"at"`
	Services        []serviceVersionResponse `json:"services"`
}

// List handles GET /inventory — what each service in an environment was running
// at a point in time, defaulting to now.
//
// The question this answers is "what was in prod when this broke?", which today
// takes archaeology in CI logs. Everything needed is already in the deploy
// events; nothing new is collected.
func (h *InventoryHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := c.Request().Context()

	raw := c.QueryParam("environment_id")
	if raw == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id is required"})
	}
	envUUID, err := uuid.Parse(raw)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
	}
	pgEnvID := pgtype.UUID{Bytes: envUUID, Valid: true}

	// Resolve the environment through the org rather than trusting the id: an
	// inventory is a list of what a tenant runs, and handing one to the wrong
	// tenant is as bad as handing over the timeline it was derived from.
	env, err := h.q.GetEnvironmentByID(ctx, pgEnvID)
	if err != nil || env.OrgID != pgOrgID {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "environment not found"})
	}

	at := time.Now()
	if v := c.QueryParam("at"); v != "" {
		at, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "at must be an RFC3339 timestamp"})
		}
	}

	versions, err := h.q.ServiceVersionsAt(ctx, pgOrgID, pgEnvID, at)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load inventory"})
	}

	out := make([]serviceVersionResponse, 0, len(versions))
	for _, v := range versions {
		out = append(out, serviceVersionResponse{
			Service:    v.Service,
			Version:    v.Version,
			EventID:    uuid.UUID(v.EventID.Bytes).String(),
			EventType:  v.EventType,
			DeployedAt: rfc3339(v.DeployedAt),
		})
	}

	return c.JSON(http.StatusOK, inventoryResponse{
		EnvironmentID:   envUUID.String(),
		EnvironmentName: env.Name,
		At:              at.UTC().Format(time.RFC3339),
		Services:        out,
	})
}
