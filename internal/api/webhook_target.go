package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/store"
)

// webhookTarget is the environment a delivery is aimed at, in the three shapes
// the ingest path needs it: the pgtype id for queries, the uuid for the stream
// topic, and the owning org.
type webhookTarget struct {
	envID   pgtype.UUID
	envUUID uuid.UUID
	orgID   uuid.UUID
}

// resolveWebhookTarget reads ?environment_id= and loads the environment it
// names.
//
// Every CI and GitOps tool that posts here can configure a destination URL and
// little else, so the query string is the only place a per-target setting can
// live. It writes the failure response itself; a false second return means the
// response has already been sent and the handler should stop.
func resolveWebhookTarget(ctx context.Context, q *store.Queries, c echo.Context) (webhookTarget, bool) {
	param := c.QueryParam("environment_id")
	if param == "" {
		_ = c.JSON(http.StatusBadRequest, echo.Map{"error": "environment_id query parameter is required"})
		return webhookTarget{}, false
	}
	envUUID, err := uuid.Parse(param)
	if err != nil {
		_ = c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid environment_id"})
		return webhookTarget{}, false
	}

	envID := pgtype.UUID{Bytes: envUUID, Valid: true}
	env, err := q.GetEnvironmentByID(ctx, envID)
	if err != nil {
		_ = c.JSON(http.StatusNotFound, echo.Map{"error": "environment not found"})
		return webhookTarget{}, false
	}

	return webhookTarget{
		envID:   envID,
		envUUID: envUUID,
		orgID:   uuid.UUID(env.OrgID.Bytes),
	}, true
}
