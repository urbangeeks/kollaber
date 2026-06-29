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

type CommentsHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewCommentsHandler(q *store.Queries, hub *Hub) *CommentsHandler {
	return &CommentsHandler{q: q, hub: hub}
}

type commentResponse struct {
	ID        string `json:"id"`
	EventID   string `json:"event_id"`
	UserID    string `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func toCommentResponse(c store.Comment) commentResponse {
	return commentResponse{
		ID:        uuid.UUID(c.ID.Bytes).String(),
		EventID:   uuid.UUID(c.EventID.Bytes).String(),
		UserID:    uuid.UUID(c.UserID.Bytes).String(),
		Body:      c.Body,
		CreatedAt: c.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

type createCommentRequest struct {
	Body string `json:"body"`
}

func (h *CommentsHandler) Create(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}

	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}

	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	var req createCommentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Body == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "body is required"})
	}

	comment, err := h.q.CreateComment(context.Background(), store.CreateCommentParams{
		EventID: pgtype.UUID{Bytes: eventID, Valid: true},
		UserID:  pgtype.UUID{Bytes: userID, Valid: true},
		Body:    req.Body,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create comment"})
	}

	resp := toCommentResponse(comment)

	// Push the new comment to anyone watching this event's environment so open
	// timeline threads update live. Best-effort: a failed env lookup just skips
	// the broadcast, it doesn't fail the request.
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	if event, err := h.q.GetEventByIDForOrg(c.Request().Context(),
		pgtype.UUID{Bytes: eventID, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true}); err == nil {
		broadcastComment(h.hub, orgID.String(), uuid.UUID(event.EnvironmentID.Bytes).String(), eventID.String(), resp)
	}

	return c.JSON(http.StatusCreated, resp)
}

func (h *CommentsHandler) List(c echo.Context) error {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}

	comments, err := h.q.ListCommentsByEvent(context.Background(), pgtype.UUID{Bytes: eventID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch comments"})
	}
	out := make([]commentResponse, len(comments))
	for i, cm := range comments {
		out[i] = toCommentResponse(cm)
	}
	return c.JSON(http.StatusOK, out)
}
