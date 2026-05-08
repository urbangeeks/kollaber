package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/slack"
	"github.com/urbangeeks/kollaber/internal/store"
)

type SlackSettingsHandler struct{ q *store.Queries }

func NewSlackSettingsHandler(q *store.Queries) *SlackSettingsHandler {
	return &SlackSettingsHandler{q}
}

func (h *SlackSettingsHandler) Get(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	url, err := h.q.GetOrgSlackWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load slack settings"})
	}
	return c.JSON(http.StatusOK, echo.Map{"webhook_url": url})
}

type updateSlackRequest struct {
	WebhookURL string `json:"webhook_url"`
}

func (h *SlackSettingsHandler) Put(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	var req updateSlackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	if err := h.q.SetOrgSlackWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true}, req.WebhookURL); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not save slack settings"})
	}
	return c.JSON(http.StatusOK, echo.Map{"webhook_url": req.WebhookURL})
}

func (h *SlackSettingsHandler) Test(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	url, err := h.q.GetOrgSlackWebhook(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil || url == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "no slack webhook configured"})
	}

	if err := slack.SendEventNotification(url, "deploy", "test-service", "production"); err != nil {
		return c.JSON(http.StatusBadGateway, echo.Map{"error": "slack delivery failed: " + err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"ok": true})
}
