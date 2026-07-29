"use client"

import { useState } from "react"
import { toast } from "sonner"
import { generatePostmortem, type Postmortem } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { FileText, Loader2, Download, Copy, Check } from "lucide-react"

// What each narrative_status means to the person looking at the document. The
// factual half is always there, so none of these is an error state — they
// explain a missing section, not a failed request.
const NARRATIVE_NOTE: Record<Postmortem["narrative_status"], string> = {
  included: "",
  not_requested: "",
  upgrade_required:
    "The AI summary section needs the Pro plan. Everything below is from your own timeline.",
  unavailable:
    "No AI summary: this instance has no Anthropic API key configured.",
  failed: "The AI summary could not be generated. The rest of the document is complete.",
}

// datetime-local wants "YYYY-MM-DDTHH:mm" in local time; the API wants RFC3339.
function toLocalInput(d: Date) {
  const pad = (n: number) => String(n).padStart(2, "0")
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

// The timeline page knows the environment id but not its name, and fetching one
// just to title a dialog is not worth a request — the generated document
// carries the name, so the title fills in once there is something to title.
export function PostmortemDialog({
  environmentId,
  open,
  onOpenChange,
}: {
  environmentId: string
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const now = new Date()
  const dayAgo = new Date(now.getTime() - 24 * 60 * 60 * 1000)

  const [from, setFrom] = useState(toLocalInput(dayAgo))
  const [to, setTo] = useState(toLocalInput(now))
  const [doc, setDoc] = useState<Postmortem | null>(null)
  const [generating, setGenerating] = useState(false)
  const [copied, setCopied] = useState(false)

  async function generate() {
    if (!from || !to) return
    if (new Date(from) >= new Date(to)) {
      toast.error("The start of the window must be before the end")
      return
    }
    setGenerating(true)
    try {
      const result = await generatePostmortem(
        environmentId,
        new Date(from).toISOString(),
        new Date(to).toISOString(),
      )
      setDoc(result)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Could not generate postmortem")
    } finally {
      setGenerating(false)
    }
  }

  function download() {
    if (!doc) return
    const stamp = doc.from.slice(0, 10)
    const blob = new Blob([doc.markdown], { type: "text/markdown" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `postmortem-${doc.environment_name}-${stamp}.md`
    a.click()
    URL.revokeObjectURL(url)
  }

  async function copy() {
    if (!doc) return
    await navigator.clipboard.writeText(doc.markdown)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const note = doc ? NARRATIVE_NOTE[doc.narrative_status] : ""

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-[calc(100vw-2rem)] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            {doc ? `Postmortem — ${doc.environment_name}` : "Postmortem"}
          </DialogTitle>
        </DialogHeader>

        <div className="flex flex-wrap items-end gap-3">
          <div className="space-y-1">
            <Label htmlFor="pm-from" className="text-xs">
              From
            </Label>
            <Input
              id="pm-from"
              type="datetime-local"
              value={from}
              onChange={(e) => setFrom(e.target.value)}
              className="h-8 w-auto text-xs"
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="pm-to" className="text-xs">
              To
            </Label>
            <Input
              id="pm-to"
              type="datetime-local"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              className="h-8 w-auto text-xs"
            />
          </div>
          <Button size="sm" onClick={generate} disabled={generating}>
            {generating ? (
              <>
                <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                Generating…
              </>
            ) : (
              "Generate"
            )}
          </Button>
        </div>

        {doc && (
          <div className="space-y-3">
            <div className="text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
              <span>
                {doc.event_count} event{doc.event_count === 1 ? "" : "s"}
              </span>
              <span>
                {doc.comment_count} comment{doc.comment_count === 1 ? "" : "s"}
              </span>
              {doc.participants.length > 0 && (
                <span>{doc.participants.length} participant{doc.participants.length === 1 ? "" : "s"}</span>
              )}
              <div className="ml-auto flex gap-2">
                <Button size="sm" variant="outline" onClick={copy}>
                  {copied ? (
                    <Check className="mr-1.5 h-3.5 w-3.5" />
                  ) : (
                    <Copy className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {copied ? "Copied" : "Copy"}
                </Button>
                <Button size="sm" variant="outline" onClick={download}>
                  <Download className="mr-1.5 h-3.5 w-3.5" />
                  Download
                </Button>
              </div>
            </div>

            {note && (
              <p className="text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs">
                {note}
                {doc.narrative_status === "upgrade_required" && (
                  <>
                    {" "}
                    <a
                      href="/settings/billing"
                      className="text-primary underline underline-offset-2"
                    >
                      Upgrade
                    </a>
                  </>
                )}
              </p>
            )}

            {doc.truncated && (
              <p className="text-muted-foreground rounded-md border border-dashed px-3 py-2 text-xs">
                This window has more events than one document holds. Narrow the
                range for a complete picture.
              </p>
            )}

            <pre className="bg-muted/40 max-h-[45vh] overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
              {doc.markdown}
            </pre>
          </div>
        )}

        {!doc && !generating && (
          <p className="text-muted-foreground rounded-md border border-dashed px-3 py-6 text-center text-sm">
            Pick a window and generate a markdown postmortem: the event
            sequence, every comment thread, and who was involved.
          </p>
        )}
      </DialogContent>
    </Dialog>
  )
}
