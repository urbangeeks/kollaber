"use client"

import { useEffect, useState, type ReactNode } from "react"
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
import { AppShell } from "@/components/app-shell"
import { Card, CardContent } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { Loader2, Rocket, Timer, TriangleAlert, Wrench } from "lucide-react"

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

function MetricCard({
  def,
  metric,
  note,
  sparkline,
}: {
  def: CardDef
  metric: DoraMetric
  note?: string
  sparkline?: ReactNode
}) {
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
        {sparkline ? <div className="mt-3">{sparkline}</div> : null}
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

// fmtDuration renders a span of seconds compactly (e.g. "3.2h", "1.5d"),
// mirroring the backend's humanDuration so card and sparkline agree.
function fmtDuration(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}s`
  if (seconds < 3600) return `${(seconds / 60).toFixed(1)}m`
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)}h`
  return `${(seconds / 86400).toFixed(1)}d`
}

// MiniSparkline draws a compact bar-per-point trend for a secondary metric,
// sized to fit inside a metric card. Heights are relative to the series max, so
// the tallest bar is the period's worst (lead time / failure rate / MTTR are
// all "lower is better"). `format` builds each bar's hover label; points with
// no value render as a faint stub.
function MiniSparkline({
  series,
  format,
}: {
  series: { label: string; value: number }[]
  format: (v: number) => string
}) {
  const max = Math.max(...series.map((p) => p.value), 0)
  if (series.length === 0 || max === 0) {
    return <div className="h-8 text-[10px] leading-8 text-muted-foreground">no trend data</div>
  }
  return (
    <TooltipProvider delayDuration={0}>
      <div className="flex h-8 items-end gap-px">
        {series.map((p, i) => (
          <Tooltip key={i}>
            <TooltipTrigger asChild>
              <div
                className="flex-1 rounded-t bg-muted-foreground/40 transition-colors hover:bg-muted-foreground data-[state=delayed-open]:bg-muted-foreground"
                style={{ height: p.value > 0 ? `${Math.max(6, (p.value / max) * 100)}%` : "2px" }}
              />
            </TooltipTrigger>
            <TooltipContent>
              <div className="font-medium">{p.label}</div>
              <div className="opacity-80">{p.value > 0 ? format(p.value) : "—"}</div>
            </TooltipContent>
          </Tooltip>
        ))}
      </div>
    </TooltipProvider>
  )
}

// TrendChart draws a bar-per-day sparkline of deploy counts. Bars are
// height-scaled to the busiest day so a quiet week still reads clearly.
// Because heights are relative, the chart gives the reader a key and scale:
// y-axis ticks + gridlines (0 / mid / peak), the window's start/end dates (X),
// and a total/average/failed caption. Days that shipped a failed deploy are
// tinted with the destructive colour so the chart shows quality, not just
// volume. Each bar is a shadcn Tooltip trigger; content renders in a portal so
// it is never clipped by the card or the chart's own bounds.
function TrendChart({ trend }: { trend: Dora["trend"] }) {
  const counts = trend.map((p) => p.deploys)
  const total = counts.reduce((a, b) => a + b, 0)
  const failedTotal = trend.reduce((a, p) => a + p.failed, 0)

  // Empty state: no deploys at all in the window. Keep the chart's height so
  // the surrounding layout doesn't jump when a window has data vs. not.
  if (total === 0) {
    return (
      <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
        No deploys in the last {trend.length || "—"} days.
      </div>
    )
  }

  const max = Math.max(...counts, 1)
  const perDay = total / trend.length
  const mid = Math.round(max / 2)
  const showMid = max > 1 && mid > 0 && mid < max

  return (
    <TooltipProvider delayDuration={0}>
      <div className="mb-2 flex items-baseline justify-between text-xs text-muted-foreground">
        <span>
          <span className="font-medium text-foreground">{total}</span> deploy
          {total === 1 ? "" : "s"} · ~{perDay.toFixed(1)}/day
          {failedTotal > 0 && (
            <span className="text-destructive"> · {failedTotal} failed</span>
          )}
        </span>
        <span>peak {max}/day</span>
      </div>

      <div className="flex gap-2">
        {/* Y-axis tick labels, aligned to the gridlines in the plot area. */}
        <div className="flex h-32 w-5 flex-col justify-between text-right text-[10px] leading-none text-muted-foreground tabular-nums">
          <span>{max}</span>
          {showMid ? <span>{mid}</span> : <span />}
          <span>0</span>
        </div>

        {/* Plot area: gridlines behind, bars in front on a shared baseline. */}
        <div className="relative h-32 flex-1">
          <div className="pointer-events-none absolute inset-0">
            <div className="absolute inset-x-0 top-0 border-t border-border/60" />
            {showMid && (
              <div className="absolute inset-x-0 top-1/2 border-t border-dashed border-border/40" />
            )}
          </div>
          <div className="flex h-full items-end gap-1 border-b border-border">
            {trend.map((p) => {
              const failed = p.failed > 0
              return (
                <Tooltip key={p.day}>
                  <TooltipTrigger asChild>
                    <div
                      className={
                        failed
                          ? "flex-1 rounded-t bg-destructive/70 transition-colors hover:bg-destructive data-[state=delayed-open]:bg-destructive"
                          : "flex-1 rounded-t bg-primary/70 transition-colors hover:bg-primary data-[state=delayed-open]:bg-primary"
                      }
                      style={{ height: `${Math.max(4, (p.deploys / max) * 100)}%` }}
                    />
                  </TooltipTrigger>
                  <TooltipContent>
                    <div className="font-medium">{fmtDay(p.day)}</div>
                    <div className="opacity-80">
                      {p.deploys} deploy{p.deploys === 1 ? "" : "s"}
                      {failed && <span> · {p.failed} failed</span>}
                    </div>
                  </TooltipContent>
                </Tooltip>
              )
            })}
          </div>
        </div>
      </div>

      {/* X labels offset by the y-axis column width so they track the plot. */}
      <div className="mt-1 flex justify-between pl-7 text-xs text-muted-foreground">
        <span>{fmtDay(trend[0].day)}</span>
        <span>{fmtDay(trend[trend.length - 1].day)}</span>
      </div>
    </TooltipProvider>
  )
}

// sparklineFor builds the in-card trend for a secondary metric from the DORA
// payload, or returns null for deployment frequency (which has the big chart).
// Lead time and CFR share the dense per-day deploy series; MTTR uses the sparse
// per-incident restore series.
function sparklineFor(key: CardDef["key"], data: Dora): ReactNode {
  switch (key) {
    case "lead_time":
      return (
        <MiniSparkline
          series={data.trend.map((p) => ({ label: fmtDay(p.day), value: p.lead_seconds }))}
          format={fmtDuration}
        />
      )
    case "change_failure_rate":
      return (
        <MiniSparkline
          series={data.trend.map((p) => ({
            label: fmtDay(p.day),
            value: p.deploys > 0 ? p.failed / p.deploys : 0,
          }))}
          format={(v) => `${Math.round(v * 100)}%`}
        />
      )
    case "time_to_restore":
      return (
        <MiniSparkline
          series={data.restore_trend.map((p) => ({ label: fmtDay(p.day), value: p.seconds }))}
          format={fmtDuration}
        />
      )
    default:
      return null
  }
}

export default function MetricsPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [envId, setEnvId] = useState<string>("") // "" = all environments
  const [days, setDays] = useState<number>(30)
  // Keyed by the inputs the data was fetched for, so "is it loading?" is
  // derived rather than set synchronously inside the effect. This also drops a
  // slow earlier response that would otherwise land after a newer one and
  // overwrite it.
  const [loaded, setLoaded] = useState<{ key: string; data: Dora | null }>({ key: "", data: null })
  const dataKey = `${days}|${envId}`
  const data = loaded.key === dataKey ? loaded.data : null
  const loading = loaded.key !== dataKey

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
    let cancelled = false
    getDora(days, envId || undefined)
      .then((d) => { if (!cancelled) setLoaded({ key: dataKey, data: d }) })
      .catch((err) => {
        if (cancelled) return
        toast.error(err.message)
        setLoaded({ key: dataKey, data: null })
      })
    return () => { cancelled = true }
  }, [days, envId, dataKey])

  const scopedToEnv = envId !== ""

  return (
    <AppShell title="Metrics">
      <div>
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
                  // Deployment frequency owns the big chart below; the other
                  // three get an in-card sparkline of their own trend.
                  sparkline={sparklineFor(def.key, data)}
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
      </div>
    </AppShell>
  )
}
