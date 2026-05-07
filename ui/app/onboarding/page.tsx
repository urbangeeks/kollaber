"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { createEnvironment, generateCLIToken, getToken } from "@/lib/api"
import { createEnvSchema } from "@/lib/schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { CheckCircle, Copy, Check } from "lucide-react"

type Step = "create-env" | "quickstart"

function CopyBlock({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  async function copy() {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }
  return (
    <div className="flex items-center gap-2 rounded bg-muted p-3">
      <code className="flex-1 overflow-x-auto whitespace-pre text-xs leading-relaxed">{text}</code>
      <button
        onClick={copy}
        className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
        title="Copy"
      >
        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      </button>
    </div>
  )
}

export default function OnboardingPage() {
  const router = useRouter()
  const [step, setStep] = useState<Step>("create-env")
  const [name, setName] = useState("")
  const [clusterName, setClusterName] = useState("")
  const [fieldErrors, setFieldErrors] = useState<{
    name?: string
    clusterName?: string
  }>({})
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [envName, setEnvName] = useState("")
  const [cliToken, setCliToken] = useState("")

  if (!getToken()) {
    router.replace("/login")
    return null
  }

  async function handleCreateEnv(e: React.FormEvent) {
    e.preventDefault()
    setError("")

    const result = createEnvSchema.safeParse({ name, clusterName })
    if (!result.success) {
      const flat = result.error.flatten().fieldErrors
      setFieldErrors({
        name: flat.name?.[0],
        clusterName: flat.clusterName?.[0],
      })
      return
    }
    setFieldErrors({})
    setLoading(true)
    try {
      const env = await createEnvironment(
        result.data.name,
        result.data.clusterName
      )
      setEnvName(env.name)
      const tok = await generateCLIToken()
      setCliToken(tok)
      setStep("quickstart")
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to create environment"
      )
    } finally {
      setLoading(false)
    }
  }

  if (step === "quickstart") {
    const apiBase = process.env.NEXT_PUBLIC_API_URL ?? (typeof window !== "undefined" ? window.location.origin : "https://kollaber.io")
    const loginCmd = `kollaber login --api ${apiBase} --token ${cliToken}`
    const deployCmd = `kollaber deploy --env ${envName} --service api --version v1.0.0`
    const noteCmd = `kollaber note --env ${envName} "Deployed to production"`

    return (
      <div className="flex min-h-screen items-start justify-center overflow-y-auto p-8 py-16">
        <Card className="w-full max-w-lg">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CheckCircle className="h-5 w-5 text-green-500" />
              <CardTitle>You&apos;re set up!</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="space-y-5">
            <p className="text-sm text-muted-foreground">
              Your{" "}
              <span className="font-medium text-foreground">{envName}</span>{" "}
              environment is ready. Install the CLI and send your first event.
            </p>

            <div className="space-y-3">
              <p className="text-xs font-medium tracking-wide uppercase">
                Install CLI
              </p>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">macOS / Linux</p>
                <CopyBlock text="curl -sSfL https://raw.githubusercontent.com/urbangeeks/kollaber/master/install.sh | sh" />
              </div>
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Go</p>
                <CopyBlock text="go install github.com/urbangeeks/kollaber/cmd/kollaber@latest" />
              </div>
              <p className="text-xs text-muted-foreground">
                Windows and other platforms:{" "}
                <Link href="/download" className="text-foreground underline underline-offset-2">
                  download page
                </Link>
              </p>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">Authenticate</p>
              <CopyBlock text={loginCmd} />
              <p className="text-xs text-muted-foreground">This token is valid for 90 days.</p>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">Send a deploy event</p>
              <CopyBlock text={deployCmd} />
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">Add a note</p>
              <CopyBlock text={noteCmd} />
            </div>

            <Button
              className="w-full"
              onClick={() => router.push("/dashboard")}
            >
              Go to dashboard
            </Button>
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex min-h-screen items-center justify-center p-8">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Create your first environment</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCreateEnv} className="space-y-4">
            <div className="space-y-1">
              <Label htmlFor="name">Environment name</Label>
              <Input
                id="name"
                placeholder="prod"
                value={name}
                onChange={(e) => setName(e.target.value)}
                autoFocus
              />
              {fieldErrors.name && (
                <p className="text-xs text-destructive">{fieldErrors.name}</p>
              )}
            </div>
            <div className="space-y-1">
              <Label htmlFor="clusterName">
                Cluster name{" "}
                <span className="text-muted-foreground">(optional)</span>
              </Label>
              <Input
                id="clusterName"
                placeholder="prod-cluster"
                value={clusterName}
                onChange={(e) => setClusterName(e.target.value)}
              />
              {fieldErrors.clusterName && (
                <p className="text-xs text-destructive">
                  {fieldErrors.clusterName}
                </p>
              )}
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Creating…" : "Create environment"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
