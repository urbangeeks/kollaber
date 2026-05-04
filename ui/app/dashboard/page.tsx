"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getEnvironments, getToken, removeToken, isAdmin, createInvite, createEnvironment, type Environment } from "@/lib/api"
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
import { Server, UserPlus, Copy, Check, Plus, Download } from "lucide-react"

const REPO = "https://github.com/urbangeeks/kollaber_devops"
const LATEST = `${REPO}/releases/latest`

type Platform = { label: string; os: string; arch: string; ext: string; badge: string }

const platforms: Platform[] = [
  { label: "macOS (Apple Silicon)", os: "Darwin",  arch: "arm64", ext: "tar.gz", badge: "M1/M2/M3" },
  { label: "macOS (Intel)",         os: "Darwin",  arch: "amd64", ext: "tar.gz", badge: "Intel"    },
  { label: "Linux (x86-64)",        os: "Linux",   arch: "amd64", ext: "tar.gz", badge: "amd64"    },
  { label: "Linux (ARM64)",         os: "Linux",   arch: "arm64", ext: "tar.gz", badge: "arm64"    },
  { label: "Windows (x86-64)",      os: "Windows", arch: "amd64", ext: "zip",    badge: "amd64"    },
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
  const [newEnvErrors, setNewEnvErrors] = useState<{ name?: string; clusterName?: string }>({})
  const [newEnvLoading, setNewEnvLoading] = useState(false)

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
    const result = createEnvSchema.safeParse({ name: newEnvName, clusterName: newClusterName })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setNewEnvErrors({ name: flat.name?.[0], clusterName: flat.clusterName?.[0] })
      return
    }
    setNewEnvErrors({})
    setNewEnvLoading(true)
    try {
      const env = await createEnvironment(result.data.name, result.data.clusterName)
      setEnvs((prev) => [...prev, env])
      setNewEnvOpen(false)
      toast.success(`Environment "${env.name}" created`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create environment")
    } finally {
      setNewEnvLoading(false)
    }
  }

  async function handleGenerateInvite() {
    setInviteLoading(true)
    setInviteLink("")
    try {
      const token = await createInvite()
      setInviteLink(`${window.location.origin}/invite/${token}`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to create invite")
      setInviteOpen(false)
    } finally {
      setInviteLoading(false)
    }
  }

  function openInvite() {
    setInviteLink("")
    setCopied(false)
    setInviteOpen(true)
    handleGenerateInvite()
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(inviteLink)
    setCopied(true)
    toast.success("Invite link copied")
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">Environments</h1>
          <div className="flex items-center gap-2">
            <OrgSwitcher />
            {isAdmin() && (
              <Button variant="outline" size="sm" onClick={() => router.push("/admin")}>
                Admin
              </Button>
            )}
            <Button variant="outline" size="sm" onClick={openNewEnv}>
              <Plus className="mr-1.5 h-4 w-4" />
              New environment
            </Button>
            <Button variant="outline" size="sm" onClick={openInvite} disabled={inviteLoading}>
              <UserPlus className="mr-1.5 h-4 w-4" />
              Invite teammate
            </Button>
            <Button variant="ghost" size="sm" onClick={() => setDownloadOpen(true)}>
              <Download className="mr-1.5 h-4 w-4" />
              Download CLI
            </Button>
            <Button variant="ghost" size="sm" onClick={handleLogout}>
              Sign out
            </Button>
          </div>
        </div>

        <div className="grid gap-4">
          {envs.map((env) => (
            <Card
              key={env.id}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => router.push(`/env/${env.id}`)}
            >
              <CardHeader className="flex flex-row items-center gap-3 pb-2">
                <Server className="text-muted-foreground h-5 w-5" />
                <CardTitle className="text-base">{env.name}</CardTitle>
                {env.cluster_name && (
                  <Badge variant="secondary" className="ml-auto">
                    {env.cluster_name}
                  </Badge>
                )}
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground text-xs">
                  Created {new Date(env.created_at).toLocaleDateString()}
                </p>
              </CardContent>
            </Card>
          ))}

          {envs.length === 0 && (
            <p className="text-muted-foreground text-sm">No environments yet.</p>
          )}
        </div>
      </div>

      {/* New environment dialog */}
      <Dialog open={newEnvOpen} onOpenChange={setNewEnvOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>New environment</DialogTitle>
          </DialogHeader>
          <form id="new-env-form" onSubmit={handleCreateEnv} className="space-y-4 py-2">
            <div className="space-y-1">
              <Label htmlFor="new-env-name">Name</Label>
              <Input
                id="new-env-name"
                placeholder="prod"
                value={newEnvName}
                onChange={(e) => setNewEnvName(e.target.value)}
                autoFocus
              />
              {newEnvErrors.name && <p className="text-destructive text-xs">{newEnvErrors.name}</p>}
            </div>
            <div className="space-y-1">
              <Label htmlFor="new-cluster-name">
                Cluster name <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                id="new-cluster-name"
                placeholder="prod-cluster"
                value={newClusterName}
                onChange={(e) => setNewClusterName(e.target.value)}
              />
              {newEnvErrors.clusterName && <p className="text-destructive text-xs">{newEnvErrors.clusterName}</p>}
            </div>
          </form>
          <DialogFooter>
            <Button variant="outline" onClick={() => setNewEnvOpen(false)}>Cancel</Button>
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
          <div className="py-2">
            {inviteLoading ? (
              <p className="text-muted-foreground text-sm">Generating link…</p>
            ) : inviteLink ? (
              <div className="flex items-center gap-2">
                <Input value={inviteLink} readOnly className="font-mono text-xs" />
                <Button size="sm" variant="outline" onClick={handleCopy}>
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            ) : null}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setInviteOpen(false)}>Close</Button>
            {!inviteLoading && inviteLink && (
              <Button onClick={handleGenerateInvite}>Regenerate</Button>
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
              Send deploy events, add notes, and view the timeline from your terminal or CI pipeline.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <p className="text-xs font-medium uppercase tracking-wide">Binaries</p>
              <div className="divide-y rounded-md border">
                {platforms.map((p) => (
                  <div key={`${p.os}-${p.arch}`} className="flex items-center gap-3 px-3 py-2">
                    <span className="flex-1 text-sm">{p.label}</span>
                    <Badge variant="secondary" className="shrink-0">{p.badge}</Badge>
                    <Button size="sm" variant="outline" asChild>
                      <a href={LATEST} target="_blank" rel="noopener noreferrer">
                        .{p.ext}
                      </a>
                    </Button>
                  </div>
                ))}
              </div>
            </div>
            <div className="space-y-2">
              <p className="text-xs font-medium uppercase tracking-wide">Go</p>
              <pre className="bg-muted overflow-x-auto rounded p-3 text-xs">
                go install github.com/urbangeeks/kollaber/cmd/kollaber@latest
              </pre>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDownloadOpen(false)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
