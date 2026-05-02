package api

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type EnvironmentsHandler struct{ q *store.Queries }

func NewEnvironmentsHandler(q *store.Queries) *EnvironmentsHandler {
	return &EnvironmentsHandler{q}
}

func (h *EnvironmentsHandler) List(c echo.Context) error {
	envs, err := h.q.ListEnvironments(context.Background())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch environments"})
	}
	if envs == nil {
		envs = []store.Environment{}
	}
	return c.JSON(http.StatusOK, envs)
}

type createEnvRequest struct {
	Name        string `json:"name"`
	ClusterName string `json:"cluster_name"`
}

func (h *EnvironmentsHandler) Create(c echo.Context) error {
	var req createEnvRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "name is required"})
	}

	env, err := h.q.CreateEnvironment(context.Background(), store.CreateEnvironmentParams{
		Name:        req.Name,
		ClusterName: req.ClusterName,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create environment"})
	}

	return c.JSON(http.StatusCreated, env)
}
