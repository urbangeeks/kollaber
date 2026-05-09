"use client"

import { useState } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import Link from "next/link"

export default function SSOLoginPage() {
  const [slug, setSlug] = useState("")

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!slug.trim()) return
    window.location.href = `/api/auth/sso?org=${encodeURIComponent(slug.trim())}`
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <div className="w-full max-w-sm space-y-6">
        <div className="space-y-1">
          <h1 className="text-2xl font-semibold">Sign in with SSO</h1>
          <p className="text-sm text-muted-foreground">Enter your organisation slug to continue.</p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="slug">Organisation slug</Label>
            <Input
              id="slug"
              placeholder="acme"
              value={slug}
              onChange={(e) => setSlug(e.target.value)}
              autoFocus
            />
          </div>
          <Button type="submit" className="w-full" disabled={!slug.trim()}>
            Continue with SSO
          </Button>
        </form>

        <p className="text-center text-xs text-muted-foreground">
          <Link href="/login" className="underline underline-offset-2">Back to login</Link>
        </p>
      </div>
    </div>
  )
}
