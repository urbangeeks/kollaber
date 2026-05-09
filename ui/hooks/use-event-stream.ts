"use client"

import { useEffect, useRef } from "react"
import { getToken } from "@/lib/api"

const API_BASE =
  typeof window !== "undefined" && window.location.hostname !== "localhost"
    ? ""
    : "http://localhost:8080"

export function useEventStream(
  envID: string,
  onEvent: (data: unknown) => void,
  enabled = true,
) {
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  useEffect(() => {
    if (!enabled || !envID) return

    let active = true
    let retryTimer: ReturnType<typeof setTimeout>

    async function connect() {
      const token = getToken()
      if (!token) return

      const url = `${API_BASE}/events/stream?environment_id=${envID}`
      let response: Response
      try {
        response = await fetch(url, {
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch {
        if (active) retryTimer = setTimeout(connect, 5000)
        return
      }

      if (!response.ok || !response.body) {
        if (active) retryTimer = setTimeout(connect, 5000)
        return
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buf = ""

      try {
        while (active) {
          const { done, value } = await reader.read()
          if (done) break

          buf += decoder.decode(value, { stream: true })
          const lines = buf.split("\n")
          buf = lines.pop() ?? ""

          for (const line of lines) {
            if (line.startsWith("data: ")) {
              try {
                onEventRef.current(JSON.parse(line.slice(6)))
              } catch {
                // malformed JSON — ignore
              }
            }
          }
        }
      } catch {
        // stream error
      } finally {
        reader.cancel()
      }

      if (active) retryTimer = setTimeout(connect, 3000)
    }

    connect()

    return () => {
      active = false
      clearTimeout(retryTimer)
    }
  }, [envID, enabled])
}
