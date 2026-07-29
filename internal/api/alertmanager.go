package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type AlertmanagerHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewAlertmanagerHandler(q *store.Queries, hub *Hub) *AlertmanagerHandler {
	return &AlertmanagerHandler{q, hub}
}

// alertmanagerPayload is the v4 webhook body Alertmanager POSTs. Fields we do
// not map onto an event (groupKey, receiver, externalURL, ...) are omitted.
type alertmanagerPayload struct {
	Version string             `json:"version"`
	Status  string             `json:"status"`
	Alerts  []alertmanagerItem `json:"alerts"`
}

type alertmanagerItem struct {
	Status       string            `json:"status"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Fingerprint  string            `json:"fingerprint"`
}

// checkWebhookSecret authorizes a delivery against WEBHOOK_SECRET.
//
// Alertmanager cannot compute an HMAC over the body, so unlike /webhooks/events
// this also accepts the secret as a bearer token — that is what a receiver's
// http_config.authorization block can actually send. The HMAC path is still
// honored for non-Alertmanager clients posting the same shape.
//
// Atlantis and Argo CD land here too: both send arbitrary static headers
// (--webhook-http-headers, the notification service's headers block) and
// neither signs the body, so a shared secret is the whole of what they can do.
func checkWebhookSecret(c echo.Context, body []byte) bool {
	secret := os.Getenv("WEBHOOK_SECRET")
	if secret == "" {
		return true
	}
	if sig := c.Request().Header.Get("X-Hub-Signature-256"); sig != "" {
		return verifyHMAC(secret, body, sig)
	}
	if got := c.Request().Header.Get("X-Kollaber-Secret"); got != "" {
		return got == secret
	}
	auth := c.Request().Header.Get("Authorization")
	if rest, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(rest) == secret
	}
	return false
}

// resolveService picks the event's service name from an alert's labels, in
// descending order of specificity. alertname is always present in practice, so
// this only falls through to "unknown" for hand-rolled payloads.
func resolveService(labels map[string]string) string {
	for _, key := range []string{"service", "job", "app", "alertname"} {
		if v := strings.TrimSpace(labels[key]); v != "" {
			return v
		}
	}
	return "unknown"
}

// Ingest maps an Alertmanager webhook delivery onto alert events — one per
// entry in alerts[]. The target environment comes from the ?environment_id=
// query parameter, since a receiver's only configurable per-target field is
// its URL.
func (h *AlertmanagerHandler) Ingest(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "could not read body"})
	}

	if !checkWebhookSecret(c, body) {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid webhook secret"})
	}

	envParam := c.QueryParam("environment_id")
	if envParam == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id query parameter is required"})
	}
	envUUID, err := uuid.Parse(envParam)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
	}
	envID := pgtype.UUID{Bytes: envUUID, Valid: true}

	var payload alertmanagerPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if len(payload.Alerts) == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "payload contains no alerts"})
	}

	ctx := context.Background()

	env, err := h.q.GetEnvironmentByID(ctx, envID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "environment not found"})
	}
	orgID := uuid.UUID(env.OrgID.Bytes)

	// Alertmanager groups related alerts into a single delivery, so notifying
	// per alert would fire a burst of near-identical Slack messages for what a
	// human reads as one incident. Collect the distinct services instead and
	// notify once each, after the batch lands.
	var ingested, skipped int
	notifyServices := map[string]bool{}

	for _, alert := range payload.Alerts {
		status := "failure"
		if alert.Status == "resolved" {
			status = "success"
		}

		// Alertmanager re-delivers firing alerts every repeat_interval. Skip a
		// delivery whose fingerprint already sits at this status; a
		// firing->resolved transition differs, so it still lands.
		if alert.Fingerprint != "" {
			prev, found, err := h.q.LatestAlertStatusByFingerprint(ctx, envID, alert.Fingerprint)
			if err != nil {
				return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not check for duplicate alert"})
			}
			if found && prev == status {
				skipped++
				continue
			}
		}

		metadata := map[string]any{
			"source":      "alertmanager",
			"alertname":   alert.Labels["alertname"],
			"severity":    alert.Labels["severity"],
			"summary":     alert.Annotations["summary"],
			"description": alert.Annotations["description"],
			"fingerprint": alert.Fingerprint,
			"labels":      alert.Labels,
			"starts_at":   alert.StartsAt.UTC().Format(time.RFC3339),
		}
		if alert.GeneratorURL != "" {
			metadata["generator_url"] = alert.GeneratorURL
		}
		if alert.Status == "resolved" && !alert.EndsAt.IsZero() {
			metadata["ends_at"] = alert.EndsAt.UTC().Format(time.RFC3339)
		}
		metaBytes, _ := json.Marshal(metadata)

		// Timestamp the event when the alert actually started (or ended, once
		// resolved) rather than when the webhook happened to arrive — grouping
		// and repeat_interval can delay delivery by minutes, which would
		// scramble the timeline ordering against deploys.
		ts := alert.StartsAt
		if alert.Status == "resolved" && !alert.EndsAt.IsZero() {
			ts = alert.EndsAt
		}
		if ts.IsZero() {
			ts = time.Now()
		}

		event, err := h.q.CreateEventAt(ctx, store.CreateEventAtParams{
			Type:          "alert",
			Service:       resolveService(alert.Labels),
			EnvironmentID: envID,
			Metadata:      metaBytes,
			Status:        status,
			Timestamp:     pgtype.Timestamptz{Time: ts, Valid: true},
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not ingest alert"})
		}
		ingested++

		broadcastEvent(h.hub, orgID.String(), envUUID.String(), toEventResponse(event))
		notifyServices[event.Service] = true
	}

	for service := range notifyServices {
		go notifyEvent(ctx, h.q, orgID, envUUID, "alert", service)
	}

	return c.JSON(http.StatusOK, echo.Map{"ingested": ingested, "skipped": skipped})
}
