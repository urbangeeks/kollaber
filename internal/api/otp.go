package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/resend"
	"github.com/urbangeeks/kollaber/internal/store"
)

type OTPHandler struct{ q *store.Queries }

func NewOTPHandler(q *store.Queries) *OTPHandler { return &OTPHandler{q} }

type otpSendRequest struct {
	Email string `json:"email"`
}

type otpVerifyRequest struct {
	Email   string `json:"email"`
	Code    string `json:"code"`
	OrgName string `json:"org_name"`
}

func generateCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1_000_000
	return fmt.Sprintf("%06d", n), nil
}

// Send generates a 6-digit OTP and emails it. Always returns 200 to
// avoid revealing whether an address is registered.
func (h *OTPHandler) Send(c echo.Context) error {
	var req otpSendRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Email == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "email is required"})
	}

	code, err := generateCode()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate code"})
	}

	ctx := context.Background()
	_ = h.q.DeleteOTPCodesByEmail(ctx, req.Email)
	if _, err := h.q.CreateOTPCode(ctx, store.CreateOTPCodeParams{
		Email: req.Email,
		Code:  code,
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not store code"})
	}

	if err := resend.SendOTP(req.Email, code); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not send email"})
	}

	return c.JSON(http.StatusOK, echo.Map{"ok": true})
}

// Verify validates the OTP and returns a JWT.
// - If the user exists, they are logged in.
// - If the user does not exist, org_name is required and a new account is created.
func (h *OTPHandler) Verify(c echo.Context) error {
	var req otpVerifyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Email == "" || req.Code == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "email and code are required"})
	}

	ctx := context.Background()

	otpRow, err := h.q.GetValidOTPCode(ctx, store.GetValidOTPCodeParams{
		Email: req.Email,
		Code:  req.Code,
	})
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid or expired code"})
	}
	if err := h.q.MarkOTPCodeUsed(ctx, otpRow.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not mark code used"})
	}

	// Try to find existing user.
	user, err := h.q.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// New user — org_name required.
		if req.OrgName == "" {
			return c.JSON(http.StatusUnprocessableEntity, echo.Map{"error": "org_name is required for new accounts"})
		}

		user, err = h.q.CreateUser(ctx, store.CreateUserParams{
			Email:        req.Email,
			PasswordHash: "",
		})
		if err != nil {
			return c.JSON(http.StatusConflict, echo.Map{"error": "could not create user"})
		}

		slug := uniqueSlug(slugify(req.OrgName))
		org, err := h.q.CreateOrg(ctx, store.CreateOrgParams{Name: req.OrgName, Slug: slug})
		if err != nil {
			return c.JSON(http.StatusConflict, echo.Map{"error": "org name already taken"})
		}
		if err := h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
			OrgID:  org.ID,
			UserID: user.ID,
			Role:   "owner",
		}); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create membership"})
		}

		token, err := makeToken(uuid.UUID(user.ID.Bytes).String(), uuid.UUID(org.ID.Bytes).String(), user.Email, "owner", false)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
		}
		return c.JSON(http.StatusCreated, echo.Map{"token": token, "new_user": true})
	}

	// Existing user — find their primary org.
	org, err := h.q.GetOrgByUserID(ctx, user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load org"})
	}
	member, err := h.q.GetOrgMember(ctx, store.GetOrgMemberParams{
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
	return c.JSON(http.StatusOK, echo.Map{"token": token, "new_user": false})
}
