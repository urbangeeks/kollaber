"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { toast } from "sonner"
import { getEvents, createEvent, getServices, getToken, type Event, type EventFilters } from "@/lib/api"
import { createEventSchema } from "@/lib/schemas"
import { usePoll } from "@/hooks/use-poll"
import { TimelineEvent } from "@/components/timeline-event"
import { Separator } from "@/components/ui/separator"
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
import { ArrowLeft, Plus, Loader2 } from "lucide-react"

type EventType = "deploy" | "alert" | "note"
type EventStatus = "success" | "failure" | "in_progress"

const EVENT_TYPES: { value: EventType; label: string }[] = [
  { value: "deploy", label: "Deploy" },
  { value: "alert", label: "Alert" },
  { value: "note",  label: "Note"   },
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
  const [polling, setPolling] = useState(false)
  const [error, setError] = useState("")

  const PAGE_SIZE = 50
  const allEvents = [...topEvents, ...moreEvents]

  const [dialogOpen, setDialogOpen] = useState(false)
  const [type, setType] = useState<EventType>("note")
  const [service, setService] = useState("")
  const [version, setVersion] = useState("")
  const [message, setMessage] = useState("")
  const [status, setStatus] = useState<EventStatus>("success")
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [submitting, setSubmitting] = useState(false)

  const [filterType, setFilterType] = useState("")
  const [filterStatus, setFilterStatus] = useState("")
  const [filterService, setFilterService] = useState("")
  const [knownServices, setKnownServices] = useState<string[]>([])

  const isAuthed = Boolean(getToken())

  const activeFilters: EventFilters = {}
  if (filterType)    activeFilters.type    = filterType
  if (filterStatus)  activeFilters.status  = filterStatus
  if (filterService) activeFilters.service = filterService

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
  }, [id, filterType, filterStatus, filterService])

  // Background poll — shows a subtle spinner, does not replace the list with a skeleton.
  usePoll(
    () => {
      if (!isAuthed) { router.replace("/login"); return }
      if (!id || loading) return
      setPolling(true)
      getEvents(id, PAGE_SIZE, 0, activeFilters)
        .then((evts) => {
          setTopEvents(evts)
          setHasMore((prev) => moreEvents.length > 0 ? prev : evts.length === PAGE_SIZE)
          getServices(id).then(setKnownServices).catch(() => {})
          setError("")
        })
        .catch((err) => setError(err.message))
        .finally(() => setPolling(false))
    },
    7000,
    isAuthed,
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
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">Timeline</h1>
          <div className="ml-auto flex items-center gap-3">
            <span className="text-muted-foreground flex items-center gap-1.5 text-xs">
              {polling
                ? <Loader2 className="h-3 w-3 animate-spin" />
                : <span className="h-1.5 w-1.5 rounded-full bg-green-500" />}
              Refreshes every 7s
            </span>
            <Button size="sm" onClick={openDialog}>
              <Plus className="mr-1.5 h-4 w-4" />
              New event
            </Button>
          </div>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <div className="flex flex-wrap items-center gap-2">
          <span className="text-muted-foreground text-xs font-medium">Type:</span>
          {EVENT_TYPES.map((t) => (
            <Button
              key={t.value}
              size="sm"
              variant={filterType === t.value ? "secondary" : "ghost"}
              className="h-7 px-2.5 text-xs"
              onClick={() => setFilterType(filterType === t.value ? "" : t.value)}
            >
              {t.label}
            </Button>
          ))}
          <span className="text-muted-foreground ml-2 text-xs font-medium">Status:</span>
          {EVENT_STATUSES.map((s) => (
            <Button
              key={s.value}
              size="sm"
              variant={filterStatus === s.value ? "secondary" : "ghost"}
              className="h-7 px-2.5 text-xs"
              onClick={() => setFilterStatus(filterStatus === s.value ? "" : s.value)}
            >
              {s.label}
            </Button>
          ))}
          {knownServices.length > 0 && (
            <div className="ml-2 flex items-center gap-2">
              <span className="text-muted-foreground text-xs font-medium">Service:</span>
              <select
                value={filterService}
                onChange={(e) => setFilterService(e.target.value)}
                className="border-input bg-background h-7 rounded-md border px-2 text-xs"
              >
                <option value="">All</option>
                {knownServices.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </div>
          )}
          {(filterType || filterStatus || filterService) && (
            <Button
              size="sm"
              variant="ghost"
              className="text-muted-foreground h-7 px-2 text-xs"
              onClick={() => { setFilterType(""); setFilterStatus(""); setFilterService("") }}
            >
              Clear
            </Button>
          )}
        </div>

        <div className="space-y-4">
          {loading ? (
            Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="space-y-2 animate-pulse">
                <div className="flex items-center gap-3">
                  <div className="h-8 w-8 rounded-full bg-muted" />
                  <div className="space-y-1.5 flex-1">
                    <div className="h-3 w-1/3 rounded bg-muted" />
                    <div className="h-3 w-1/2 rounded bg-muted" />
                  </div>
                  <div className="h-3 w-16 rounded bg-muted" />
                </div>
                {i < 4 && <Separator className="mt-4" />}
              </div>
            ))
          ) : (
            <>
              {allEvents.map((event, i) => (
                <div key={event.id}>
                  <TimelineEvent event={event} />
                  {i < allEvents.length - 1 && <Separator className="mt-4" />}
                </div>
              ))}

              {allEvents.length === 0 && !error && (
                <p className="text-muted-foreground text-sm">No events yet.</p>
              )}

              {hasMore && (
                <div className="pt-2 text-center">
                  <Button variant="outline" size="sm" onClick={handleLoadMore} disabled={loadingMore}>
                    {loadingMore ? "Loading…" : "Load more"}
                  </Button>
                </div>
              )}
            </>
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
    </div>
  )
}

export default function EnvPage() {
  return <Suspense><EnvPageInner /></Suspense>
}
