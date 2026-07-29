"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  getInventory,
  getEnvironments,
  getToken,
  type Inventory,
  type Environment,
} from "@/lib/api"
import { DotBackground } from "@/components/dot-background"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft, Boxes, Loader2, Undo2 } from "lucide-react"

// datetime-local wants "YYYY-MM-DDTHH:mm" in local time; the API wants RFC3339.
function toLocalInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function fmtDate(ts: string) {
  return new Date(ts).toLocaleString([], {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

function InventoryInner() {
  const router = useRouter()
  const searchParams = useSearchParams()

  // Both the environment and the instant live in the URL, so "what was in prod
  // when this broke?" is a link you can paste into an incident thread.
  const urlEnv = searchParams.get("environment_id") ?? ""
  const urlAt = searchParams.get("at") ?? ""
  const urlKey = `${urlEnv}|${urlAt}`

  const [envs, setEnvs] = useState<Environment[]>([])
  const [atInput, setAtInput] = useState(urlAt ? toLocalInput(new Date(urlAt)) : "")
  const [result, setResult] = useState<{ key: string; data: Inventory } | null>(null)
  const [failure, setFailure] = useState<{ key: string; message: string } | null>(null)

  const inventory = result?.key === urlKey ? result.data : null
  const error = failure?.key === urlKey ? failure.message : ""
  const loading = urlEnv !== "" && inventory === null && !error

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments().then(setEnvs).catch(() => {})
  }, [router])

  useEffect(() => {
    if (!urlEnv) return
    let cancelled = false
    getInventory(urlEnv, urlAt || undefined)
      .then((data) => {
        if (!cancelled) setResult({ key: urlKey, data })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setFailure({
          key: urlKey,
          message: err instanceof Error ? err.message : "Could not load the inventory",
        })
      })
    return () => {
      cancelled = true
    }
  }, [urlEnv, urlAt, urlKey])

  function navigate(env: string, at: string) {
    if (!env) {
      router.replace("/inventory")
      return
    }
    const params = new URLSearchParams({ environment_id: env })
    if (at) params.set("at", new Date(at).toISOString())
    router.replace(`/inventory?${params}`)
  }

  return (
    <div className="min-h-screen px-4 py-6 sm:p-8">
      <DotBackground />
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">Inventory</h1>
        </div>

        <p className="text-muted-foreground text-sm">
          What each service was running at a moment in time, derived from your deploy
          history. Leave the time blank for right now.
        </p>

        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label className="text-xs">Environment</Label>
            <select
              value={urlEnv}
              onChange={(e) => navigate(e.target.value, atInput)}
              className="border-input bg-background h-8 rounded-md border px-2 text-xs"
            >
              <option value="">Select an environment…</option>
              {envs.map((env) => (
                <option key={env.id} value={env.id}>
                  {env.name}
                </option>
              ))}
            </select>
          </div>
          <div className="space-y-1">
            <Label htmlFor="inv-at" className="text-xs">
              At
            </Label>
            <Input
              id="inv-at"
              type="datetime-local"
              value={atInput}
              onChange={(e) => setAtInput(e.target.value)}
              className="h-8 w-auto text-xs"
            />
          </div>
          <Button size="sm" disabled={!urlEnv} onClick={() => navigate(urlEnv, atInput)}>
            {loading ? <Loader2 className="h-4 w-4 animate-spin" /> : "Show"}
          </Button>
          {atInput && (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => {
                setAtInput("")
                navigate(urlEnv, "")
              }}
            >
              Now
            </Button>
          )}
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        {!urlEnv && (
          <p className="text-muted-foreground rounded-md border border-dashed px-3 py-6 text-center text-sm">
            Pick an environment to see what it was running.
          </p>
        )}

        {inventory && (
          <div className="space-y-3">
            <p className="text-muted-foreground text-xs">
              {inventory.environment_name} · as of {fmtDate(inventory.at)}
            </p>

            {inventory.services.length === 0 ? (
              <p className="text-muted-foreground rounded-md border border-dashed px-3 py-6 text-center text-sm">
                No successful deploys on record at that time.
              </p>
            ) : (
              <div className="space-y-2">
                {inventory.services.map((s) => (
                  <div
                    key={s.service}
                    className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border px-3 py-2.5 text-sm"
                  >
                    <Boxes className="text-muted-foreground h-3.5 w-3.5 shrink-0" />
                    <span className="font-medium">{s.service}</span>
                    {s.version ? (
                      <code className="bg-muted rounded px-1.5 py-0.5 font-mono text-xs">
                        {s.version}
                      </code>
                    ) : (
                      <span className="text-muted-foreground text-xs italic">
                        version not recorded
                      </span>
                    )}
                    {/* A rollback is worth flagging: what is running is not the
                        newest thing anyone shipped. */}
                    {s.event_type === "rollback" && (
                      <Badge variant="outline" className="gap-1">
                        <Undo2 className="h-3 w-3" />
                        rolled back
                      </Badge>
                    )}
                    <span className="text-muted-foreground ml-auto shrink-0 text-xs">
                      {fmtDate(s.deployed_at)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default function InventoryPage() {
  return (
    <Suspense fallback={null}>
      <InventoryInner />
    </Suspense>
  )
}
