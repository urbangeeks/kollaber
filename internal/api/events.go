package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type EventsHandler struct{ q *store.Queries }

func NewEventsHandler(q *store.Queries) *EventsHandler { return &EventsHandler{q} }

type createEventRequest struct {
	Type          string         `json:"type"`
	Service       string         `json:"service"`
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Metadata      map[string]any `json:"metadata"`
}

func (h *EventsHandler) Create(c echo.Context) error {
	var req createEventRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Type == "" || req.Service == "" || req.EnvironmentID == uuid.Nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "type, service, and environment_id are required"})
	}
	switch req.Type {
	case "deploy", "alert", "note":
	default:
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "type must be deploy, alert, or note"})
	}

	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	metaBytes, _ := json.Marshal(req.Metadata)

	event, err := h.q.CreateEvent(context.Background(), store.CreateEventParams{
		Type:          req.Type,
		Service:       req.Service,
		EnvironmentID: pgtype.UUID{Bytes: req.EnvironmentID, Valid: true},
		Metadata:      metaBytes,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create event"})
	}

	return c.JSON(http.StatusCreated, event)
}

func (h *EventsHandler) List(c echo.Context) error {
	envIDStr := c.QueryParam("environment_id")
	limit := parseInt32(c.QueryParam("limit"), 50, 200)
	offset := parseInt32(c.QueryParam("offset"), 0, 0)

	var (
		events []store.Event
		err    error
	)

	if envIDStr != "" {
		envID, parseErr := uuid.Parse(envIDStr)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
		}
		events, err = h.q.ListEventsByEnvironment(context.Background(), store.ListEventsByEnvironmentParams{
			EnvironmentID: pgtype.UUID{Bytes: envID, Valid: true},
			Limit:         limit,
			Offset:        offset,
		})
	} else {
		events, err = h.q.ListEvents(context.Background(), store.ListEventsParams{
			Limit:  limit,
			Offset: offset,
		})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch events"})
	}
	if events == nil {
		events = []store.Event{}
	}

	return c.JSON(http.StatusOK, events)
}

func parseInt32(s string, defaultVal, max int32) int32 {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil || v <= 0 {
		return defaultVal
	}
	if max > 0 && int32(v) > max {
		return max
	}
	return int32(v)
}
