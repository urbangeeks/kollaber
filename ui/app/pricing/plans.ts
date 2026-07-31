/**
 * Plan data for the pricing page.
 *
 * Extracted from page.tsx so the route's structured data (layout.tsx) is built
 * from the same source the table renders from — schema prices that drift from
 * displayed prices are a rich-result violation.
 */

export type Plan = {
  id: string
  name: string
  price: string
  per: string
  description: string
  cta: string
  ctaVariant: "default" | "outline"
  highlight: boolean
  badge?: string
  features: string[]
}

export const PLANS: Plan[] = [
  {
    id: "free",
    name: "Free",
    price: "$0",
    per: "forever",
    description: "For small teams getting started.",
    cta: "Get started",
    ctaVariant: "outline",
    highlight: false,
    features: [
      "Up to 5 members",
      "2 environments",
      "30-day event history",
      "Webhook ingestion",
      "CLI tool",
      "Email notifications",
    ],
  },
  {
    id: "team",
    name: "Team",
    price: "$12",
    per: "seat/mo",
    description: "Growing teams that need more room.",
    cta: "Start free trial",
    ctaVariant: "outline",
    highlight: false,
    features: [
      "Up to 25 members",
      "Unlimited environments",
      "Unlimited history",
      "AI event summaries",
      "AI timeline assistant",
      "Slack & Teams notifications",
      "Email support",
    ],
  },
  {
    id: "pro",
    name: "Pro",
    price: "$24",
    per: "seat/mo",
    description: "Engineering orgs that run on Kubernetes.",
    cta: "Start free trial",
    ctaVariant: "default",
    highlight: true,
    badge: "Most popular",
    features: [
      "Unlimited members",
      "Unlimited environments",
      "Unlimited history",
      "AI summaries + postmortems",
      "AI timeline assistant",
      "Kubernetes watcher ingestion",
      "SSO (SAML / OIDC)",
      "Audit logs",
      "Priority support",
    ],
  },
  {
    id: "enterprise",
    name: "Enterprise",
    price: "Custom",
    per: "per org",
    description: "Dedicated support and custom contracts.",
    cta: "Contact us",
    ctaVariant: "outline",
    highlight: false,
    features: [
      "Everything in Pro",
      "Volume seat pricing",
      "Dedicated Slack channel",
      "SLA guarantees",
      "Custom data retention",
      "On-prem deployment option",
    ],
  },
]
