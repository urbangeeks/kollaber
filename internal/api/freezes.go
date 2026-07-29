package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

// freezeRelevantTypes are the event types a freeze is about. A freeze is a
// statement about changing things, so an alert firing or a note being written
// during one is not a violation of anything.
var freezeRelevantTypes = map[string]bool{
	"deploy": true, "rollback": true, "scale": true, "teardown": true,
}

type FreezesHandler struct{ q *store.Queries }

func NewFreezesHandler(q *store.Queries) *FreezesHandler { return &FreezesHandler{q} }

// freezeNotice is the "you shipped during a freeze" payload returned alongside
// a created event. The CLI turns it into a non-zero exit; CI decides from there
// whether that is a hard failure.
type freezeNotice struct {
	ID      string `json:"id"`
	Reason  string `json:"reason"`
	EndsAt  string `json:"ends_at"`
	OrgWide bool   `json:"org_wide"`
}

type freezeResponse struct {
	ID            string  `json:"id"`
	EnvironmentID *string `json:"environment_id"`
	Reason        string  `json:"reason"`
	StartsAt      string  `json:"starts_at"`
	EndsAt        string  `json:"ends_at"`
	CreatedAt     string  `json:"created_at"`
	Active        bool    `json:"active"`
}

func toFreezeResponse(f store.FreezeWindow, now time.Time) freezeResponse {
	out := freezeResponse{
		ID:        uuid.UUID(f.ID.Bytes).String(),
		Reason:    f.Reason,
		StartsAt:  rfc3339(f.StartsAt),
		EndsAt:    rfc3339(f.EndsAt),
		CreatedAt: rfc3339(f.CreatedAt),
		Active:    !f.StartsAt.Time.After(now) && f.EndsAt.Time.After(now),
	}
	if f.EnvironmentID != nil && f.EnvironmentID.Valid {
		s := uuid.UUID(f.EnvironmentID.Bytes).String()
		out.EnvironmentID = &s
	}
	return out
}

func toFreezeNotice(f store.FreezeWindow) *freezeNotice {
	return &freezeNotice{
		ID:      uuid.UUID(f.ID.Bytes).String(),
		Reason:  f.Reason,
		EndsAt:  rfc3339(f.EndsAt),
		OrgWide: f.EnvironmentID == nil || !f.EnvironmentID.Valid,
	}
}

type createFreezeRequest struct {
	// EnvironmentID is optional. Omitted means every environment in the org,
	// which is what a company-wide Black Friday freeze actually is.
	EnvironmentID *uuid.UUID `json:"environment_id"`
	Reason        string     `json:"reason"`
	StartsAt      string     `json:"starts_at"`
	EndsAt        string     `json:"ends_at"`
}

// Create handles POST /freezes. Admins only: a freeze is a statement the org
// makes about itself, and anyone able to declare one can make every deploy in
// the company report a violation.
func (h *FreezesHandler) Create(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := c.Request().Context()

	var req createFreezeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "reason is required"})
	}

	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "starts_at must be an RFC3339 timestamp"})
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "ends_at must be an RFC3339 timestamp"})
	}
	if !endsAt.After(startsAt) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "ends_at must be after starts_at"})
	}

	var envID *pgtype.UUID
	if req.EnvironmentID != nil {
		pg := pgtype.UUID{Bytes: *req.EnvironmentID, Valid: true}
		env, err := h.q.GetEnvironmentByID(ctx, pg)
		if err != nil || env.OrgID != pgOrgID {
			return c.JSON(http.StatusNotFound, echo.Map{"error": "environment not found"})
		}
		envID = &pg
	}

	created, err := h.q.CreateFreezeWindow(ctx, store.CreateFreezeWindowParams{
		OrgID:         pgOrgID,
		EnvironmentID: envID,
		Reason:        req.Reason,
		StartsAt:      startsAt,
		EndsAt:        endsAt,
		CreatedBy:     pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create freeze window"})
	}

	return c.JSON(http.StatusCreated, toFreezeResponse(created, time.Now()))
}

// List handles GET /freezes.
func (h *FreezesHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	windows, err := h.q.ListFreezeWindows(c.Request().Context(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load freeze windows"})
	}

	now := time.Now()
	out := make([]freezeResponse, 0, len(windows))
	for _, w := range windows {
		out = append(out, toFreezeResponse(w, now))
	}
	return c.JSON(http.StatusOK, out)
}

// Delete handles DELETE /freezes/:id. Changes that already landed inside the
// window keep their mark: they recorded what was true when they happened.
func (h *FreezesHandler) Delete(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid freeze window id"})
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	found, err := h.q.DeleteFreezeWindow(c.Request().Context(),
		pgtype.UUID{Bytes: id, Valid: true},
		pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not delete freeze window"})
	}
	if !found {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "freeze window not found"})
	}
	return c.NoContent(http.StatusNoContent)
}

// annotateFreeze stamps freeze context onto a change event's metadata when it
// lands inside a declared window, and reports the window so the caller can tell
// whoever shipped it.
//
// The mark is written onto the event rather than derived at read time so it
// survives the window being edited or deleted afterwards. What matters six
// months later is that this deploy went out during a declared freeze, not what
// the freeze calendar happens to say today.
//
// Failures here are swallowed: a freeze is advisory, and a database hiccup in
// the annotation must never cost someone their deploy event.
func annotateFreeze(
	ctx context.Context,
	q *store.Queries,
	orgID, envID pgtype.UUID,
	eventType string,
	at time.Time,
	metadata map[string]any,
) *store.FreezeWindow {
	if !freezeRelevantTypes[eventType] {
		return nil
	}

	window, frozen, err := q.ActiveFreezeWindow(ctx, orgID, envID, at)
	if err != nil || !frozen {
		return nil
	}

	metadata["frozen"] = true
	metadata["freeze_reason"] = window.Reason
	return &window
}
