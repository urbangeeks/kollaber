"use client"

import { useState } from "react"
import { type Event, type Comment, getComments, createComment } from "@/lib/api"
import { commentSchema } from "@/lib/schemas"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { Separator } from "@/components/ui/separator"
import { Rocket, Bell, StickyNote, MessageCircle, CheckCircle2, XCircle, Loader2 } from "lucide-react"

const TYPE_CONFIG = {
  deploy: { icon: Rocket, label: "Deploy", variant: "default" },
  alert: { icon: Bell, label: "Alert", variant: "destructive" },
  note: { icon: StickyNote, label: "Note", variant: "secondary" },
} as const

const STATUS_CONFIG = {
  success:     { label: "success",     icon: CheckCircle2, className: "text-green-600 dark:text-green-400" },
  failure:     { label: "failure",     icon: XCircle,      className: "text-red-600 dark:text-red-400"   },
  in_progress: { label: "in progress", icon: Loader2,      className: "text-amber-500 dark:text-amber-400" },
} as const

function formatTime(ts: string) {
  return new Date(ts).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
}

export function TimelineEvent({ event }: { event: Event }) {
  const [comments, setComments] = useState<Comment[] | null>(null)
  const [body, setBody] = useState("")
  const [bodyError, setBodyError] = useState("")
  const [open, setOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const { icon: Icon, label, variant } = TYPE_CONFIG[event.type]
  const statusCfg = STATUS_CONFIG[event.status ?? "success"]
  const StatusIcon = statusCfg.icon

  async function toggleComments() {
    if (!open && comments === null) {
      const data = await getComments(event.id)
      setComments(data)
    }
    setOpen((v) => !v)
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
        <div className="bg-muted mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full">
          <Icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant={variant as "default" | "destructive" | "secondary"}>{label}</Badge>
            <span className="font-medium">{event.service}</span>
            {event.metadata?.version != null && (
              <span className="text-muted-foreground text-sm">{String(event.metadata.version)}</span>
            )}
            <span className={`flex items-center gap-1 text-xs font-medium ${statusCfg.className}`}>
              <StatusIcon className="h-3.5 w-3.5" />
              {statusCfg.label}
            </span>
            <span className="text-muted-foreground ml-auto text-xs">{formatTime(event.timestamp)}</span>
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
          <button
            onClick={toggleComments}
            className="text-muted-foreground hover:text-foreground mt-1 flex items-center gap-1 text-xs transition-colors"
          >
            <MessageCircle className="h-3 w-3" />
            {open ? "Hide comments" : "Comments"}
          </button>
        </div>
      </div>

      {open && (
        <div className="ml-11 space-y-2">
          {(comments ?? []).map((c) => (
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
    </div>
  )
}
