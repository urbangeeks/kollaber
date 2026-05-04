package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

type GitHubAuthHandler struct {
	q      *store.Queries
	cfg    *oauth2.Config
	states sync.Map // state string → expiry time.Time
}

func NewGitHubAuthHandler(q *store.Queries) *GitHubAuthHandler {
	cfg := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		Scopes:       []string{"user:email"},
		Endpoint:     githubOAuth.Endpoint,
		RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
	}
	if cfg.RedirectURL == "" {
		cfg.RedirectURL = "http://localhost:8080/auth/github/callback"
	}
	return &GitHubAuthHandler{q: q, cfg: cfg}
}

func (h *GitHubAuthHandler) Redirect(c echo.Context) error {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not generate state"})
	}
	state := hex.EncodeToString(b)
	h.states.Store(state, time.Now().Add(10*time.Minute))
	return c.Redirect(http.StatusTemporaryRedirect, h.cfg.AuthCodeURL(state))
}

func (h *GitHubAuthHandler) Callback(c echo.Context) error {
	state := c.QueryParam("state")
	code := c.QueryParam("code")
	base := frontendBase()

	expiry, ok := h.states.LoadAndDelete(state)
	if !ok || time.Now().After(expiry.(time.Time)) {
		return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=invalid_state")
	}

	token, err := h.cfg.Exchange(context.Background(), code)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=oauth_failed")
	}

	ghID, email, err := fetchGitHubUser(token.AccessToken)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=github_api_failed")
	}

	ctx := context.Background()
	user, isNew, err := h.findOrCreateUser(ctx, ghID, email)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=user_error")
	}

	org, orgErr := h.q.GetOrgByUserID(ctx, user.ID)
	if orgErr != nil {
		// new user — auto-create an org from their email
		orgName := orgNameFromEmail(email)
		org, err = h.q.CreateOrg(ctx, store.CreateOrgParams{
			Name: orgName,
			Slug: uniqueSlug(slugify(orgName)),
		})
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=org_error")
		}
		_ = h.q.CreateOrgMember(ctx, store.CreateOrgMemberParams{
			OrgID:  org.ID,
			UserID: user.ID,
			Role:   "owner",
		})
	}

	jwt, err := makeToken(uuid.UUID(user.ID.Bytes).String(), uuid.UUID(org.ID.Bytes).String(), user.IsAdmin)
	if err != nil {
		return c.Redirect(http.StatusTemporaryRedirect, base+"/login?error=token_error")
	}

	dest := base + "/auth/callback?token=" + jwt
	if isNew {
		dest += "&new=true"
	}
	return c.Redirect(http.StatusTemporaryRedirect, dest)
}

func (h *GitHubAuthHandler) findOrCreateUser(ctx context.Context, ghID, email string) (store.User, bool, error) {
	// look up by GitHub ID first
	if user, err := h.q.GetUserByGitHubID(ctx, &ghID); err == nil {
		return user, false, nil
	}

	// existing email → link GitHub ID
	if user, err := h.q.GetUserByEmail(ctx, email); err == nil {
		linked, err := h.q.LinkGitHubID(ctx, store.LinkGitHubIDParams{GithubID: &ghID, Email: email})
		if err != nil {
			return user, false, nil
		}
		return linked, false, nil
	}

	// brand new user
	user, err := h.q.CreateGitHubUser(ctx, store.CreateGitHubUserParams{
		Email:    email,
		GithubID: &ghID,
	})
	return user, true, err
}

type githubUserResp struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

type githubEmailResp struct {
	Email   string `json:"email"`
	Primary bool   `json:"primary"`
}

func fetchGitHubUser(accessToken string) (ghID string, email string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var u githubUserResp
	if err := json.Unmarshal(body, &u); err != nil {
		return "", "", err
	}

	ghID = fmt.Sprintf("%d", u.ID)
	email = u.Email

	if email == "" {
		email, err = fetchPrimaryEmail(client, accessToken)
		if err != nil {
			return "", "", err
		}
	}
	return ghID, email, nil
}

func fetchPrimaryEmail(client *http.Client, accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var emails []githubEmailResp
	if err := json.Unmarshal(body, &emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, nil
		}
	}
	if len(emails) > 0 {
		return emails[0].Email, nil
	}
	return "", fmt.Errorf("no email found on GitHub account")
}

func frontendBase() string {
	if u := os.Getenv("FRONTEND_URL"); u != "" {
		return u
	}
	return "http://localhost:3000"
}

func orgNameFromEmail(email string) string {
	for i, ch := range email {
		if ch == '@' {
			return email[:i]
		}
	}
	return email
}

func uniqueSlug(base string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return base + "-" + hex.EncodeToString(b)
}

var _ = pgtype.UUID{} // keep import
