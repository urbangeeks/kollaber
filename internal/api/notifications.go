package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type NotificationsHandler struct{ q *store.Queries }

func NewNotificationsHandler(q *store.Queries) *NotificationsHandler {
	return &NotificationsHandler{q}
}

// Preference values. "incident" and "digest" are not event types — they name
// the other two things a person can subscribe to, and share notify_on so there
// is one place to unsubscribe from Kollaber's mail rather than three.
var validEventTypes = map[string]bool{
	"deploy": true, "alert": true, "note": true, "teardown": true,
	"incident": true, "digest": true,
}

func (h *NotificationsHandler) Get(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	prefs, err := h.q.GetNotificationPrefs(context.Background(), store.GetNotificationPrefsParams{
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load preferences"})
	}
	if prefs.NotifyOn == nil {
		prefs.NotifyOn = []string{}
	}
	return c.JSON(http.StatusOK, echo.Map{
		"notify_on":          prefs.NotifyOn,
		"notification_email": prefs.NotificationEmail,
	})
}

type updateNotificationPrefsRequest struct {
	NotifyOn          []string `json:"notify_on"`
	NotificationEmail string   `json:"notification_email"`
}

func (h *NotificationsHandler) Put(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	var req updateNotificationPrefsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.NotifyOn == nil {
		req.NotifyOn = []string{}
	}
	for _, t := range req.NotifyOn {
		if !validEventTypes[t] {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event type: " + t})
		}
	}

	if err := h.q.UpsertNotificationPrefs(context.Background(), store.UpsertNotificationPrefsParams{
		UserID:            pgtype.UUID{Bytes: userID, Valid: true},
		OrgID:             pgtype.UUID{Bytes: orgID, Valid: true},
		NotifyOn:          req.NotifyOn,
		NotificationEmail: req.NotificationEmail,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not save preferences"})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"notify_on":          req.NotifyOn,
		"notification_email": req.NotificationEmail,
	})
}
