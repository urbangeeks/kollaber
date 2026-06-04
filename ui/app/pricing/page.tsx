"use client"

import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Check, ArrowLeft } from "lucide-react"

const PLANS = [
  {
    id: "free",
    name: "Free",
    price: "$0",
    per: "forever",
    description: "For small teams getting started.",
    cta: "Get started",
    ctaVariant: "outline" as const,
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
    ctaVariant: "outline" as const,
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
    ctaVariant: "default" as const,
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
    ctaVariant: "outline" as const,
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

export default function PricingPage() {
  const router = useRouter()

  return (
    <div className="min-h-screen px-6 py-16">
      <div className="mx-auto max-w-5xl space-y-12">
        <div>
          <Button
            variant="ghost"
            size="sm"
            className="mb-6 text-muted-foreground"
            onClick={() => router.back()}
          >
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            Back
          </Button>
          <h1 className="text-3xl font-bold tracking-tight">Simple, transparent pricing</h1>
          <p className="mt-3 text-muted-foreground max-w-xl">
            Start free. Scale as your team grows. No hidden fees, no per-environment charges.
          </p>
        </div>

        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {PLANS.map((plan) => (
            <Card
              key={plan.id}
              className={`flex flex-col ${plan.highlight ? "border-violet-500/50 shadow-sm shadow-violet-500/10" : ""}`}
            >
              <CardHeader className="pb-4">
                <div className="flex items-start justify-between gap-2">
                  <CardTitle className="text-base">{plan.name}</CardTitle>
                  {plan.badge && (
                    <Badge variant="secondary" className="text-[10px] shrink-0">
                      {plan.badge}
                    </Badge>
                  )}
                </div>
                <div className="mt-1">
                  <span className="text-2xl font-bold">{plan.price}</span>
                  {plan.per !== "forever" && plan.price !== "Custom" && (
                    <span className="text-sm text-muted-foreground ml-1">{plan.per}</span>
                  )}
                  {plan.price === "Custom" && (
                    <span className="text-sm text-muted-foreground ml-1">{plan.per}</span>
                  )}
                  {plan.per === "forever" && (
                    <span className="text-sm text-muted-foreground ml-1">forever</span>
                  )}
                </div>
                <CardDescription className="mt-1 text-xs">{plan.description}</CardDescription>
              </CardHeader>

              <CardContent className="flex flex-col flex-1 gap-6">
                <ul className="space-y-2 text-sm flex-1">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-start gap-2 text-muted-foreground">
                      <Check className="h-3.5 w-3.5 shrink-0 mt-0.5 text-green-500" />
                      {f}
                    </li>
                  ))}
                </ul>

                <Button
                  variant={plan.ctaVariant}
                  className="w-full"
                  onClick={() => {
                    if (plan.id === "enterprise") {
                      window.location.href = "mailto:hello@kollaber.io"
                    } else {
                      router.push("/register")
                    }
                  }}
                >
                  {plan.cta}
                </Button>
              </CardContent>
            </Card>
          ))}
        </div>

        {/* FAQ */}
        <div className="space-y-6 pt-4">
          <h2 className="text-xl font-semibold">Common questions</h2>
          <div className="grid gap-6 sm:grid-cols-2">
            {[
              {
                q: "What counts as a seat?",
                a: "Owner, admin, and member roles use a seat. Viewer access is always free — perfect for stakeholders who only need read access.",
              },
              {
                q: "Can I switch plans?",
                a: "Yes. Upgrade or downgrade any time from Settings → Billing. Stripe handles proration automatically.",
              },
              {
                q: "What happens when I hit a limit?",
                a: "You'll see a clear error and a link to upgrade. No silent overages or surprise charges.",
              },
              {
                q: "Is there a free trial?",
                a: "Team and Pro plans include a 14-day trial. No credit card required to start.",
              },
            ].map(({ q, a }) => (
              <div key={q} className="space-y-1">
                <p className="font-medium text-sm">{q}</p>
                <p className="text-sm text-muted-foreground">{a}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
