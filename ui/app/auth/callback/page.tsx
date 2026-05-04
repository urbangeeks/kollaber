"use client"

import { useEffect } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { setToken } from "@/lib/api"
import { Suspense } from "react"

function CallbackInner() {
  const router = useRouter()
  const params = useSearchParams()

  useEffect(() => {
    const token = params.get("token")
    const isNew = params.get("new") === "true"

    if (!token) {
      router.replace("/login?error=missing_token")
      return
    }

    setToken(token)
    router.replace(isNew ? "/onboarding" : "/dashboard")
  }, [params, router])

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-muted-foreground text-sm">Signing you in…</p>
    </div>
  )
}

export default function CallbackPage() {
  return (
    <Suspense>
      <CallbackInner />
    </Suspense>
  )
}
