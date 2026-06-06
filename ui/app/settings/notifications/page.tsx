"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getNotificationPrefs, updateNotificationPrefs, getCurrentEmail } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import Link from "next/link"

const EVENT_TYPES = [
  { value: "deploy",   label: "Deployments", description: "When a deploy event is recorded" },
  { value: "alert",    label: "Alerts",      description: "When an alert event is recorded" },
  { value: "note",     label: "Notes",       description: "When a note is added to the timeline" },
  { value: "teardown", label: "Teardowns",   description: "When a workload is removed from a cluster" },
]

export default function NotificationsSettingsPage() {
  const router = useRouter()
  const [notifyOn, setNotifyOn] = useState<string[]>([])
  const [notificationEmail, setNotificationEmail] = useState("")
  const [accountEmail, setAccountEmail] = useState("")
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    setAccountEmail(getCurrentEmail() ?? "")
    getNotificationPrefs()
      .then(({ notifyOn, notificationEmail }) => {
        setNotifyOn(notifyOn)
        setNotificationEmail(notificationEmail)
      })
      .catch(() => toast.error("Failed to load notification preferences"))
      .finally(() => setLoading(false))
  }, [router])

  function toggle(value: string) {
    setNotifyOn(prev =>
      prev.includes(value) ? prev.filter(v => v !== value) : [...prev, value]
    )
  }

  async function save() {
    setSaving(true)
    try {
      await updateNotificationPrefs(notifyOn, notificationEmail)
      toast.success("Notification preferences saved")
    } catch {
      toast.error("Failed to save preferences")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">

        <Card>
          <CardHeader>
            <CardTitle>Email Notifications</CardTitle>
            <p className="text-sm text-muted-foreground">
              Choose which events send you an email for this organisation.
            </p>
          </CardHeader>
          <CardContent className="space-y-4">
            {loading ? (
              <p className="text-sm text-muted-foreground">Loading…</p>
            ) : (
              <>
                <div className="space-y-2">
                  <Label htmlFor="notification_email">Notification email</Label>
                  <Input
                    id="notification_email"
                    type="email"
                    placeholder={accountEmail || "you@example.com"}
                    value={notificationEmail}
                    onChange={(e) => setNotificationEmail(e.target.value)}
                  />
                  <p className="text-xs text-muted-foreground">
                    Leave blank to use your account email ({accountEmail}).
                  </p>
                </div>

                <div className="border-t pt-4 space-y-3">
                  {EVENT_TYPES.map(({ value, label, description }) => (
                    <div key={value} className="flex items-start gap-3">
                      <Checkbox
                        id={value}
                        checked={notifyOn.includes(value)}
                        onCheckedChange={() => toggle(value)}
                      />
                      <div className="grid gap-0.5">
                        <Label htmlFor={value} className="cursor-pointer font-medium">
                          {label}
                        </Label>
                        <p className="text-xs text-muted-foreground">{description}</p>
                      </div>
                    </div>
                  ))}
                </div>

                <div className="pt-2">
                  <Button onClick={save} disabled={saving}>
                    {saving ? "Saving…" : "Save preferences"}
                  </Button>
                </div>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Slack</CardTitle>
            <p className="text-sm text-muted-foreground">
              Post events to a Slack channel for your whole team.
            </p>
          </CardHeader>
          <CardContent>
            <Link href="/settings/slack">
              <Button variant="outline">Configure Slack webhook</Button>
            </Link>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Microsoft Teams</CardTitle>
            <p className="text-sm text-muted-foreground">
              Post events to a Teams channel for your whole team.
            </p>
          </CardHeader>
          <CardContent>
            <Link href="/settings/teams">
              <Button variant="outline">Configure Teams webhook</Button>
            </Link>
          </CardContent>
        </Card>
    </div>
  )
}
