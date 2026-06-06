"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { toast } from "sonner"
import { getEvents, createEvent, getServices, getToken, type Event, type EventFilters } from "@/lib/api"
import { createEventSchema } from "@/lib/schemas"
import { useEventStream } from "@/hooks/use-event-stream"
import { TimelineEvent } from "@/components/timeline-event"
import { AgentChat } from "@/components/agent-chat"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { ArrowLeft, Plus } from "lucide-react"
import { DotBackground } from "@/components/dot-background"

function groupByDate(events: Event[]): { label: string; events: Event[] }[] {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const yesterday = new Date(today)
  yesterday.setDate(yesterday.getDate() - 1)
  const map = new Map<string, Event[]>()
  for (const event of events) {
    const d = new Date(event.timestamp)
    const day = new Date(d.getFullYear(), d.getMonth(), d.getDate())
    let label: string
    if (day.getTime() === today.getTime()) label = "Today"
    else if (day.getTime() === yesterday.getTime()) label = "Yesterday"
    else label = day.toLocaleDateString([], { weekday: "long", month: "short", day: "numeric" })
    const bucket = map.get(label)
    if (bucket) bucket.push(event)
    else map.set(label, [event])
  }
  return Array.from(map.entries()).map(([label, events]) => ({ label, events }))
}

// Teardown is emitted by kube-watcher, not created by hand — so it's filterable
// but not a creatable type.
type CreatableEventType = "deploy" | "alert" | "note"
type EventType = CreatableEventType | "teardown"
type EventStatus = "success" | "failure" | "in_progress"

// Creatable types — shown in the "New event" form.
const EVENT_TYPES: { value: CreatableEventType; label: string }[] = [
  { value: "deploy", label: "Deploy" },
  { value: "alert", label: "Alert" },
  { value: "note",  label: "Note"   },
]

// Filterable types — includes teardown so watcher-emitted teardowns can be
// filtered on the timeline.
const FILTER_TYPES: { value: EventType; label: string }[] = [
  ...EVENT_TYPES,
  { value: "teardown", label: "Teardown" },
]

const EVENT_STATUSES: { value: EventStatus; label: string }[] = [
  { value: "success",     label: "Success"     },
  { value: "failure",     label: "Failure"     },
  { value: "in_progress", label: "In Progress" },
]

function EnvPageInner() {
  const searchParams = useSearchParams()
  const id = searchParams.get("id") ?? ""
  const router = useRouter()
  const [topEvents, setTopEvents] = useState<Event[]>([])
  const [moreEvents, setMoreEvents] = useState<Event[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  const PAGE_SIZE = 50
  const allEvents = [...topEvents, ...moreEvents]

  const [dialogOpen, setDialogOpen] = useState(false)
  const [type, setType] = useState<CreatableEventType>("note")
  const [service, setService] = useState("")
  const [version, setVersion] = useState("")
  const [message, setMessage] = useState("")
  const [status, setStatus] = useState<EventStatus>("success")
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)

  const [filterType, setFilterType] = useState("")
  const [filterStatus, setFilterStatus] = useState("")
  const [filterService, setFilterService] = useState("")
  const [filterAfter, setFilterAfter] = useState("")
  const [filterBefore, setFilterBefore] = useState("")
  const [knownServices, setKnownServices] = useState<string[]>([])

  const PRESETS = [
    { label: "Today",   after: () => startOfDay(new Date()) },
    { label: "7d",      after: () => daysAgo(7) },
    { label: "30d",     after: () => daysAgo(30) },
  ]

  function startOfDay(d: Date) {
    d.setHours(0, 0, 0, 0)
    return d.toISOString()
  }
  function daysAgo(n: number) {
    const d = new Date()
    d.setDate(d.getDate() - n)
    d.setHours(0, 0, 0, 0)
    return d.toISOString()
  }
  function toISODate(iso: string) {
    return iso ? iso.slice(0, 10) : ""
  }
  function dateInputToISO(val: string) {
    return val ? new Date(val).toISOString() : ""
  }

  const isAuthed = Boolean(getToken())

  const activeFilters: EventFilters = {}
  if (filterType)    activeFilters.type    = filterType
  if (filterStatus)  activeFilters.status  = filterStatus
  if (filterService) activeFilters.service = filterService
  if (filterAfter)   activeFilters.after   = filterAfter
  if (filterBefore)  activeFilters.before  = filterBefore

  // Immediately re-fetch and show skeleton whenever filters change.
  useEffect(() => {
    if (!isAuthed || !id) return
    setMoreEvents([])
    setHasMore(false)
    setLoading(true)
    setError("")
    getEvents(id, PAGE_SIZE, 0, activeFilters)
      .then((evts) => {
        setTopEvents(evts)
        setHasMore(evts.length === PAGE_SIZE)
        getServices(id).then(setKnownServices).catch(() => {})
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false))
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, filterType, filterStatus, filterService, filterAfter, filterBefore])

  // Real-time updates via SSE — prepend new events as they arrive.
  useEventStream(
    id,
    (data) => {
      const event = data as Event
      if (filterType && event.type !== filterType) return
      if (filterStatus && event.status !== filterStatus) return
      if (filterService && event.service !== filterService) return
      if (filterAfter && new Date(event.timestamp) < new Date(filterAfter)) return
      if (filterBefore && new Date(event.timestamp) > new Date(filterBefore)) return
      setTopEvents((prev) => {
        if (prev.some((e) => e.id === event.id)) return prev
        return [event, ...prev]
      })
      if (!knownServices.includes(event.service)) {
        setKnownServices((prev) => [...prev, event.service])
      }
    },
    isAuthed && Boolean(id),
  )

  async function handleLoadMore() {
    const cursor = allEvents[allEvents.length - 1]?.timestamp
    if (!cursor) return
    setLoadingMore(true)
    try {
      const older = await getEvents(id, PAGE_SIZE, 0, { ...activeFilters, before: cursor })
      setMoreEvents((prev) => [...prev, ...older])
      setHasMore(older.length === PAGE_SIZE)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to load more")
    } finally {
      setLoadingMore(false)
    }
  }

  function openDialog() {
    setType("note")
    setService("")
    setVersion("")
    setMessage("")
    setStatus("success")
    setFieldErrors({})
    setDialogOpen(true)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const result = createEventSchema.safeParse({ type, service, version, message, status })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setFieldErrors(Object.fromEntries(Object.entries(flat).map(([k, v]) => [k, v?.[0] ?? ""])))
      return
    }
    setFieldErrors({})
    setSubmitting(true)

    const metadata: Record<string, string> = {}
    if (type === "deploy" && result.data.version) metadata.version = result.data.version
    if ((type === "note" || type === "alert") && result.data.message) metadata.body = result.data.message

    try {
      const event = await createEvent(id, type, result.data.service, metadata, result.data.status)
      setTopEvents((prev) => [event, ...prev])
      setDialogOpen(false)
      toast.success(`${type.charAt(0).toUpperCase() + type.slice(1)} event created`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create event")
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
          <h1 className="text-xl font-semibold">Timeline</h1>
          <div className="ml-auto flex items-center gap-3">
            <span className="text-muted-foreground flex items-center gap-1.5 text-xs">
              <span className="h-1.5 w-1.5 rounded-full bg-green-500 animate-pulse" />
              Live
            </span>
            <Button size="sm" onClick={openDialog}>
              <Plus className="mr-1.5 h-4 w-4" />
              New event
            </Button>
          </div>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <div className="flex flex-wrap items-center gap-x-1 gap-y-2">
          {/* Type group */}
          <div className="flex items-center rounded-md border border-border overflow-hidden">
            {FILTER_TYPES.map((t) => (
              <button
                key={t.value}
                onClick={() => setFilterType(filterType === t.value ? "" : t.value)}
                className={`h-7 px-3 text-xs font-medium transition-colors
                  ${filterType === t.value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                  }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className="h-4 w-px bg-border mx-1" />

          {/* Status group */}
          <div className="flex items-center rounded-md border border-border overflow-hidden">
            {EVENT_STATUSES.map((s) => (
              <button
                key={s.value}
                onClick={() => setFilterStatus(filterStatus === s.value ? "" : s.value)}
                className={`h-7 px-3 text-xs font-medium transition-colors
                  ${filterStatus === s.value
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:text-foreground hover:bg-muted"
                  }`}
              >
                {s.label}
              </button>
            ))}
          </div>

          {/* Service dropdown */}
          {knownServices.length > 0 && (
            <>
              <div className="h-4 w-px bg-border mx-1" />
              <select
                value={filterService}
                onChange={(e) => setFilterService(e.target.value)}
                className={`h-7 rounded-md border border-border bg-background px-2 text-xs font-medium transition-colors cursor-pointer
                  ${filterService ? "text-foreground" : "text-muted-foreground"}`}
              >
                <option value="">All services</option>
                {knownServices.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </>
          )}

          <div className="h-4 w-px bg-border mx-1" />

          {/* Date range */}
          <div className="flex flex-wrap items-center gap-1">
            {PRESETS.map((p) => {
              const active = filterAfter === p.after() || (filterAfter && !filterBefore &&
                Math.abs(new Date(filterAfter).getTime() - new Date(p.after()).getTime()) < 60000)
              return (
                <button
                  key={p.label}
                  onClick={() => { setFilterAfter(p.after()); setFilterBefore("") }}
                  className={`h-7 px-3 rounded-md text-xs font-medium transition-colors
                    ${filterAfter === p.after()
                      ? "bg-primary text-primary-foreground"
                      : "text-muted-foreground hover:text-foreground hover:bg-muted border border-border"
                    }`}
                >
                  {p.label}
                </button>
              )
            })}
            <input
              type="date"
              value={toISODate(filterAfter)}
              onChange={(e) => setFilterAfter(dateInputToISO(e.target.value))}
              className="h-7 rounded-md border border-border bg-background px-2 text-xs text-muted-foreground cursor-pointer"
              title="From"
            />
            <span className="text-muted-foreground text-xs">→</span>
            <input
              type="date"
              value={toISODate(filterBefore)}
              onChange={(e) => setFilterBefore(dateInputToISO(e.target.value))}
              className="h-7 rounded-md border border-border bg-background px-2 text-xs text-muted-foreground cursor-pointer"
              title="To"
            />
          </div>

          {/* Clear */}
          {(filterType || filterStatus || filterService || filterAfter || filterBefore) && (
            <>
              <div className="h-4 w-px bg-border mx-1" />
              <button
                onClick={() => { setFilterType(""); setFilterStatus(""); setFilterService(""); setFilterAfter(""); setFilterBefore("") }}
                className="h-7 px-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                Clear
              </button>
            </>
          )}
        </div>

        <div>
          {loading ? (
            <div className="relative">
              <div className="absolute left-4 inset-y-0 w-px bg-border" />
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="flex items-start gap-3 mb-6 animate-pulse">
                  <div className="h-8 w-8 shrink-0 rounded-full bg-muted relative z-10" />
                  <div className="space-y-1.5 flex-1 pt-1.5">
                    <div className="h-3 w-1/3 rounded bg-muted" />
                    <div className="h-3 w-1/2 rounded bg-muted" />
                  </div>
                </div>
              ))}
            </div>
          ) : allEvents.length === 0 && !error ? (
            <p className="text-muted-foreground text-sm">No events yet.</p>
          ) : (
            <div className="relative">
              <div className="absolute left-4 inset-y-0 w-px bg-border" />
              {groupByDate(allEvents).map((group) => (
                <div key={group.label}>
                  <div className="relative flex items-center gap-3 mb-3 mt-1">
                    <div className="w-8 shrink-0" />
                    <span className="text-[11px] font-semibold uppercase tracking-widest text-muted-foreground/60">
                      {group.label}
                    </span>
                  </div>
                  {group.events.map((event) => (
                    <div key={event.id} className="mb-5">
                      <TimelineEvent event={event} />
                    </div>
                  ))}
                </div>
              ))}
              {hasMore && (
                <div className="pl-11 pt-2">
                  <Button variant="outline" size="sm" onClick={handleLoadMore} disabled={loadingMore}>
                    {loadingMore ? "Loading…" : "Load more"}
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New event</DialogTitle>
          </DialogHeader>
          <form id="new-event-form" onSubmit={handleSubmit} className="space-y-4 py-2">
            <div className="space-y-1">
              <Label>Type</Label>
              <div className="flex gap-2">
                {EVENT_TYPES.map((t) => (
                  <Button
                    key={t.value}
                    type="button"
                    size="sm"
                    variant={type === t.value ? "default" : "outline"}
                    onClick={() => setType(t.value)}
                  >
                    {t.label}
                  </Button>
                ))}
              </div>
            </div>

            <div className="space-y-1">
              <Label htmlFor="event-service">Service</Label>
              <Input
                id="event-service"
                list="known-services"
                placeholder="api-service"
                value={service}
                onChange={(e) => setService(e.target.value)}
                autoFocus
              />
              {knownServices.length > 0 && (
                <datalist id="known-services">
                  {knownServices.map((s) => <option key={s} value={s} />)}
                </datalist>
              )}
              {fieldErrors.service && <p className="text-destructive text-xs">{fieldErrors.service}</p>}
            </div>

            {type === "deploy" && (
              <div className="space-y-1">
                <Label htmlFor="event-version">Version</Label>
                <Input
                  id="event-version"
                  placeholder="v1.2.3"
                  value={version}
                  onChange={(e) => setVersion(e.target.value)}
                />
                {fieldErrors.version && <p className="text-destructive text-xs">{fieldErrors.version}</p>}
              </div>
            )}

            {(type === "note" || type === "alert") && (
              <div className="space-y-1">
                <Label htmlFor="event-message">Message</Label>
                <Textarea
                  id="event-message"
                  placeholder={type === "alert" ? "Describe the alert…" : "Add a note…"}
                  rows={3}
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                />
                {fieldErrors.message && <p className="text-destructive text-xs">{fieldErrors.message}</p>}
              </div>
            )}

            <div className="space-y-1">
              <Label>Status</Label>
              <div className="flex gap-2">
                {EVENT_STATUSES.map((s) => (
                  <Button
                    key={s.value}
                    type="button"
                    size="sm"
                    variant={status === s.value ? "default" : "outline"}
                    onClick={() => setStatus(s.value)}
                  >
                    {s.label}
                  </Button>
                ))}
              </div>
            </div>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>Cancel</Button>
            <Button type="submit" form="new-event-form" disabled={submitting}>
              {submitting ? "Creating…" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AgentChat envId={id} />
    </div>
  )
}

export default function EnvPage() {
  return <Suspense><EnvPageInner /></Suspense>
}
