"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getSlackSettings, updateSlackSettings, testSlackSettings, getCurrentRole } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ExternalLink } from "lucide-react"

export default function SlackSettingsPage() {
  const router = useRouter()
  const [webhookUrl, setWebhookUrl] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)

  const role = getCurrentRole()
  const canEdit = role === "owner" || role === "admin"

  if (!getToken()) {
    router.replace("/login")
    return null
  }

  useEffect(() => {
    getSlackSettings()
      .then(setWebhookUrl)
      .catch(() => toast.error("Failed to load Slack settings"))
      .finally(() => setLoading(false))
  }, [])

  async function save() {
    setSaving(true)
    try {
      await updateSlackSettings(webhookUrl)
      toast.success("Slack webhook saved")
    } catch {
      toast.error("Failed to save Slack settings")
    } finally {
      setSaving(false)
    }
  }

  async function test() {
    setTesting(true)
    try {
      await testSlackSettings()
      toast.success("Test message sent to Slack!")
    } catch (e: unknown) {
      toast.error(e instanceof Error ? e.message : "Test failed")
    } finally {
      setTesting(false)
    }
  }

  return (
    <div className="space-y-6">

        <Card>
          <CardHeader>
            <CardTitle>Slack Integration</CardTitle>
            <p className="text-sm text-muted-foreground">
              Post a message to a Slack channel whenever a deploy, alert, or note is recorded.
            </p>
          </CardHeader>
          <CardContent className="space-y-5">
            {loading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="webhook">Incoming Webhook URL</Label>
                  <Input
                    id="webhook"
                    type="url"
                    placeholder="https://hooks.slack.com/services/…"
                    value={webhookUrl}
                    onChange={(e) => setWebhookUrl(e.target.value)}
                    disabled={!canEdit}
                  />
                  {!canEdit && (
                    <p className="text-xs text-muted-foreground">Only admins and owners can change this setting.</p>
                  )}
                </div>

                <div className="rounded-md border border-dashed p-4 text-sm text-muted-foreground space-y-1">
                  <p className="font-medium text-foreground">How to get a webhook URL</p>
                  <ol className="list-decimal list-inside space-y-1 text-xs">
                    <li>Go to your Slack workspace's <strong>App Directory</strong></li>
                    <li>Search for and install <strong>Incoming Webhooks</strong></li>
                    <li>Choose a channel and copy the generated webhook URL</li>
                  </ol>
                  <a
                    href="https://api.slack.com/messaging/webhooks"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-blue-500 hover:underline mt-1"
                  >
                    Slack docs <ExternalLink className="h-3 w-3" />
                  </a>
                </div>

                {canEdit && (
                  <div className="flex gap-3">
                    <Button onClick={save} disabled={saving}>
                      {saving ? "Saving…" : "Save"}
                    </Button>
                    {webhookUrl && (
                      <Button variant="outline" onClick={test} disabled={testing}>
                        {testing ? "Sending…" : "Send test message"}
                      </Button>
                    )}
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>
    </div>
  )
}
