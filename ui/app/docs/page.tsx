import Link from "next/link"

const NAV = [
  { id: "getting-started", label: "Getting Started" },
  { id: "dashboard", label: "Dashboard" },
  { id: "cli", label: "CLI Reference" },
  { id: "webhooks", label: "Webhooks" },
  { id: "api", label: "API Reference" },
]

function Code({ children }: { children: string }) {
  return (
    <pre className="my-4 overflow-x-auto rounded-lg border border-white/10 bg-white/5 p-4 font-mono text-sm leading-relaxed text-white/80">
      {children}
    </pre>
  )
}

function InlineCode({ children }: { children: string }) {
  return (
    <code className="rounded bg-white/10 px-1.5 py-0.5 font-mono text-sm text-[#a78bfa]">
      {children}
    </code>
  )
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section id={id} className="scroll-mt-24 py-12 border-b border-white/10 last:border-0">
      <h2 className="text-2xl font-bold mb-6">{title}</h2>
      <div className="space-y-6 text-white/70 leading-relaxed">{children}</div>
    </section>
  )
}

function SubSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mt-8">
      <h3 className="text-lg font-semibold text-white mb-3">{title}</h3>
      <div className="space-y-3">{children}</div>
    </div>
  )
}

function Badge({ children }: { children: string }) {
  return (
    <span className="inline-block rounded border border-white/20 bg-white/5 px-2 py-0.5 font-mono text-xs text-white/60">
      {children}
    </span>
  )
}

function ApiRow({ method, path, desc }: { method: string; path: string; desc: string }) {
  const colors: Record<string, string> = {
    GET: "text-green-400",
    POST: "text-blue-400",
    PUT: "text-yellow-400",
    PATCH: "text-orange-400",
    DELETE: "text-red-400",
  }
  return (
    <div className="flex flex-col gap-1 rounded-lg border border-white/10 bg-white/5 p-4 sm:flex-row sm:items-center sm:gap-4">
      <span className={`shrink-0 font-mono text-sm font-bold ${colors[method] ?? "text-white"}`}>{method}</span>
      <span className="shrink-0 font-mono text-sm text-white/80">{path}</span>
      <span className="text-sm text-white/50">{desc}</span>
    </div>
  )
}

export default function DocsPage() {
  return (
    <div className="min-h-screen bg-[#0a0a0a] text-white font-sans antialiased">

      {/* Nav */}
      <nav className="sticky top-0 z-50 w-full border-b border-white/10 bg-[#0a0a0a]/80 backdrop-blur-md">
        <div className="container mx-auto flex h-16 items-center justify-between px-6">
          <div className="flex items-center gap-6">
            <Link href="/" className="flex items-center gap-2">
              <div className="h-6 w-6 rounded bg-gradient-to-br from-[#60a5fa] to-[#a78bfa]" />
              <span className="text-xl font-bold tracking-tight">Kollaber</span>
            </Link>
            <span className="hidden text-sm text-white/30 sm:block">Docs</span>
          </div>
          <div className="flex items-center gap-4">
            <Link href="/login" className="text-sm text-white/60 hover:text-white transition-colors">
              Sign in
            </Link>
            <Link
              href="/register"
              className="rounded-full bg-white px-4 py-1.5 text-sm font-medium text-black hover:bg-neutral-200 transition-colors"
            >
              Get started free
            </Link>
          </div>
        </div>
      </nav>

      <div className="container mx-auto flex gap-12 px-6 pb-24">

        {/* Sidebar */}
        <aside className="hidden w-48 shrink-0 lg:block">
          <div className="sticky top-28 space-y-1">
            <p className="mb-3 text-xs font-semibold uppercase tracking-widest text-white/30">On this page</p>
            {NAV.map((item) => (
              <a
                key={item.id}
                href={`#${item.id}`}
                className="block rounded px-2 py-1.5 text-sm text-white/50 transition-colors hover:bg-white/5 hover:text-white"
              >
                {item.label}
              </a>
            ))}
          </div>
        </aside>

        {/* Content */}
        <main className="min-w-0 flex-1 pt-12">

          <div className="mb-10">
            <h1 className="text-4xl font-bold tracking-tight">Documentation</h1>
            <p className="mt-3 text-lg text-white/50">
              Everything you need to get Kollaber running and capturing your infrastructure events.
            </p>
          </div>

          {/* ── Getting Started ── */}
          <Section id="getting-started" title="Getting Started">
            <p>
              Kollaber captures deploys, alerts, and manual notes in a shared timeline your entire team can see.
              Getting started takes about five minutes.
            </p>

            <SubSection title="1. Create an account">
              <p>
                Visit <Link href="/register" className="text-[#60a5fa] hover:underline">/register</Link> and sign up
                with your email address. You can also sign in with GitHub if your organization uses it.
              </p>
              <p>
                On first login you will be prompted to create an <strong className="text-white">organization</strong> —
                this is the shared workspace for your team.
              </p>
            </SubSection>

            <SubSection title="2. Create an environment">
              <p>
                Go to <strong className="text-white">Dashboard → New Environment</strong> and give it a name like{" "}
                <InlineCode>production</InlineCode> or <InlineCode>staging</InlineCode>. You can add an optional
                cluster name for Kubernetes setups.
              </p>
            </SubSection>

            <SubSection title="3. Send your first event">
              <p>
                Install the CLI (see <a href="#cli" className="text-[#60a5fa] hover:underline">CLI Reference</a>) and
                run:
              </p>
              <Code>{`kollaber login --api https://kollaber.io --email you@example.com --password yourpassword
kollaber deploy --env production --service api --version v1.0.0`}</Code>
              <p>
                Head back to the dashboard — your deploy event will appear in the timeline immediately.
              </p>
            </SubSection>
          </Section>

          {/* ── Dashboard ── */}
          <Section id="dashboard" title="Dashboard">
            <p>
              The dashboard is the primary UI for browsing your infrastructure history and collaborating with your team.
            </p>

            <SubSection title="Timeline view">
              <p>
                Each environment has its own timeline. Events are sorted newest-first and show the event type, service
                name, timestamp, and any attached metadata (version, author, etc.).
              </p>
              <p>
                The timeline polls for new events every 10 seconds — no refresh needed. A coloured badge indicates
                the event type:
              </p>
              <ul className="mt-2 space-y-1 pl-4 text-sm">
                <li><span className="text-green-400 font-mono">DEPLOY</span> — a service release recorded via the CLI, CI, or webhook</li>
                <li><span className="text-red-400 font-mono">ALERT</span> — an alert ingested via webhook</li>
                <li><span className="text-blue-400 font-mono">NOTE</span> — a manual note added by a team member</li>
              </ul>
            </SubSection>

            <SubSection title="Commenting on events">
              <p>
                Click any event to expand it. Use the comment box to leave context — root cause, rollback decision,
                follow-up ticket, anything. Comments are timestamped and attributed to the author.
              </p>
            </SubSection>

            <SubSection title="Team management">
              <p>
                Go to <strong className="text-white">Settings → Members</strong> to invite teammates via email. Each
                member can be assigned one of four roles:
              </p>
              <ul className="mt-2 space-y-1 pl-4 text-sm">
                <li><strong className="text-white">Owner</strong> — full control including billing and org deletion</li>
                <li><strong className="text-white">Admin</strong> — manage members, environments, and all events</li>
                <li><strong className="text-white">Member</strong> — create events and comments</li>
                <li><strong className="text-white">Viewer</strong> — read-only access to the timeline</li>
              </ul>
            </SubSection>
          </Section>

          {/* ── CLI ── */}
          <Section id="cli" title="CLI Reference">
            <p>
              The <InlineCode>kollaber</InlineCode> CLI lets you send events, view the timeline, and manage environments
              directly from your terminal or CI pipeline.
            </p>

            <SubSection title="Installation">
              <p>Install with Go:</p>
              <Code>go install github.com/urbangeeks/kollaber/cmd/kollaber@latest</Code>
              <p>Or download a pre-built binary from the <Link href="/download" className="text-[#60a5fa] hover:underline">downloads page</Link>.</p>
              <p className="mt-2 text-sm">
                Set the <InlineCode>KOLLABER_API</InlineCode> environment variable to point at your self-hosted
                instance. Defaults to <InlineCode>http://localhost:8080</InlineCode>.
              </p>
            </SubSection>

            <SubSection title="kollaber login">
              <p>Authenticate and save a token to <InlineCode>~/.kollaber/config.json</InlineCode>.</p>
              <Code>{`# Email + password
kollaber login --api https://kollaber.io --email you@example.com --password yourpassword

# CLI token from Settings → API Tokens (for GitHub OAuth users)
kollaber login --api https://kollaber.io --token <your-token>`}</Code>
            </SubSection>

            <SubSection title="kollaber envs">
              <p>List all environments in your organization.</p>
              <Code>kollaber envs</Code>
            </SubSection>

            <SubSection title="kollaber deploy">
              <p>Record a deploy event.</p>
              <Code>{`kollaber deploy \\
  --env production \\
  --service api \\
  --version v1.2.3`}</Code>
              <div className="mt-2 space-y-1 text-sm">
                <div className="flex gap-3"><Badge>--env</Badge><span>Environment name or UUID (required)</span></div>
                <div className="flex gap-3"><Badge>--service</Badge><span>Service name (required)</span></div>
                <div className="flex gap-3"><Badge>--version</Badge><span>Version string, e.g. <InlineCode>v1.2.3</InlineCode> (required)</span></div>
              </div>
            </SubSection>

            <SubSection title="kollaber note">
              <p>Drop a manual note into the timeline — useful for maintenance windows or on-call observations.</p>
              <Code>{`kollaber note --env production "Rolling back v1.2.3 due to 5xx spike"`}</Code>
              <div className="mt-2 text-sm">
                <div className="flex gap-3"><Badge>--env</Badge><span>Environment name or UUID (required)</span></div>
              </div>
            </SubSection>

            <SubSection title="kollaber timeline">
              <p>View recent events for an environment.</p>
              <Code>{`kollaber timeline --env production --limit 20`}</Code>
              <div className="mt-2 space-y-1 text-sm">
                <div className="flex gap-3"><Badge>--env</Badge><span>Environment name or UUID (required)</span></div>
                <div className="flex gap-3"><Badge>--limit</Badge><span>Number of events to fetch (default 20)</span></div>
              </div>
            </SubSection>
          </Section>

          {/* ── Webhooks ── */}
          <Section id="webhooks" title="Webhooks">
            <p>
              Send events directly from your CI/CD pipeline or any HTTP-capable tool by posting to the webhook
              endpoint — no CLI install required.
            </p>

            <SubSection title="Endpoint">
              <Code>POST https://kollaber.io/webhooks/events</Code>
              <p>No authentication required on the webhook endpoint. Payloads are normalized into the events table.</p>
            </SubSection>

            <SubSection title="GitHub Actions">
              <p>Add a step at the end of your deploy job:</p>
              <Code>{`- name: Notify Kollaber
  run: |
    curl -sS -X POST https://kollaber.io/webhooks/events \\
      -H "Content-Type: application/json" \\
      -d '{
        "type": "deploy",
        "service": "\${{ github.repository }}",
        "environment_id": "\${{ secrets.KOLLABER_ENV_ID }}",
        "metadata": {
          "version": "\${{ github.sha }}",
          "author": "\${{ github.actor }}",
          "ref": "\${{ github.ref }}"
        }
      }'`}</Code>
              <p className="text-sm">
                Store your environment UUID as a GitHub secret named <InlineCode>KOLLABER_ENV_ID</InlineCode>.
                Find it on the Dashboard under your environment settings.
              </p>
            </SubSection>

            <SubSection title="Generic JSON">
              <p>Any JSON body with the following shape is accepted:</p>
              <Code>{`{
  "type":           "deploy" | "alert" | "note",
  "service":        "your-service-name",
  "environment_id": "uuid-of-environment",
  "metadata":       { /* any key-value pairs */ }
}`}</Code>
            </SubSection>
          </Section>

          {/* ── API Reference ── */}
          <Section id="api" title="API Reference">
            <p>
              All endpoints accept and return JSON. Authenticated endpoints require an{" "}
              <InlineCode>Authorization: Bearer &lt;token&gt;</InlineCode> header.
            </p>

            <SubSection title="Auth">
              <div className="space-y-2">
                <ApiRow method="POST" path="/auth/register" desc="Create a new account" />
                <ApiRow method="POST" path="/auth/login" desc="Log in and receive a JWT" />
                <ApiRow method="POST" path="/auth/token" desc="Generate a long-lived CLI token (authenticated)" />
                <ApiRow method="GET"  path="/auth/orgs"  desc="List organizations for the current user (authenticated)" />
                <ApiRow method="POST" path="/auth/switch" desc="Switch active organization (authenticated)" />
              </div>
            </SubSection>

            <SubSection title="Environments">
              <div className="space-y-2">
                <ApiRow method="GET"    path="/environments"     desc="List all environments (authenticated)" />
                <ApiRow method="POST"   path="/environments"     desc="Create an environment (authenticated)" />
                <ApiRow method="PUT"    path="/environments/:id" desc="Update an environment (authenticated)" />
                <ApiRow method="DELETE" path="/environments/:id" desc="Delete an environment (authenticated)" />
              </div>
            </SubSection>

            <SubSection title="Events">
              <div className="space-y-2">
                <ApiRow method="GET"  path="/events"                desc="List events, filter by environment_id and limit (authenticated)" />
                <ApiRow method="POST" path="/events"                desc="Create an event (authenticated)" />
                <ApiRow method="POST" path="/webhooks/events"       desc="Ingest an event via webhook (unauthenticated)" />
              </div>
              <p className="mt-3 text-sm">
                Query parameters for <InlineCode>GET /events</InlineCode>:{" "}
                <InlineCode>environment_id</InlineCode> (required) and <InlineCode>limit</InlineCode> (default 50).
              </p>
            </SubSection>

            <SubSection title="Comments">
              <div className="space-y-2">
                <ApiRow method="GET"  path="/events/:id/comments" desc="List comments on an event (authenticated)" />
                <ApiRow method="POST" path="/events/:id/comments" desc="Add a comment to an event (authenticated)" />
              </div>
            </SubSection>

            <SubSection title="Members & Invites">
              <div className="space-y-2">
                <ApiRow method="GET"    path="/members"                    desc="List org members (authenticated)" />
                <ApiRow method="PATCH"  path="/members/:userID"            desc="Update a member's role (authenticated)" />
                <ApiRow method="DELETE" path="/members/:userID"            desc="Remove a member (authenticated)" />
                <ApiRow method="POST"   path="/invites"                    desc="Create an invite (authenticated)" />
                <ApiRow method="GET"    path="/invites/:token"             desc="Look up an invite by token" />
                <ApiRow method="POST"   path="/invites/:token/accept"      desc="Accept an invite and create account" />
                <ApiRow method="GET"    path="/members/invites"            desc="List pending invites (authenticated)" />
                <ApiRow method="DELETE" path="/members/invites/:token"     desc="Revoke a pending invite (authenticated)" />
              </div>
            </SubSection>
          </Section>

        </main>
      </div>
    </div>
  )
}
