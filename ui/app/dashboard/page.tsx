"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getEnvironments,
  getToken,
  removeToken,
  isAdmin,
  getCurrentEmail,
  getCurrentRole,
  generateCLIToken,
  createInvite,
  createEnvironment,
  updateEnvironment,
  deleteEnvironment,
  type Environment,
} from "@/lib/api"
import { createEnvSchema } from "@/lib/schemas"
import { OrgSwitcher } from "@/components/org-switcher"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
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
import {
  Server,
  UserPlus,
  Copy,
  Check,
  Plus,
  Download,
  User,
  Trash2,
  Pencil,
  TriangleAlert,
  Users,
} from "lucide-react"

const REPO = "https://github.com/urbangeeks/kollaber"
const LATEST = `${REPO}/releases/latest`

type Platform = {
  label: string
  os: string
  arch: string
  ext: string
  badge: string
}

const platforms: Platform[] = [
  {
    label: "macOS (Apple Silicon)",
    os: "Darwin",
    arch: "arm64",
    ext: "tar.gz",
    badge: "M1/M2/M3",
  },
  {
    label: "macOS (Intel)",
    os: "Darwin",
    arch: "amd64",
    ext: "tar.gz",
    badge: "Intel",
  },
  {
    label: "Linux (x86-64)",
    os: "Linux",
    arch: "amd64",
    ext: "tar.gz",
    badge: "amd64",
  },
  {
    label: "Linux (ARM64)",
    os: "Linux",
    arch: "arm64",
    ext: "tar.gz",
    badge: "arm64",
  },
  {
    label: "Windows (x86-64)",
    os: "Windows",
    arch: "amd64",
    ext: "zip",
    badge: "amd64",
  },
]

export default function DashboardPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [inviteLoading, setInviteLoading] = useState(false)
  const [inviteLink, setInviteLink] = useState("")
  const [copied, setCopied] = useState(false)

  // dialogs
  const [newEnvOpen, setNewEnvOpen] = useState(false)
  const [inviteOpen, setInviteOpen] = useState(false)
  const [downloadOpen, setDownloadOpen] = useState(false)

  // new env form
  const [newEnvName, setNewEnvName] = useState("")
  const [newClusterName, setNewClusterName] = useState("")
  const [newEnvErrors, setNewEnvErrors] = useState<{
    name?: string
    clusterName?: string
  }>({})
  const [newEnvLoading, setNewEnvLoading] = useState(false)

  const [cliToken, setCliToken] = useState("")
  const [cliTokenLoading, setCliTokenLoading] = useState(false)
  const [cliTokenCopied, setCliTokenCopied] = useState(false)
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<
    "owner" | "admin" | "member" | "viewer" | null
  >(null)
  const [inviteRole, setInviteRole] = useState<"admin" | "member" | "viewer">(
    "member"
  )

  const [deleteTarget, setDeleteTarget] = useState<Environment | null>(null)
  const [deleteLoading, setDeleteLoading] = useState(false)

  const [editTarget, setEditTarget] = useState<Environment | null>(null)
  const [editName, setEditName] = useState("")
  const [editCluster, setEditCluster] = useState("")
  const [editErrors, setEditErrors] = useState<{
    name?: string
    clusterName?: string
  }>({})
  const [editLoading, setEditLoading] = useState(false)

  useEffect(() => {
    setEmail(getCurrentEmail())
    setRole(getCurrentRole())
  }, [])

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments()
      .then(setEnvs)
      .catch((err) => toast.error(err.message))
  }, [router])

  function handleLogout() {
    removeToken()
    router.push("/login")
  }

  function openNewEnv() {
    setNewEnvName("")
    setNewClusterName("")
    setNewEnvErrors({})
    setNewEnvOpen(true)
  }

  async function handleCreateEnv(e: React.FormEvent) {
    e.preventDefault()
    const result = createEnvSchema.safeParse({
      name: newEnvName,
      clusterName: newClusterName,
    })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setNewEnvErrors({
        name: flat.name?.[0],
        clusterName: flat.clusterName?.[0],
      })
      return
    }
    setNewEnvErrors({})
    setNewEnvLoading(true)
    try {
      const env = await createEnvironment(
        result.data.name,
        result.data.clusterName
      )
      setEnvs((prev) => [...prev, env])
      setNewEnvOpen(false)
      toast.success(`Environment "${env.name}" created`)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to create environment"
      )
    } finally {
      setNewEnvLoading(false)
    }
  }

  function openEdit(env: Environment) {
    setEditTarget(env)
    setEditName(env.name)
    setEditCluster(env.cluster_name)
    setEditErrors({})
  }

  async function handleEditEnv(e: React.FormEvent) {
    e.preventDefault()
    if (!editTarget) return
    const result = createEnvSchema.safeParse({
      name: editName,
      clusterName: editCluster,
    })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setEditErrors({
        name: flat.name?.[0],
        clusterName: flat.clusterName?.[0],
      })
      return
    }
    setEditErrors({})
    setEditLoading(true)
    try {
      const updated = await updateEnvironment(
        editTarget.id,
        result.data.name,
        result.data.clusterName
      )
      setEnvs((prev) => prev.map((e) => (e.id === updated.id ? updated : e)))
      setEditTarget(null)
      toast.success("Environment updated")
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to update environment"
      )
    } finally {
      setEditLoading(false)
    }
  }

  async function handleDeleteEnv() {
    if (!deleteTarget) return
    setDeleteLoading(true)
    try {
      await deleteEnvironment(deleteTarget.id)
      setEnvs((prev) => prev.filter((e) => e.id !== deleteTarget.id))
      toast.success(`Environment "${deleteTarget.name}" deleted`)
      setDeleteTarget(null)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to delete environment"
      )
    } finally {
      setDeleteLoading(false)
    }
  }

  async function handleGenerateInvite(
    forRole: "admin" | "member" | "viewer" = inviteRole
  ) {
    setInviteLoading(true)
    setInviteLink("")
    try {
      const token = await createInvite(forRole)
      setInviteLink(`${window.location.origin}/invite?token=${token}`)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to create invite"
      )
      setInviteOpen(false)
    } finally {
      setInviteLoading(false)
    }
  }

  function openInvite() {
    setInviteLink("")
    setCopied(false)
    setInviteRole("member")
    setInviteOpen(true)
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(inviteLink)
    setCopied(true)
    toast.success("Invite link copied")
    setTimeout(() => setCopied(false), 2000)
  }

  const isAdminOrOwner = role === "owner" || role === "admin"

  const ROLE_BADGE: Record<string, { label: string; className: string }> = {
    owner: {
      label: "Owner",
      className: "bg-amber-500/15 text-amber-600 border-amber-500/30",
    },
    admin: {
      label: "Admin",
      className: "bg-violet-500/15 text-violet-600 border-violet-500/30",
    },
    member: {
      label: "Member",
      className: "bg-blue-500/15 text-blue-600 border-blue-500/30",
    },
    viewer: {
      label: "Viewer",
      className: "bg-muted text-muted-foreground border-border",
    },
  }

  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="space-y-3">
          {/* Row 1: title + user */}
          <div className="flex items-center justify-between gap-4">
            <h1 className="text-2xl font-semibold">Environments</h1>
            <div className="flex items-center gap-2">
              <OrgSwitcher />
              <div className="flex items-center gap-1.5">
                <User className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span className="max-w-[160px] truncate text-xs text-muted-foreground">
                  {email}
                </span>
                {role && ROLE_BADGE[role] && (
                  <span
                    className={`rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium ${ROLE_BADGE[role].className}`}
                  >
                    {ROLE_BADGE[role].label}
                  </span>
                )}
              </div>
              <Button variant="ghost" size="sm" onClick={handleLogout}>
                Sign out
              </Button>
            </div>
          </div>

          {/* Row 2: actions */}
          <div className="flex flex-wrap items-center gap-2">
            {isAdmin() && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => router.push("/admin")}
              >
                Admin
              </Button>
            )}
            {isAdminOrOwner && (
              <Button variant="outline" size="sm" onClick={openNewEnv}>
                <Plus className="mr-1.5 h-4 w-4" />
                New environment
              </Button>
            )}
            {isAdminOrOwner && (
              <Button
                variant="outline"
                size="sm"
                onClick={openInvite}
                disabled={inviteLoading}
              >
                <UserPlus className="mr-1.5 h-4 w-4" />
                Invite teammate
              </Button>
            )}
            {isAdminOrOwner && (
              <Button variant="outline" size="sm" onClick={() => router.push("/settings/members")}>
                <Users className="mr-1.5 h-4 w-4" />
                Members
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setCliToken("")
                setDownloadOpen(true)
              }}
            >
              <Download className="mr-1.5 h-4 w-4" />
              Download CLI
            </Button>
          </div>
        </div>

        <div className="grid gap-4">
          {envs.map((env) => (
            <Card
              key={env.id}
              className="cursor-pointer transition-colors hover:bg-muted/50"
              onClick={() => router.push(`/env?id=${env.id}`)}
            >
              <CardHeader className="flex flex-row items-center gap-3 pb-2">
                <Server className="h-5 w-5 text-muted-foreground" />
                <CardTitle className="text-base">{env.name}</CardTitle>
                <div className="ml-auto flex items-center gap-2">
                  {env.cluster_name && (
                    <Badge variant="secondary">{env.cluster_name}</Badge>
                  )}
                  {isAdminOrOwner && (
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 shrink-0 text-muted-foreground hover:text-foreground"
                        onClick={(e) => {
                          e.stopPropagation()
                          openEdit(env)
                        }}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 shrink-0 text-muted-foreground hover:text-destructive"
                        onClick={(e) => {
                          e.stopPropagation()
                          setDeleteTarget(env)
                        }}
                      >
                        <Trash2 className="h-4 w-4" />
                      </Button>
                    </div>
                  )}
                </div>
              </CardHeader>
              <CardContent>
                <p className="text-xs text-muted-foreground">
                  Created {new Date(env.created_at).toLocaleDateString()}
                </p>
              </CardContent>
            </Card>
          ))}

          {envs.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No environments yet.
            </p>
          )}
        </div>
      </div>

      {/* New environment dialog */}
      <Dialog open={newEnvOpen} onOpenChange={setNewEnvOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New environment</DialogTitle>
          </DialogHeader>
          <form
            id="new-env-form"
            onSubmit={handleCreateEnv}
            className="space-y-4 py-2"
          >
            <div className="space-y-1">
              <Label htmlFor="new-env-name">Name</Label>
              <Input
                id="new-env-name"
                placeholder="prod"
                value={newEnvName}
                onChange={(e) => setNewEnvName(e.target.value)}
                autoFocus
              />
              {newEnvErrors.name && (
                <p className="text-xs text-destructive">{newEnvErrors.name}</p>
              )}
            </div>
            <div className="space-y-1">
              <Label htmlFor="new-cluster-name">
                Cluster name{" "}
                <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                id="new-cluster-name"
                placeholder="prod-cluster"
                value={newClusterName}
                onChange={(e) => setNewClusterName(e.target.value)}
              />
              {newEnvErrors.clusterName && (
                <p className="text-xs text-destructive">
                  {newEnvErrors.clusterName}
                </p>
              )}
            </div>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewEnvOpen(false)}>
              Cancel
            </Button>
            <Button type="submit" form="new-env-form" disabled={newEnvLoading}>
              {newEnvLoading ? "Creating…" : "Create"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Invite teammate dialog */}
      <Dialog open={inviteOpen} onOpenChange={setInviteOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Invite teammate</DialogTitle>
            <DialogDescription>
              Share this link with your teammate. It expires after one use.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-3 py-2">
            {/* Role picker */}
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
            ) : (
              <Button size="sm" variant="outline" disabled={inviteLoading} onClick={() => handleGenerateInvite()}>
                Generate link
              </Button>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setInviteOpen(false)}>
              Close
            </Button>
            {!inviteLoading && inviteLink && (
              <Button onClick={() => handleGenerateInvite()}>Regenerate</Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Download CLI dialog */}
      <Dialog open={downloadOpen} onOpenChange={setDownloadOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Download Kollaber CLI</DialogTitle>
            <DialogDescription>
              Send deploy events, add notes, and view the timeline from your
              terminal or CI pipeline.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">
                Binaries
              </p>
              <div className="divide-y rounded-md border">
                {platforms.map((p) => (
                  <div
                    key={`${p.os}-${p.arch}`}
                    className="flex items-center gap-3 px-3 py-2"
                  >
                    <span className="flex-1 text-sm">{p.label}</span>
                    <Badge variant="secondary" className="shrink-0">
                      {p.badge}
                    </Badge>
                    <Button size="sm" variant="outline" asChild>
                      <a
                        href={LATEST}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        .{p.ext}
                      </a>
                    </Button>
                  </div>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">Go</p>
              <pre className="overflow-x-auto rounded bg-muted p-3 text-xs">
                go install github.com/urbangeeks/kollaber/cmd/kollaber@latest
              </pre>
            </div>
            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">
                Authenticate CLI
              </p>
              <p className="text-xs text-muted-foreground">
                If you signed in with GitHub, use a token instead of a password.
              </p>
              {cliToken ? (
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Input
                      value={`kollaber login --api ${process.env.NEXT_PUBLIC_API_URL ?? (typeof window !== "undefined" ? window.location.origin : "https://kollaber.io")} --token ${cliToken}`}
                      readOnly
                      className="font-mono text-xs"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={async () => {
                        await navigator.clipboard.writeText(
                          `kollaber login --api ${process.env.NEXT_PUBLIC_API_URL ?? (typeof window !== "undefined" ? window.location.origin : "https://kollaber.io")} --token ${cliToken}`
                        )
                        setCliTokenCopied(true)
                        toast.success("Copied")
                        setTimeout(() => setCliTokenCopied(false), 2000)
                      }}
                    >
                      {cliTokenCopied ? (
                        <Check className="h-4 w-4" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Valid for 90 days. Treat it like a password.
                  </p>
                </div>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={cliTokenLoading}
                  onClick={async () => {
                    setCliTokenLoading(true)
                    try {
                      setCliToken(await generateCLIToken())
                    } catch (err) {
                      toast.error(
                        err instanceof Error
                          ? err.message
                          : "Failed to generate token"
                      )
                    } finally {
                      setCliTokenLoading(false)
                    }
                  }}
                >
                  {cliTokenLoading ? "Generating…" : "Generate CLI token"}
                </Button>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDownloadOpen(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit environment dialog */}
      <Dialog
        open={!!editTarget}
        onOpenChange={(open) => {
          if (!open) setEditTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Edit environment</DialogTitle>
          </DialogHeader>
          <form
            id="edit-env-form"
            onSubmit={handleEditEnv}
            className="space-y-4 py-2"
          >
            <div className="space-y-1">
              <Label htmlFor="edit-env-name">Name</Label>
              <Input
                id="edit-env-name"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                autoFocus
              />
              {editErrors.name && (
                <p className="text-xs text-destructive">{editErrors.name}</p>
              )}
              {editName !== editTarget?.name && editName !== "" && (
                <div className="flex items-start gap-1.5 rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-500">
                  <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
                  <span>
                    Any CLI commands or scripts using{" "}
                    <code className="font-mono">--env {editTarget?.name}</code>{" "}
                    will break. Update them to use{" "}
                    <code className="font-mono">--env {editName}</code> or pass
                    the environment UUID directly.
                  </span>
                </div>
              )}
            </div>
            <div className="space-y-1">
              <Label htmlFor="edit-cluster-name">
                Cluster name{" "}
                <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                id="edit-cluster-name"
                value={editCluster}
                onChange={(e) => setEditCluster(e.target.value)}
              />
              {editErrors.clusterName && (
                <p className="text-xs text-destructive">
                  {editErrors.clusterName}
                </p>
              )}
            </div>
          </form>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setEditTarget(null)}
              disabled={editLoading}
            >
              Cancel
            </Button>
            <Button type="submit" form="edit-env-form" disabled={editLoading}>
              {editLoading ? "Saving…" : "Save"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete environment confirmation */}
      <Dialog
        open={!!deleteTarget}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Delete environment</DialogTitle>
            <DialogDescription>
              This will permanently delete{" "}
              <span className="font-medium text-foreground">
                {deleteTarget?.name}
              </span>{" "}
              and all its events and comments. This cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setDeleteTarget(null)}
              disabled={deleteLoading}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteEnv}
              disabled={deleteLoading}
            >
              {deleteLoading ? "Deleting…" : "Delete"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
