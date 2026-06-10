"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { toast } from "sonner"
import {
  getIncidents,
  getIncident,
  createIncident,
  updateIncident,
  getIncidentPostmortem,
  getToken,
  getCurrentRole,
  type Incident,
  type IncidentStatus,
  type IncidentSeverity,
  type Event,
} from "@/lib/api"
import { createIncidentSchema } from "@/lib/schemas"
import { TimelineEvent } from "@/components/timeline-event"
import { DotBackground } from "@/components/dot-background"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { ArrowLeft, Plus, FileText, Loader2, RefreshCw } from "lucide-react"

const SEVERITY: Record<IncidentSeverity, { label: string; className: string }> = {
  sev1: { label: "SEV1", className: "bg-red-500/15 text-red-600 border-red-500/30" },
  sev2: { label: "SEV2", className: "bg-orange-500/15 text-orange-600 border-orange-500/30" },
  sev3: { label: "SEV3", className: "bg-amber-500/15 text-amber-600 border-amber-500/30" },
  sev4: { label: "SEV4", className: "bg-muted text-muted-foreground border-border" },
}

const STATUS: Record<IncidentStatus, { label: string; className: string; dot: string }> = {
  open:      { label: "Open",      className: "text-red-600 dark:text-red-400",     dot: "bg-red-500 animate-pulse" },
  mitigated: { label: "Mitigated", className: "text-blue-600 dark:text-blue-400",   dot: "bg-blue-500" },
  resolved:  { label: "Resolved",  className: "text-green-600 dark:text-green-400", dot: "bg-green-500" },
}

const SEVERITIES: IncidentSeverity[] = ["sev1", "sev2", "sev3", "sev4"]
const STATUSES: IncidentStatus[] = ["open", "mitigated", "resolved"]

function fmtDate(ts: string) {
  return new Date(ts).toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })
}

function canEdit() {
  const role = getCurrentRole()
  return role === "owner" || role === "admin" || role === "member"
}

// ---- List view ----

function IncidentList() {
  const router = useRouter()
  const [incidents, setIncidents] = useState<Incident[]>([])
  const [loading, setLoading] = useState(true)
  const [filter, setFilter] = useState<IncidentStatus | "">("")

  const [dialogOpen, setDialogOpen] = useState(false)
  const [title, setTitle] = useState("")
  const [severity, setSeverity] = useState<IncidentSeverity>("sev3")
  const [titleError, setTitleError] = useState("")
  const [submitting, setSubmitting] = useState(false)

  useEffect(() => {
    setLoading(true)
    getIncidents(filter || undefined)
      .then(setIncidents)
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }, [filter])

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault()
    const result = createIncidentSchema.safeParse({ title, severity })
    if (!result.success) {
      setTitleError(result.error.flatten().fieldErrors.title?.[0] ?? "Invalid")
      return
    }
    setTitleError("")
    setSubmitting(true)
    try {
      const inc = await createIncident(result.data.title, result.data.severity)
      setDialogOpen(false)
      toast.success("Incident opened")
      router.push(`/incidents?id=${inc.id}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create incident")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen px-4 py-6 sm:p-8">
      <DotBackground />
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">Incidents</h1>
          {canEdit() && (
            <Button size="sm" className="ml-auto" onClick={() => { setTitle(""); setSeverity("sev3"); setTitleError(""); setDialogOpen(true) }}>
              <Plus className="mr-1.5 h-4 w-4" />
              New incident
            </Button>
          )}
        </div>

        <div className="flex items-center rounded-md border border-border overflow-hidden w-fit">
          {([["", "All"], ...STATUSES.map((s) => [s, STATUS[s].label] as const)] as const).map(([value, label]) => (
            <button
              key={value}
              onClick={() => setFilter(value as IncidentStatus | "")}
              className={`h-7 px-3 text-xs font-medium transition-colors
                ${filter === value ? "bg-primary text-primary-foreground" : "text-muted-foreground hover:text-foreground hover:bg-muted"}`}
            >
              {label}
            </button>
          ))}
        </div>

        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="h-16 rounded-lg bg-muted animate-pulse" />
            ))}
          </div>
        ) : incidents.length === 0 ? (
          <p className="text-muted-foreground text-sm">No incidents{filter ? ` (${STATUS[filter].label.toLowerCase()})` : ""} yet.</p>
        ) : (
          <div className="space-y-2">
            {incidents.map((inc) => (
              <button
                key={inc.id}
                onClick={() => router.push(`/incidents?id=${inc.id}`)}
                className="w-full rounded-lg border bg-background px-4 py-3 text-left transition-all hover:border-border/80 hover:shadow-sm"
              >
                <div className="flex items-center gap-2.5">
                  <span className={`h-2 w-2 shrink-0 rounded-full ${STATUS[inc.status].dot}`} />
                  <span className="font-medium truncate">{inc.title}</span>
                  <span className={`ml-auto shrink-0 rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium ${SEVERITY[inc.severity].className}`}>
                    {SEVERITY[inc.severity].label}
                  </span>
                </div>
                <div className="mt-1.5 flex items-center gap-2 pl-[18px] text-xs text-muted-foreground">
                  <span className={STATUS[inc.status].className}>{STATUS[inc.status].label}</span>
                  <span>·</span>
                  <span>opened {fmtDate(inc.opened_at)}</span>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New incident</DialogTitle>
          </DialogHeader>
          <form id="new-incident-form" onSubmit={handleCreate} className="space-y-4 py-2">
            <div className="space-y-1">
              <Label htmlFor="incident-title">Title</Label>
              <Input
                id="incident-title"
                placeholder="Elevated 5xx on api-service"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                autoFocus
              />
              {titleError && <p className="text-destructive text-xs">{titleError}</p>}
            </div>
            <div className="space-y-1">
              <Label>Severity</Label>
              <div className="flex gap-2">
                {SEVERITIES.map((s) => (
                  <Button
                    key={s}
                    type="button"
                    size="sm"
                    variant={severity === s ? "default" : "outline"}
                    onClick={() => setSeverity(s)}
                  >
                    {SEVERITY[s].label}
                  </Button>
                ))}
              </div>
            </div>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button type="submit" form="new-incident-form" disabled={submitting}>
              {submitting ? "Opening…" : "Open incident"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

// ---- Detail view ----

function IncidentDetail({ id }: { id: string }) {
  const router = useRouter()
  const [incident, setIncident] = useState<Incident | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [updating, setUpdating] = useState(false)
  const [postmortem, setPostmortem] = useState<string | null>(null)
  const [postmortemOpen, setPostmortemOpen] = useState(false)
  const [postmortemLoading, setPostmortemLoading] = useState(false)
  const [postmortemError, setPostmortemError] = useState<string | null>(null)

  useEffect(() => {
    setLoading(true)
    getIncident(id)
      .then(({ incident, events }) => { setIncident(incident); setEvents(events) })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  }, [id])

  async function setStatus(status: IncidentStatus) {
    if (!incident || incident.status === status) return
    setUpdating(true)
    try {
      const updated = await updateIncident(id, { status })
      setIncident(updated)
      toast.success(`Marked ${STATUS[status].label.toLowerCase()}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update incident")
    } finally {
      setUpdating(false)
    }
  }

  async function handlePostmortem(refresh = false) {
    if (postmortem && !refresh) { setPostmortemOpen(true); return }
    setPostmortemLoading(true)
    setPostmortemError(null)
    try {
      const text = await getIncidentPostmortem(id, refresh)
      setPostmortem(text)
      setPostmortemOpen(true)
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to generate postmortem"
      setPostmortemError(msg.includes("402") || msg.toLowerCase().includes("upgrade") ? "upgrade" : msg)
      setPostmortemOpen(true)
    } finally {
      setPostmortemLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="min-h-screen px-4 py-6 sm:p-8">
        <DotBackground />
        <div className="mx-auto max-w-2xl space-y-4">
          <div className="h-8 w-1/2 rounded bg-muted animate-pulse" />
          <div className="h-24 rounded-lg bg-muted animate-pulse" />
        </div>
      </div>
    )
  }

  if (error || !incident) {
    return (
      <div className="min-h-screen px-4 py-6 sm:p-8">
        <DotBackground />
        <div className="mx-auto max-w-2xl space-y-4">
          <Button variant="ghost" size="sm" onClick={() => router.push("/incidents")}>
            <ArrowLeft className="mr-1.5 h-4 w-4" /> Incidents
          </Button>
          <p className="text-destructive text-sm">{error || "Incident not found"}</p>
        </div>
      </div>
    )
  }

  const editable = canEdit()

  return (
    <div className="min-h-screen px-4 py-6 sm:p-8">
      <DotBackground />
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/incidents")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold truncate">{incident.title}</h1>
          <span className={`ml-auto shrink-0 rounded border px-2 py-0.5 text-xs font-medium ${SEVERITY[incident.severity].className}`}>
            {SEVERITY[incident.severity].label}
          </span>
        </div>

        <div className="rounded-lg border bg-background px-4 py-3 space-y-3">
          <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <span className={`flex items-center gap-1.5 font-medium ${STATUS[incident.status].className}`}>
              <span className={`h-1.5 w-1.5 rounded-full ${STATUS[incident.status].dot}`} />
              {STATUS[incident.status].label}
            </span>
            <span>Opened {fmtDate(incident.opened_at)}</span>
            {incident.resolved_at && <span>Resolved {fmtDate(incident.resolved_at)}</span>}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {editable && STATUSES.map((s) => (
              <Button
                key={s}
                size="sm"
                variant={incident.status === s ? "default" : "outline"}
                disabled={updating}
                onClick={() => setStatus(s)}
              >
                {STATUS[s].label}
              </Button>
            ))}
            <Button
              size="sm"
              variant="outline"
              className={editable ? "ml-auto" : ""}
              disabled={postmortemLoading}
              onClick={() => handlePostmortem()}
            >
              {postmortemLoading
                ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                : <FileText className="mr-1.5 h-4 w-4" />}
              {postmortemLoading ? "Generating…" : "Postmortem"}
            </Button>
          </div>
        </div>

        <div>
          <h2 className="mb-3 text-[11px] font-semibold uppercase tracking-widest text-muted-foreground/60">
            Linked events ({events.length})
          </h2>
          {events.length === 0 ? (
            <p className="text-muted-foreground text-sm">
              No events linked yet. Use “Attach to incident” on a timeline event.
            </p>
          ) : (
            <div className="relative">
              <div className="absolute left-4 inset-y-0 w-px bg-border" />
              {events.map((event) => (
                <div key={event.id} className="mb-5">
                  <TimelineEvent event={event} />
                </div>
              ))}
            </div>
          )}
        </div>
        <Dialog open={postmortemOpen} onOpenChange={setPostmortemOpen}>
          <DialogContent className="max-w-[calc(100vw-2rem)] sm:max-w-2xl max-h-[80vh] overflow-y-auto">
            <DialogHeader>
              <DialogTitle className="flex items-center gap-2">
                <FileText className="h-4 w-4" />
                Postmortem — {incident.title}
              </DialogTitle>
            </DialogHeader>
            {postmortemError === "upgrade" ? (
              <p className="text-muted-foreground text-sm">
                AI postmortems are available on the{" "}
                <span className="text-foreground font-medium">Pro plan</span>.{" "}
                <a href="/settings/billing" className="text-primary underline underline-offset-2">Upgrade</a>
              </p>
            ) : postmortemError ? (
              <p className="text-destructive text-sm">{postmortemError}</p>
            ) : (
              <>
                <pre className="text-sm leading-relaxed whitespace-pre-wrap font-sans">{postmortem}</pre>
                <button
                  onClick={() => handlePostmortem(true)}
                  disabled={postmortemLoading}
                  className="text-muted-foreground hover:text-foreground mt-2 flex items-center gap-1 text-xs transition-colors disabled:opacity-50"
                >
                  {postmortemLoading
                    ? <Loader2 className="h-3 w-3 animate-spin" />
                    : <RefreshCw className="h-3 w-3" />}
                  {postmortemLoading ? "Regenerating…" : "Regenerate"}
                </button>
              </>
            )}
          </DialogContent>
        </Dialog>
      </div>
    </div>
  )
}

function IncidentsInner() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const id = searchParams.get("id")

  useEffect(() => {
    if (!getToken()) router.replace("/login")
  }, [router])

  return id ? <IncidentDetail id={id} /> : <IncidentList />
}

export default function IncidentsPage() {
  return <Suspense><IncidentsInner /></Suspense>
}
