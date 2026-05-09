"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getTeamsSettings, updateTeamsSettings, testTeamsSettings, getCurrentRole } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ExternalLink } from "lucide-react"

export default function TeamsSettingsPage() {
  const router = useRouter()
  const [webhookUrl, setWebhookUrl] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [canEdit, setCanEdit] = useState(false)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    const role = getCurrentRole()
    setCanEdit(role === "owner" || role === "admin")
    getTeamsSettings()
      .then(setWebhookUrl)
      .catch(() => toast.error("Failed to load Teams settings"))
      .finally(() => setLoading(false))
  }, [])

  async function save() {
    setSaving(true)
    try {
      await updateTeamsSettings(webhookUrl)
      toast.success("Teams webhook saved")
    } catch {
      toast.error("Failed to save Teams settings")
    } finally {
      setSaving(false)
    }
  }

  async function test() {
    setTesting(true)
    try {
      await testTeamsSettings()
      toast.success("Test message sent to Teams!")
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
            <CardTitle>Microsoft Teams Integration</CardTitle>
            <p className="text-sm text-muted-foreground">
              Post a message to a Teams channel whenever a deploy, alert, or note is recorded.
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
                    placeholder="https://…webhook.office.com/webhookb2/…"
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
                    <li>Open the channel in Teams and click <strong>···</strong> → <strong>Connectors</strong></li>
                    <li>Search for <strong>Incoming Webhook</strong> and click Configure</li>
                    <li>Give it a name, optionally upload an icon, then click Create</li>
                    <li>Copy the generated webhook URL</li>
                  </ol>
                  <a
                    href="https://learn.microsoft.com/en-us/microsoftteams/platform/webhooks-and-connectors/how-to/add-incoming-webhook"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1 text-xs text-blue-500 hover:underline mt-1"
                  >
                    Microsoft docs <ExternalLink className="h-3 w-3" />
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
