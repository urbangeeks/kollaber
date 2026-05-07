"use client"

import { Suspense, useEffect, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { getInvite, acceptInvite, joinInvite, sendOTP, setToken, getToken } from "@/lib/api"
import { otpEmailSchema, otpCodeSchema } from "@/lib/schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

type InviteInfo = { org_name: string; expires_at: string }

function InvitePageInner() {
  const searchParams = useSearchParams()
  const inviteToken = searchParams.get("token") ?? ""
  const router = useRouter()
  const [invite, setInvite] = useState<InviteInfo | null>(null)
  const [loadError, setLoadError] = useState("")
  const [step, setStep] = useState<"email" | "code">("email")
  const [email, setEmail] = useState("")
  const [code, setCode] = useState("")
  const [emailError, setEmailError] = useState("")
  const [codeError, setCodeError] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const isLoggedIn = Boolean(getToken())

  useEffect(() => {
    if (!inviteToken) { setLoadError("No invite token provided"); return }
    getInvite(inviteToken)
      .then(setInvite)
      .catch((err) => setLoadError(err instanceof Error ? err.message : "Invalid invite"))
  }, [inviteToken])

  async function handleJoin() {
    setLoading(true)
    try {
      const jwt = await joinInvite(inviteToken)
      setToken(jwt)
      router.push("/dashboard")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to join org")
    } finally {
      setLoading(false)
    }
  }

  async function handleSendCode(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    const result = otpEmailSchema.safeParse({ email })
    if (!result.success) {
      setEmailError(result.error.flatten().fieldErrors.email?.[0] ?? "")
      return
    }
    setEmailError("")
    setLoading(true)
    try {
      await sendOTP(result.data.email)
      setStep("code")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send code")
    } finally {
      setLoading(false)
    }
  }

  async function handleAccept(e: React.FormEvent) {
    e.preventDefault()
    setError("")
    const result = otpCodeSchema.safeParse({ code })
    if (!result.success) {
      setCodeError(result.error.flatten().fieldErrors.code?.[0] ?? "")
      return
    }
    setCodeError("")
    setLoading(true)
    try {
      const jwt = await acceptInvite(inviteToken, email, result.data.code)
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
                You&apos;re signed in. Click below to join{" "}
                <span className="text-foreground font-medium">{invite.org_name}</span> and switch to it.
              </p>
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button className="w-full" onClick={handleJoin} disabled={loading}>
                {loading ? "Joining…" : `Join ${invite.org_name}`}
              </Button>
            </>
          ) : step === "email" ? (
            <>
              <p className="text-muted-foreground text-sm">
                Create an account to join{" "}
                <span className="text-foreground font-medium">{invite.org_name}</span>.
              </p>
              <form onSubmit={handleSendCode} className="space-y-4">
                <div className="space-y-1">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    autoComplete="email"
                    autoFocus
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    required
                  />
                  {emailError && <p className="text-destructive text-xs">{emailError}</p>}
                </div>
                {error && <p className="text-destructive text-sm">{error}</p>}
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? "Sending…" : "Send code"}
                </Button>
              </form>
            </>
          ) : (
            <>
              <p className="text-muted-foreground text-sm">
                We sent a 6-digit code to{" "}
                <span className="text-foreground font-medium">{email}</span>.
              </p>
              <form onSubmit={handleAccept} className="space-y-4">
                <div className="space-y-1">
                  <Label htmlFor="code">Code</Label>
                  <Input
                    id="code"
                    type="text"
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    autoFocus
                    maxLength={6}
                    placeholder="123456"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                    required
                  />
                  {codeError && <p className="text-destructive text-xs">{codeError}</p>}
                </div>
                {error && <p className="text-destructive text-sm">{error}</p>}
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading ? "Joining…" : `Join ${invite.org_name}`}
                </Button>
                <button
                  type="button"
                  className="text-muted-foreground w-full text-center text-sm hover:text-foreground"
                  onClick={() => { setStep("email"); setCode(""); setError("") }}
                >
                  Use a different email
                </button>
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
