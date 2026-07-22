"use client"

import * as React from "react"
import { useClientValue } from "@/hooks/use-client-value"

type Theme = "light" | "dark" | "system"
type ResolvedTheme = "light" | "dark"

const ThemeContext = React.createContext<{
  theme: Theme
  resolvedTheme: ResolvedTheme
  setTheme: (t: Theme) => void
}>({
  theme: "system",
  resolvedTheme: "light",
  setTheme: () => {},
})

export function useTheme() {
  return React.useContext(ThemeContext)
}

// The OS colour-scheme preference is external mutable state, so it is read
// through useSyncExternalStore rather than mirrored into React state by an
// effect. subscribe is module-scope to keep a stable identity across renders.
function subscribeToSystemTheme(onChange: () => void) {
  const mq = window.matchMedia("(prefers-color-scheme: dark)")
  mq.addEventListener("change", onChange)
  return () => mq.removeEventListener("change", onChange)
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  // The stored preference lives in localStorage, unreadable while prerendering,
  // so it is read through the same external-store mechanism. A local override
  // takes precedence once the user picks a theme in this session.
  const storedTheme = useClientValue<Theme>(
    () => (localStorage.getItem("theme") as Theme | null) ?? "system",
    "system",
  )
  const [override, setOverride] = React.useState<Theme | null>(null)
  const theme = override ?? storedTheme

  const systemTheme = React.useSyncExternalStore(
    subscribeToSystemTheme,
    () => (window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light") as ResolvedTheme,
    () => "light" as ResolvedTheme,
  )
  const resolvedTheme: ResolvedTheme = theme === "system" ? systemTheme : theme

  // Reflecting the resolved theme onto <html> is a side effect on something
  // React does not own, which is what an effect is for. It runs after paint on
  // mount and on every subsequent change, replacing two effects that each had
  // to remember to toggle the class themselves.
  React.useEffect(() => {
    document.documentElement.classList.toggle("dark", resolvedTheme === "dark")
  }, [resolvedTheme])

  // Toggle hotkey: press "d" to switch
  React.useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.defaultPrevented || e.repeat || e.metaKey || e.ctrlKey || e.altKey) return
      if (e.key.toLowerCase() !== "d") return
      const t = e.target as HTMLElement
      if (t.isContentEditable || ["INPUT","TEXTAREA","SELECT"].includes(t.tagName)) return
      setTheme(resolvedTheme === "dark" ? "light" : "dark")
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [resolvedTheme])

  function setTheme(newTheme: Theme) {
    localStorage.setItem("theme", newTheme)
    // localStorage is not reactive, so the override is what re-renders; the
    // effect above then syncs the <html> class.
    setOverride(newTheme)
  }

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}
