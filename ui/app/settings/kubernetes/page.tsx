"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getBillingStatus, type BillingStatus } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Copy, Check } from "lucide-react"
import Link from "next/link"

const SAMPLE_MANIFEST = (apiURL: string, tokenPlaceholder: string) => `apiVersion: v1
kind: Secret
metadata:
  name: kollaber-token
  namespace: kollaber
stringData:
  token: "${tokenPlaceholder}"
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kube-watcher
  namespace: kollaber
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kube-watcher
  template:
    metadata:
      labels:
        app: kube-watcher
    spec:
      serviceAccountName: kube-watcher
      containers:
        - name: kube-watcher
          image: ghcr.io/urbangeeks/kollaber/kube-watcher:latest
          args:
            - --env=prod
            - --api=${apiURL}
          env:
            - name: KOLLABER_TOKEN
              valueFrom:
                secretKeyRef:
                  name: kollaber-token
                  key: token
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kube-watcher
  namespace: kollaber
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kube-watcher
rules:
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-watcher
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kube-watcher
subjects:
  - kind: ServiceAccount
    name: kube-watcher
    namespace: kollaber`

function CodeBlock({ code }: { code: string }) {
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    await navigator.clipboard.writeText(code)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="relative">
      <pre className="rounded-md bg-muted p-4 text-xs font-mono overflow-x-auto whitespace-pre leading-relaxed">
        {code}
      </pre>
      <Button
        type="button"
        size="icon"
        variant="ghost"
        className="absolute right-2 top-2 h-7 w-7"
        onClick={handleCopy}
      >
        {copied ? <Check className="h-3.5 w-3.5" /> : <Copy className="h-3.5 w-3.5" />}
      </Button>
    </div>
  )
}

export default function KubernetesSettingsPage() {
  const router = useRouter()
  const [billing, setBilling] = useState<BillingStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [upgradeRequired, setUpgradeRequired] = useState(false)

  const [apiBase, setApiBase] = useState("https://kollaber.io")

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    setApiBase(window.location.origin.replace(/:3000$/, ":8080"))
    getBillingStatus()
      .then(setBilling)
      .catch((err) => {
        if (err.message?.includes("402")) {
          setUpgradeRequired(true)
        } else {
          toast.error(err.message)
        }
      })
      .finally(() => setLoading(false))
  }, [router])

  const isPro = billing?.entitlements?.kubernetes_ingestion === true

  if (loading) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-xl font-semibold">Kubernetes Auto-Ingestion</h1>
          <p className="text-sm text-muted-foreground mt-1">Loading…</p>
        </div>
      </div>
    )
  }

  if (upgradeRequired || !isPro) {
    return (
      <div className="space-y-6">
        <div>
          <h1 className="text-xl font-semibold">Kubernetes Auto-Ingestion</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Automatically capture deployments and crash alerts from your Kubernetes clusters.
          </p>
        </div>
        <div className="rounded-md border border-dashed px-6 py-8 text-center space-y-2">
          <p className="text-sm font-medium">Kubernetes auto-ingestion is available on the Pro plan</p>
          <p className="text-xs text-muted-foreground">
            Upgrade to automatically ingest Deployment rollouts and CrashLoopBackOff alerts.
          </p>
          <Link href="/settings/billing" className="text-primary text-xs underline underline-offset-2">
            View billing
          </Link>
        </div>
      </div>
    )
  }

  const manifest = SAMPLE_MANIFEST(apiBase, "<YOUR_TOKEN>")
  const cliCommand = `kube-watcher \\
  --env prod \\
  --api ${apiBase} \\
  --token <YOUR_TOKEN> \\
  --namespace ""   # empty = all namespaces`

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Kubernetes Auto-Ingestion</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Run <code className="text-xs bg-muted px-1 py-0.5 rounded">kube-watcher</code> to automatically send Deployment rollouts and CrashLoopBackOff alerts to Kollaber.
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Step 1 — Generate a CLI token</CardTitle>
          <CardDescription>
            Go to your profile and generate a CLI token. This authenticates kube-watcher with the Kollaber API.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            In the top-right menu, click <strong>Generate CLI token</strong>. Copy the token — you will not see it again.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Step 2 — Run kube-watcher</CardTitle>
          <CardDescription>
            Download the binary and run it pointed at your cluster. It uses your current kubeconfig by default.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Install</p>
            <CodeBlock code="go install github.com/urbangeeks/kollaber/cmd/kube-watcher@latest" />
          </div>
          <div className="space-y-1.5">
            <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">Run</p>
            <CodeBlock code={cliCommand} />
          </div>
          <p className="text-xs text-muted-foreground">
            kube-watcher fires a <code className="bg-muted px-1 rounded">deploy</code> event on each completed Deployment rollout and an <code className="bg-muted px-1 rounded">alert</code> event when a Pod enters CrashLoopBackOff.
          </p>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Step 3 (optional) — Run in-cluster</CardTitle>
          <CardDescription>
            Deploy kube-watcher as a Kubernetes Deployment so it runs continuously inside your cluster.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <p className="text-xs text-muted-foreground">
            Apply the manifest below. Replace <code className="bg-muted px-1 rounded">&lt;YOUR_TOKEN&gt;</code> in the Secret with your CLI token before applying.
          </p>
          <CodeBlock code={manifest} />
          <CodeBlock code={`kubectl apply -f kube-watcher.yaml`} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Environment mapping</CardTitle>
          <CardDescription>
            kube-watcher maps Kubernetes clusters to Kollaber environments by name.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            The <code className="text-xs bg-muted px-1 py-0.5 rounded">--env</code> flag must match the exact name of a Kollaber environment (e.g. <code className="text-xs bg-muted px-1 py-0.5 rounded">prod</code>, <code className="text-xs bg-muted px-1 py-0.5 rounded">staging</code>).
            Run one kube-watcher instance per cluster, each pointing at the appropriate environment.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
