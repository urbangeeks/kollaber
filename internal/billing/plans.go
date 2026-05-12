package billing

import "os"

const (
	PlanFree       = "free"
	PlanTeam       = "team"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// Entitlements describes what an org is allowed to do under a given plan.
// -1 means unlimited.
type Entitlements struct {
	Plan                string
	MaxMembers          int
	MaxEnvironments     int
	HistoryDays         int
	SlackIntegration    bool
	TeamsIntegration    bool
	KubernetesIngestion bool
	AISummaries         bool
	AIPostmortems       bool
	SSO                 bool
	AuditLogs           bool
}

var planEntitlements = map[string]Entitlements{
	PlanFree: {
		Plan:             PlanFree,
		MaxMembers:       5,
		MaxEnvironments:  2,
		HistoryDays:      30,
		SlackIntegration: true,
		TeamsIntegration: true,
	},
	PlanTeam: {
		Plan:             PlanTeam,
		MaxMembers:       25,
		MaxEnvironments:  -1,
		HistoryDays:      -1,
		SlackIntegration: true,
		TeamsIntegration: true,
		AISummaries:      true,
	},
	PlanPro: {
		Plan:                PlanPro,
		MaxMembers:          -1,
		MaxEnvironments:     -1,
		HistoryDays:         -1,
		SlackIntegration:    true,
		TeamsIntegration:    true,
		KubernetesIngestion: true,
		AISummaries:         true,
		AIPostmortems:       true,
		SSO:                 true,
		AuditLogs:           true,
	},
	PlanEnterprise: {
		Plan:                PlanEnterprise,
		MaxMembers:          -1,
		MaxEnvironments:     -1,
		HistoryDays:         -1,
		SlackIntegration:    true,
		TeamsIntegration:    true,
		KubernetesIngestion: true,
		AISummaries:         true,
		AIPostmortems:       true,
		SSO:                 true,
		AuditLogs:           true,
	},
}

// PricingInfo describes public-facing plan pricing.
type PricingInfo struct {
	ID          string
	Name        string
	PricePerSeat int // USD cents per seat per month; 0 = free; -1 = contact sales
	Description string
	Features    []string
}

var PricingTable = []PricingInfo{
	{
		ID:          PlanFree,
		Name:        "Free",
		Description: "For small teams getting started.",
		Features: []string{
			"Up to 5 members",
			"2 environments",
			"30-day history",
			"Slack & Teams notifications",
			"CLI + webhooks",
		},
	},
	{
		ID:           PlanTeam,
		Name:         "Team",
		PricePerSeat: 1500,
		Description:  "For growing engineering teams.",
		Features: []string{
			"Up to 25 members",
			"Unlimited environments",
			"Unlimited history",
			"AI event summaries",
			"All Free features",
		},
	},
	{
		ID:           PlanPro,
		Name:         "Pro",
		PricePerSeat: 3500,
		Description:  "For teams running production at scale.",
		Features: []string{
			"Unlimited members",
			"Kubernetes auto-ingestion",
			"AI postmortems",
			"SSO",
			"Audit logs",
			"All Team features",
		},
	},
	{
		ID:           PlanEnterprise,
		Name:         "Enterprise",
		PricePerSeat: -1,
		Description:  "Custom contracts, SLAs, and dedicated support.",
		Features: []string{
			"SAML / SCIM",
			"Custom data retention",
			"Data residency",
			"Dedicated support",
			"All Pro features",
		},
	},
}

// EntitlementsFor returns the entitlements for a given plan.
// When SELF_HOSTED=true all features are unlocked regardless of plan.
func EntitlementsFor(plan string) Entitlements {
	if os.Getenv("SELF_HOSTED") == "true" {
		return planEntitlements[PlanEnterprise]
	}
	if e, ok := planEntitlements[plan]; ok {
		return e
	}
	return planEntitlements[PlanFree]
}
