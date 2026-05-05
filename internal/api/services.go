package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type ServicesHandler struct{ q *store.Queries }

func NewServicesHandler(q *store.Queries) *ServicesHandler { return &ServicesHandler{q} }

func (h *ServicesHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	envIDStr := c.QueryParam("environment_id")
	if envIDStr == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id is required"})
	}
	envID, err := uuid.Parse(envIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
	}

	services, err := h.q.ListServicesByEnvironment(context.Background(), store.ListServicesByEnvironmentParams{
		EnvironmentID: pgtype.UUID{Bytes: envID, Valid: true},
		OrgID:         pgtype.UUID{Bytes: orgID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch services"})
	}
	if services == nil {
		services = []string{}
	}
	return c.JSON(http.StatusOK, services)
}
