"use client"

import { useMemo, useState } from "react"
import { toast } from "sonner"
import { type Event, type Comment, type Incident, type IncidentSeverity, getComments, createComment, getEventSummary, getEventPostmortem, getIncidents, createIncident, attachEvents, getCurrentRole } from "@/lib/api"
import { commentSchema, createIncidentSchema } from "@/lib/schemas"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Rocket, Bell, StickyNote, Trash2, MessageCircle, CheckCircle2, XCircle, Loader2, Sparkles, FileText, RefreshCw, Link2, Undo2, Scaling } from "lucide-react"

const SEVERITIES: IncidentSeverity[] = ["sev1", "sev2", "sev3", "sev4"]

const TYPE_CONFIG = {
  deploy: { icon: Rocket, label: "Deploy", variant: "default" },
  alert: { icon: Bell, label: "Alert", variant: "destructive" },
  note: { icon: StickyNote, label: "Note", variant: "secondary" },
  teardown: { icon: Trash2, label: "Teardown", variant: "outline" },
  rollback: { icon: Undo2, label: "Rollback", variant: "outline" },
  scale: { icon: Scaling, label: "Scale", variant: "secondary" },
} as const

const STATUS_CONFIG = {
  success:     { label: "success",     icon: CheckCircle2, className: "text-green-600 dark:text-green-400" },
  failure:     { label: "failure",     icon: XCircle,      className: "text-red-600 dark:text-red-400"   },
  in_progress: { label: "in progress", icon: Loader2,      className: "text-amber-500 dark:text-amber-400" },
} as const

function formatTime(ts: string) {
  return new Date(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

export function TimelineEvent({
  event,
  liveComments,
}: {
  event: Event
  liveComments?: Comment[]
}) {
  const [comments, setComments] = useState<Comment[] | null>(null)
  const [body, setBody] = useState("")
  const [bodyError, setBodyError] = useState("")
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [summary, setSummary] = useState<string | null>(null)
  const [summaryLoading, setSummaryLoading] = useState(false)
  const [summaryError, setSummaryError] = useState<string | null>(null)
  const [postmortem, setPostmortem] = useState<string | null>(null)
  const [postmortemOpen, setPostmortemOpen] = useState(false)
  const [postmortemLoading, setPostmortemLoading] = useState(false)
  const [postmortemError, setPostmortemError] = useState<string | null>(null)
  const [attachOpen, setAttachOpen] = useState(false)
  const [openIncidents, setOpenIncidents] = useState<Incident[] | null>(null)
  const [attaching, setAttaching] = useState(false)
  const [newTitle, setNewTitle] = useState("")
  const [newSeverity, setNewSeverity] = useState<IncidentSeverity>("sev3")

  // Merge fetched comments with any that arrived live over SSE, de-duped by id
  // (the poster receives their own broadcast) and ordered oldest-first.
  const mergedComments = useMemo(() => {
    const byID = new Map<string, Comment>()
    for (const c of comments ?? []) byID.set(c.id, c)
    for (const c of liveComments ?? []) byID.set(c.id, c)
    return [...byID.values()].sort((a, b) => a.created_at.localeCompare(b.created_at))
  }, [comments, liveComments])

  // Live comments not yet in the loaded set — surfaced as a hint on the closed
  // thread so you know there's new discussion without opening it.
  const unseenLive = (liveComments ?? []).filter(
    (c) => !(comments ?? []).some((x) => x.id === c.id),
  ).length

  const role = getCurrentRole()
  const canAttach = role === "owner" || role === "admin" || role === "member"

  const { icon: Icon, label, variant } = TYPE_CONFIG[event.type]
  const statusCfg = STATUS_CONFIG[event.status ?? "success"]
  const StatusIcon = statusCfg.icon

  const { dotBorder, dotText } = (() => {
    if (event.type === "note") return { dotBorder: "border-border", dotText: "text-muted-foreground" }
    if (event.type === "teardown") return { dotBorder: "border-orange-200 dark:border-orange-900", dotText: "text-orange-600 dark:text-orange-400" }
    if (event.type === "alert") return { dotBorder: "border-red-200 dark:border-red-900", dotText: "text-red-600 dark:text-red-400" }
    if (event.type === "rollback") return { dotBorder: "border-amber-200 dark:border-amber-900", dotText: "text-amber-600 dark:text-amber-400" }
    if (event.type === "scale") return { dotBorder: "border-blue-200 dark:border-blue-900", dotText: "text-blue-600 dark:text-blue-400" }
    const s = event.status ?? "success"
    if (s === "failure")     return { dotBorder: "border-red-200 dark:border-red-900",   dotText: "text-red-600 dark:text-red-400" }
    if (s === "in_progress") return { dotBorder: "border-amber-200 dark:border-amber-900", dotText: "text-amber-500 dark:text-amber-400" }
    return { dotBorder: "border-green-200 dark:border-green-900", dotText: "text-green-600 dark:text-green-400" }
  })()

  async function toggleComments() {
    if (!open && comments === null) {
      const data = await getComments(event.id)
      setComments(data)
    }
    setOpen((v) => !v)
  }

  async function handlePostmortem(refresh = false) {
    if (postmortem && !refresh) { setPostmortemOpen(true); return }
    setPostmortemLoading(true)
    setPostmortemError(null)
    try {
      const text = await getEventPostmortem(event.id, refresh)
      setPostmortem(text)
      setPostmortemOpen(true)
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to generate postmortem"
      if (msg.includes("402") || msg.toLowerCase().includes("upgrade")) {
        setPostmortemError("upgrade")
        setPostmortemOpen(true)
      } else {
        setPostmortemError(msg)
        setPostmortemOpen(true)
      }
    } finally {
      setPostmortemLoading(false)
    }
  }

  async function handleSummarize() {
    if (summary) { setSummary(null); return }
    setSummaryLoading(true)
    setSummaryError(null)
    try {
      const text = await getEventSummary(event.id)
      setSummary(text)
    } catch (err) {
      const msg = err instanceof Error ? err.message : "Failed to generate summary"
      if (msg.includes("402") || msg.toLowerCase().includes("upgrade")) {
        setSummaryError("upgrade")
      } else {
        setSummaryError(msg)
      }
    } finally {
      setSummaryLoading(false)
    }
  }

  async function openAttach() {
    setAttachOpen(true)
    setNewTitle("")
    setNewSeverity("sev3")
    if (openIncidents === null) {
      try {
        setOpenIncidents(await getIncidents("open"))
      } catch {
        setOpenIncidents([])
      }
    }
  }

  async function attachTo(incidentId: string) {
    setAttaching(true)
    try {
      await attachEvents(incidentId, [event.id])
      setAttachOpen(false)
      toast.success("Event attached to incident")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to attach event")
    } finally {
      setAttaching(false)
    }
  }

  async function createAndAttach(e: React.FormEvent) {
    e.preventDefault()
    const result = createIncidentSchema.safeParse({ title: newTitle, severity: newSeverity })
    if (!result.success) {
      toast.error(result.error.flatten().fieldErrors.title?.[0] ?? "Invalid title")
      return
    }
    setAttaching(true)
    try {
      await createIncident(result.data.title, result.data.severity, [event.id])
      setAttachOpen(false)
      toast.success("Incident opened with this event")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create incident")
    } finally {
      setAttaching(false)
    }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const result = commentSchema.safeParse({ body: body.trim() })
    if (!result.success) {
      setBodyError(result.error.flatten().fieldErrors.body?.[0] ?? "Invalid comment")
      return
    }
    setBodyError("")
    setSubmitting(true)
    try {
      const comment = await createComment(event.id, result.data.body)
      setComments((prev) => [...(prev ?? []), comment])
      setBody("")
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-start gap-3">
        <div className={`relative z-10 mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border bg-background ${dotBorder} ${dotText}`}>
          <Icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
            <Badge variant={variant as "default" | "destructive" | "secondary" | "outline"}>{label}</Badge>
            <span className="font-medium truncate max-w-[180px] sm:max-w-none">{event.service}</span>
            {event.metadata?.version != null && (
              <span className="text-muted-foreground text-sm">{String(event.metadata.version)}</span>
            )}
            <span className={`flex items-center gap-1 text-xs font-medium ${statusCfg.className}`}>
              <StatusIcon className="h-3.5 w-3.5" />
              {statusCfg.label}
            </span>
            <span className="text-muted-foreground ml-auto text-xs shrink-0">{formatTime(event.timestamp)}</span>
          </div>
          {Object.keys(event.metadata ?? {}).length > 0 && (
            <div className="mt-1 flex flex-wrap gap-2">
              {Object.entries(event.metadata).map(([k, v]) =>
                k === "version" ? null : (
                  <span key={k} className="text-muted-foreground text-xs">
                    {k}: {String(v)}
                  </span>
                ),
              )}
            </div>
          )}
          <div className="mt-1 flex items-center gap-3">
            <button
              onClick={toggleComments}
              className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs transition-colors"
            >
              <MessageCircle className="h-3 w-3" />
              {open ? "Hide comments" : "Comments"}
              {!open && unseenLive > 0 && (
                <span className="ml-0.5 rounded-full bg-primary/15 px-1.5 text-[10px] font-medium text-primary">
                  {unseenLive} new
                </span>
              )}
            </button>
            <button
              onClick={handleSummarize}
              disabled={summaryLoading}
              className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs transition-colors disabled:opacity-50"
            >
              {summaryLoading
                ? <Loader2 className="h-3 w-3 animate-spin" />
                : <Sparkles className="h-3 w-3" />}
              {summaryLoading ? "Summarizing…" : summary ? "Hide summary" : "Summarize"}
            </button>
            <button
              onClick={() => handlePostmortem()}
              disabled={postmortemLoading}
              className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs transition-colors disabled:opacity-50"
            >
              {postmortemLoading
                ? <Loader2 className="h-3 w-3 animate-spin" />
                : <FileText className="h-3 w-3" />}
              {postmortemLoading ? "Generating…" : "Postmortem"}
            </button>
            {canAttach && (
              <button
                onClick={openAttach}
                className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-xs transition-colors"
              >
                <Link2 className="h-3 w-3" />
                Attach to incident
              </button>
            )}
          </div>
        </div>
      </div>

      {(summary || summaryError) && (
        <div className="ml-10 sm:ml-11">
          {summaryError === "upgrade" ? (
            <p className="text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs">
              <Sparkles className="mr-1 inline h-3 w-3" />
              AI summaries are available on the <span className="text-foreground font-medium">Team plan</span>.{" "}
              <a href="/settings/billing" className="text-primary underline underline-offset-2">Upgrade</a>
            </p>
          ) : summaryError ? (
            <p className="text-destructive text-xs">{summaryError}</p>
          ) : (
            <p className="text-foreground bg-muted/50 rounded-md border px-3 py-2 text-sm leading-relaxed">
              <Sparkles className="text-primary mr-1.5 inline h-3 w-3" />
              {summary}
            </p>
          )}
        </div>
      )}

      {open && (
        <div className="ml-10 sm:ml-11 space-y-2">
          {mergedComments.map((c) => (
            <p key={c.id} className="bg-muted rounded px-3 py-2 text-sm">
              💬 {c.body}
            </p>
          ))}
          <form onSubmit={handleSubmit} className="space-y-1">
            <div className="flex gap-2">
              <Textarea
                rows={1}
                placeholder="Add a comment…"
                value={body}
                onChange={(e) => setBody(e.target.value)}
                className="min-h-0 resize-none"
              />
              <Button type="submit" size="sm" disabled={submitting || !body.trim()}>
                Post
              </Button>
            </div>
            {bodyError && <p className="text-destructive text-xs">{bodyError}</p>}
          </form>
        </div>
      )}

      <Dialog open={attachOpen} onOpenChange={setAttachOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Link2 className="h-4 w-4" />
              Attach to incident
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-1">
            <div className="space-y-2">
              <p className="text-muted-foreground text-xs font-medium uppercase tracking-wide">Open incidents</p>
              {openIncidents === null ? (
                <p className="text-muted-foreground text-sm">Loading…</p>
              ) : openIncidents.length === 0 ? (
                <p className="text-muted-foreground text-sm">No open incidents.</p>
              ) : (
                <div className="max-h-48 space-y-1 overflow-y-auto">
                  {openIncidents.map((inc) => (
                    <button
                      key={inc.id}
                      onClick={() => attachTo(inc.id)}
                      disabled={attaching}
                      className="w-full rounded-md border px-3 py-2 text-left text-sm transition-colors hover:bg-muted disabled:opacity-50"
                    >
                      {inc.title}
                    </button>
                  ))}
                </div>
              )}
            </div>
            <Separator />
            <form onSubmit={createAndAttach} className="space-y-3">
              <p className="text-muted-foreground text-xs font-medium uppercase tracking-wide">Or open a new incident</p>
              <Input
                placeholder="Incident title"
                value={newTitle}
                onChange={(e) => setNewTitle(e.target.value)}
              />
              <div className="flex gap-2">
                {SEVERITIES.map((s) => (
                  <Button
                    key={s}
                    type="button"
                    size="sm"
                    variant={newSeverity === s ? "default" : "outline"}
                    onClick={() => setNewSeverity(s)}
                  >
                    {s.toUpperCase()}
                  </Button>
                ))}
              </div>
              <Button type="submit" size="sm" disabled={attaching || !newTitle.trim()}>
                {attaching ? "Working…" : "Create & attach"}
              </Button>
            </form>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={postmortemOpen} onOpenChange={setPostmortemOpen}>
        <DialogContent className="max-w-[calc(100vw-2rem)] sm:max-w-2xl max-h-[80vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FileText className="h-4 w-4" />
              Postmortem — {event.service}
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
  )
}
