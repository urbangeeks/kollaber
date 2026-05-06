package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type InviteHandler struct{ q *store.Queries }

func NewInviteHandler(q *store.Queries) *InviteHandler { return &InviteHandler{q} }

type createInviteRequest struct {
	Role string `json:"role"`
}

func (h *InviteHandler) Create(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	if err := requireAdmin(c); err != nil {
		return err
	}

	var req createInviteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	role := req.Role
	if role == "" {
		role = "member"
	}
	if role != "admin" && role != "member" && role != "viewer" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "role must be admin, member, or viewer"})
	}

	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate token"})
	}
	token := hex.EncodeToString(b)

	_, err := h.q.CreateInvite(context.Background(), store.CreateInviteParams{
		OrgID:     pgtype.UUID{Bytes: orgID, Valid: true},
		CreatedBy: pgtype.UUID{Bytes: userID, Valid: true},
		Token:     token,
		Role:      role,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create invite"})
	}

	return c.JSON(http.StatusCreated, echo.Map{"token": token})
}

func (h *InviteHandler) Get(c echo.Context) error {
	token := c.Param("token")
	invite, err := h.q.GetInviteByToken(context.Background(), token)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "invite not found"})
	}
	if invite.AcceptedAt.Valid {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite already used"})
	}
	if time.Now().After(invite.ExpiresAt.Time) {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite expired"})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"org_name":   invite.OrgName,
		"role":       invite.Role,
		"expires_at": invite.ExpiresAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	})
}

type acceptInviteRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *InviteHandler) Accept(c echo.Context) error {
	token := c.Param("token")

	invite, err := h.q.GetInviteByToken(context.Background(), token)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "invite not found"})
	}
	if invite.AcceptedAt.Valid {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite already used"})
	}
	if time.Now().After(invite.ExpiresAt.Time) {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite expired"})
	}

	var req acceptInviteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Email == "" || len(req.Password) < 8 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "email and password (min 8 chars) required"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not hash password"})
	}

	ctx := context.Background()

	user, err := h.q.CreateUser(ctx, store.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "email already registered"})
	}

	if err := h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
		OrgID:  invite.OrgID,
		UserID: user.ID,
		Role:   invite.Role,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not join org"})
	}

	if err := h.q.AcceptInvite(ctx, token); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not mark invite used"})
	}

	jwtToken, err := makeToken(
		uuid.UUID(user.ID.Bytes).String(),
		uuid.UUID(invite.OrgID.Bytes).String(),
		user.Email,
		invite.Role,
		false,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}

	return c.JSON(http.StatusCreated, echo.Map{"token": jwtToken})
}

// Join is for authenticated users accepting an invite (no password needed).
func (h *InviteHandler) Join(c echo.Context) error {
	token := c.Param("token")
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	ctx := context.Background()
	invite, err := h.q.GetInviteByToken(ctx, token)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "invite not found"})
	}
	if invite.AcceptedAt.Valid {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite already used"})
	}
	if time.Now().After(invite.ExpiresAt.Time) {
		return c.JSON(http.StatusGone, echo.Map{"error": "invite expired"})
	}

	// already a member?
	existingMember, err := h.q.GetOrgMember(ctx, store.GetOrgMemberParams{
		OrgID:  invite.OrgID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err == nil {
		// already in org — just switch to it
		existing, err := h.q.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load user"})
		}
		jwtToken, err := makeToken(
			userID.String(),
			uuid.UUID(invite.OrgID.Bytes).String(),
			existing.Email,
			existingMember.Role,
			existing.IsAdmin,
		)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
		}
		return c.JSON(http.StatusOK, echo.Map{"token": jwtToken})
	}

	if err := h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
		OrgID:  invite.OrgID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		Role:   invite.Role,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not join org"})
	}

	if err := h.q.AcceptInvite(ctx, token); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not mark invite used"})
	}

	user, err := h.q.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load user"})
	}

	jwtToken, err := makeToken(
		userID.String(),
		uuid.UUID(invite.OrgID.Bytes).String(),
		user.Email,
		invite.Role,
		user.IsAdmin,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}
	return c.JSON(http.StatusOK, echo.Map{"token": jwtToken})
}
