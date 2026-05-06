package api

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{ q *store.Queries }

func NewAuthHandler(q *store.Queries) *AuthHandler { return &AuthHandler{q} }

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	OrgName  string `json:"org_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Email == "" || len(req.Password) < 8 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "email and password (min 8 chars) required"})
	}
	if req.OrgName == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "org_name is required"})
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

	slug := slugify(req.OrgName)
	org, err := h.q.CreateOrg(ctx, store.CreateOrgParams{
		Name: req.OrgName,
		Slug: slug,
	})
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "org name already taken"})
	}

	if err := h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
		OrgID:  org.ID,
		UserID: user.ID,
		Role:   "owner",
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create org membership"})
	}

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String(), uuid.UUID(org.ID.Bytes).String(), user.Email, "owner", false)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}

	return c.JSON(http.StatusCreated, echo.Map{"token": token})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "email and password required"})
	}

	user, err := h.q.GetUserByEmail(context.Background(), req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid credentials"})
	}

	org, err := h.q.GetOrgByUserID(context.Background(), user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load org"})
	}

	member, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  org.ID,
		UserID: user.ID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load membership"})
	}

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String(), uuid.UUID(org.ID.Bytes).String(), user.Email, member.Role, user.IsAdmin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}

	return c.JSON(http.StatusOK, echo.Map{"token": token})
}

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func makeToken(userID, orgID, email, role string, isAdmin bool) (string, error) {
	return makeTokenWithExpiry(userID, orgID, email, role, isAdmin, 24*time.Hour)
}

func makeTokenWithExpiry(userID, orgID, email, role string, isAdmin bool, ttl time.Duration) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "changeme-set-JWT_SECRET-in-env"
	}
	claims := jwt.MapClaims{
		"sub":      userID,
		"org_id":   orgID,
		"email":    email,
		"role":     role,
		"is_admin": isAdmin,
		"exp":      jwt.NewNumericDate(time.Now().Add(ttl)),
		"iat":      jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func (h *AuthHandler) GenerateCLIToken(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	user, err := h.q.GetUserByID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load user"})
	}

	member, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load membership"})
	}

	token, err := makeTokenWithExpiry(userID.String(), orgID.String(), user.Email, member.Role, user.IsAdmin, 90*24*time.Hour)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}
	return c.JSON(http.StatusOK, echo.Map{"token": token})
}

func (h *AuthHandler) ListOrgs(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)
	orgs, err := h.q.ListOrgsByUserID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch orgs"})
	}
	type orgItem struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Slug string `json:"slug"`
		Role string `json:"role"`
	}
	out := make([]orgItem, len(orgs))
	for i, o := range orgs {
		out[i] = orgItem{
			ID:   uuid.UUID(o.ID.Bytes).String(),
			Name: o.Name,
			Slug: o.Slug,
			Role: o.Role,
		}
	}
	return c.JSON(http.StatusOK, out)
}

type createOrgRequest struct {
	Name string `json:"name"`
}

func (h *AuthHandler) CreateOrg(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	var req createOrgRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "name is required"})
	}

	ctx := context.Background()

	org, err := h.q.CreateOrg(ctx, store.CreateOrgParams{
		Name: req.Name,
		Slug: uniqueSlug(slugify(req.Name)),
	})
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "org name already taken"})
	}

	if err := h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
		OrgID:  org.ID,
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
		Role:   "owner",
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create org membership"})
	}

	user, err := h.q.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load user"})
	}

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String(), uuid.UUID(org.ID.Bytes).String(), user.Email, "owner", user.IsAdmin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}
	return c.JSON(http.StatusCreated, echo.Map{"token": token, "org_id": uuid.UUID(org.ID.Bytes).String(), "org_name": org.Name})
}

type renameOrgRequest struct {
	Name string `json:"name"`
}

func (h *AuthHandler) RenameOrg(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	orgID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid org_id"})
	}

	member, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil || (member.Role != "owner" && member.Role != "admin") {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "only admins can rename an org"})
	}

	var req renameOrgRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "name is required"})
	}

	org, err := h.q.UpdateOrg(context.Background(), store.UpdateOrgParams{
		ID:   pgtype.UUID{Bytes: orgID, Valid: true},
		Name: req.Name,
		Slug: slugify(req.Name),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not rename org"})
	}
	return c.JSON(http.StatusOK, echo.Map{"id": uuid.UUID(org.ID.Bytes).String(), "name": org.Name, "slug": org.Slug})
}

type switchOrgRequest struct {
	OrgID string `json:"org_id"`
}

func (h *AuthHandler) SwitchOrg(c echo.Context) error {
	userID := c.Get(middleware.UserIDKey).(uuid.UUID)

	var req switchOrgRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	orgID, err := uuid.Parse(req.OrgID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid org_id"})
	}

	// verify membership
	membership, err := h.q.GetOrgMember(context.Background(), store.GetOrgMemberParams{
		OrgID:  pgtype.UUID{Bytes: orgID, Valid: true},
		UserID: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "not a member of that org"})
	}

	user, err := h.q.GetUserByID(context.Background(), pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load user"})
	}

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String(), orgID.String(), user.Email, membership.Role, user.IsAdmin)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}
	return c.JSON(http.StatusOK, echo.Map{"token": token})
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
