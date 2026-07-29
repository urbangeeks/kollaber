"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { getCurrentEmail, getCurrentRole, removeToken } from "@/lib/api"
import { useClientValue } from "@/hooks/use-client-value"
import { AppNav, AppNavMobile } from "@/components/app-nav"
import { CliDownload } from "@/components/cli-download"
import { OrgSwitcher } from "@/components/org-switcher"
import { DotBackground } from "@/components/dot-background"
import { Button } from "@/components/ui/button"
import { LogOut, User } from "lucide-react"

const ROLE_BADGE: Record<string, { label: string; className: string }> = {
  owner: {
    label: "Owner",
    className: "bg-primary/10 text-primary border-primary/20",
  },
  admin: {
    label: "Admin",
    className: "bg-primary/10 text-primary border-primary/20",
  },
  member: {
    label: "Member",
    className: "bg-muted text-muted-foreground border-border",
  },
  viewer: {
    label: "Viewer",
    className: "bg-muted text-muted-foreground border-border",
  },
}

// AppShell is the chrome every signed-in page shares: identity and org on top,
// destinations down the side.
//
// Pages opt in rather than inheriting it from a route group, because two of
// them deliberately opt out — the environment timeline runs full-width, and the
// auth pages have no nav at all. Making that a visible line in each page keeps
// the exception obvious instead of encoding it in directory structure.
export function AppShell({
  title,
  actions,
  children,
}: {
  title?: string
  actions?: React.ReactNode
  children: React.ReactNode
}) {
  const router = useRouter()
  const email = useClientValue(getCurrentEmail, "")
  const role = useClientValue<"owner" | "admin" | "member" | "viewer" | null>(
    getCurrentRole,
    null
  )

  function handleLogout() {
    removeToken()
    router.push("/login")
  }

  return (
    <div className="min-h-screen">
      <DotBackground />

      <header className="border-b px-4 py-4 sm:px-8">
        <div className="mx-auto flex max-w-6xl items-center gap-4">
          <Link href="/dashboard" className="font-semibold tracking-tight">
            Kollaber
          </Link>
          <div className="ml-auto flex items-center gap-2">
            <OrgSwitcher />
            {actions}
            <CliDownload />
            <div className="hidden h-4 w-px bg-border sm:block" />
            <div className="flex items-center gap-1.5">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-xs font-medium">
                {email ? (
                  email[0].toUpperCase()
                ) : (
                  <User className="h-3.5 w-3.5" />
                )}
              </div>
              <span className="hidden max-w-[140px] truncate text-xs text-muted-foreground sm:block">
                {email}
              </span>
              {role && ROLE_BADGE[role] && (
                <span
                  className={`hidden rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium sm:inline ${ROLE_BADGE[role].className}`}
                >
                  {ROLE_BADGE[role].label}
                </span>
              )}
            </div>
            <Button
              variant="ghost"
              size="sm"
              className="hidden text-muted-foreground sm:inline-flex"
              onClick={handleLogout}
            >
              Sign out
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8 text-muted-foreground sm:hidden"
              onClick={handleLogout}
            >
              <LogOut className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </header>

      <div className="overflow-x-auto border-b lg:hidden">
        <div className="min-w-max px-4 py-2.5">
          <AppNavMobile />
        </div>
      </div>

      <div className="mx-auto flex max-w-6xl gap-10 px-4 py-6 sm:px-8 sm:py-8">
        <aside className="hidden w-44 shrink-0 lg:block">
          <div className="sticky top-8">
            <AppNav />
          </div>
        </aside>
        <main className="min-w-0 flex-1 space-y-6">
          {title && <h1 className="text-xl font-semibold">{title}</h1>}
          {children}
        </main>
      </div>
    </div>
  )
}
