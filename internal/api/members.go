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

type MembersHandler struct{ q *store.Queries }

func NewMembersHandler(q *store.Queries) *MembersHandler { return &MembersHandler{q} }

func (h *MembersHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	members, err := h.q.ListOrgMembers(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch members"})
	}

	type memberItem struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		Role      string `json:"role"`
		JoinedAt  string `json:"joined_at"`
	}

	out := make([]memberItem, len(members))
	for i, m := range members {
		out[i] = memberItem{
			ID:       uuid.UUID(m.ID.Bytes).String(),
			Email:    m.Email,
			Role:     m.Role,
			JoinedAt: m.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(http.StatusOK, out)
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

func (h *MembersHandler) UpdateRole(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	callerID := c.Get(middleware.UserIDKey).(uuid.UUID)

	if err := requireAdmin(c); err != nil {
		return err
	}

	targetID, err := uuid.Parse(c.Param("userID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid user id"})
	}

	// cannot change the owner's role
	target, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: targetID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "member not found"})
	}
	if target.Role == "owner" {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "cannot change the owner's role"})
	}

	// admins cannot promote to owner or above their own role
	callerRole, _ := c.Get(middleware.RoleKey).(string)
	if callerRole == "admin" && target.Role == "admin" && callerID == targetID {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "cannot change your own role"})
	}

	var req updateRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Role != "admin" && req.Role != "member" && req.Role != "viewer" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "role must be admin, member, or viewer"})
	}

	if err := h.q.UpdateOrgMemberRole(context.Background(), store.UpdateOrgMemberRoleParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: targetID, Valid: true},
		Role:   req.Role,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update role"})
	}

	return c.JSON(http.StatusOK, echo.Map{"role": req.Role})
}

func (h *MembersHandler) Remove(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	if err := requireAdmin(c); err != nil {
		return err
	}

	targetID, err := uuid.Parse(c.Param("userID"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid user id"})
	}

	// cannot remove the owner
	target, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: targetID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "member not found"})
	}
	if target.Role == "owner" {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "cannot remove the org owner"})
	}

	if err := h.q.DeleteOrgMember(context.Background(), store.DeleteOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: targetID, Valid: true},
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not remove member"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *MembersHandler) ListInvites(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	if err := requireAdmin(c); err != nil {
		return err
	}

	invites, err := h.q.ListPendingInvites(context.Background(), pgtype.UUID{Bytes: orgID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch invites"})
	}

	type inviteItem struct {
		Token          string `json:"token"`
		Role           string `json:"role"`
		CreatedByEmail string `json:"created_by_email"`
		CreatedAt      string `json:"created_at"`
		ExpiresAt      string `json:"expires_at"`
	}

	out := make([]inviteItem, len(invites))
	for i, inv := range invites {
		out[i] = inviteItem{
			Token:          inv.Token,
			Role:           inv.Role,
			CreatedByEmail: inv.CreatedByEmail,
			CreatedAt:      inv.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt:      inv.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	return c.JSON(http.StatusOK, out)
}

func (h *MembersHandler) RevokeInvite(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	if err := requireAdmin(c); err != nil {
		return err
	}

	token := c.Param("token")
	if err := h.q.RevokeInvite(context.Background(), store.RevokeInviteParams{
		Token: token,
		OrgID: pgtype.UUID{Bytes: orgID, Valid: true},
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not revoke invite"})
	}

	return c.NoContent(http.StatusNoContent)
}
