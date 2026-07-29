"use client"

import { Suspense, useCallback, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  listDecisions,
  getEnvironments,
  getToken,
  type Decision,
  type Environment,
} from "@/lib/api"
import { AppShell } from "@/components/app-shell"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Bookmark,
  Loader2,
  Rocket,
  Bell,
  StickyNote,
  Trash2,
  Undo2,
  Scaling,
} from "lucide-react"

const TYPE_CONFIG = {
  deploy:   { icon: Rocket,     label: "Deploy",   variant: "default"     },
  alert:    { icon: Bell,       label: "Alert",    variant: "destructive" },
  note:     { icon: StickyNote, label: "Note",     variant: "secondary"   },
  teardown: { icon: Trash2,     label: "Teardown", variant: "outline"     },
  rollback: { icon: Undo2,      label: "Rollback", variant: "outline"     },
  scale:    { icon: Scaling,    label: "Scale",    variant: "secondary"   },
} as const

const PAGE_SIZE = 50

function fmtDate(ts: string) {
  return new Date(ts).toLocaleString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function DecisionRow({ decision }: { decision: Decision }) {
  const router = useRouter()
  const config = TYPE_CONFIG[decision.event_type as keyof typeof TYPE_CONFIG]
  const Icon = config?.icon ?? StickyNote

  return (
    <button
      onClick={() => router.push(`/env?id=${decision.environment_id}`)}
      className="hover:bg-muted/50 w-full rounded-md border px-3 py-2.5 text-left transition-colors"
    >
      {/* The decision text leads. The event it was made about is context, and
          context belongs underneath the thing it explains. */}
      <p className="flex items-start gap-1.5 text-sm">
        <Bookmark className="text-primary mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span className="min-w-0 break-words">{decision.body}</span>
      </p>

      <div className="text-muted-foreground mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs">
        <Icon className="h-3.5 w-3.5 shrink-0" />
        {config && (
          <Badge
            variant={config.variant as "default" | "destructive" | "secondary" | "outline"}
          >
            {config.label}
          </Badge>
        )}
        <span className="truncate font-medium">{decision.event_service}</span>
        <span>· {decision.environment_name}</span>
        <span className="ml-auto shrink-0">
          {decision.author} · {fmtDate(decision.created_at)}
        </span>
      </div>
    </button>
  )
}

function DecisionsInner() {
  const router = useRouter()
  const searchParams = useSearchParams()

  // The environment filter lives in the URL so a filtered log is a shareable
  // link, the same way search works.
  const urlEnv = searchParams.get("environment_id") ?? ""

  const [envs, setEnvs] = useState<Environment[]>([])
  // Both are keyed by the filter they belong to, so switching environments does
  // not need a synchronous reset in the effect, and a slow response for the
  // previous filter cannot overwrite the current one.
  const [result, setResult] = useState<{ key: string; decisions: Decision[]; total: number } | null>(null)
  const [failure, setFailure] = useState<{ key: string; message: string } | null>(null)
  const [loadingMore, setLoadingMore] = useState(false)

  const decisions = result?.key === urlEnv ? result.decisions : null
  const total = result?.key === urlEnv ? result.total : 0
  const error = failure?.key === urlEnv ? failure.message : ""

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments().then(setEnvs).catch(() => {})
  }, [router])

  useEffect(() => {
    let cancelled = false
    listDecisions(urlEnv || undefined, PAGE_SIZE, 0)
      .then((data) => {
        if (cancelled) return
        setResult({ key: urlEnv, decisions: data.decisions, total: data.total })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setFailure({
          key: urlEnv,
          message: err instanceof Error ? err.message : "Could not load decisions",
        })
      })
    return () => {
      cancelled = true
    }
  }, [urlEnv])

  const loadMore = useCallback(async () => {
    if (!decisions) return
    setLoadingMore(true)
    try {
      const data = await listDecisions(urlEnv || undefined, PAGE_SIZE, decisions.length)
      setResult((prev) =>
        prev?.key === urlEnv
          ? { key: urlEnv, decisions: [...prev.decisions, ...data.decisions], total: data.total }
          : prev,
      )
    } catch (err) {
      setFailure({
        key: urlEnv,
        message: err instanceof Error ? err.message : "Could not load more",
      })
    } finally {
      setLoadingMore(false)
    }
  }, [decisions, urlEnv])

  function setEnv(id: string) {
    const params = new URLSearchParams()
    if (id) params.set("environment_id", id)
    router.replace(params.toString() ? `/decisions?${params}` : "/decisions")
  }

  return (
    <div className="max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <h1 className="text-xl font-semibold">Decisions</h1>
          {decisions !== null && (
            <span className="text-muted-foreground ml-auto text-xs">
              {total} {total === 1 ? "decision" : "decisions"}
            </span>
          )}
        </div>

        <select
          value={urlEnv}
          onChange={(e) => setEnv(e.target.value)}
          className="border-input bg-background h-8 rounded-md border px-2 text-xs"
        >
          <option value="">All environments</option>
          {envs.map((env) => (
            <option key={env.id} value={env.id}>
              {env.name}
            </option>
          ))}
        </select>

        {error && <p className="text-destructive text-sm">{error}</p>}

        {decisions === null && !error && (
          <div className="text-muted-foreground flex items-center gap-2 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading…
          </div>
        )}

        {decisions !== null && decisions.length === 0 && (
          <p className="text-muted-foreground rounded-md border border-dashed px-3 py-6 text-center text-sm">
            {/* Explicit {" "} on both sides: JSX strips the leading and
                trailing whitespace of a text node that spans lines, so a space
                written next to the tag disappears at build time. */}
            No decisions yet. Open a comment thread on the timeline and choose{" "}
            <strong className="text-foreground">Mark as decision</strong>{" "}
            to record what the team settled on — &ldquo;we&rsquo;re rolling
            back&rdquo;, &ldquo;accepting this risk until Q3&rdquo;.
          </p>
        )}

        {decisions !== null && decisions.length > 0 && (
          <div className="space-y-2">
            {decisions.map((d) => (
              <DecisionRow key={d.id} decision={d} />
            ))}
            {decisions.length < total && (
              <Button
                variant="outline"
                size="sm"
                onClick={loadMore}
                disabled={loadingMore}
                className="w-full"
              >
                {loadingMore ? <Loader2 className="h-4 w-4 animate-spin" /> : "Load more"}
              </Button>
            )}
          </div>
        )}
    </div>
  )
}

export default function DecisionsPage() {
  // The shell sits outside Suspense so the nav is in the prerendered
  // HTML. Inside it, useSearchParams forces this subtree client-only and
  // the nav would pop in after hydration.
  return (
    <AppShell>
      <Suspense fallback={null}>
        <DecisionsInner />
      </Suspense>
    </AppShell>
  )
}
