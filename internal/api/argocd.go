package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type ArgoCDHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewArgoCDHandler(q *store.Queries, hub *Hub) *ArgoCDHandler {
	return &ArgoCDHandler{q, hub}
}

// argocdPayload is the body Argo CD sends, which — unlike Alertmanager,
// Terraform and Atlantis — is a shape we choose rather than one we are handed.
// Argo CD's notification service builds the body from a Go template the
// operator writes, so the contract is the template in our docs.
//
// Every field except app is optional. A template renders a missing field as the
// empty string rather than omitting it, so this has to read as "absent" and not
// fail, or an app that has never synced would 400 on its first notification.
type argocdPayload struct {
	App            string `json:"app"`
	Type           string `json:"type"`
	Revision       string `json:"revision"`
	SyncStatus     string `json:"sync_status"`
	HealthStatus   string `json:"health_status"`
	OperationPhase string `json:"operation_phase"`
	Project        string `json:"project"`
	Namespace      string `json:"namespace"`
	URL            string `json:"url"`
	Message        string `json:"message"`
}

// argocdEventStatus reads the operation phase first and the health status
// second.
//
// The phase describes what the sync just did; health describes what the app is
// now. They disagree in the case that matters most — a sync that succeeded onto
// an app that is Degraded — and there the sync is the change worth recording,
// with health left in the metadata for whoever reads the event.
func argocdEventStatus(phase, health string) string {
	switch phase {
	case "Succeeded":
		return "success"
	case "Failed", "Error":
		return "failure"
	case "Running", "Terminating":
		return "in_progress"
	}

	switch health {
	case "Healthy":
		return "success"
	case "Degraded", "Missing":
		return "failure"
	case "Progressing":
		return "in_progress"
	}

	// A template that sends neither field still records that a sync happened;
	// treating the absence as a failure would be a louder claim than the data
	// supports.
	return "success"
}

// Ingest maps an Argo CD notification onto an event.
func (h *ArgoCDHandler) Ingest(c echo.Context) error {
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

	var payload argocdPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	service := strings.TrimSpace(payload.App)
	if service == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "app is required"})
	}

	// The type is in the payload because Argo CD notifies on app deletion as
	// well as sync, and only the template author knows which trigger fired.
	eventType := strings.TrimSpace(payload.Type)
	if eventType == "" {
		eventType = "deploy"
	}
	if !store.IsValidEventType(eventType) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "unknown event type " + eventType})
	}

	metadata := map[string]any{
		"source":          "argocd",
		"app":             service,
		"revision":        payload.Revision,
		"sync_status":     payload.SyncStatus,
		"health_status":   payload.HealthStatus,
		"operation_phase": payload.OperationPhase,
	}
	if payload.Project != "" {
		metadata["project"] = payload.Project
	}
	if payload.Namespace != "" {
		metadata["namespace"] = payload.Namespace
	}
	if payload.URL != "" {
		metadata["url"] = payload.URL
	}
	if payload.Message != "" {
		metadata["message"] = payload.Message
	}
	annotateFreeze(ctx, h.q, target.pgOrg(), target.envID, eventType, time.Now(), metadata)
	metaBytes, _ := json.Marshal(metadata)

	event, err := h.q.CreateEvent(ctx, store.CreateEventParams{
		Type:          eventType,
		Service:       service,
		EnvironmentID: target.envID,
		Metadata:      metaBytes,
		Status:        argocdEventStatus(payload.OperationPhase, payload.HealthStatus),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not ingest notification"})
	}

	broadcastEvent(h.hub, target.orgID.String(), target.envUUID.String(), toEventResponse(event))
	go notifyEvent(ctx, h.q, target.orgID, target.envUUID, eventType, service)

	return c.JSON(http.StatusCreated, event)
}
