package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/resend"
	"github.com/urbangeeks/kollaber/internal/slack"
	"github.com/urbangeeks/kollaber/internal/store"
	"github.com/urbangeeks/kollaber/internal/teams"
)

type IncidentsHandler struct{ q *store.Queries }

func NewIncidentsHandler(q *store.Queries) *IncidentsHandler { return &IncidentsHandler{q} }

type incidentResponse struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Status     string `json:"status"`
	OwnerID    string `json:"owner_id,omitempty"`
	OpenedAt   string `json:"opened_at"`
	ResolvedAt string `json:"resolved_at,omitempty"`
	CreatedAt  string `json:"created_at"`
	EventCount int    `json:"event_count"`
}

const tsLayout = "2006-01-02T15:04:05Z07:00"

func toIncidentResponse(i store.Incident) incidentResponse {
	r := incidentResponse{
		ID:        uuid.UUID(i.ID.Bytes).String(),
		Title:     i.Title,
		Severity:  i.Severity,
		Status:    i.Status,
		OpenedAt:  i.OpenedAt.Time.Format(tsLayout),
		CreatedAt: i.CreatedAt.Time.Format(tsLayout),
	}
	if i.OwnerID.Valid {
		r.OwnerID = uuid.UUID(i.OwnerID.Bytes).String()
	}
	if i.ResolvedAt.Valid {
		r.ResolvedAt = i.ResolvedAt.Time.Format(tsLayout)
	}
	return r
}

func validSeverity(s string) bool {
	switch s {
	case "sev1", "sev2", "sev3", "sev4":
		return true
	}
	return false
}

func validIncidentStatus(s string) bool {
	switch s {
	case "open", "mitigated", "resolved":
		return true
	}
	return false
}

// notifyIncident fans an incident update out to email (opted-in members), Slack,
// and Teams. verb describes what happened, e.g. "opened" or "resolved". Runs in
// a goroutine off the request path, mirroring the event notification flow.
func (h *IncidentsHandler) notifyIncident(orgID uuid.UUID, inc store.Incident, verb string) {
	go func() {
		ctx := context.Background()
		pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

		recipients, err := h.q.GetOrgMembersToNotify(ctx, store.GetOrgMembersToNotifyParams{
			OrgID:   pgOrgID,
			Column2: "incident",
		})
		if err == nil && len(recipients) > 0 {
			_ = resend.SendIncidentNotification(recipients, inc.Title, inc.Severity, verb)
		}

		slackURL, _ := h.q.GetOrgSlackWebhook(ctx, pgOrgID)
		_ = slack.SendIncidentNotification(slackURL, inc.Title, inc.Severity, verb)

		teamsURL, _ := h.q.GetOrgTeamsWebhook(ctx, pgOrgID)
		_ = teams.SendIncidentNotification(teamsURL, inc.Title, inc.Severity, verb)
	}()
}

type createIncidentRequest struct {
	Title    string      `json:"title"`
	Severity string      `json:"severity"`
	OwnerID  string      `json:"owner_id"`
	EventIDs []uuid.UUID `json:"event_ids"`
}

func (h *IncidentsHandler) Create(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}

	var req createIncidentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "title is required"})
	}
	if req.Severity == "" {
		req.Severity = "sev3"
	}
	if !validSeverity(req.Severity) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "severity must be sev1, sev2, sev3, or sev4"})
	}

	var ownerID pgtype.UUID
	if req.OwnerID != "" {
		id, err := uuid.Parse(req.OwnerID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid owner_id"})
		}
		ownerID = pgtype.UUID{Bytes: id, Valid: true}
	}

	ctx := context.Background()
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	incident, err := h.q.CreateIncident(ctx, store.CreateIncidentParams{
		OrgID:    pgOrgID,
		Title:    req.Title,
		Severity: req.Severity,
		OwnerID:  ownerID,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create incident"})
	}

	if len(req.EventIDs) > 0 {
		if _, err := h.q.AttachEventsToIncident(ctx, incident.ID, pgOrgID, req.EventIDs); err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "incident created but events could not be attached"})
		}
	}

	h.notifyIncident(orgID, incident, "opened")

	return c.JSON(http.StatusCreated, toIncidentResponse(incident))
}

func (h *IncidentsHandler) List(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)

	status := c.QueryParam("status")
	if status != "" && !validIncidentStatus(status) {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "status must be open, mitigated, or resolved"})
	}

	ctx := context.Background()
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	incidents, err := h.q.ListIncidents(ctx, pgOrgID, status)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch incidents"})
	}
	// Best-effort event counts; absence just renders as zero.
	counts, _ := h.q.CountEventsByIncidentForOrg(ctx, pgOrgID)

	out := make([]incidentResponse, len(incidents))
	for i, inc := range incidents {
		r := toIncidentResponse(inc)
		r.EventCount = counts[inc.ID.Bytes]
		out[i] = r
	}
	return c.JSON(http.StatusOK, out)
}

func (h *IncidentsHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid incident id"})
	}
	ctx := context.Background()
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgID := pgtype.UUID{Bytes: id, Valid: true}
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	incident, err := h.q.GetIncidentByID(ctx, pgID, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "incident not found"})
	}

	events, err := h.q.ListIncidentEvents(ctx, pgID, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not fetch incident events"})
	}
	eventsOut := make([]eventResponse, len(events))
	for i, e := range events {
		eventsOut[i] = toEventResponse(e)
	}

	incResp := toIncidentResponse(incident)
	incResp.EventCount = len(events)

	return c.JSON(http.StatusOK, echo.Map{
		"incident": incResp,
		"events":   eventsOut,
	})
}

type updateIncidentRequest struct {
	Title    *string `json:"title"`
	Severity *string `json:"severity"`
	Status   *string `json:"status"`
	OwnerID  *string `json:"owner_id"`
}

func (h *IncidentsHandler) Update(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid incident id"})
	}

	ctx := context.Background()
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgID := pgtype.UUID{Bytes: id, Valid: true}
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	current, err := h.q.GetIncidentByID(ctx, pgID, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "incident not found"})
	}

	var req updateIncidentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}

	upd := store.UpdateIncidentParams{
		ID:       pgID,
		OrgID:    pgOrgID,
		Title:    current.Title,
		Severity: current.Severity,
		Status:   current.Status,
		OwnerID:  current.OwnerID,
	}

	if req.Title != nil {
		if *req.Title == "" {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "title cannot be empty"})
		}
		upd.Title = *req.Title
	}
	if req.Severity != nil {
		if !validSeverity(*req.Severity) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "severity must be sev1, sev2, sev3, or sev4"})
		}
		upd.Severity = *req.Severity
	}
	if req.Status != nil {
		if !validIncidentStatus(*req.Status) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "status must be open, mitigated, or resolved"})
		}
		upd.Status = *req.Status
	}
	if req.OwnerID != nil {
		if *req.OwnerID == "" {
			upd.OwnerID = pgtype.UUID{}
		} else {
			ownerID, err := uuid.Parse(*req.OwnerID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid owner_id"})
			}
			upd.OwnerID = pgtype.UUID{Bytes: ownerID, Valid: true}
		}
	}

	incident, err := h.q.UpdateIncident(ctx, upd)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not update incident"})
	}

	// Notify only when the status actually changed, using the new status as the verb.
	if req.Status != nil && incident.Status != current.Status {
		h.notifyIncident(orgID, incident, incident.Status)
	}

	return c.JSON(http.StatusOK, toIncidentResponse(incident))
}

type attachEventsRequest struct {
	EventIDs []uuid.UUID `json:"event_ids"`
}

func (h *IncidentsHandler) AttachEvents(c echo.Context) error {
	if err := requireMember(c); err != nil {
		return err
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid incident id"})
	}

	var req attachEventsRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if len(req.EventIDs) == 0 {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "event_ids is required"})
	}

	ctx := context.Background()
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgID := pgtype.UUID{Bytes: id, Valid: true}
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}

	// Ensure the incident exists in this org before attaching.
	if _, err := h.q.GetIncidentByID(ctx, pgID, pgOrgID); err != nil {
		return c.JSON(http.StatusNotFound, echo.Map{"error": "incident not found"})
	}

	attached, err := h.q.AttachEventsToIncident(ctx, pgID, pgOrgID, req.EventIDs)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not attach events"})
	}
	return c.JSON(http.StatusOK, echo.Map{"attached": attached})
}
