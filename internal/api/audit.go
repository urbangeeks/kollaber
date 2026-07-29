package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type AuditHandler struct{ q *store.Queries }

func NewAuditHandler(q *store.Queries) *AuditHandler { return &AuditHandler{q} }

func auditActor(c echo.Context) (pgtype.UUID, string) {
	userID, _ := c.Get(middleware.UserIDKey).(uuid.UUID)
	email, _ := c.Get(middleware.EmailKey).(string)
	return pgtype.UUID{Bytes: [16]byte(userID), Valid: true}, email
}

func (h *AuditHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}
	if !billing.EntitlementsFor(b.Plan).AuditLogs {
		return c.JSON(http.StatusPaymentRequired, echo.Map{
			"error":            "Audit logs require the Pro plan",
			"upgrade_required": true,
		})
	}

	limit := parseInt32(c.QueryParam("limit"), 50, 200)
	offset := parseInt32(c.QueryParam("offset"), 0, 0)

	logs, err := h.q.ListAuditLogs(ctx, pgOrgID, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch audit logs"})
	}

	type logItem struct {
		ID         string          `json:"id"`
		ActorEmail string          `json:"actor_email"`
		Action     string          `json:"action"`
		TargetType string          `json:"target_type"`
		TargetID   string          `json:"target_id"`
		Metadata   json.RawMessage `json:"metadata"`
		CreatedAt  string          `json:"created_at"`
	}

	out := make([]logItem, len(logs))
	for i, l := range logs {
		meta := json.RawMessage(l.Metadata)
		if len(meta) == 0 {
			meta = json.RawMessage(`{}`)
		}
		out[i] = logItem{
			ID:         uuid.UUID(l.ID.Bytes).String(),
			ActorEmail: l.ActorEmail,
			Action:     l.Action,
			TargetType: l.TargetType,
			TargetID:   l.TargetID,
			Metadata:   meta,
			CreatedAt:  l.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(http.StatusOK, out)
}
