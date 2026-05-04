"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { getAdminOrgs, isAdmin, type OrgStat } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ArrowLeft, Building2 } from "lucide-react"

export default function AdminPage() {
  const router = useRouter()
  const [orgs, setOrgs] = useState<OrgStat[]>([])
  const [error, setError] = useState("")

  useEffect(() => {
    if (!isAdmin()) {
      router.replace("/dashboard")
      return
    }
    getAdminOrgs()
      .then(setOrgs)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load orgs"))
  }, [router])

  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" size="icon" onClick={() => router.push("/dashboard")}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <h1 className="text-2xl font-semibold">Admin — All Orgs</h1>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <div className="grid gap-4">
          {orgs.map((org) => (
            <Card key={org.id}>
              <CardHeader className="flex flex-row items-center gap-3 pb-2">
                <Building2 className="text-muted-foreground h-5 w-5" />
                <CardTitle className="text-base">{org.name}</CardTitle>
                <Badge variant="secondary" className="ml-auto font-mono text-xs">
                  {org.slug}
                </Badge>
              </CardHeader>
              <CardContent className="flex gap-6">
                <Stat label="Members" value={org.member_count} />
                <Stat label="Environments" value={org.env_count} />
                <span className="text-muted-foreground ml-auto self-end text-xs">
                  Created {new Date(org.created_at).toLocaleDateString()}
                </span>
              </CardContent>
            </Card>
          ))}

          {orgs.length === 0 && !error && (
            <p className="text-muted-foreground text-sm">No orgs yet.</p>
          )}
        </div>
      </div>
    </div>
  )
}

function Stat({ label, value }: { label: string; value: number }) {
  return (
    <div>
      <p className="text-2xl font-semibold">{value}</p>
      <p className="text-muted-foreground text-xs">{label}</p>
    </div>
  )
}
