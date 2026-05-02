package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct{ q *store.Queries }

func NewAuthHandler(q *store.Queries) *AuthHandler { return &AuthHandler{q} }

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not hash password"})
	}

	user, err := h.q.CreateUser(context.Background(), store.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return c.JSON(http.StatusConflict, echo.Map{"error": "email already registered"})
	}

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String())
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

	token, err := makeToken(uuid.UUID(user.ID.Bytes).String())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}

	return c.JSON(http.StatusOK, echo.Map{"token": token})
}

func toPgtypeUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func makeToken(userID string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "changeme-set-JWT_SECRET-in-env"
	}
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
