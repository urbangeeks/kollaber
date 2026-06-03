package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v4"
	stripe "github.com/stripe/stripe-go/v82"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/middleware"
	"github.com/urbangeeks/kollaber/internal/store"
)

type BillingHandler struct{ q *store.Queries }

func NewBillingHandler(q *store.Queries) *BillingHandler { return &BillingHandler{q} }

type billingStatusResponse struct {
	Plan               string `json:"plan"`
	SubscriptionStatus string `json:"subscription_status"`
	SeatsUsed          int    `json:"seats_used"`
	SeatsLimit         int    `json:"seats_limit"`
	EnvironmentsUsed   int    `json:"environments_used"`
	EnvironmentsLimit  int    `json:"environments_limit"`
	HistoryDays        int    `json:"history_days"`
	Entitlements       struct {
		KubernetesIngestion bool `json:"kubernetes_ingestion"`
		AISummaries         bool `json:"ai_summaries"`
		AIPostmortems       bool `json:"ai_postmortems"`
		AIAgent             bool `json:"ai_agent"`
		SSO                 bool `json:"sso"`
		AuditLogs           bool `json:"audit_logs"`
	} `json:"entitlements"`
	HasStripeCustomer bool `json:"has_stripe_customer"`
}

func (h *BillingHandler) Get(c echo.Context) error {
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}

	seats, _ := h.q.CountBillableMembers(ctx, pgOrgID)
	envs, _ := h.q.CountEnvironments(ctx, pgOrgID)
	ent := billing.EntitlementsFor(b.Plan)

	resp := billingStatusResponse{
		Plan:               b.Plan,
		SubscriptionStatus: b.SubscriptionStatus,
		SeatsUsed:          seats,
		SeatsLimit:         ent.MaxMembers,
		EnvironmentsUsed:   envs,
		EnvironmentsLimit:  ent.MaxEnvironments,
		HistoryDays:        ent.HistoryDays,
		HasStripeCustomer:  b.StripeCustomerID != "",
	}
	resp.Entitlements.KubernetesIngestion = ent.KubernetesIngestion
	resp.Entitlements.AISummaries = ent.AISummaries
	resp.Entitlements.AIPostmortems = ent.AIPostmortems
	resp.Entitlements.AIAgent = ent.AIAgent
	resp.Entitlements.SSO = ent.SSO
	resp.Entitlements.AuditLogs = ent.AuditLogs

	return c.JSON(http.StatusOK, resp)
}

type checkoutRequest struct {
	Plan string `json:"plan"`
}

func (h *BillingHandler) Checkout(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	var req checkoutRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": err.Error()})
	}
	if req.Plan != billing.PlanTeam && req.Plan != billing.PlanPro {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "plan must be team or pro"})
	}

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not load billing"})
	}

	if b.StripeCustomerID == "" {
		orgName, _ := h.q.GetOrgName(ctx, pgOrgID)
		customerID, err := billing.EnsureCustomer(orgName, orgID.String())
		if err != nil {
			return c.JSON(http.StatusInternalServerError, echo.Map{"error": "could not create billing account"})
		}
		_ = h.q.SetOrgStripeCustomer(ctx, pgOrgID, customerID)
		b.StripeCustomerID = customerID
	}

	seats, _ := h.q.CountBillableMembers(ctx, pgOrgID)
	if seats < 1 {
		seats = 1
	}

	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}

	url, err := billing.CreateCheckoutSession(
		req.Plan,
		b.StripeCustomerID,
		orgID.String(),
		baseURL+"/settings/billing?upgraded=true",
		baseURL+"/settings/billing",
		int64(seats),
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"url": url})
}

func (h *BillingHandler) Portal(c echo.Context) error {
	if err := requireAdmin(c); err != nil {
		return err
	}
	orgID := c.Get(middleware.OrgIDKey).(uuid.UUID)
	pgOrgID := pgtype.UUID{Bytes: orgID, Valid: true}
	ctx := context.Background()

	b, err := h.q.GetOrgBilling(ctx, pgOrgID)
	if err != nil || b.StripeCustomerID == "" {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "no billing account found"})
	}

	baseURL := os.Getenv("FRONTEND_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3000"
	}

	url, err := billing.CreatePortalSession(b.StripeCustomerID, baseURL+"/settings/billing")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, echo.Map{"url": url})
}

// StripeWebhook handles Stripe webhook events (unauthenticated, verified by signature).
func (h *BillingHandler) StripeWebhook(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "could not read body"})
	}

	event, err := billing.ParseWebhook(body, c.Request().Header.Get("Stripe-Signature"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, echo.Map{"error": "invalid webhook signature"})
	}

	ctx := context.Background()

	switch event.Type {
	case stripe.EventTypeCheckoutSessionCompleted:
		h.handleCheckoutCompleted(ctx, event)
	case stripe.EventTypeCustomerSubscriptionUpdated:
		h.handleSubscriptionUpdated(ctx, event)
	case stripe.EventTypeCustomerSubscriptionDeleted:
		h.handleSubscriptionDeleted(ctx, event)
	case stripe.EventTypeInvoicePaid:
		h.handleInvoicePaid(ctx, event)
	case stripe.EventTypeInvoicePaymentFailed:
		h.handlePaymentFailed(ctx, event)
	}

	return c.JSON(http.StatusOK, echo.Map{"received": true})
}

func (h *BillingHandler) handleCheckoutCompleted(ctx context.Context, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return
	}
	if session.Subscription == nil || session.Customer == nil {
		return
	}
	// session.Subscription is a stub in this event — read plan from session metadata
	plan := session.Metadata["plan"]
	if plan == "" {
		plan = billing.PlanTeam
	}
	orgID, _, err := h.q.GetOrgBillingByStripeCustomerID(ctx, session.Customer.ID)
	if err != nil {
		return
	}
	_ = h.q.SetOrgSubscription(ctx, orgID, plan, session.Subscription.ID, string(session.Subscription.Status))
}

func (h *BillingHandler) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return
	}
	plan := sub.Metadata["plan"]
	if plan == "" {
		plan = billing.PlanTeam
	}
	orgID, _, err := h.q.GetOrgBillingByStripeSubscriptionID(ctx, sub.ID)
	if err != nil {
		return
	}
	_ = h.q.SetOrgSubscription(ctx, orgID, plan, sub.ID, string(sub.Status))
}

func (h *BillingHandler) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		return
	}
	orgID, _, err := h.q.GetOrgBillingByStripeSubscriptionID(ctx, sub.ID)
	if err != nil {
		return
	}
	_ = h.q.SetOrgSubscription(ctx, orgID, billing.PlanFree, "", "canceled")
}

func (h *BillingHandler) handleInvoicePaid(ctx context.Context, event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return
	}
	if inv.Customer == nil {
		return
	}
	orgID, _, err := h.q.GetOrgBillingByStripeCustomerID(ctx, inv.Customer.ID)
	if err != nil {
		return
	}
	_ = h.q.SetOrgSubscriptionStatus(ctx, orgID, "active")
}

func (h *BillingHandler) handlePaymentFailed(ctx context.Context, event stripe.Event) {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return
	}
	if inv.Customer == nil {
		return
	}
	orgID, _, err := h.q.GetOrgBillingByStripeCustomerID(ctx, inv.Customer.ID)
	if err != nil {
		return
	}
	_ = h.q.SetOrgSubscriptionStatus(ctx, orgID, "past_due")
}
