"use client"

import { useEffect, useEffectEvent } from "react"

export function usePoll(fn: () => void, intervalMs: number, enabled = true) {
  // useEffectEvent always sees the latest fn without being a dependency, so the
  // interval is not torn down and recreated every time the caller passes a new
  // closure. This replaces a useRef assigned during render, which is unsafe
  // once React renders concurrently.
  const tick = useEffectEvent(() => fn())

  useEffect(() => {
    if (!enabled) return
    tick()
    const id = setInterval(() => tick(), intervalMs)
    return () => clearInterval(id)
  }, [intervalMs, enabled])
}
