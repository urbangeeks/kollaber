"use client"

import { useState } from "react"
import { toast } from "sonner"
import { generateCLIToken } from "@/lib/api"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { Check, Copy, Download } from "lucide-react"

const REPO = "https://github.com/urbangeeks/kollaber"
const LATEST = `${REPO}/releases/latest`

const PLATFORMS = [
  { label: "macOS (Apple Silicon)", os: "Darwin",  arch: "arm64", ext: "tar.gz", badge: "M1/M2/M3" },
  { label: "macOS (Intel)",         os: "Darwin",  arch: "amd64", ext: "tar.gz", badge: "Intel"    },
  { label: "Linux (x86-64)",        os: "Linux",   arch: "amd64", ext: "tar.gz", badge: "amd64"    },
  { label: "Linux (ARM64)",         os: "Linux",   arch: "arm64", ext: "tar.gz", badge: "arm64"    },
  { label: "Windows (x86-64)",      os: "Windows", arch: "amd64", ext: "zip",    badge: "amd64"    },
]

// The API the CLI should be pointed at: the configured backend in local dev,
// otherwise wherever this page is being served from, since the single binary
// serves both.
function apiBase() {
  if (process.env.NEXT_PUBLIC_API_URL) return process.env.NEXT_PUBLIC_API_URL
  if (typeof window !== "undefined") return window.location.origin
  return "https://kollaber.io"
}

// CliDownload owns the button and its dialog together, so any page can carry it
// by rendering one element. It previously lived inline in the dashboard, which
// meant the CLI was only reachable from the one page someone lands on first —
// and not from the timeline, where the thought "I should script this" actually
// occurs.
export function CliDownload() {
  const [open, setOpen] = useState(false)
  const [token, setToken] = useState("")
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(false)

  const loginCommand = `kollaber login --api ${apiBase()} --token ${token}`

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        className="hidden sm:inline-flex"
        onClick={() => {
          // A fresh dialog never shows the previous visit's token.
          setToken("")
          setOpen(true)
        }}
      >
        <Download className="mr-1.5 h-4 w-4" />
        CLI
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Download Kollaber CLI</DialogTitle>
            <DialogDescription>
              Send deploy events, add notes, and view the timeline from your
              terminal or CI pipeline.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">
                Binaries
              </p>
              <div className="divide-y rounded-md border">
                {PLATFORMS.map((p) => (
                  <div
                    key={`${p.os}-${p.arch}`}
                    className="flex items-center gap-3 px-3 py-2"
                  >
                    <span className="flex-1 text-sm">{p.label}</span>
                    <Badge variant="secondary" className="shrink-0">
                      {p.badge}
                    </Badge>
                    <Button size="sm" variant="outline" asChild>
                      <a href={LATEST} target="_blank" rel="noopener noreferrer">
                        .{p.ext}
                      </a>
                    </Button>
                  </div>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">Go</p>
              <pre className="overflow-x-auto rounded bg-muted p-3 text-xs">
                go install github.com/urbangeeks/kollaber/cmd/kollaber@latest
              </pre>
            </div>

            <div className="space-y-2">
              <p className="text-xs font-medium tracking-wide uppercase">
                Authenticate CLI
              </p>
              <p className="text-xs text-muted-foreground">
                If you signed in with GitHub, use a token instead of a password.
              </p>
              {token ? (
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <Input value={loginCommand} readOnly className="font-mono text-xs" />
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={async () => {
                        await navigator.clipboard.writeText(loginCommand)
                        setCopied(true)
                        toast.success("Copied")
                        setTimeout(() => setCopied(false), 2000)
                      }}
                    >
                      {copied ? (
                        <Check className="h-4 w-4" />
                      ) : (
                        <Copy className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Valid for 90 days. Treat it like a password.
                  </p>
                </div>
              ) : (
                <Button
                  size="sm"
                  variant="outline"
                  disabled={loading}
                  onClick={async () => {
                    setLoading(true)
                    try {
                      setToken(await generateCLIToken())
                    } catch (err) {
                      toast.error(
                        err instanceof Error ? err.message : "Failed to generate token",
                      )
                    } finally {
                      setLoading(false)
                    }
                  }}
                >
                  {loading ? "Generating…" : "Generate CLI token"}
                </Button>
              )}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)}>
              Close
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
