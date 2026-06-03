"use client"

import { useEffect, useRef, useState } from "react"
import { Bot, Send, X, Sparkles, Loader2, Wrench } from "lucide-react"
import { chatWithAgent, type ChatMessage, type AgentStep } from "@/lib/api"
import { Button } from "@/components/ui/button"

type DisplayMessage = ChatMessage & { steps?: AgentStep[] }

const SUGGESTIONS = [
  "What happened in this environment today?",
  "Summarize the latest alert and its discussion",
  "Which services deployed most recently?",
]

export function AgentChat({ envId }: { envId?: string }) {
  const [open, setOpen] = useState(false)
  const [messages, setMessages] = useState<DisplayMessage[]>([])
  const [input, setInput] = useState("")
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const scrollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [messages, loading])

  async function send(text: string) {
    const content = text.trim()
    if (!content || loading) return
    setError("")
    setInput("")
    const history: ChatMessage[] = [...messages.map(({ role, content }) => ({ role, content })), { role: "user", content }]
    setMessages((m) => [...m, { role: "user", content }])
    setLoading(true)
    try {
      const { answer, steps } = await chatWithAgent(history, envId)
      setMessages((m) => [...m, { role: "assistant", content: answer, steps }])
    } catch (e) {
      setError(e instanceof Error ? e.message : "Something went wrong")
    } finally {
      setLoading(false)
    }
  }

  if (!open) {
    return (
      <Button
        onClick={() => setOpen(true)}
        size="icon"
        className="fixed bottom-6 right-6 z-40 h-12 w-12 rounded-full shadow-lg"
        aria-label="Open AI assistant"
      >
        <Sparkles className="h-5 w-5" />
      </Button>
    )
  }

  return (
    <div className="fixed bottom-6 right-6 z-40 flex h-[32rem] max-h-[calc(100vh-3rem)] w-[22rem] max-w-[calc(100vw-3rem)] flex-col rounded-xl border bg-background shadow-xl">
      <div className="flex items-center gap-2 border-b px-4 py-3">
        <Bot className="h-4 w-4 text-primary" />
        <span className="text-sm font-semibold">Timeline assistant</span>
        <Button variant="ghost" size="icon" className="ml-auto h-7 w-7" onClick={() => setOpen(false)} aria-label="Close">
          <X className="h-4 w-4" />
        </Button>
      </div>

      <div ref={scrollRef} className="flex-1 space-y-3 overflow-y-auto px-4 py-3">
        {messages.length === 0 && (
          <div className="space-y-3">
            <p className="text-muted-foreground text-sm">
              Ask about deploys, alerts, and notes in your timeline. I read your events directly — I won&apos;t make things up.
            </p>
            <div className="space-y-1.5">
              {SUGGESTIONS.map((s) => (
                <button
                  key={s}
                  onClick={() => send(s)}
                  className="block w-full rounded-md border px-3 py-2 text-left text-xs text-muted-foreground hover:bg-muted"
                >
                  {s}
                </button>
              ))}
            </div>
          </div>
        )}

        {messages.map((m, i) => (
          <div key={i} className={m.role === "user" ? "flex justify-end" : "flex justify-start"}>
            <div
              className={
                m.role === "user"
                  ? "max-w-[85%] rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground"
                  : "max-w-[90%] space-y-1.5"
              }
            >
              {m.steps && m.steps.length > 0 && (
                <details className="text-muted-foreground text-xs">
                  <summary className="flex cursor-pointer items-center gap-1.5">
                    <Wrench className="h-3 w-3" />
                    {m.steps.length} lookup{m.steps.length > 1 ? "s" : ""}
                  </summary>
                  <ul className="mt-1 space-y-0.5 pl-4">
                    {m.steps.map((s, j) => (
                      <li key={j} className="font-mono">{s.tool}</li>
                    ))}
                  </ul>
                </details>
              )}
              <div className={m.role === "assistant" ? "whitespace-pre-wrap rounded-lg bg-muted px-3 py-2 text-sm" : ""}>
                {m.content}
              </div>
            </div>
          </div>
        ))}

        {loading && (
          <div className="text-muted-foreground flex items-center gap-2 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            Looking through the timeline…
          </div>
        )}
        {error && <p className="text-sm text-red-500">{error}</p>}
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault()
          send(input)
        }}
        className="flex items-center gap-2 border-t px-3 py-3"
      >
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask about your timeline…"
          className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
        />
        <Button type="submit" size="icon" className="h-8 w-8" disabled={loading || !input.trim()}>
          <Send className="h-4 w-4" />
        </Button>
      </form>
    </div>
  )
}
