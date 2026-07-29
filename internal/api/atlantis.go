package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type AtlantisHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewAtlantisHandler(q *store.Queries, hub *Hub) *AtlantisHandler {
	return &AtlantisHandler{q, hub}
}

// atlantisPayload is the ApplyResult struct Atlantis marshals for an
// `event: apply, kind: http` webhook.
//
// The field names are capitalised because Atlantis marshals the struct without
// json tags, so Go's exported field names are the wire format. Tags are written
// out here anyway rather than relying on encoding/json's case-insensitive
// match, so the contract is visible in the file that depends on it.
type atlantisPayload struct {
	Workspace   string       `json:"Workspace"`
	Repo        atlantisRepo `json:"Repo"`
	Pull        atlantisPull `json:"Pull"`
	User        atlantisUser `json:"User"`
	Success     bool         `json:"Success"`
	Directory   string       `json:"Directory"`
	ProjectName string       `json:"ProjectName"`
}

type atlantisRepo struct {
	FullName string `json:"FullName"`
	Owner    string `json:"Owner"`
	Name     string `json:"Name"`
}

type atlantisPull struct {
	Num        int    `json:"Num"`
	HeadCommit string `json:"HeadCommit"`
	URL        string `json:"URL"`
	HeadBranch string `json:"HeadBranch"`
	BaseBranch string `json:"BaseBranch"`
	Author     string `json:"Author"`
}

type atlantisUser struct {
	Username string `json:"Username"`
}

// atlantisService names the thing that changed, most specific first.
//
// ProjectName is set only when the repo declares projects in atlantis.yaml, and
// Directory is empty for a root-level module, so both have to be able to fall
// through — to the repo name, which is at least always populated.
func atlantisService(p atlantisPayload) string {
	for _, candidate := range []string{p.ProjectName, p.Directory, p.Repo.Name, p.Repo.FullName} {
		if v := strings.TrimSpace(candidate); v != "" && v != "." {
			return v
		}
	}
	return "terraform"
}

// Ingest maps an Atlantis apply webhook onto a deploy event.
//
// Atlantis posts only after an apply has run, so every delivery is a change
// that reached the infrastructure — there is no plan-stage noise to filter out
// the way there is with Terraform Cloud.
func (h *AtlantisHandler) Ingest(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "could not read body"})
	}

	if !checkWebhookSecret(c, body) {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid webhook secret"})
	}

	ctx := context.Background()
	target, ok := resolveWebhookTarget(ctx, h.q, c)
	if !ok {
		return nil
	}

	var payload atlantisPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	status := "failure"
	if payload.Success {
		status = "success"
	}

	metadata := map[string]any{
		"source":       "atlantis",
		"workspace":    payload.Workspace,
		"directory":    payload.Directory,
		"project_name": payload.ProjectName,
		"repo":         payload.Repo.FullName,
	}
	// The person who commented `atlantis apply` is the one who made the change
	// happen, which is not always whoever opened the pull request.
	if payload.User.Username != "" {
		metadata["author"] = payload.User.Username
	} else if payload.Pull.Author != "" {
		metadata["author"] = payload.Pull.Author
	}
	if payload.Pull.Num != 0 {
		metadata["pull_num"] = payload.Pull.Num
		metadata["pull_url"] = payload.Pull.URL
		metadata["head_commit"] = payload.Pull.HeadCommit
		metadata["head_branch"] = payload.Pull.HeadBranch
		metadata["base_branch"] = payload.Pull.BaseBranch
	}
	metaBytes, _ := json.Marshal(metadata)

	// Atlantis sends no timestamp, and it posts as the apply finishes, so the
	// arrival time is the truest reading available.
	service := atlantisService(payload)
	event, err := h.q.CreateEvent(ctx, store.CreateEventParams{
		Type:          "deploy",
		Service:       service,
		EnvironmentID: target.envID,
		Metadata:      metaBytes,
		Status:        status,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not ingest apply"})
	}

	broadcastEvent(h.hub, target.orgID.String(), target.envUUID.String(), toEventResponse(event))
	go notifyEvent(ctx, h.q, target.orgID, target.envUUID, "deploy", service)

	return c.JSON(http.StatusCreated, event)
}
