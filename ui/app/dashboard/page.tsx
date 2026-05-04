"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { getEnvironments, getToken, removeToken, isAdmin, createInvite, type Environment } from "@/lib/api"
import { OrgSwitcher } from "@/components/org-switcher"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Server, UserPlus, Copy, Check } from "lucide-react"

export default function DashboardPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [error, setError] = useState("")
  const [inviteLink, setInviteLink] = useState("")
  const [inviteLoading, setInviteLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments()
      .then(setEnvs)
      .catch((err) => setError(err.message))
  }, [router])

  function handleLogout() {
    removeToken()
    router.push("/login")
  }

  async function handleInvite() {
    setInviteLoading(true)
    try {
      const token = await createInvite()
      setInviteLink(`${window.location.origin}/invite/${token}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create invite")
    } finally {
      setInviteLoading(false)
    }
  }

  async function handleCopy() {
    await navigator.clipboard.writeText(inviteLink)
    setCopied(true)
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
            <Button variant="outline" size="sm" onClick={handleInvite} disabled={inviteLoading}>
              <UserPlus className="mr-1.5 h-4 w-4" />
              {inviteLoading ? "Generating…" : "Invite teammate"}
            </Button>
            <Button variant="ghost" size="sm" asChild>
              <Link href="/download">Download CLI</Link>
            </Button>
            <Button variant="ghost" size="sm" onClick={handleLogout}>
              Sign out
            </Button>
          </div>
        </div>

        {inviteLink && (
          <div className="flex items-center gap-2">
            <Input value={inviteLink} readOnly className="font-mono text-xs" />
            <Button size="sm" variant="outline" onClick={handleCopy}>
              {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
            </Button>
          </div>
        )}

        {error && <p className="text-destructive text-sm">{error}</p>}

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

          {envs.length === 0 && !error && (
            <p className="text-muted-foreground text-sm">No environments yet.</p>
          )}
        </div>
      </div>
    </div>
  )
}
