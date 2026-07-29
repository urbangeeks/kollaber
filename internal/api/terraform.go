package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type TerraformHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewTerraformHandler(q *store.Queries, hub *Hub) *TerraformHandler {
	return &TerraformHandler{q, hub}
}

// terraformPayload is the generic run notification HCP Terraform and Terraform
// Enterprise POST. Fields we do not map onto an event (payload_version,
// notification_configuration_id, workspace_id) are omitted.
type terraformPayload struct {
	RunID            string                  `json:"run_id"`
	RunURL           string                  `json:"run_url"`
	RunMessage       string                  `json:"run_message"`
	RunCreatedAt     time.Time               `json:"run_created_at"`
	RunCreatedBy     string                  `json:"run_created_by"`
	WorkspaceName    string                  `json:"workspace_name"`
	OrganizationName string                  `json:"organization_name"`
	Notifications    []terraformNotification `json:"notifications"`
}

type terraformNotification struct {
	Message      string    `json:"message"`
	Trigger      string    `json:"trigger"`
	RunStatus    string    `json:"run_status"`
	RunUpdatedAt time.Time `json:"run_updated_at"`
	RunUpdatedBy string    `json:"run_updated_by"`
}

// terraformEventStatus maps a run status onto an event status, and reports
// whether the run status is worth recording at all.
//
// Only terminal outcomes become events. A plan is not a change: recording
// "planning" or "planned" would put a marker on the timeline for a run that
// touched nothing, inflate DORA deployment counts, and offer suspect detection
// a change that never happened. "canceled" and "discarded" are skipped for the
// same reason — the run ended without applying.
//
// This means one event per run rather than one per notification, however many
// triggers the workspace has enabled. HCP Terraform also sends a payload with
// no run status at all when a notification config is first saved; that falls
// through here and is skipped rather than landing as a mystery deploy.
func terraformEventStatus(runStatus string) (status string, ok bool) {
	switch runStatus {
	case "applied":
		return "success", true
	case "errored":
		return "failure", true
	default:
		return "", false
	}
}

// checkTerraformSignature authorizes a delivery against WEBHOOK_SECRET.
//
// HCP Terraform signs the body with HMAC-SHA512 and sends the hex digest bare
// in X-TFE-Notification-Signature — no algorithm prefix, and a different hash
// from the SHA-256 the other webhooks use, so it cannot share verifyHMAC.
//
// A delivery without that header falls back to the shared-secret paths, which
// is what Terraform Enterprise installs that leave the token unset will send.
func checkTerraformSignature(c echo.Context, body []byte) bool {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return true
	}

	sig := c.Request().Header.Get("X-TFE-Notification-Signature")
	if sig == "" {
		return checkWebhookSecret(c, body)
	}

	want, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), want)
}

// Ingest maps an HCP Terraform run notification onto a deploy event.
//
// The workspace is the service: a Terraform workspace is the unit that owns a
// piece of infrastructure, which is the closest thing Terraform has to the
// thing a deploy changes.
func (h *TerraformHandler) Ingest(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "could not read body"})
	}

	if !checkTerraformSignature(c, body) {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid webhook signature"})
	}

	ctx := context.Background()
	target, ok := resolveWebhookTarget(ctx, h.q, c)
	if !ok {
		return nil
	}

	var payload terraformPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	service := payload.WorkspaceName
	if service == "" {
		service = "terraform"
	}

	var ingested, skipped int
	for _, n := range payload.Notifications {
		status, keep := terraformEventStatus(n.RunStatus)
		if !keep {
			skipped++
			continue
		}

		metadata := map[string]any{
			"source":       "terraform",
			"run_id":       payload.RunID,
			"run_url":      payload.RunURL,
			"run_message":  payload.RunMessage,
			"run_status":   n.RunStatus,
			"trigger":      n.Trigger,
			"workspace":    payload.WorkspaceName,
			"organization": payload.OrganizationName,
		}
		// The person who pushed the button is the one to ask about the change,
		// and on a queued run that is not who created it.
		if n.RunUpdatedBy != "" {
			metadata["author"] = n.RunUpdatedBy
		} else if payload.RunCreatedBy != "" {
			metadata["author"] = payload.RunCreatedBy
		}
		metaBytes, _ := json.Marshal(metadata)

		// Timestamp the event when the run reached this status, not when the
		// webhook arrived. A long apply and a retried delivery both push the
		// two apart, and the timeline's whole value is the ordering.
		ts := n.RunUpdatedAt
		if ts.IsZero() {
			ts = payload.RunCreatedAt
		}
		if ts.IsZero() {
			ts = time.Now()
		}

		event, err := h.q.CreateEventAt(ctx, store.CreateEventAtParams{
			Type:          "deploy",
			Service:       service,
			EnvironmentID: target.envID,
			Metadata:      metaBytes,
			Status:        status,
			Timestamp:     pgtype.Timestamptz{Time: ts, Valid: true},
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not ingest run"})
		}
		ingested++

		broadcastEvent(h.hub, target.orgID.String(), target.envUUID.String(), toEventResponse(event))
	}

	if ingested > 0 {
		go notifyEvent(ctx, h.q, target.orgID, target.envUUID, "deploy", service)
	}

	return c.JSON(http.StatusOK, echo.Map{"ingested": ingested, "skipped": skipped})
}
