"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  searchTimeline,
  getEnvironments,
  getToken,
  type SearchHit,
  type Environment,
} from "@/lib/api"
import { DotBackground } from "@/components/dot-background"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  ArrowLeft,
  Search as SearchIcon,
  Loader2,
  Rocket,
  Bell,
  StickyNote,
  Trash2,
  Undo2,
  Scaling,
  MessageCircle,
} from "lucide-react"

const TYPE_CONFIG = {
  deploy:   { icon: Rocket,     label: "Deploy",   variant: "default"     },
  alert:    { icon: Bell,       label: "Alert",    variant: "destructive" },
  note:     { icon: StickyNote, label: "Note",     variant: "secondary"   },
  teardown: { icon: Trash2,     label: "Teardown", variant: "outline"     },
  rollback: { icon: Undo2,      label: "Rollback", variant: "outline"     },
  scale:    { icon: Scaling,    label: "Scale",    variant: "secondary"   },
} as const

function fmtDate(ts: string) {
  return new Date(ts).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function SearchResult({ hit, envName }: { hit: SearchHit; envName: string }) {
  const router = useRouter()
  const { icon: Icon, label, variant } = TYPE_CONFIG[hit.event.type]

  return (
    <button
      onClick={() => router.push(`/env?id=${hit.event.environment_id}`)}
      className="hover:bg-muted/50 w-full rounded-md border px-3 py-2.5 text-left transition-colors"
    >
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <Icon className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
        <Badge variant={variant as "default" | "destructive" | "secondary" | "outline"}>
          {label}
        </Badge>
        <span className="truncate font-medium">{hit.event.service}</span>
        {hit.event.metadata?.version != null && (
          <span className="text-muted-foreground text-xs">
            {String(hit.event.metadata.version)}
          </span>
        )}
        {envName && <span className="text-muted-foreground text-xs">· {envName}</span>}
        <span className="text-muted-foreground ml-auto shrink-0 text-xs">
          {fmtDate(hit.event.timestamp)}
        </span>
      </div>

      {/* A comment match shows the comment; an event match shows what it was
          matched on, which lives in the metadata. */}
      {hit.kind === "comment" && hit.comment ? (
        <p className="text-foreground mt-1.5 flex items-start gap-1.5 text-sm">
          <MessageCircle className="text-muted-foreground mt-0.5 h-3 w-3 shrink-0" />
          <span className="line-clamp-3">{hit.comment.body}</span>
        </p>
      ) : (
        Object.keys(hit.event.metadata ?? {}).length > 0 && (
          <div className="mt-1 flex flex-wrap gap-2">
            {Object.entries(hit.event.metadata).map(([k, v]) =>
              k === "version" ? null : (
                <span key={k} className="text-muted-foreground text-xs">
                  {k}: {String(v)}
                </span>
              ),
            )}
          </div>
        )
      )}
    </button>
  )
}

function SearchInner() {
  const router = useRouter()
  const searchParams = useSearchParams()

  // The URL is the source of truth for what is displayed: submitting only
  // rewrites it, and the fetch below reacts. A pasted link and a typed query
  // therefore travel exactly the same path, and a search worth sending to a
  // colleague is always in the address bar.
  const urlQuery = (searchParams.get("q") ?? "").trim()
  const urlEnv = searchParams.get("environment_id") ?? ""
  const urlKey = `${urlQuery}|${urlEnv}`

  const [query, setQuery] = useState(urlQuery)
  const [envID, setEnvID] = useState(urlEnv)
  const [envs, setEnvs] = useState<Environment[]>([])
  // Both are keyed by the search they belong to, so "searching" is derived
  // rather than set synchronously, and a slow response for an older query
  // cannot overwrite a newer one.
  const [result, setResult] = useState<{ key: string; hits: SearchHit[] } | null>(null)
  const [failure, setFailure] = useState<{ key: string; message: string } | null>(null)

  const hits = result?.key === urlKey ? result.hits : null
  const error = failure?.key === urlKey ? failure.message : ""
  const searching = urlQuery !== "" && hits === null && !error

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments().then(setEnvs).catch(() => {})
  }, [router])

  useEffect(() => {
    if (!urlQuery) return
    let cancelled = false
    searchTimeline(urlQuery, urlEnv || undefined)
      .then((data) => {
        if (!cancelled) setResult({ key: urlKey, hits: data.results })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setFailure({
          key: urlKey,
          message: err instanceof Error ? err.message : "Search failed",
        })
      })
    return () => {
      cancelled = true
    }
  }, [urlQuery, urlEnv, urlKey])

  function submit(q: string, env: string) {
    const trimmed = q.trim()
    if (!trimmed) return
    const params = new URLSearchParams({ q: trimmed })
    if (env) params.set("environment_id", env)
    router.replace(`/search?${params}`)
  }

  const envName = (id: string) => envs.find((e) => e.id === id)?.name ?? ""

  return (
    <div className="min-h-screen px-4 py-6 sm:p-8">
      <DotBackground />
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">Search</h1>
        </div>

        <form
          onSubmit={(e) => {
            e.preventDefault()
            submit(query, envID)
          }}
          className="space-y-2"
        >
          <div className="flex gap-2">
            <Input
              autoFocus
              placeholder="Search events and comments…"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <Button type="submit" disabled={searching || !query.trim()}>
              {searching ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <SearchIcon className="h-4 w-4" />
              )}
            </Button>
          </div>
          <select
            value={envID}
            onChange={(e) => {
              setEnvID(e.target.value)
              if (urlQuery) submit(query, e.target.value)
            }}
            className="border-input bg-background h-8 rounded-md border px-2 text-xs"
          >
            <option value="">All environments</option>
            {envs.map((env) => (
              <option key={env.id} value={env.id}>
                {env.name}
              </option>
            ))}
          </select>
        </form>

        {error && <p className="text-destructive text-sm">{error}</p>}

        {hits !== null && !error && (
          <div className="space-y-2">
            <p className="text-muted-foreground text-xs">
              {hits.length === 0
                ? `No matches for "${urlQuery}".`
                : `${hits.length} match${hits.length === 1 ? "" : "es"} for "${urlQuery}"`}
            </p>
            {hits.map((hit) => (
              <SearchResult
                key={`${hit.kind}-${hit.comment?.id ?? hit.event.id}`}
                hit={hit}
                envName={envName(hit.event.environment_id)}
              />
            ))}
          </div>
        )}

        {hits === null && !error && !searching && (
          <p className="text-muted-foreground rounded-md border border-dashed px-3 py-6 text-center text-sm">
            Search deploys, alerts, notes, and every comment your team has left
            on them.
          </p>
        )}

        {searching && (
          <p className="text-muted-foreground flex items-center justify-center gap-2 rounded-md border border-dashed px-3 py-6 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            Searching…
          </p>
        )}
      </div>
    </div>
  )
}

export default function SearchPage() {
  return (
    <Suspense>
      <SearchInner />
    </Suspense>
  )
}
