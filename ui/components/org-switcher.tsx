"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { getMyOrgs, switchOrg, setToken, type OrgItem } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { ChevronsUpDown } from "lucide-react"

function getCurrentOrgId(): string | null {
  if (typeof window === "undefined") return null
  const token = localStorage.getItem("token")
  if (!token) return null
  try {
    return JSON.parse(atob(token.split(".")[1])).org_id ?? null
  } catch {
    return null
  }
}

function getCurrentOrgName(orgs: OrgItem[]): string {
  const id = getCurrentOrgId()
  return orgs.find((o) => o.id === id)?.name ?? "Select org"
}

export function OrgSwitcher() {
  const router = useRouter()
  const [orgs, setOrgs] = useState<OrgItem[]>([])
  const [open, setOpen] = useState(false)
  const [switching, setSwitching] = useState(false)

  useEffect(() => {
    getMyOrgs().then(setOrgs).catch(() => {})
  }, [])

  if (orgs.length <= 1) return null

  async function handleSwitch(orgId: string) {
    setOpen(false)
    if (orgId === getCurrentOrgId()) return
    setSwitching(true)
    try {
      const token = await switchOrg(orgId)
      setToken(token)
      router.refresh()
      window.location.reload()
    } finally {
      setSwitching(false)
    }
  }

  return (
    <div className="relative">
      <Button
        variant="outline"
        size="sm"
        disabled={switching}
        onClick={() => setOpen((v) => !v)}
        className="max-w-[160px]"
      >
        <span className="truncate">{switching ? "Switching…" : getCurrentOrgName(orgs)}</span>
        <ChevronsUpDown className="ml-1.5 h-3 w-3 shrink-0 opacity-50" />
      </Button>

      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="bg-popover border-border absolute right-0 z-20 mt-1 min-w-[160px] rounded-md border shadow-md">
            {orgs.map((org) => (
              <button
                key={org.id}
                onClick={() => handleSwitch(org.id)}
                className={`hover:bg-muted flex w-full items-center px-3 py-2 text-left text-sm ${
                  org.id === getCurrentOrgId() ? "font-medium" : ""
                }`}
              >
                {org.name}
                {org.id === getCurrentOrgId() && (
                  <span className="text-muted-foreground ml-auto text-xs">active</span>
                )}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}
