"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getNotificationPrefs, updateNotificationPrefs } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { ArrowLeft } from "lucide-react"

const EVENT_TYPES = [
  { value: "deploy", label: "Deployments", description: "When a deploy event is recorded" },
  { value: "alert",  label: "Alerts",      description: "When an alert event is recorded" },
  { value: "note",   label: "Notes",       description: "When a note is added to the timeline" },
]

export default function NotificationsSettingsPage() {
  const router = useRouter()
  const [notifyOn, setNotifyOn] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  if (!getToken()) {
    router.replace("/login")
    return null
  }

  useEffect(() => {
    getNotificationPrefs()
      .then(setNotifyOn)
      .catch(() => toast.error("Failed to load notification preferences"))
      .finally(() => setLoading(false))
  }, [])

  function toggle(value: string) {
    setNotifyOn(prev =>
      prev.includes(value) ? prev.filter(v => v !== value) : [...prev, value]
    )
  }

  async function save() {
    setSaving(true)
    try {
      await updateNotificationPrefs(notifyOn)
      toast.success("Notification preferences saved")
    } catch {
      toast.error("Failed to save preferences")
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex min-h-screen items-start justify-center p-8 py-16">
      <div className="w-full max-w-lg space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="sm" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="mr-1.5 h-4 w-4" />
            Dashboard
          </Button>
        </div>

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
              EVENT_TYPES.map(({ value, label, description }) => (
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
              ))
            )}

            <div className="pt-2">
              <Button onClick={save} disabled={saving || loading}>
                {saving ? "Saving…" : "Save preferences"}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}
