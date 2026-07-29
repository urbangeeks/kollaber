package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
	"golang.org/x/oauth2"
)

type SSOHandler struct{ q *store.Queries }

func NewSSOHandler(q *store.Queries) *SSOHandler { return &SSOHandler{q} }

// --- Settings (admin, Pro-gated) ---

type ssoConfigRequest struct {
	IssuerURL    string `json:"issuer_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Domain       string `json:"domain"`
	Enabled      bool   `json:"enabled"`
}

func (h *SSOHandler) GetConfig(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	if err := requireAdmin(c); err != nil {
		return err
	}
	if err := h.checkSSOEntitlement(ctx, pgOrgID); err != nil {
		return err
	}

	cfg, err := h.q.GetSSOConfig(ctx, pgOrgID)
	if err != nil {
		// no config yet — return empty
		return c.JSON(http.StatusOK, echo.Map{
			"issuer_url":    "",
			"client_id":     "",
			"client_secret": "",
			"domain":        "",
			"enabled":       false,
			"callback_url":  callbackURL(),
		})
	}
	return c.JSON(http.StatusOK, echo.Map{
		"issuer_url":    cfg.IssuerURL,
		"client_id":     cfg.ClientID,
		"client_secret": "••••••••",
		"domain":        cfg.Domain,
		"enabled":       cfg.Enabled,
		"callback_url":  callbackURL(),
	})
}

func (h *SSOHandler) SaveConfig(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	if err := requireAdmin(c); err != nil {
		return err
	}
	if err := h.checkSSOEntitlement(ctx, pgOrgID); err != nil {
		return err
	}

	var req ssoConfigRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
	if req.IssuerURL == "" || req.ClientID == "" || req.Domain == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "issuer_url, client_id, and domain are required"})
	}

	// if secret is masked, keep the existing one
	params := store.UpsertSSOConfigParams{
		OrgID:     pgOrgID,
		IssuerURL: req.IssuerURL,
		ClientID:  req.ClientID,
		Domain:    req.Domain,
		Enabled:   req.Enabled,
	}
	if req.ClientSecret != "" && req.ClientSecret != "••••••••" {
		params.ClientSecret = req.ClientSecret
	} else {
		existing, err := h.q.GetSSOConfig(ctx, pgOrgID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "client_secret is required"})
		}
		params.ClientSecret = existing.ClientSecret
	}

	if err := h.q.UpsertSSOConfig(ctx, params); err != nil {
		if strings.Contains(err.Error(), "unique") {
			return c.JSON(http.StatusConflict, echo.Map{"error": "that domain is already used by another organisation"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not save SSO config"})
	}
	return c.JSON(http.StatusOK, echo.Map{"ok": true})
}

// --- Auth flow ---

// Initiate redirects the browser to the IdP. Called with ?org=<slug>.
func (h *SSOHandler) Initiate(c echo.Context) error {
	slug := c.QueryParam("org")
	if slug == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "org is required"})
	}

	cfg, err := h.q.GetSSOConfigByOrgSlug(context.Background(), slug)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "SSO not configured for this organisation"})
	}

	oidcCfg, err := buildOAuthConfig(c.Request().Context(), cfg)
	if err != nil {
		return c.JSON(http.StatusBadGateway, echo.Map{"error": fmt.Sprintf("could not reach identity provider: %s", err)})
	}

	state, err := signedState(uuid.UUID(cfg.OrgID.Bytes).String())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate state"})
	}

	url := oidcCfg.oauth2.AuthCodeURL(state, oauth2.AccessTypeOnline)
	return c.Redirect(http.StatusFound, url)
}

// Callback handles the IdP redirect, issues a Kollaber JWT.
func (h *SSOHandler) Callback(c echo.Context) error {
	ctx := c.Request().Context()

	orgIDStr, err := verifyState(c.QueryParam("state"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid state"})
	}
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid org in state"})
	}
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	cfg, err := h.q.GetSSOConfig(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "SSO not configured"})
	}

	oidcCfg, err := buildOAuthConfig(ctx, cfg)
	if err != nil {
		return c.JSON(http.StatusBadGateway, echo.Map{"error": "could not reach identity provider"})
	}

	token, err := oidcCfg.oauth2.Exchange(ctx, c.QueryParam("code"))
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "could not exchange code"})
	}

	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "no id_token in response"})
	}
	idToken, err := oidcCfg.verifier.Verify(ctx, rawID)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "could not verify id token"})
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil || claims.Email == "" {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "no email in id token"})
	}

	// enforce domain
	parts := strings.SplitN(claims.Email, "@", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[1], cfg.Domain) {
		return c.JSON(http.StatusForbidden, echo.Map{"error": "email domain does not match SSO configuration"})
	}

	dbCtx := context.Background()
	user, err := h.q.GetOrCreateUserByEmail(dbCtx, claims.Email)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not provision user"})
	}

	member, err := h.q.GetOrgMember(dbCtx, store.GetOrgMemberParams{
		OrgID:  pgOrgID,
		UserID: user.ID,
	})
	if err != nil {
		// auto-provision as member on first SSO login
		if err2 := h.q.CreateOrgMember(dbCtx, store.CreateOrgMemberParams{
			OrgID:  pgOrgID,
			UserID: user.ID,
			Role:   "member",
		}); err2 != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not join org"})
		}
		member.Role = "member"
	}

	jwtToken, err := makeToken(
		uuid.UUID(user.ID.Bytes).String(),
		orgID.String(),
		user.Email,
		member.Role,
		user.IsAdmin,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create token"})
	}

	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	return c.Redirect(http.StatusFound, fmt.Sprintf("%s/auth/callback?token=%s", frontendURL, jwtToken))
}

// --- helpers ---

type oidcBundle struct {
	oauth2   *oauth2.Config
	verifier *gooidc.IDTokenVerifier
}

func buildOAuthConfig(ctx context.Context, cfg store.SSOConfig) (*oidcBundle, error) {
	provider, err := gooidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, err
	}
	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  callbackURL(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}
	verifier := provider.Verifier(&gooidc.Config{ClientID: cfg.ClientID})
	return &oidcBundle{oauth2: oauth2Cfg, verifier: verifier}, nil
}

func callbackURL() string {
	if v := os.Getenv("API_URL"); v != "" {
		return strings.TrimRight(v, "/") + "/auth/sso/callback"
	}
	return "http://localhost:8080/auth/sso/callback"
}

type statePayload struct {
	OrgID string `json:"org_id"`
	Nonce string `json:"nonce"`
	Exp   int64  `json:"exp"`
}

func signedState(orgID string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	p := statePayload{
		OrgID: orgID,
		Nonce: base64.RawURLEncoding.EncodeToString(b),
		Exp:   time.Now().Add(10 * time.Minute).Unix(),
	}
	raw, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func verifyState(state string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", err
	}
	var p statePayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return "", err
	}
	if time.Now().Unix() > p.Exp {
		return "", fmt.Errorf("state expired")
	}
	return p.OrgID, nil
}

func (h *SSOHandler) checkSSOEntitlement(ctx context.Context, orgID pgtype.UUID) error {
	b, err := h.q.GetOrgBilling(ctx, orgID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load billing")
	}
	if !billing.EntitlementsFor(b.Plan).SSO {
		return echo.NewHTTPError(http.StatusPaymentRequired, echo.Map{
			"error":            "SSO requires the Pro plan",
			"upgrade_required": true,
		})
	}
	return nil
}
