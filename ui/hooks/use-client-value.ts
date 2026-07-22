"use client"

import { useSyncExternalStore } from "react"

// Nothing to subscribe to: these values (auth token claims, window.location)
// are fixed for the life of the page. useSyncExternalStore still wants a
// subscribe function, so this is a stable no-op — defined at module scope
// because an inline one would be a new identity on every render.
const noSubscribe = () => () => {}

/**
 * useClientValue reads a value that only exists in the browser — localStorage,
 * window.location — without a hydration mismatch.
 *
 * The obvious alternatives are both wrong here. A lazy useState initializer
 * runs during prerender, where localStorage does not exist, and would render
 * different markup on the server than the client. Setting state from an effect
 * works but triggers a second render pass for every such value, which is what
 * react-hooks/set-state-in-effect warns about.
 *
 * useSyncExternalStore is the supported way to say "this value comes from
 * outside React": prerender and hydration use serverValue, and the client
 * switches to the real one in the same commit.
 */
export function useClientValue<T>(read: () => T, serverValue: T): T {
  return useSyncExternalStore(noSubscribe, read, () => serverValue)
}
