"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useRouter } from "next/navigation"
import { useTheme } from "next-themes"
import { Button } from "@/components/ui/button"
import { ArrowLeft, Bell, Hash, Users, MessageSquare, CreditCard, ScrollText, ShieldCheck, Container, Sun, Moon } from "lucide-react"

const NAV = [
  { href: "/settings/notifications", label: "Notifications", icon: Bell },
  { href: "/settings/slack",         label: "Slack",         icon: Hash },
  { href: "/settings/teams",         label: "Teams",         icon: MessageSquare },
  { href: "/settings/members",       label: "Members",       icon: Users },
  { href: "/settings/billing",       label: "Billing",       icon: CreditCard },
  { href: "/settings/kubernetes",    label: "Kubernetes",    icon: Container },
  { href: "/settings/sso",           label: "SSO",           icon: ShieldCheck },
  { href: "/settings/audit-logs",    label: "Audit Logs",    icon: ScrollText },
]

export function SettingsNav() {
  const pathname = usePathname()
  const router = useRouter()
  const { resolvedTheme, setTheme } = useTheme()

  return (
    <div className="space-y-4">
      <Button variant="ghost" size="sm" className="w-full justify-start px-2 text-muted-foreground" onClick={() => router.push("/dashboard")}>
        <ArrowLeft className="mr-1.5 h-4 w-4" />
        Dashboard
      </Button>

      <div className="space-y-1">
        <p className="px-2 pb-1 text-xs font-semibold uppercase tracking-widest text-muted-foreground/50">
          Settings
        </p>
        {NAV.map(({ href, label, icon: Icon }) => (
          <Link
            key={href}
            href={href}
            className={`flex items-center gap-2.5 rounded-md px-2 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground ${
              pathname === href
                ? "bg-accent text-accent-foreground"
                : "text-muted-foreground"
            }`}
          >
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </Link>
        ))}
      </div>

      <div className="border-t pt-3">
        <button
          onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
          className="flex w-full items-center gap-2.5 rounded-md px-2 py-2 text-sm font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
        >
          {resolvedTheme === "dark"
            ? <Sun className="h-4 w-4 shrink-0" />
            : <Moon className="h-4 w-4 shrink-0" />}
          {resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
        </button>
      </div>
    </div>
  )
}

export function SettingsNavMobile() {
  const pathname = usePathname()
  const { resolvedTheme, setTheme } = useTheme()

  return (
    <div className="flex items-center gap-1">
      {NAV.map(({ href, label, icon: Icon }) => (
        <Link
          key={href}
          href={href}
          className={`flex items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors hover:bg-accent hover:text-accent-foreground ${
            pathname === href
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground"
          }`}
        >
          <Icon className="h-3.5 w-3.5 shrink-0" />
          {label}
        </Link>
      ))}
      <div className="mx-1 h-4 w-px bg-border shrink-0" />
      <button
        onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
        className="flex items-center gap-1.5 whitespace-nowrap rounded-md px-2.5 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
      >
        {resolvedTheme === "dark" ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
        {resolvedTheme === "dark" ? "Light" : "Dark"}
      </button>
    </div>
  )
}

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme()
  return (
    <Button
      variant="ghost"
      size="icon"
      className="h-8 w-8"
      onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
    >
      {resolvedTheme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}
