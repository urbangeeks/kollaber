"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getToken,
  getCurrentRole,
  getEnvironments,
  listFreezes,
  createFreeze,
  deleteFreeze,
  type Environment,
  type FreezeWindow,
} from "@/lib/api"
import { useClientValue } from "@/hooks/use-client-value"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Badge } from "@/components/ui/badge"
import { Snowflake, Trash2 } from "lucide-react"

function toLocalInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

function fmt(ts: string) {
  return new Date(ts).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export default function FreezesSettingsPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [freezes, setFreezes] = useState<FreezeWindow[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const now = new Date()
  const [reason, setReason] = useState("")
  const [envID, setEnvID] = useState("")
  const [startsAt, setStartsAt] = useState(toLocalInput(now))
  const [endsAt, setEndsAt] = useState(
    toLocalInput(new Date(now.getTime() + 24 * 60 * 60 * 1000)),
  )

  const canEdit = useClientValue(() => {
    const role = getCurrentRole()
    return role === "owner" || role === "admin"
  }, false)

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments().then(setEnvs).catch(() => {})
    listFreezes()
      .then(setFreezes)
      .catch(() => toast.error("Failed to load freeze windows"))
      .finally(() => setLoading(false))
  }, [router])

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    if (!reason.trim()) {
      toast.error("Give the freeze a reason — it is what people will see")
      return
    }
    if (new Date(startsAt) >= new Date(endsAt)) {
      toast.error("The freeze must end after it starts")
      return
    }
    setSaving(true)
    try {
      const created = await createFreeze(
        reason.trim(),
        new Date(startsAt).toISOString(),
        new Date(endsAt).toISOString(),
        envID || undefined,
      )
      setFreezes((prev) => [created, ...prev])
      setReason("")
      toast.success("Freeze window declared")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not declare the freeze")
    } finally {
      setSaving(false)
    }
  }

  async function remove(id: string) {
    try {
      await deleteFreeze(id)
      setFreezes((prev) => prev.filter((f) => f.id !== id))
      toast.success("Freeze window removed")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not remove the freeze")
    }
  }

  const envName = (id?: string | null) =>
    id ? (envs.find((e) => e.id === id)?.name ?? "unknown") : "All environments"

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Snowflake className="h-4 w-4" />
            Change freezes
          </CardTitle>
          <p className="text-muted-foreground text-sm">
            Declare a period when the team would rather nothing changed — Black Friday,
            quarter end, the week a migration lands. Kollaber does not block anything:
            deploys that land inside a freeze are recorded as having done so, and{" "}
            <code className="bg-muted rounded px-1 py-0.5 font-mono text-xs">
              kollaber deploy
            </code>{" "}
            exits non-zero so CI can decide.
          </p>
        </CardHeader>

        {canEdit && (
          <CardContent>
            <form onSubmit={submit} className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="reason" className="text-xs">
                  Reason
                </Label>
                <Input
                  id="reason"
                  placeholder="Black Friday"
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
              </div>
              <div className="flex flex-wrap items-end gap-3">
                <div className="space-y-1">
                  <Label className="text-xs">Scope</Label>
                  <select
                    value={envID}
                    onChange={(e) => setEnvID(e.target.value)}
                    className="border-input bg-background h-9 rounded-md border px-2 text-sm"
                  >
                    <option value="">All environments</option>
                    {envs.map((env) => (
                      <option key={env.id} value={env.id}>
                        {env.name}
                      </option>
                    ))}
                  </select>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="from" className="text-xs">
                    From
                  </Label>
                  <Input
                    id="from"
                    type="datetime-local"
                    value={startsAt}
                    onChange={(e) => setStartsAt(e.target.value)}
                    className="w-auto"
                  />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="to" className="text-xs">
                    Until
                  </Label>
                  <Input
                    id="to"
                    type="datetime-local"
                    value={endsAt}
                    onChange={(e) => setEndsAt(e.target.value)}
                    className="w-auto"
                  />
                </div>
                <Button type="submit" disabled={saving}>
                  Declare freeze
                </Button>
              </div>
            </form>
          </CardContent>
        )}
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Declared windows</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <p className="text-muted-foreground text-sm">Loading…</p>
          ) : freezes.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No freeze windows declared.
            </p>
          ) : (
            <div className="space-y-2">
              {freezes.map((f) => (
                <div
                  key={f.id}
                  className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border px-3 py-2.5 text-sm"
                >
                  <span className="font-medium">{f.reason}</span>
                  {f.active && (
                    <Badge variant="destructive" className="gap-1">
                      <Snowflake className="h-3 w-3" />
                      active
                    </Badge>
                  )}
                  <span className="text-muted-foreground text-xs">
                    {envName(f.environment_id)}
                  </span>
                  <span className="text-muted-foreground text-xs">
                    {fmt(f.starts_at)} → {fmt(f.ends_at)}
                  </span>
                  {canEdit && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="ml-auto"
                      title="Remove this window"
                      onClick={() => remove(f.id)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}
          {/* Removing a window does not unmark changes that already landed
              inside it — those recorded what was true when they happened. */}
          <p className="text-muted-foreground mt-3 text-xs">
            Removing a window does not unmark deploys that already landed inside it.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
