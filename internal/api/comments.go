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
	ID         string  `json:"id"`
	EventID    string  `json:"event_id"`
	UserID     string  `json:"user_id"`
	Body       string  `json:"body"`
	CreatedAt  string  `json:"created_at"`
	IsDecision bool    `json:"is_decision"`
	DecidedBy  *string `json:"decided_by"`
	DecidedAt  *string `json:"decided_at"`
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

func toCommentDetailResponse(c store.CommentDetail) commentResponse {
	out := commentResponse{
		ID:         uuid.UUID(c.ID.Bytes).String(),
		EventID:    uuid.UUID(c.EventID.Bytes).String(),
		UserID:     uuid.UUID(c.UserID.Bytes).String(),
		Body:       c.Body,
		CreatedAt:  rfc3339(c.CreatedAt),
		IsDecision: c.IsDecision,
	}
	if c.DecidedBy != nil && c.DecidedBy.Valid {
		s := uuid.UUID(c.DecidedBy.Bytes).String()
		out.DecidedBy = &s
	}
	if c.DecidedAt.Valid {
		s := rfc3339(c.DecidedAt)
		out.DecidedAt = &s
	}
	return out
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
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	var req createCommentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Body == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "body is required"})
	}

	// Confirm the event belongs to the caller's org before writing, not after.
	// Events carry no org_id, so without this any authenticated user could post
	// into another tenant's thread by guessing an id.
	event, err := h.q.GetEventByIDForOrg(context.Background(),
		pgtype.UUID{Bytes: eventID, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "event not found"})
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
	// timeline threads update live.
	broadcastComment(h.hub, orgID.String(), uuid.UUID(event.EnvironmentID.Bytes).String(), eventID.String(), resp)

	return c.JSON(http.StatusCreated, resp)
}

func (h *CommentsHandler) List(c echo.Context) error {
	eventID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid event id"})
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	comments, err := h.q.ListCommentsForEvent(c.Request().Context(),
		pgtype.UUID{Bytes: eventID, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch comments"})
	}

	out := make([]commentResponse, 0, len(comments))
	for _, cm := range comments {
		out = append(out, toCommentDetailResponse(cm))
	}
	return c.JSON(http.StatusOK, out)
}

type setDecisionRequest struct {
	IsDecision bool `json:"is_decision"`
}

// SetDecision handles PATCH /comments/:id — promoting a comment to a decision,
// or demoting it again.
//
// A separate verb from editing the body on purpose: marking is curation, and
// the text of what was decided stays exactly as it was written.
func (h *CommentsHandler) SetDecision(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}

	commentID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid comment id"})
	}

	var req setDecisionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	updated, found, err := h.q.SetCommentDecision(c.Request().Context(),
		pgtype.UUID{Bytes: commentID, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true},
		pgtype.UUID{Bytes: userID, Valid: true},
		req.IsDecision)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update comment"})
	}
	if !found {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "comment not found"})
	}

	return c.JSON(http.StatusOK, toCommentDetailResponse(updated))
}
