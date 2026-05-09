"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { getToken, getAuditLogs, type AuditLogEntry } from "@/lib/api"
import Link from "next/link"

const ACTION_LABEL: Record<string, string> = {
  "member.role_changed": "Role changed",
  "member.removed":      "Member removed",
  "invite.created":      "Invite created",
}

function actionLabel(action: string) {
  return ACTION_LABEL[action] ?? action
}

function MetaBadges({ meta }: { meta: Record<string, unknown> }) {
  const entries = Object.entries(meta).filter(([, v]) => v !== null && v !== "")
  if (entries.length === 0) return null
  return (
    <span className="flex flex-wrap gap-1">
      {entries.map(([k, v]) => (
        <span key={k} className="bg-muted rounded px-1.5 py-0.5 text-[10px] text-muted-foreground">
          {k}: {String(v)}
        </span>
      ))}
    </span>
  )
}

export default function AuditLogsPage() {
  const router = useRouter()
  const [logs, setLogs] = useState<AuditLogEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [upgradeRequired, setUpgradeRequired] = useState(false)

  useEffect(() => {
    if (!getToken()) { router.replace("/login"); return }
    getAuditLogs()
      .then(setLogs)
      .catch((err) => {
        if (err.message?.includes("402") || err.message?.toLowerCase().includes("pro")) {
          setUpgradeRequired(true)
        } else {
          toast.error(err.message)
        }
      })
      .finally(() => setLoading(false))
  }, [router])

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold">Audit Logs</h1>
        <p className="text-sm text-muted-foreground mt-1">
          A record of administrative actions in your organisation.
        </p>
      </div>

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading…</p>
      ) : upgradeRequired ? (
        <div className="rounded-md border border-dashed px-6 py-8 text-center space-y-2">
          <p className="text-sm font-medium">Audit logs are available on the Pro plan</p>
          <p className="text-xs text-muted-foreground">Upgrade to access a full history of admin actions.</p>
          <Link href="/settings/billing" className="text-primary text-xs underline underline-offset-2">
            View billing
          </Link>
        </div>
      ) : logs.length === 0 ? (
        <p className="text-sm text-muted-foreground">No audit log entries yet.</p>
      ) : (
        <div className="rounded-md border divide-y">
          {logs.map((log) => (
            <div key={log.id} className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-4 px-4 py-3">
              <div className="flex-1 min-w-0 space-y-0.5">
                <p className="text-sm font-medium">{actionLabel(log.action)}</p>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs text-muted-foreground">{log.actor_email}</span>
                  <MetaBadges meta={log.metadata} />
                </div>
              </div>
              <time className="text-xs text-muted-foreground shrink-0 sm:pt-0.5">
                {new Date(log.created_at).toLocaleDateString([], { month: "short", day: "numeric", year: "numeric" })}{" "}
                <span className="hidden sm:inline">
                  {new Date(log.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                </span>
              </time>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
