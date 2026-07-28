"use client"

import { useEffect, useEffectEvent, useState } from "react"
import { API_BASE, getToken } from "@/lib/api"

// useEventStream subscribes to the SSE event stream. Pass a specific envID to
// scope to one environment, or "" to receive every environment in the org
// (used by org-wide views like the dashboard).
//
// Returns whether the stream is currently connected, so callers can tell the
// user when updates have stopped arriving. Without it a dropped stream is
// invisible: the timeline simply stops moving and looks like a quiet period.
export function useEventStream(
  envID: string,
  onEvent: (data: unknown) => void,
  enabled = true,
): { connected: boolean } {
  const [connected, setConnected] = useState(false)

  // Always calls the latest onEvent without being a dependency, so a new
  // callback identity does not tear down and reconnect the SSE stream. This
  // replaces a useRef assigned during render, which is unsafe under concurrent
  // rendering.
  const emit = useEffectEvent((data: unknown) => onEvent(data))

  useEffect(() => {
    if (!enabled) return

    let active = true
    let retryTimer: ReturnType<typeof setTimeout>

    async function connect() {
      const token = getToken()
      if (!token) return

      const url = `${API_BASE}/events/stream${envID ? `?environment_id=${envID}` : ""}`
      let response: Response
      try {
        response = await fetch(url, {
          headers: { Authorization: `Bearer ${token}` },
        })
      } catch {
        if (active) {
          setConnected(false)
          retryTimer = setTimeout(connect, 5000)
        }
        return
      }

      if (!response.ok || !response.body) {
        if (active) {
          setConnected(false)
          retryTimer = setTimeout(connect, 5000)
        }
        return
      }

      // Headers are in and the body is open: the server has accepted the
      // subscription, which is the earliest point we can honestly claim live.
      if (active) setConnected(true)

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
                emit(JSON.parse(line.slice(6)))
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

      if (active) {
        setConnected(false)
        retryTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      active = false
      setConnected(false)
      clearTimeout(retryTimer)
    }
  }, [envID, enabled])

  return { connected }
}
