"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { getInvite, acceptInvite, joinInvite, setToken, getToken } from "@/lib/api"
import { loginSchema } from "@/lib/schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

type InviteInfo = { org_name: string; expires_at: string }

function InvitePageInner() {
  const searchParams = useSearchParams()
  const token = searchParams.get("token") ?? ""
  const router = useRouter()
  const [invite, setInvite] = useState<InviteInfo | null>(null)
  const [loadError, setLoadError] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [fieldErrors, setFieldErrors] = useState<{ email?: string; password?: string }>({})
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const isLoggedIn = Boolean(getToken())

  useEffect(() => {
    if (!token) {
      setLoadError("No invite token provided")
      return
    }
    getInvite(token)
      .then(setInvite)
      .catch((err) => setLoadError(err instanceof Error ? err.message : "Invalid invite"))
  }, [token])

  async function handleJoin() {
    setLoading(true)
    try {
      const jwt = await joinInvite(token)
      setToken(jwt)
      router.push("/dashboard")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join org")
    } finally {
      setLoading(false)
    }
  }

  async function handleRegisterAndJoin(e: React.FormEvent) {
    e.preventDefault()
    setError("")

    const result = loginSchema.safeParse({ email, password })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setFieldErrors({ email: flat.email?.[0], password: flat.password?.[0] })
      return
    }
    setFieldErrors({})

    setLoading(true)
    try {
      const jwt = await acceptInvite(token, result.data.email, result.data.password)
      setToken(jwt)
      router.push("/dashboard")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join org")
    } finally {
      setLoading(false)
    }
  }

  if (loadError) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader><CardTitle>Invalid invite</CardTitle></CardHeader>
          <CardContent><p className="text-muted-foreground text-sm">{loadError}</p></CardContent>
        </Card>
      </div>
    )
  }

  if (!invite) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground text-sm">Loading…</p>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-2xl">Join {invite.org_name}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {isLoggedIn ? (
            <>
              <p className="text-muted-foreground text-sm">
                You&apos;re signed in. Click below to join <span className="text-foreground font-medium">{invite.org_name}</span> and switch to it.
              </p>
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button className="w-full" onClick={handleJoin} disabled={loading}>
                {loading ? "Joining…" : `Join ${invite.org_name}`}
              </Button>
            </>
          ) : (
            <>
              <p className="text-muted-foreground text-sm">
                Create an account to join <span className="text-foreground font-medium">{invite.org_name}</span>.
              </p>
              <form onSubmit={handleRegisterAndJoin} className="space-y-4">
                <div className="space-y-1">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    autoComplete="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                  />
                  {fieldErrors.email && <p className="text-destructive text-xs">{fieldErrors.email}</p>}
                </div>
                <div className="space-y-1">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    autoComplete="new-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                  />
                  {fieldErrors.password && <p className="text-destructive text-xs">{fieldErrors.password}</p>}
                </div>
                {error && <p className="text-destructive text-sm">{error}</p>}
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? "Joining…" : `Join ${invite.org_name}`}
                </Button>
              </form>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default function InvitePage() {
  return <Suspense><InvitePageInner /></Suspense>
}
