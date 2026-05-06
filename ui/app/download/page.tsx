import Link from "next/link"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"

const REPO = "https://github.com/urbangeeks/kollaber"
const LATEST = `${REPO}/releases/latest`

type Platform = {
  label: string
  os: string
  arch: string
  ext: string
  badge: string
}

const platforms: Platform[] = [
  {
    label: "macOS (Apple Silicon)",
    os: "darwin",
    arch: "arm64",
    ext: "tar.gz",
    badge: "M1/M2/M3",
  },
  {
    label: "macOS (Intel)",
    os: "darwin",
    arch: "amd64",
    ext: "tar.gz",
    badge: "Intel",
  },
  {
    label: "Linux (x86-64)",
    os: "linux",
    arch: "amd64",
    ext: "tar.gz",
    badge: "amd64",
  },
  {
    label: "Linux (ARM64)",
    os: "linux",
    arch: "arm64",
    ext: "tar.gz",
    badge: "arm64",
  },
  {
    label: "Windows (x86-64)",
    os: "windows",
    arch: "amd64",
    ext: "zip",
    badge: "amd64",
  },
]

export default function DownloadPage() {
  return (
    <div className="min-h-screen p-8">
      <div className="mx-auto max-w-2xl space-y-8">
        <div className="space-y-2">
          <h1 className="text-3xl font-semibold">Download Kollaber CLI</h1>
          <p className="text-muted-foreground">
            The CLI lets you send deploy events, add notes, and view the
            timeline from your terminal or CI pipeline.
          </p>
        </div>

        {/* Platform downloads */}
        <div className="space-y-3">
          <h2 className="text-sm font-medium tracking-wide uppercase">
            Download
          </h2>
          {platforms.map((p) => (
            <Card key={`${p.os}-${p.arch}`}>
              <CardHeader className="flex flex-row items-center gap-3 py-3">
                <CardTitle className="text-sm font-medium">{p.label}</CardTitle>
                <Badge variant="secondary" className="ml-auto">
                  {p.badge}
                </Badge>
                <Button size="sm" asChild>
                  <a href={LATEST} target="_blank" rel="noopener noreferrer">
                    Download .{p.ext}
                  </a>
                </Button>
              </CardHeader>
            </Card>
          ))}
        </div>

        {/* Go install */}
        <div className="space-y-3">
          <h2 className="text-sm font-medium tracking-wide uppercase">
            Install with Go
          </h2>
          <pre className="rounded-lg bg-muted p-4 text-sm">
            go install github.com/urbangeeks/kollaber/cmd/kollaber@latest
          </pre>
        </div>

        {/* Quick start */}
        <div className="space-y-3">
          <h2 className="text-sm font-medium tracking-wide uppercase">
            Quick start
          </h2>
          <Card>
            <CardContent className="pt-4">
              <pre className="text-sm leading-relaxed">{`# Log in
kollaber login --email you@example.com --password yourpassword

# List environments
kollaber envs

# Send a deploy event
kollaber deploy --env prod --service api --version v1.2.3

# Add a note
kollaber note --env prod "Deployed new auth flow"

# View the timeline
kollaber timeline --env prod`}</pre>
            </CardContent>
          </Card>
        </div>

        <p className="text-sm text-muted-foreground">
          All releases and changelogs are on{" "}
          <Link
            href={LATEST}
            className="underline underline-offset-4"
            target="_blank"
            rel="noopener noreferrer"
          >
            GitHub Releases
          </Link>
          .
        </p>
      </div>
    </div>
  )
}
