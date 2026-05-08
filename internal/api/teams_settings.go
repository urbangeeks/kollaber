package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"github.com/urbangeeks/kollaber/internal/teams"
)

type TeamsSettingsHandler struct{ q *store.Queries }

func NewTeamsSettingsHandler(q *store.Queries) *TeamsSettingsHandler {
	return &TeamsSettingsHandler{q}
}

func (h *TeamsSettingsHandler) Get(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	url, err := h.q.GetOrgTeamsWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load teams settings"})
	}
	return c.JSON(http.StatusOK, echo.Map{"webhook_url": url})
}

type updateTeamsRequest struct {
	WebhookURL string `json:"webhook_url"`
}

func (h *TeamsSettingsHandler) Put(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	var req updateTeamsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.q.SetOrgTeamsWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true}, req.WebhookURL); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not save teams settings"})
	}
	return c.JSON(http.StatusOK, echo.Map{"webhook_url": req.WebhookURL})
}

func (h *TeamsSettingsHandler) Test(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	url, err := h.q.GetOrgTeamsWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil || url == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "no teams webhook configured"})
	}

	if err := teams.SendEventNotification(url, "deploy", "test-service", "production"); err != nil {
		return c.JSON(http.StatusBadGateway, echo.Map{"error": "teams delivery failed: " + err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"ok": true})
}
