package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

type WebhookHandler struct {
	q   *store.Queries
	hub *Hub
}

func NewWebhookHandler(q *store.Queries, hub *Hub) *WebhookHandler { return &WebhookHandler{q, hub} }

type genericWebhookPayload struct {
	Type          string         `json:"type"`
	Service       string         `json:"service"`
	EnvironmentID string         `json:"environment_id"`
	Metadata      map[string]any `json:"metadata"`

	// GitHub Actions fields
	Repository string `json:"repository"`
	Ref        string `json:"ref"`
	Workflow   string `json:"workflow"`
}

func verifyHMAC(secret string, body []byte, signature string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	sig, err := hex.DecodeString(signature[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(mac.Sum(nil), sig)
}

func (h *WebhookHandler) Ingest(c echo.Context) error {
	secret := os.Getenv("WEBHOOK_SECRET")

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "could not read body"})
	}

	if secret != "" {
		if sig := c.Request().Header.Get("X-Hub-Signature-256"); sig != "" {
			if !verifyHMAC(secret, body, sig) {
				return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid webhook signature"})
			}
		} else if c.Request().Header.Get("X-Kollaber-Secret") != secret {
			return c.JSON(http.StatusUnauthorized, echo.Map{"error": "invalid webhook secret"})
		}
	}

	var payload genericWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	// Normalize GitHub Actions payload
	if payload.Type == "" && payload.Workflow != "" {
		payload.Type = "deploy"
		if payload.Service == "" {
			payload.Service = payload.Repository
		}
		if payload.Metadata == nil {
			payload.Metadata = map[string]any{}
		}
		payload.Metadata["ref"] = payload.Ref
		payload.Metadata["workflow"] = payload.Workflow
	}

	if payload.Type == "" || payload.Service == "" || payload.EnvironmentID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "type, service, and environment_id are required"})
	}

	envID, err := uuid.Parse(payload.EnvironmentID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
	}

	if payload.Metadata == nil {
		payload.Metadata = map[string]any{}
	}
	metaBytes, _ := json.Marshal(payload.Metadata)

	ctx := context.Background()
	event, err := h.q.CreateEvent(ctx, store.CreateEventParams{
		Type:          payload.Type,
		Service:       payload.Service,
		EnvironmentID: pgtype.UUID{Bytes: envID, Valid: true},
		Metadata:      metaBytes,
		Status:        "success",
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not ingest event"})
	}

	go func() {
		env, err := h.q.GetEnvironmentByID(ctx, pgtype.UUID{Bytes: envID, Valid: true})
		if err != nil {
			return
		}
		orgID := uuid.UUID(env.OrgID.Bytes)
		broadcastEvent(h.hub, orgID.String(), envID.String(), toEventResponse(event))
	}()

	return c.JSON(http.StatusCreated, event)
}
