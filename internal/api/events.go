package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/resend"
	"github.com/urbangeeks/kollaber/internal/store"
)

type EventsHandler struct{ q *store.Queries }

func NewEventsHandler(q *store.Queries) *EventsHandler { return &EventsHandler{q} }

type eventResponse struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Service       string          `json:"service"`
	EnvironmentID string          `json:"environment_id"`
	Timestamp     string          `json:"timestamp"`
	Metadata      json.RawMessage `json:"metadata"`
	Status        string          `json:"status"`
	CreatedAt     string          `json:"created_at"`
}

func toEventResponse(e store.Event) eventResponse {
	meta := json.RawMessage(e.Metadata)
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	return eventResponse{
		ID:            uuid.UUID(e.ID.Bytes).String(),
		Type:          e.Type,
		Service:       e.Service,
		EnvironmentID: uuid.UUID(e.EnvironmentID.Bytes).String(),
		Timestamp:     e.Timestamp.Time.Format("2006-01-02T15:04:05Z07:00"),
		Metadata:      meta,
		Status:        e.Status,
		CreatedAt:     e.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type createEventRequest struct {
	Type          string         `json:"type"`
	Service       string         `json:"service"`
	EnvironmentID uuid.UUID      `json:"environment_id"`
	Metadata      map[string]any `json:"metadata"`
	Status        string         `json:"status"`
}

func (h *EventsHandler) Create(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}

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
	if req.Status == "" {
		req.Status = "success"
	}
	switch req.Status {
	case "success", "failure", "in_progress":
	default:
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "status must be success, failure, or in_progress"})
	}

	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	metaBytes, _ := json.Marshal(req.Metadata)

	ctx := context.Background()
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	event, err := h.q.CreateEvent(ctx, store.CreateEventParams{
		Type:          req.Type,
		Service:       req.Service,
		EnvironmentID: pgtype.UUID{Bytes: req.EnvironmentID, Valid: true},
		Metadata:      metaBytes,
		Status:        req.Status,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create event"})
	}

	go func() {
		env, err := h.q.GetEnvironmentByID(ctx, pgtype.UUID{Bytes: req.EnvironmentID, Valid: true})
		if err != nil {
			return
		}
		recipients, err := h.q.GetOrgMembersToNotify(ctx, store.GetOrgMembersToNotifyParams{
			OrgID:   pgtype.UUID{Bytes: orgID, Valid: true},
			Column2: req.Type,
		})
		if err != nil || len(recipients) == 0 {
			return
		}
		_ = resend.SendEventNotification(recipients, req.Type, req.Service, env.Name)
	}()

	return c.JSON(http.StatusCreated, toEventResponse(event))
}

func (h *EventsHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	limit := parseInt32(c.QueryParam("limit"), 50, 200)
	offset := parseInt32(c.QueryParam("offset"), 0, 0)

	filterType := c.QueryParam("type")
	filterService := c.QueryParam("service")
	filterStatus := c.QueryParam("status")

	params := store.ListEventsFilterParams{
		OrgID:   pgtype.UUID{Bytes: orgID, Valid: true},
		Type:    filterType,
		Service: filterService,
		Status:  filterStatus,
		Limit:   limit,
		Offset:  offset,
	}

	if beforeStr := c.QueryParam("before"); beforeStr != "" {
		t, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "before must be an RFC3339 timestamp"})
		}
		params.Before = &t
	}

	if envIDStr := c.QueryParam("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
		}
		pgEnvID := pgtype.UUID{Bytes: envID, Valid: true}
		params.EnvironmentID = &pgEnvID
	}

	events, err := h.q.ListEventsFiltered(context.Background(), params)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch events"})
	}

	out := make([]eventResponse, len(events))
	for i, e := range events {
		out[i] = toEventResponse(e)
	}
	return c.JSON(http.StatusOK, out)
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
