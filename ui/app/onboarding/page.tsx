"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import { createEnvironment, getToken } from "@/lib/api"
import { createEnvSchema } from "@/lib/schemas"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { CheckCircle } from "lucide-react"

type Step = "create-env" | "quickstart"

export default function OnboardingPage() {
  const router = useRouter()
  const [step, setStep] = useState<Step>("create-env")
  const [name, setName] = useState("")
  const [clusterName, setClusterName] = useState("")
  const [fieldErrors, setFieldErrors] = useState<{ name?: string; clusterName?: string }>({})
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)
  const [envName, setEnvName] = useState("")

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
      setFieldErrors({ name: flat.name?.[0], clusterName: flat.clusterName?.[0] })
      return
    }
    setFieldErrors({})
    setLoading(true)
    try {
      const env = await createEnvironment(result.data.name, result.data.clusterName)
      setEnvName(env.name)
      setStep("quickstart")
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create environment")
    } finally {
      setLoading(false)
    }
  }

  if (step === "quickstart") {
    return (
      <div className="flex min-h-screen items-center justify-center p-8">
        <Card className="w-full max-w-lg">
          <CardHeader>
            <div className="flex items-center gap-2">
              <CheckCircle className="text-green-500 h-5 w-5" />
              <CardTitle>You&apos;re set up!</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="space-y-4">
            <p className="text-muted-foreground text-sm">
              Your <span className="text-foreground font-medium">{envName}</span> environment is ready.
              Install the CLI and send your first event.
            </p>

            <div className="space-y-2">
              <p className="text-xs font-medium uppercase tracking-wide">Install CLI</p>
              <pre className="bg-muted rounded p-3 text-xs">go install github.com/urbangeeks/kollaber/cmd/kollaber@latest</pre>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium uppercase tracking-wide">Send a deploy event</p>
              <pre className="bg-muted rounded p-3 text-xs leading-relaxed">
{`kollaber login --email you@example.com --password yourpassword
kollaber deploy --env ${envName} --service api --version v1.0.0`}
              </pre>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium uppercase tracking-wide">Add a note</p>
              <pre className="bg-muted rounded p-3 text-xs">{`kollaber note --env ${envName} "Deployed to production"`}</pre>
            </div>

            <Button className="w-full" onClick={() => router.push("/dashboard")}>
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
                required
              />
              {fieldErrors.name && <p className="text-destructive text-xs">{fieldErrors.name}</p>}
            </div>
            <div className="space-y-1">
              <Label htmlFor="clusterName">Cluster name <span className="text-muted-foreground">(optional)</span></Label>
              <Input
                id="clusterName"
                placeholder="prod-cluster"
                value={clusterName}
                onChange={(e) => setClusterName(e.target.value)}
              />
              {fieldErrors.clusterName && <p className="text-destructive text-xs">{fieldErrors.clusterName}</p>}
            </div>
            {error && <p className="text-destructive text-sm">{error}</p>}
            <Button type="submit" className="w-full" disabled={loading}>
              {loading ? "Creating…" : "Create environment"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
