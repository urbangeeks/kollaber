package billing

import (
	"fmt"
	"os"

	stripe "github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
)

func key() string { return os.Getenv("STRIPE_SECRET_KEY") }

func priceID(plan string) (string, error) {
	var env string
	switch plan {
	case PlanTeam:
		env = "STRIPE_TEAM_PRICE_ID"
	case PlanPro:
		env = "STRIPE_PRO_PRICE_ID"
	default:
		return "", fmt.Errorf("no stripe price for plan %q", plan)
	}
	id := os.Getenv(env)
	if id == "" {
		return "", fmt.Errorf("%s not set", env)
	}
	return id, nil
}

// EnsureCustomer creates a Stripe customer for the org if one doesn't exist yet.
func EnsureCustomer(orgName, orgID string) (string, error) {
	stripe.Key = key()
	c, err := customer.New(&stripe.CustomerParams{
		Name: stripe.String(orgName),
		Metadata: map[string]string{
			"org_id": orgID,
		},
	})
	if err != nil {
		return "", fmt.Errorf("stripe create customer: %w", err)
	}
	return c.ID, nil
}

// CreateCheckoutSession creates a Stripe hosted checkout session for upgrading
// to a paid plan. seats is the number of billable members.
func CreateCheckoutSession(plan, stripeCustomerID, orgID, successURL, cancelURL string, seats int64) (string, error) {
	stripe.Key = key()

	pid, err := priceID(plan)
	if err != nil {
		return "", err
	}

	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(stripeCustomerID),
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(successURL),
		CancelURL:  stripe.String(cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(pid),
				Quantity: stripe.Int64(seats),
			},
		},
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"org_id": orgID,
				"plan":   plan,
			},
		},
	}

	s, err := checkoutsession.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe checkout session: %w", err)
	}
	return s.URL, nil
}

// CreatePortalSession creates a Stripe billing portal session so the customer
// can manage their subscription, invoices, and payment method.
func CreatePortalSession(stripeCustomerID, returnURL string) (string, error) {
	stripe.Key = key()
	ps, err := portalsession.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(stripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", fmt.Errorf("stripe portal session: %w", err)
	}
	return ps.URL, nil
}

// ParseWebhook validates and parses an incoming Stripe webhook payload.
func ParseWebhook(body []byte, sigHeader string) (stripe.Event, error) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		return stripe.Event{}, fmt.Errorf("STRIPE_WEBHOOK_SECRET not set")
	}
	return webhook.ConstructEvent(body, sigHeader, secret)
}
