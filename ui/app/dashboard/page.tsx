"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { getEnvironments, getToken, removeToken, type Environment } from "@/lib/api"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Server } from "lucide-react"

export default function DashboardPage() {
  const router = useRouter()
  const [envs, setEnvs] = useState<Environment[]>([])
  const [error, setError] = useState("")

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login")
      return
    }
    getEnvironments()
      .then(setEnvs)
      .catch((err) => setError(err.message))
  }, [router])

  function handleLogout() {
    removeToken()
    router.push("/login")
  }

  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold">Environments</h1>
          <Button variant="ghost" size="sm" onClick={handleLogout}>
            Sign out
          </Button>
        </div>

        {error && <p className="text-destructive text-sm">{error}</p>}

        <div className="grid gap-4">
          {envs.map((env) => (
            <Card
              key={env.id}
              className="cursor-pointer hover:bg-muted/50 transition-colors"
              onClick={() => router.push(`/env/${env.id}`)}
            >
              <CardHeader className="flex flex-row items-center gap-3 pb-2">
                <Server className="text-muted-foreground h-5 w-5" />
                <CardTitle className="text-base">{env.name}</CardTitle>
                {env.cluster_name && (
                  <Badge variant="secondary" className="ml-auto">
                    {env.cluster_name}
                  </Badge>
                )}
              </CardHeader>
              <CardContent>
                <p className="text-muted-foreground text-xs">
                  Created {new Date(env.created_at).toLocaleDateString()}
                </p>
              </CardContent>
            </Card>
          ))}

          {envs.length === 0 && !error && (
            <p className="text-muted-foreground text-sm">No environments yet.</p>
          )}
        </div>
      </div>
    </div>
  )
}
