"use client"

import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { useTheme } from "@/components/theme-provider"
import { useClientValue } from "@/hooks/use-client-value"
import { isAdmin } from "@/lib/api"
import {
  Boxes,
  BarChart3,
  Bookmark,
  LayoutGrid,
  Search,
  Settings,
  ShieldCheck,
  Sun,
  Moon,
  TriangleAlert,
} from "lucide-react"

// The signed-in product, in the order someone works through it: pick an
// environment, look something up, then the three views built on top of the
// timeline. Named for what they hold rather than what produces them —
// "Environments" is the page's own heading, and the timeline itself lives one
// level down inside one of them.
const NAV = [
  { href: "/dashboard", label: "Environments", icon: LayoutGrid },
  { href: "/search", label: "Search", icon: Search },
  { href: "/incidents", label: "Incidents", icon: TriangleAlert },
  { href: "/decisions", label: "Decisions", icon: Bookmark },
  { href: "/inventory", label: "Inventory", icon: Boxes },
  { href: "/metrics", label: "Metrics", icon: BarChart3 },
]

function useIsAdmin() {
  return useClientValue(isAdmin, false)
}

const linkClass = (active: boolean) =>
  `flex items-center gap-2.5 rounded-md px-2 py-2 text-sm font-medium transition-colors hover:bg-accent hover:text-accent-foreground ${
    active ? "bg-accent text-accent-foreground" : "text-muted-foreground"
  }`

export function AppNav() {
  const pathname = usePathname()
  const router = useRouter()
  const { resolvedTheme, setTheme } = useTheme()
  const admin = useIsAdmin()

  return (
    <div className="space-y-4">
      <div className="space-y-1">
        {NAV.map(({ href, label, icon: Icon }) => (
          <Link key={href} href={href} className={linkClass(pathname === href)}>
            <Icon className="h-4 w-4 shrink-0" />
            {label}
          </Link>
        ))}
      </div>

      <div className="space-y-1 border-t pt-3">
        <button
          onClick={() => router.push("/settings/notifications")}
          className={linkClass(pathname.startsWith("/settings"))}
        >
          <Settings className="h-4 w-4 shrink-0" />
          Settings
        </button>
        {admin && (
          <Link href="/admin" className={linkClass(pathname === "/admin")}>
            <ShieldCheck className="h-4 w-4 shrink-0" />
            Admin
          </Link>
        )}
        <button
          onClick={() => setTheme(resolvedTheme === "dark" ? "light" : "dark")}
          className={linkClass(false)}
        >
          {resolvedTheme === "dark" ? (
            <Sun className="h-4 w-4 shrink-0" />
          ) : (
            <Moon className="h-4 w-4 shrink-0" />
          )}
          {resolvedTheme === "dark" ? "Light mode" : "Dark mode"}
        </button>
      </div>
    </div>
  )
}

// Below lg the sidebar becomes a horizontal strip, the same shape settings
// already uses. It scrolls rather than collapsing into a menu because these are
// six destinations someone moves between constantly, and a tap to open a menu
// before every move is a tax on the common case.
export function AppNavMobile() {
  const pathname = usePathname()
  const admin = useIsAdmin()

  const items = admin
    ? [...NAV, { href: "/admin", label: "Admin", icon: ShieldCheck }]
    : NAV

  return (
    <div className="flex items-center gap-1">
      {items.map(({ href, label, icon: Icon }) => (
        <Link
          key={href}
          href={href}
          className={`flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium whitespace-nowrap transition-colors hover:bg-accent hover:text-accent-foreground ${
            pathname === href
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground"
          }`}
        >
          <Icon className="h-3.5 w-3.5 shrink-0" />
          {label}
        </Link>
      ))}
    </div>
  )
}
