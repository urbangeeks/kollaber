"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getToken,
  getCurrentRole,
  getBillingStatus,
  createCheckoutSession,
  createPortalSession,
  type BillingStatus,
} from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { ExternalLink, Zap } from "lucide-react"

const PLAN_LABEL: Record<string, string> = {
  free: "Free",
  team: "Team",
  pro: "Pro",
  enterprise: "Enterprise",
}

const PLAN_BADGE: Record<string, string> = {
  free:       "bg-muted text-muted-foreground border-border",
  team:       "bg-blue-500/15 text-blue-600 border-blue-500/30",
  pro:        "bg-violet-500/15 text-violet-600 border-violet-500/30",
  enterprise: "bg-amber-500/15 text-amber-600 border-amber-500/30",
}

function UsageBar({ used, limit, label }: { used: number; limit: number; label: string }) {
  const unlimited = limit === -1
  const pct = unlimited ? 0 : Math.min(100, Math.round((used / limit) * 100))
  const danger = !unlimited && pct >= 90

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-sm">
        <span className="text-muted-foreground">{label}</span>
        <span className={danger ? "font-medium text-destructive" : "text-foreground"}>
          {unlimited ? `${used} / ∞` : `${used} / ${limit}`}
        </span>
      </div>
      {!unlimited && (
        <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
          <div
            className={`h-full rounded-full transition-all ${danger ? "bg-destructive" : "bg-primary"}`}
            style={{ width: `${pct}%` }}
          />
        </div>
      )}
    </div>
  )
}

const UPGRADE_PLANS = [
  {
    id: "team",
    name: "Team",
    price: "$12",
    per: "seat/mo",
    features: ["25 members", "Unlimited environments", "Full history", "AI summaries", "Email support"],
  },
  {
    id: "pro",
    name: "Pro",
    price: "$24",
    per: "seat/mo",
    features: ["Unlimited members", "Unlimited environments", "Full history", "AI summaries + postmortems", "Kubernetes ingestion", "SSO", "Audit logs", "Priority support"],
  },
]

export default function BillingPage() {
  const router = useRouter()
  const [billing, setBilling] = useState<BillingStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [checkoutLoading, setCheckoutLoading] = useState<string | null>(null)
  const [portalLoading, setPortalLoading] = useState(false)
  const [role, setRole] = useState<string | null>(null)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    setRole(getCurrentRole())
    getBillingStatus()
      .then(setBilling)
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }, [router])

  const isAdminOrOwner = role === "owner" || role === "admin"

  async function handleUpgrade(plan: string) {
    if (!isAdminOrOwner) {
      toast.error("Only admins and owners can manage billing")
      return
    }
    setCheckoutLoading(plan)
    try {
      const url = await createCheckoutSession(plan)
      window.location.href = url
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to start checkout")
      setCheckoutLoading(null)
    }
  }

  async function handleManageBilling() {
    setPortalLoading(true)
    try {
      const url = await createPortalSession()
      window.location.href = url
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to open billing portal")
      setPortalLoading(false)
    }
  }

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-xl font-semibold">Billing</h1>
        <p className="text-sm text-muted-foreground mt-1">Manage your plan and usage.</p>
      </div>

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : billing ? (
        <>
          {/* Current plan */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-base">Current plan</CardTitle>
                <span className={`rounded border px-2 py-0.5 text-xs font-medium ${PLAN_BADGE[billing.plan] ?? PLAN_BADGE.free}`}>
                  {PLAN_LABEL[billing.plan] ?? billing.plan}
                </span>
              </div>
              {billing.subscription_status && billing.subscription_status !== "active" && (
                <p className="text-xs text-amber-600 mt-1">
                  Subscription status: {billing.subscription_status}
                </p>
              )}
            </CardHeader>
            <CardContent className="space-y-4">
              <UsageBar
                used={billing.seats_used}
                limit={billing.seats_limit}
                label="Seats (billable members)"
              />
              <UsageBar
                used={billing.environments_used}
                limit={billing.environments_limit}
                label="Environments"
              />
              {billing.history_days !== -1 && (
                <p className="text-xs text-muted-foreground">
                  Event history: {billing.history_days} days
                </p>
              )}

              {billing.has_stripe_customer && isAdminOrOwner && (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={portalLoading}
                  onClick={handleManageBilling}
                >
                  <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
                  {portalLoading ? "Opening…" : "Manage billing"}
                </Button>
              )}
            </CardContent>
          </Card>

          {/* Entitlements summary */}
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Features</CardTitle>
            </CardHeader>
            <CardContent>
              <ul className="space-y-1.5 text-sm">
                {[
                  { key: "kubernetes_ingestion", label: "Kubernetes ingestion" },
                  { key: "ai_summaries",         label: "AI summaries" },
                  { key: "ai_postmortems",       label: "AI postmortems" },
                  { key: "sso",                  label: "SSO" },
                  { key: "audit_logs",           label: "Audit logs" },
                ].map(({ key, label }) => (
                  <li key={key} className="flex items-center gap-2">
                    <span className={`h-1.5 w-1.5 rounded-full ${(billing.entitlements as Record<string, boolean>)[key] ? "bg-green-500" : "bg-muted-foreground/30"}`} />
                    <span className={(billing.entitlements as Record<string, boolean>)[key] ? "text-foreground" : "text-muted-foreground line-through"}>
                      {label}
                    </span>
                  </li>
                ))}
              </ul>
            </CardContent>
          </Card>

          {/* Upgrade options — only show when not on pro/enterprise */}
          {billing.plan === "free" || billing.plan === "team" ? (
            <div className="space-y-3">
              <h2 className="text-sm font-medium">Upgrade your plan</h2>
              <div className="grid gap-4 sm:grid-cols-2">
                {UPGRADE_PLANS.filter(p => {
                  if (billing.plan === "team") return p.id === "pro"
                  return true
                }).map((plan) => (
                  <Card key={plan.id} className={plan.id === "pro" ? "border-violet-500/40" : ""}>
                    <CardHeader className="pb-3">
                      <div className="flex items-center justify-between">
                        <CardTitle className="text-base">{plan.name}</CardTitle>
                        {plan.id === "pro" && (
                          <Badge variant="secondary" className="text-[10px]">Popular</Badge>
                        )}
                      </div>
                      <CardDescription>
                        <span className="text-xl font-semibold text-foreground">{plan.price}</span>
                        {" "}{plan.per}
                      </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <ul className="space-y-1.5 text-sm">
                        {plan.features.map((f) => (
                          <li key={f} className="flex items-center gap-2 text-muted-foreground">
                            <span className="h-1.5 w-1.5 rounded-full bg-primary shrink-0" />
                            {f}
                          </li>
                        ))}
                      </ul>
                      {isAdminOrOwner ? (
                        <Button
                          className="w-full"
                          variant={plan.id === "pro" ? "default" : "outline"}
                          disabled={checkoutLoading !== null}
                          onClick={() => handleUpgrade(plan.id)}
                        >
                          <Zap className="mr-1.5 h-3.5 w-3.5" />
                          {checkoutLoading === plan.id ? "Redirecting…" : `Upgrade to ${plan.name}`}
                        </Button>
                      ) : (
                        <p className="text-xs text-muted-foreground">Only admins can upgrade.</p>
                      )}
                    </CardContent>
                  </Card>
                ))}
              </div>
            </div>
          ) : null}
        </>
      ) : (
        <p className="text-sm text-destructive">Failed to load billing info.</p>
      )}
    </div>
  )
}
