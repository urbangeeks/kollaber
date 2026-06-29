"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getDora,
  getEnvironments,
  getToken,
  type Dora,
  type DoraMetric,
  type DoraRating,
  type Environment,
} from "@/lib/api"
import { ThemeToggle } from "@/components/settings-nav"
import { DotBackground } from "@/components/dot-background"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { ArrowLeft, Loader2, Rocket, Timer, TriangleAlert, Wrench } from "lucide-react"

// DORA performance tiers map to a consistent colour across badge and value.
const RATING: Record<DoraRating, { label: string; className: string }> = {
  elite:   { label: "Elite",  className: "bg-green-500/15 text-green-600 dark:text-green-400 border-green-500/30" },
  high:    { label: "High",   className: "bg-blue-500/15 text-blue-600 dark:text-blue-400 border-blue-500/30" },
  medium:  { label: "Medium", className: "bg-amber-500/15 text-amber-600 dark:text-amber-400 border-amber-500/30" },
  low:     { label: "Low",    className: "bg-red-500/15 text-red-600 dark:text-red-400 border-red-500/30" },
  "n/a":   { label: "N/A",    className: "bg-muted text-muted-foreground border-border" },
}

const WINDOWS = [7, 30, 90] as const

type CardDef = {
  key: keyof Pick<Dora, "deploy_frequency" | "lead_time" | "change_failure_rate" | "time_to_restore">
  label: string
  hint: string
  icon: typeof Rocket
}

const CARDS: CardDef[] = [
  { key: "deploy_frequency",    label: "Deployment frequency",  hint: "How often you ship",                 icon: Rocket },
  { key: "lead_time",           label: "Lead time for changes", hint: "Commit → deploy",                    icon: Timer },
  { key: "change_failure_rate", label: "Change failure rate",   hint: "Deploys that failed",                icon: TriangleAlert },
  { key: "time_to_restore",     label: "Time to restore",       hint: "Incident open → resolved",           icon: Wrench },
]

function MetricCard({ def, metric, note }: { def: CardDef; metric: DoraMetric; note?: string }) {
  const rating = RATING[metric.rating]
  const Icon = def.icon
  // With no data (n/a) the "display" is a short explanatory phrase, not a
  // number — render it muted and smaller so it doesn't masquerade as a value.
  const empty = metric.rating === "n/a"
  return (
    <Card>
      <CardContent className="p-5">
        <div className="flex items-start justify-between gap-2">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Icon className="h-4 w-4" />
            {def.label}
          </div>
          <span className={`shrink-0 whitespace-nowrap rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium ${rating.className}`}>
            {rating.label}
          </span>
        </div>
        <div
          className={
            empty
              ? "mt-3 text-base font-medium text-muted-foreground"
              : "mt-3 text-3xl font-semibold tracking-tight tabular-nums"
          }
        >
          {metric.display}
        </div>
        <div className="mt-1 text-xs text-muted-foreground">
          {def.hint}
          {note ? <span className="ml-1 italic">· {note}</span> : null}
          {metric.samples > 0 ? <span className="ml-1">· {metric.samples} sample{metric.samples === 1 ? "" : "s"}</span> : null}
        </div>
      </CardContent>
    </Card>
  )
}

// fmtDay turns an ISO "YYYY-MM-DD" day key into a short "Mon D" label.
function fmtDay(day: string): string {
  // Parse as UTC so the label matches the bucket key regardless of local tz.
  const d = new Date(`${day}T00:00:00Z`)
  if (isNaN(d.getTime())) return day
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" })
}

// TrendChart draws a bar-per-day sparkline of deploy counts. Bars are
// height-scaled to the busiest day so a quiet week still reads clearly.
// Because heights are relative, the chart annotates the peak (Y reference),
// the window's start/end dates (X), and the total/average (a key for scale).
// Each bar is a shadcn Tooltip trigger; content renders in a portal so it is
// never clipped by the card or the chart's own bounds.
function TrendChart({ trend }: { trend: Dora["trend"] }) {
  if (trend.length === 0) {
    return <p className="text-sm text-muted-foreground">No deploys in this window.</p>
  }
  const counts = trend.map((p) => p.deploys)
  const max = Math.max(...counts, 1)
  const total = counts.reduce((a, b) => a + b, 0)
  const perDay = total / trend.length

  return (
    <TooltipProvider delayDuration={0}>
      <div className="mb-2 flex items-baseline justify-between text-xs text-muted-foreground">
        <span>
          <span className="font-medium text-foreground">{total}</span> deploy
          {total === 1 ? "" : "s"} · ~{perDay.toFixed(1)}/day
        </span>
        <span>peak {max}/day</span>
      </div>
      {/* Bars sit on a baseline; height is a fraction of the peak day. */}
      <div className="flex h-32 items-end gap-1 border-b border-border">
        {trend.map((p) => (
          <Tooltip key={p.day}>
            <TooltipTrigger asChild>
              <div
                className="flex-1 rounded-t bg-primary/70 transition-colors hover:bg-primary data-[state=delayed-open]:bg-primary"
                style={{ height: `${Math.max(4, (p.deploys / max) * 100)}%` }}
              />
            </TooltipTrigger>
            <TooltipContent>
              <div className="font-medium">{fmtDay(p.day)}</div>
              <div className="opacity-80">
                {p.deploys} deploy{p.deploys === 1 ? "" : "s"}
              </div>
            </TooltipContent>
          </Tooltip>
        ))}
      </div>
      <div className="mt-1 flex justify-between text-xs text-muted-foreground">
        <span>{fmtDay(trend[0].day)}</span>
        <span>{fmtDay(trend[trend.length - 1].day)}</span>
      </div>
    </TooltipProvider>
  )
}

export default function MetricsPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [envId, setEnvId] = useState<string>("") // "" = all environments
  const [days, setDays] = useState<number>(30)
  const [data, setData] = useState<Dora | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments()
      .then(setEnvs)
      .catch((err) => toast.error(err.message))
  }, [router])

  useEffect(() => {
    if (!getToken()) return
    setLoading(true)
    getDora(days, envId || undefined)
      .then(setData)
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }, [days, envId])

  const scopedToEnv = envId !== ""

  return (
    <div className="min-h-screen">
      <DotBackground />
      <header className="border-b px-4 py-4 sm:px-8">
        <div className="mx-auto flex max-w-5xl items-center gap-4">
          <Button variant="ghost" size="sm" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            Dashboard
          </Button>
          <span className="font-semibold tracking-tight">DORA metrics</span>
          <div className="ml-auto">
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-4 py-8 sm:px-8">
        {/* Controls */}
        <div className="mb-6 flex flex-wrap items-center gap-4">
          <div className="flex items-center gap-1 rounded-lg border p-1">
            {WINDOWS.map((w) => (
              <Button
                key={w}
                variant={days === w ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setDays(w)}
              >
                {w}d
              </Button>
            ))}
          </div>
          <div className="flex flex-wrap items-center gap-1 rounded-lg border p-1">
            <Button
              variant={envId === "" ? "secondary" : "ghost"}
              size="sm"
              onClick={() => setEnvId("")}
            >
              All environments
            </Button>
            {envs.map((env) => (
              <Button
                key={env.id}
                variant={envId === env.id ? "secondary" : "ghost"}
                size="sm"
                onClick={() => setEnvId(env.id)}
              >
                {env.name}
              </Button>
            ))}
          </div>
          {loading && <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />}
        </div>

        {data && (
          <>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              {CARDS.map((def) => (
                <MetricCard
                  key={def.key}
                  def={def}
                  metric={data[def.key]}
                  // MTTR is computed from incidents, which carry no environment —
                  // so it stays org-wide even when a single env is selected.
                  note={def.key === "time_to_restore" && scopedToEnv ? "org-wide" : undefined}
                />
              ))}
            </div>

            <Card className="mt-6">
              <CardContent className="p-5">
                <div className="mb-4 text-sm font-medium">
                  Deployments · last {data.window_days} days
                </div>
                <TrendChart trend={data.trend} />
              </CardContent>
            </Card>
          </>
        )}

        {!data && !loading && (
          <p className="text-sm text-muted-foreground">No data.</p>
        )}
      </main>
    </div>
  )
}
