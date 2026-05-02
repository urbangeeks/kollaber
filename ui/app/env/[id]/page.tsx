"use client"

import { use, useState } from "react"
import { useRouter } from "next/navigation"
import { getEvents, getToken, type Event } from "@/lib/api"
import { usePoll } from "@/hooks/use-poll"
import { TimelineEvent } from "@/components/timeline-event"
import { Separator } from "@/components/ui/separator"
import { Button } from "@/components/ui/button"
import { ArrowLeft } from "lucide-react"

export default function EnvPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params)
  const router = useRouter()
  const [events, setEvents] = useState<Event[]>([])
  const [error, setError] = useState("")

  const isAuthed = Boolean(getToken())

  usePoll(
    () => {
      if (!isAuthed) {
        router.replace("/login")
        return
      }
      getEvents(id)
        .then(setEvents)
        .catch((err) => setError(err.message))
    },
    7000,
    isAuthed,
  )

  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-2xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-xl font-semibold">Timeline</h1>
          <span className="text-muted-foreground ml-auto text-xs">Refreshes every 7s</span>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <div className="space-y-4">
          {events.map((event, i) => (
            <div key={event.id}>
              <TimelineEvent event={event} />
              {i < events.length - 1 && <Separator className="mt-4" />}
            </div>
          ))}

          {events.length === 0 && !error && (
            <p className="text-muted-foreground text-sm">No events yet.</p>
          )}
        </div>
      </div>
    </div>
  )
}
