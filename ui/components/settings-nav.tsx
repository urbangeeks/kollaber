"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { useRouter } from "next/navigation"
import { Button } from "@/components/ui/button"
import { ArrowLeft, Bell, Hash, Users, MessageSquare, CreditCard } from "lucide-react"

const NAV = [
  { href: "/settings/notifications", label: "Notifications", icon: Bell },
  { href: "/settings/slack",         label: "Slack",         icon: Hash },
  { href: "/settings/teams",         label: "Teams",         icon: MessageSquare },
  { href: "/settings/members",       label: "Members",       icon: Users },
  { href: "/settings/billing",       label: "Billing",       icon: CreditCard },
]

export function SettingsNav() {
  const pathname = usePathname()
  const router = useRouter()

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
            className={`flex items-center gap-2.5 rounded-md px-2 py-2 text-sm transition-colors hover:bg-accent hover:text-accent-foreground ${
              pathname === href
                ? "bg-accent text-accent-foreground font-medium"
                : "text-muted-foreground"
            }`}
          >
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </Link>
        ))}
      </div>
    </div>
  )
}
