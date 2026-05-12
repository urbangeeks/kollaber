"use client"

import * as React from "react"

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

function getResolved(theme: Theme): ResolvedTheme {
  if (theme !== "system") return theme
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light"
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = React.useState<Theme>("system")
  const [resolvedTheme, setResolvedTheme] = React.useState<ResolvedTheme>("light")

  // Read stored preference on mount
  React.useEffect(() => {
    const stored = (localStorage.getItem("theme") as Theme | null) ?? "system"
    setThemeState(stored)
    const resolved = getResolved(stored)
    setResolvedTheme(resolved)
    document.documentElement.classList.toggle("dark", resolved === "dark")
  }, [])

  // Watch system preference changes
  React.useEffect(() => {
    if (theme !== "system") return
    const mq = window.matchMedia("(prefers-color-scheme: dark)")
    const handler = () => {
      const resolved = getResolved("system")
      setResolvedTheme(resolved)
      document.documentElement.classList.toggle("dark", resolved === "dark")
    }
    mq.addEventListener("change", handler)
    return () => mq.removeEventListener("change", handler)
  }, [theme])

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
    setThemeState(newTheme)
    localStorage.setItem("theme", newTheme)
    const resolved = getResolved(newTheme)
    setResolvedTheme(resolved)
    document.documentElement.classList.toggle("dark", resolved === "dark")
  }

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  )
}
