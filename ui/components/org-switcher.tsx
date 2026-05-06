"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getMyOrgs, switchOrg, createOrg, renameOrg, setToken, type OrgItem } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { ChevronsUpDown, Pencil, Plus } from "lucide-react"

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

function getCurrentOrg(orgs: OrgItem[]): OrgItem | undefined {
  const id = getCurrentOrgId()
  return orgs.find((o) => o.id === id)
}

export function OrgSwitcher() {
  const router = useRouter()
  const [orgs, setOrgs] = useState<OrgItem[]>([])
  const [open, setOpen] = useState(false)
  const [switching, setSwitching] = useState(false)

  const [renameOpen, setRenameOpen] = useState(false)
  const [renameName, setRenameName] = useState("")
  const [renameLoading, setRenameLoading] = useState(false)

  const [newOrgOpen, setNewOrgOpen] = useState(false)
  const [newOrgName, setNewOrgName] = useState("")
  const [newOrgLoading, setNewOrgLoading] = useState(false)

  useEffect(() => {
    getMyOrgs().then(setOrgs).catch(() => {})
  }, [])

  if (orgs.length === 0) return null

  const current = getCurrentOrg(orgs)
  const isOwner = current?.role === "owner" || current?.role === "admin"

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

  async function handleRename(e: React.FormEvent) {
    e.preventDefault()
    if (!current || !renameName.trim()) return
    setRenameLoading(true)
    try {
      const updated = await renameOrg(current.id, renameName.trim())
      setOrgs((prev) => prev.map((o) => o.id === updated.id ? { ...o, name: updated.name, slug: updated.slug } : o))
      setRenameOpen(false)
      toast.success("Org renamed")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to rename org")
    } finally {
      setRenameLoading(false)
    }
  }

  async function handleCreateOrg(e: React.FormEvent) {
    e.preventDefault()
    if (!newOrgName.trim()) return
    setNewOrgLoading(true)
    try {
      const result = await createOrg(newOrgName.trim())
      setToken(result.token)
      window.location.reload()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create org")
      setNewOrgLoading(false)
    }
  }

  return (
    <>
      <div className="relative">
        <Button
          variant="outline"
          size="sm"
          disabled={switching}
          onClick={() => setOpen((v) => !v)}
          className="max-w-[160px]"
        >
          <span className="truncate">{switching ? "Switching…" : (current?.name ?? "Select org")}</span>
          <ChevronsUpDown className="ml-1.5 h-3 w-3 shrink-0 opacity-50" />
        </Button>

        {open && (
          <>
            <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
            <div className="bg-popover border-border absolute right-0 z-20 mt-1 min-w-[180px] rounded-md border shadow-md">
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

              <div className="border-border border-t">
                {isOwner && (
                  <button
                    onClick={() => { setOpen(false); setRenameName(current?.name ?? ""); setRenameOpen(true) }}
                    className="hover:bg-muted text-muted-foreground hover:text-foreground flex w-full items-center gap-2 px-3 py-2 text-left text-xs"
                  >
                    <Pencil className="h-3.5 w-3.5" />
                    Rename org
                  </button>
                )}
                <button
                  onClick={() => { setOpen(false); setNewOrgName(""); setNewOrgOpen(true) }}
                  className="hover:bg-muted text-muted-foreground hover:text-foreground flex w-full items-center gap-2 px-3 py-2 text-left text-xs"
                >
                  <Plus className="h-3.5 w-3.5" />
                  New org
                </button>
              </div>
            </div>
          </>
        )}
      </div>

      {/* Rename org dialog */}
      <Dialog open={renameOpen} onOpenChange={setRenameOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Rename org</DialogTitle>
          </DialogHeader>
          <form id="rename-org-form" onSubmit={handleRename} className="py-2">
            <Label htmlFor="rename-org-input">Name</Label>
            <Input
              id="rename-org-input"
              className="mt-1"
              value={renameName}
              onChange={(e) => setRenameName(e.target.value)}
              autoFocus
            />
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRenameOpen(false)} disabled={renameLoading}>Cancel</Button>
            <Button type="submit" form="rename-org-form" disabled={renameLoading || !renameName.trim()}>
              {renameLoading ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* New org dialog */}
      <Dialog open={newOrgOpen} onOpenChange={setNewOrgOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>New org</DialogTitle>
          </DialogHeader>
          <form id="new-org-form" onSubmit={handleCreateOrg} className="py-2">
            <Label htmlFor="new-org-input">Name</Label>
            <Input
              id="new-org-input"
              className="mt-1"
              placeholder="Acme Corp"
              value={newOrgName}
              onChange={(e) => setNewOrgName(e.target.value)}
              autoFocus
            />
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewOrgOpen(false)} disabled={newOrgLoading}>Cancel</Button>
            <Button type="submit" form="new-org-form" disabled={newOrgLoading || !newOrgName.trim()}>
              {newOrgLoading ? "Creating…" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
