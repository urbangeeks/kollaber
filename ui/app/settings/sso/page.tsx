"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getCurrentRole, getSSOConfig, saveSSOConfig, type SSOConfig } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Copy, Check } from "lucide-react"
import Link from "next/link"

export default function SSOSettingsPage() {
  const router = useRouter()
  const [cfg, setCfg] = useState<SSOConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [upgradeRequired, setUpgradeRequired] = useState(false)
  const [copied, setCopied] = useState(false)
  const [isAdminOrOwner, setIsAdminOrOwner] = useState(false)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    const role = getCurrentRole()
    setIsAdminOrOwner(role === "owner" || role === "admin")
    getSSOConfig()
      .then(setCfg)
      .catch((err) => {
        if (err.message?.includes("402") || err.message?.toLowerCase().includes("pro")) {
          setUpgradeRequired(true)
        } else {
          toast.error(err.message)
        }
      })
      .finally(() => setLoading(false))
  }, [router])

  async function handleSave(e: React.FormEvent) {
    e.preventDefault()
    if (!cfg) return
    setSaving(true)
    try {
      await saveSSOConfig({
        issuer_url:    cfg.issuer_url,
        client_id:     cfg.client_id,
        client_secret: cfg.client_secret,
        domain:        cfg.domain,
        enabled:       cfg.enabled,
      })
      toast.success("SSO configuration saved")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save SSO config")
    } finally {
      setSaving(false)
    }
  }

  async function handleCopy() {
    if (!cfg?.callback_url) return
    await navigator.clipboard.writeText(cfg.callback_url)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  function set(key: keyof SSOConfig, value: string | boolean) {
    setCfg((prev) => prev ? { ...prev, [key]: value } : prev)
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Single Sign-On</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Configure OIDC-based SSO for your organisation.
        </p>
      </div>

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : upgradeRequired ? (
        <div className="rounded-md border border-dashed px-6 py-8 text-center space-y-2">
          <p className="text-sm font-medium">SSO is available on the Pro plan</p>
          <p className="text-xs text-muted-foreground">Upgrade to configure OIDC single sign-on.</p>
          <Link href="/settings/billing" className="text-primary text-xs underline underline-offset-2">
            View billing
          </Link>
        </div>
      ) : cfg ? (
        <form onSubmit={handleSave} className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle className="text-base">OIDC Configuration</CardTitle>
              <CardDescription>
                Works with Okta, Google Workspace, Azure AD, Auth0, and any OIDC-compatible provider.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-1.5">
                <Label htmlFor="issuer_url">Issuer URL</Label>
                <Input
                  id="issuer_url"
                  placeholder="https://accounts.google.com"
                  value={cfg.issuer_url}
                  onChange={(e) => set("issuer_url", e.target.value)}
                  disabled={!isAdminOrOwner}
                />
                <p className="text-[11px] text-muted-foreground">
                  The OIDC discovery URL root (without <code>/.well-known/openid-configuration</code>).
                </p>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="client_id">Client ID</Label>
                <Input
                  id="client_id"
                  value={cfg.client_id}
                  onChange={(e) => set("client_id", e.target.value)}
                  disabled={!isAdminOrOwner}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="client_secret">Client Secret</Label>
                <Input
                  id="client_secret"
                  type="password"
                  value={cfg.client_secret}
                  onChange={(e) => set("client_secret", e.target.value)}
                  disabled={!isAdminOrOwner}
                  placeholder="Leave blank to keep existing secret"
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="domain">Email domain</Label>
                <Input
                  id="domain"
                  placeholder="acme.com"
                  value={cfg.domain}
                  onChange={(e) => set("domain", e.target.value)}
                  disabled={!isAdminOrOwner}
                />
                <p className="text-[11px] text-muted-foreground">
                  Only users with this email domain can sign in via SSO.
                </p>
              </div>

              <div className="flex items-center gap-2">
                <input
                  id="enabled"
                  type="checkbox"
                  checked={cfg.enabled}
                  onChange={(e) => set("enabled", e.target.checked)}
                  disabled={!isAdminOrOwner}
                  className="h-4 w-4 rounded border-border"
                />
                <Label htmlFor="enabled" className="cursor-pointer">SSO enabled</Label>
              </div>
            </CardContent>
          </Card>

          {cfg.callback_url && (
            <Card>
              <CardHeader>
                <CardTitle className="text-base">Callback URL</CardTitle>
                <CardDescription>
                  Register this URL as the redirect URI in your identity provider.
                </CardDescription>
              </CardHeader>
              <CardContent>
                <div className="flex items-center gap-2">
                  <Input value={cfg.callback_url} readOnly className="font-mono text-xs" />
                  <Button type="button" size="icon" variant="outline" onClick={handleCopy}>
                    {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                  </Button>
                </div>
              </CardContent>
            </Card>
          )}

          {isAdminOrOwner && (
            <Button type="submit" disabled={saving}>
              {saving ? "Saving…" : "Save configuration"}
            </Button>
          )}
        </form>
      ) : null}
    </div>
  )
}
