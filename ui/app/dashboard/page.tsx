"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import {
  getEnvironments,
  getEnvStats,
  getToken,
  removeToken,
  isAdmin,
  getCurrentEmail,
  getCurrentRole,
  generateCLIToken,
  createEnvironment,
  updateEnvironment,
  deleteEnvironment,
  type Environment,
  type EnvStat,
} from "@/lib/api"
import { createEnvSchema } from "@/lib/schemas"
import { OrgSwitcher } from "@/components/org-switcher"
import { Card, CardContent } from "@/components/ui/card"
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
  Copy,
  Check,
  Plus,
  Download,
  User,
  Trash2,
  Pencil,
  TriangleAlert,
  Settings,
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
  const [stats, setStats] = useState<Record<string, EnvStat>>({})
  // dialogs
  const [newEnvOpen, setNewEnvOpen] = useState(false)
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
    getEnvStats()
      .then((rows) => setStats(Object.fromEntries(rows.map((r) => [r.environment_id, r]))))
      .catch(() => {/* non-critical */})
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
    <div className="min-h-screen">
      {/* Top bar */}
      <header className="border-b px-8 py-4">
        <div className="mx-auto max-w-5xl flex items-center gap-4">
          <span className="font-semibold tracking-tight">Kollaber</span>
          <div className="ml-auto flex items-center gap-2">
            <OrgSwitcher />
            <Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => router.push("/settings/notifications")}>
              <Settings className="h-4 w-4" />
            </Button>
            {isAdmin() && (
              <Button variant="ghost" size="sm" onClick={() => router.push("/admin")}>
                Admin
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => { setCliToken(""); setDownloadOpen(true) }}
            >
              <Download className="mr-1.5 h-4 w-4" />
              CLI
            </Button>
            <div className="h-4 w-px bg-border" />
            <div className="flex items-center gap-1.5">
              <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-xs font-medium">
                {email ? email[0].toUpperCase() : <User className="h-3.5 w-3.5" />}
              </div>
              <span className="max-w-[140px] truncate text-xs text-muted-foreground hidden sm:block">
                {email}
              </span>
              {role && ROLE_BADGE[role] && (
                <span className={`rounded border px-1.5 py-0.5 text-[10px] leading-none font-medium ${ROLE_BADGE[role].className}`}>
                  {ROLE_BADGE[role].label}
                </span>
              )}
            </div>
            <Button variant="ghost" size="sm" className="text-muted-foreground" onClick={handleLogout}>
              Sign out
            </Button>
          </div>
        </div>
      </header>

      <div className="mx-auto max-w-5xl px-8 py-8 space-y-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-xl font-semibold">Environments</h1>
            <p className="text-sm text-muted-foreground mt-0.5">Select an environment to view its timeline.</p>
          </div>
          {isAdminOrOwner && (
            <Button size="sm" onClick={openNewEnv}>
              <Plus className="mr-1.5 h-4 w-4" />
              New environment
            </Button>
          )}
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          {envs.map((env) => {
            const s = stats[env.id]
            const statusDot = s
              ? s.alerts > 0
                ? "bg-amber-400"
                : s.deploys > 0
                  ? "bg-green-400"
                  : "bg-muted-foreground/30"
              : "bg-muted-foreground/30"

            return (
              <Card
                key={env.id}
                className="cursor-pointer transition-all hover:shadow-sm hover:border-border/80 group"
                onClick={() => router.push(`/env?id=${env.id}`)}
              >
                <CardContent className="p-5">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className={`h-2 w-2 rounded-full shrink-0 ${statusDot}`} />
                      <div className="min-w-0">
                        <p className="font-semibold truncate">{env.name}</p>
                        {env.cluster_name && (
                          <p className="text-xs text-muted-foreground truncate">{env.cluster_name}</p>
                        )}
                      </div>
                    </div>
                    {isAdminOrOwner && (
                      <div className="flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity shrink-0">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-foreground"
                          onClick={(e) => { e.stopPropagation(); openEdit(env) }}
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7 text-muted-foreground hover:text-destructive"
                          onClick={(e) => { e.stopPropagation(); setDeleteTarget(env) }}
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    )}
                  </div>

                  <div className="mt-4 border-t pt-4">
                    {s ? (
                      <>
                        <div className="grid grid-cols-3 gap-3">
                          <div>
                            <p className="text-2xl font-semibold">{s.deploys}</p>
                            <p className="text-xs text-muted-foreground">deploys</p>
                          </div>
                          <div>
                            <p className={`text-2xl font-semibold ${s.alerts > 0 ? "text-amber-500" : ""}`}>{s.alerts}</p>
                            <p className="text-xs text-muted-foreground">alerts</p>
                          </div>
                          <div>
                            <p className="text-2xl font-semibold">{s.notes}</p>
                            <p className="text-xs text-muted-foreground">notes</p>
                          </div>
                        </div>
                        {s.last_event_at && (
                          <p className="text-xs text-muted-foreground mt-3">
                            Last activity {new Date(s.last_event_at).toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" })}
                          </p>
                        )}
                      </>
                    ) : (
                      <p className="text-xs text-muted-foreground">No events yet.</p>
                    )}
                  </div>
                </CardContent>
              </Card>
            )
          })}

          {envs.length === 0 && (
            <p className="text-sm text-muted-foreground col-span-2">No environments yet.</p>
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
                      value={`kollaber login --api ${process.env.NEXT_PUBLIC_API_URL || (typeof window !== "undefined" ? window.location.origin : "https://kollaber.io")} --token ${cliToken}`}
                      readOnly
                      className="font-mono text-xs"
                    />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={async () => {
                        await navigator.clipboard.writeText(
                          `kollaber login --api ${process.env.NEXT_PUBLIC_API_URL || (typeof window !== "undefined" ? window.location.origin : "https://kollaber.io")} --token ${cliToken}`
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
