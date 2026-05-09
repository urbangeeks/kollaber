package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
)

type StreamHandler struct{ hub *Hub }

func NewStreamHandler(hub *Hub) *StreamHandler { return &StreamHandler{hub} }

func (h *StreamHandler) Stream(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	envID := c.QueryParam("environment_id")

	w := c.Response()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	w.WriteHeader(http.StatusOK)

	sub := h.hub.subscribe(orgID.String(), envID)
	defer h.hub.unsubscribe(orgID.String(), sub)

	// Initial keepalive so the client knows the connection is open.
	fmt.Fprintf(w, ":\n\n")
	w.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	ctx := c.Request().Context()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			fmt.Fprintf(w, ":\n\n") // SSE comment as keepalive
			w.Flush()
		case data, ok := <-sub.ch:
			if !ok {
				return nil
			}
			fmt.Fprintf(w, "data: %s\n\n", json.RawMessage(data))
			w.Flush()
		}
	}
}
