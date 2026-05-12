"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import Image from "next/image"
import { sendOTP, verifyOTP, setToken } from "@/lib/api"
import { otpEmailSchema, otpCodeSchema } from "@/lib/schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { GitHubButton } from "@/components/github-button"
import { Separator } from "@/components/ui/separator"

export default function LoginPage() {
  const router = useRouter()
  const [step, setStep] = useState<"email" | "code">("email")
  const [email, setEmail] = useState("")
  const [code, setCode] = useState("")
  const [emailError, setEmailError] = useState("")
  const [codeError, setCodeError] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

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

  async function handleVerifyCode(e: React.FormEvent) {
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
      const { token } = await verifyOTP(email, result.data.code)
      setToken(token)
      router.push("/dashboard")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Invalid or expired code")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4">
      <Link href="/" className="transition-opacity hover:opacity-80">
        <Image src="/mascot-head.png" alt="Kollaber" width={200} height={118} style={{ height: "auto" }} priority />
      </Link>
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="text-2xl">Kollaber</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <GitHubButton label="Continue with GitHub" />
          <div className="flex items-center gap-3">
            <Separator className="flex-1" />
            <span className="text-muted-foreground text-xs">or</span>
            <Separator className="flex-1" />
          </div>

          {step === "email" ? (
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
              <p className="text-muted-foreground text-center text-sm">
                Don&apos;t have an account?{" "}
                <Link href="/register" className="text-foreground underline underline-offset-4">
                  Register
                </Link>
              </p>
            </form>
          ) : (
            <form onSubmit={handleVerifyCode} className="space-y-4">
              <p className="text-muted-foreground text-sm">
                We sent a 6-digit code to <span className="text-foreground font-medium">{email}</span>.
              </p>
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
                {loading ? "Verifying…" : "Sign in"}
              </Button>
              <button
                type="button"
                className="text-muted-foreground w-full text-center text-sm hover:text-foreground"
                onClick={() => { setStep("email"); setCode(""); setError("") }}
              >
                Use a different email
              </button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
