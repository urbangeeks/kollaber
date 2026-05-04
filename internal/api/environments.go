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

type EnvironmentsHandler struct{ q *store.Queries }

func NewEnvironmentsHandler(q *store.Queries) *EnvironmentsHandler {
	return &EnvironmentsHandler{q}
}

type envResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
	CreatedAt   string `json:"created_at"`
}

func toEnvResponse(e store.Environment) envResponse {
	return envResponse{
		ID:          uuid.UUID(e.ID.Bytes).String(),
		Name:        e.Name,
		ClusterName: e.ClusterName,
		CreatedAt:   e.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (h *EnvironmentsHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	envs, err := h.q.ListEnvironments(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch environments"})
	}
	out := make([]envResponse, len(envs))
	for i, e := range envs {
		out[i] = toEnvResponse(e)
	}
	return c.JSON(http.StatusOK, out)
}

type createEnvRequest struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
}

func (h *EnvironmentsHandler) Create(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	var req createEnvRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "name is required"})
	}

	env, err := h.q.CreateEnvironment(context.Background(), store.CreateEnvironmentParams{
		OrgID:       pgtype.UUID{Bytes: orgID, Valid: true},
		Name:        req.Name,
		ClusterName: req.ClusterName,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create environment"})
	}

	return c.JSON(http.StatusCreated, toEnvResponse(env))
}
