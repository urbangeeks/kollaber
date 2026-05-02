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

type CommentsHandler struct{ q *store.Queries }

func NewCommentsHandler(q *store.Queries) *CommentsHandler { return &CommentsHandler{q} }

type createCommentRequest struct {
	Body string `json:"body"`
}

func (h *CommentsHandler) Create(c echo.Context) error {
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

	return c.JSON(http.StatusCreated, comment)
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
	if comments == nil {
		comments = []store.Comment{}
	}

	return c.JSON(http.StatusOK, comments)
}
