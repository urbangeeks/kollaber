"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getMembers,
  getPendingInvites,
  updateMemberRole,
  removeMember,
  revokeInvite,
  createInvite,
  getCurrentRole,
  getToken,
  type Member,
  type PendingInvite,
} from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from "@/components/ui/dialog"
import { Trash2, Copy, Check } from "lucide-react"

const ROLE_BADGE: Record<string, { label: string; className: string }> = {
  owner:  { label: "Owner",  className: "bg-amber-500/15 text-amber-600 border-amber-500/30" },
  admin:  { label: "Admin",  className: "bg-violet-500/15 text-violet-600 border-violet-500/30" },
  member: { label: "Member", className: "bg-blue-500/15 text-blue-600 border-blue-500/30" },
  viewer: { label: "Viewer", className: "bg-muted text-muted-foreground border-border" },
}

function RolePill({ role }: { role: string }) {
  const cfg = ROLE_BADGE[role] ?? ROLE_BADGE.viewer
  return (
    <span className={`rounded border px-1.5 py-0.5 text-[10px] font-medium leading-none ${cfg.className}`}>
      {cfg.label}
    </span>
  )
}

export default function MembersPage() {
  const router = useRouter()
  const [members, setMembers] = useState<Member[]>([])
  const [invites, setInvites] = useState<PendingInvite[]>([])
  const [loading, setLoading] = useState(true)
  const [myRole, setMyRole] = useState<string | null>(null)

  const [removeTarget, setRemoveTarget] = useState<Member | null>(null)
  const [removeLoading, setRemoveLoading] = useState(false)
  const [revokeTarget, setRevokeTarget] = useState<PendingInvite | null>(null)
  const [revokeLoading, setRevokeLoading] = useState(false)
  const [roleLoading, setRoleLoading] = useState<string | null>(null)

  const [inviteRole, setInviteRole] = useState<"admin" | "member" | "viewer">("member")
  const [inviteLink, setInviteLink] = useState("")
  const [inviteLoading, setInviteLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    setMyRole(getCurrentRole())
    Promise.all([getMembers(), getPendingInvites()])
      .then(([m, i]) => { setMembers(m); setInvites(i) })
      .catch((err) => toast.error(err.message))
      .finally(() => setLoading(false))
  }, [router])

  const isAdminOrOwner = myRole === "owner" || myRole === "admin"

  async function handleRoleChange(member: Member, newRole: "admin" | "member" | "viewer") {
    setRoleLoading(member.id)
    try {
      await updateMemberRole(member.id, newRole)
      setMembers((prev) => prev.map((m) => m.id === member.id ? { ...m, role: newRole } : m))
      toast.success(`${member.email} is now ${newRole}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to update role")
    } finally {
      setRoleLoading(null)
    }
  }

  async function handleRemove() {
    if (!removeTarget) return
    setRemoveLoading(true)
    try {
      await removeMember(removeTarget.id)
      setMembers((prev) => prev.filter((m) => m.id !== removeTarget.id))
      toast.success(`${removeTarget.email} removed`)
      setRemoveTarget(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to remove member")
    } finally {
      setRemoveLoading(false)
    }
  }

  async function handleGenerateInvite(forRole: "admin" | "member" | "viewer" = inviteRole) {
    setInviteLoading(true)
    setInviteLink("")
    setCopied(false)
    try {
      const token = await createInvite(forRole)
      setInviteLink(`${window.location.origin}/invite?token=${token}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create invite")
    } finally {
      setInviteLoading(false)
    }
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(inviteLink)
    setCopied(true)
    toast.success("Invite link copied")
    setTimeout(() => setCopied(false), 2000)
  }

  async function handleRevoke() {
    if (!revokeTarget) return
    setRevokeLoading(true)
    try {
      await revokeInvite(revokeTarget.token)
      setInvites((prev) => prev.filter((i) => i.token !== revokeTarget.token))
      toast.success("Invite revoked")
      setRevokeTarget(null)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to revoke invite")
    } finally {
      setRevokeLoading(false)
    }
  }

  return (
    <>
    <div className="space-y-8">
        <h1 className="text-2xl font-semibold">Members</h1>

        {/* Invite teammate */}
        {isAdminOrOwner && (
          <section className="space-y-3">
            <h2 className="text-sm font-medium">Invite teammate</h2>
            <div className="rounded-md border p-4 space-y-3">
              <div className="space-y-1.5">
                <Label className="text-xs">Role</Label>
                <div className="flex gap-2">
                  {(["admin", "member", "viewer"] as const).map((r) => (
                    <button
                      key={r}
                      onClick={() => { setInviteRole(r); setInviteLink("") }}
                      className={`rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
                        inviteRole === r
                          ? r === "admin"
                            ? "border-violet-500 bg-violet-500/15 text-violet-700"
                            : r === "member"
                            ? "border-blue-500 bg-blue-500/15 text-blue-700"
                            : "border-border bg-muted text-muted-foreground"
                          : "border-border text-muted-foreground hover:bg-muted"
                      }`}
                    >
                      {r.charAt(0).toUpperCase() + r.slice(1)}
                    </button>
                  ))}
                </div>
                <p className="text-[11px] text-muted-foreground">
                  {inviteRole === "admin" && "Can manage environments and invite others."}
                  {inviteRole === "member" && "Can create events and add comments."}
                  {inviteRole === "viewer" && "Read-only access to the timeline."}
                </p>
              </div>

              {inviteLoading ? (
                <p className="text-sm text-muted-foreground">Generating link…</p>
              ) : inviteLink ? (
                <div className="flex items-center gap-2">
                  <Input value={inviteLink} readOnly className="font-mono text-xs" />
                  <Button size="sm" variant="outline" onClick={handleCopy}>
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>
              ) : null}

              <div className="flex gap-2">
                <Button size="sm" variant="outline" disabled={inviteLoading} onClick={() => handleGenerateInvite()}>
                  {inviteLink ? "Regenerate" : "Generate invite link"}
                </Button>
              </div>
            </div>
          </section>
        )}

        {/* Members table */}
        <section className="space-y-3">
          <h2 className="text-sm font-medium">Team</h2>
          <div className="rounded-md border">
            {loading ? (
              <p className="text-muted-foreground p-4 text-sm">Loading…</p>
            ) : members.length === 0 ? (
              <p className="text-muted-foreground p-4 text-sm">No members found.</p>
            ) : (
              members.map((m, idx) => (
                <div
                  key={m.id}
                  className={`flex items-center gap-3 px-4 py-3 ${idx !== members.length - 1 ? "border-b" : ""}`}
                >
                  <div className="flex-1 min-w-0">
                    <p className="truncate text-sm">{m.email}</p>
                    <p className="text-muted-foreground text-xs">
                      Joined {new Date(m.joined_at).toLocaleDateString()}
                    </p>
                  </div>

                  {/* Role selector — admins/owners can change non-owner roles */}
                  {isAdminOrOwner && m.role !== "owner" ? (
                    <div className="flex items-center gap-1">
                      {(["admin", "member", "viewer"] as const).map((r) => (
                        <button
                          key={r}
                          disabled={roleLoading === m.id}
                          onClick={() => m.role !== r && handleRoleChange(m, r)}
                          className={`rounded border px-2 py-0.5 text-[11px] font-medium transition-colors ${
                            m.role === r
                              ? r === "admin"
                                ? "border-violet-500 bg-violet-500/15 text-violet-700"
                                : r === "member"
                                ? "border-blue-500 bg-blue-500/15 text-blue-700"
                                : "border-border bg-muted text-muted-foreground"
                              : "border-border text-muted-foreground hover:bg-muted cursor-pointer"
                          }`}
                        >
                          {r.charAt(0).toUpperCase() + r.slice(1)}
                        </button>
                      ))}
                    </div>
                  ) : (
                    <RolePill role={m.role} />
                  )}

                  {isAdminOrOwner && m.role !== "owner" && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-muted-foreground hover:text-destructive h-7 w-7 shrink-0"
                      onClick={() => setRemoveTarget(m)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  )}
                </div>
              ))
            )}
          </div>
        </section>

        {/* Pending invites */}
        {isAdminOrOwner && (
          <section className="space-y-3">
            <h2 className="text-sm font-medium">Pending invites</h2>
            <div className="rounded-md border">
              {invites.length === 0 ? (
                <p className="text-muted-foreground p-4 text-sm">No pending invites.</p>
              ) : (
                invites.map((inv, idx) => (
                  <div
                    key={inv.token}
                    className={`flex items-center gap-3 px-4 py-3 ${idx !== invites.length - 1 ? "border-b" : ""}`}
                  >
                    <div className="flex-1 min-w-0">
                      <p className="text-muted-foreground truncate font-mono text-xs">{inv.token.slice(0, 16)}…</p>
                      <p className="text-muted-foreground text-xs">
                        By {inv.created_by_email} · expires {new Date(inv.expires_at).toLocaleDateString()}
                      </p>
                    </div>
                    <RolePill role={inv.role} />
                    <Button
                      variant="ghost"
                      size="icon"
                      className="text-muted-foreground hover:text-destructive h-7 w-7 shrink-0"
                      onClick={() => setRevokeTarget(inv)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                ))
              )}
            </div>
          </section>
        )}
      </div>

      {/* Remove member confirmation */}
      <Dialog open={!!removeTarget} onOpenChange={(open) => { if (!open) setRemoveTarget(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Remove member</DialogTitle>
            <DialogDescription>
              Remove <span className="text-foreground font-medium">{removeTarget?.email}</span> from the org?
              They will lose access immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRemoveTarget(null)} disabled={removeLoading}>Cancel</Button>
            <Button variant="destructive" onClick={handleRemove} disabled={removeLoading}>
              {removeLoading ? "Removing…" : "Remove"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke invite confirmation */}
      <Dialog open={!!revokeTarget} onOpenChange={(open) => { if (!open) setRevokeTarget(null) }}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Revoke invite</DialogTitle>
            <DialogDescription>
              This invite link will stop working immediately.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRevokeTarget(null)} disabled={revokeLoading}>Cancel</Button>
            <Button variant="destructive" onClick={handleRevoke} disabled={revokeLoading}>
              {revokeLoading ? "Revoking…" : "Revoke"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
