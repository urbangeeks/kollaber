import Link from "next/link"
import Image from "next/image"

const NAV = [
  { id: "getting-started", label: "Getting Started" },
  { id: "dashboard", label: "Dashboard" },
  { id: "notifications", label: "Notifications" },
  { id: "integrations", label: "Integrations" },
  { id: "cli", label: "CLI Reference" },
  { id: "kubernetes", label: "Kubernetes" },
  { id: "webhooks", label: "Webhooks" },
  { id: "self-hosting", label: "Self-hosting" },
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
              <Image src="/logo.png" alt="Kollaber" width={28} height={27} className="rounded" style={{ height: "auto" }} />
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

            <SubSection title="AI timeline assistant">
              <p>
                On the Team plan and above, open the assistant from the spark icon in the bottom-right of the timeline.
                Ask questions in plain language — &ldquo;what deployed today?&rdquo;, &ldquo;summarize the latest alert
                and its discussion&rdquo; — and it answers by reading your real events and comments, streaming the reply
                as it goes. It only reports what it finds in your data and never invents events. You can also query it
                from the terminal with <InlineCode>kollaber ask</InlineCode>.
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

          {/* ── Notifications ── */}
          <Section id="notifications" title="Notifications">
            <p>
              Kollaber notifies you when events are recorded on your timeline — via email, Slack, or Microsoft Teams.
              Email notifications are opt-in and per-user; Slack and Teams are configured once per organization.
            </p>

            <SubSection title="Email preferences">
              <p>
                Go to <strong className="text-white">Settings → Notifications</strong>. Check the event types you want
                to be notified about and optionally enter a notification email address:
              </p>
              <ul className="mt-2 space-y-1 pl-4 text-sm">
                <li><strong className="text-white">Deployments</strong> — emailed when a <InlineCode>deploy</InlineCode> event is recorded</li>
                <li><strong className="text-white">Alerts</strong> — emailed when an <InlineCode>alert</InlineCode> event is recorded</li>
                <li><strong className="text-white">Notes</strong> — emailed when a <InlineCode>note</InlineCode> is added to the timeline</li>
              </ul>
              <p className="mt-3">
                The <strong className="text-white">Notification email</strong> field lets you receive alerts at a
                different address than your account email — useful for shared inboxes or on-call aliases. Leave it
                blank to use your account email.
              </p>
              <p className="mt-2">
                Preferences are saved per organization. Members who have not configured preferences receive no emails
                by default.
              </p>
            </SubSection>

            <SubSection title="How it works">
              <p>
                When any event is created (via the CLI, webhook, or UI), Kollaber fires all configured channels
                asynchronously — email recipients, the Slack webhook, and the Teams webhook all receive a notification
                without blocking the API response.
              </p>
            </SubSection>
          </Section>

          {/* ── Integrations ── */}
          <Section id="integrations" title="Integrations">
            <p>
              In addition to email, Kollaber can post to a Slack channel or Microsoft Teams channel when events are
              recorded. These are org-level settings — one webhook URL covers all team members.
            </p>

            <SubSection title="Slack">
              <p>
                Go to <strong className="text-white">Settings → Slack</strong> and paste an Incoming Webhook URL.
                To get one:
              </p>
              <ol className="mt-2 list-decimal list-inside space-y-1 pl-2 text-sm">
                <li>Open your Slack workspace's App Directory and install <strong className="text-white">Incoming Webhooks</strong></li>
                <li>Choose a channel and click <strong className="text-white">Add Incoming Webhooks integration</strong></li>
                <li>Copy the generated webhook URL and paste it into Kollaber</li>
              </ol>
              <p className="mt-3">
                Use the <strong className="text-white">Send test message</strong> button to verify delivery. Slack
                messages include the event type (with an emoji), the service name, and the environment.
              </p>
            </SubSection>

            <SubSection title="Microsoft Teams">
              <p>
                Go to <strong className="text-white">Settings → Teams</strong> and paste an Incoming Webhook URL.
                To get one:
              </p>
              <ol className="mt-2 list-decimal list-inside space-y-1 pl-2 text-sm">
                <li>Open the target Teams channel and click <strong className="text-white">···</strong> → <strong className="text-white">Connectors</strong></li>
                <li>Search for <strong className="text-white">Incoming Webhook</strong> and click Configure</li>
                <li>Give it a name, click Create, then copy the webhook URL</li>
              </ol>
              <p className="mt-3">
                Teams messages are sent as colour-coded <InlineCode>MessageCard</InlineCode> payloads — blue for
                deploys, red for alerts, purple for notes.
              </p>
            </SubSection>

            <SubSection title="Permissions">
              <p>
                Only <strong className="text-white">owners</strong> and <strong className="text-white">admins</strong>{" "}
                can save or clear the Slack and Teams webhook URLs. Members and viewers can see whether a webhook is
                configured but cannot change it.
              </p>
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

            <SubSection title="kollaber ask">
              <p>
                Ask the <strong className="text-white">AI timeline assistant</strong> a natural-language question about
                your events. The answer streams to stdout while tool lookups print to stderr, so you can pipe the
                answer cleanly. Requires the Team plan or higher.
              </p>
              <Code>{`kollaber ask --env production "what deployed in the last hour?"
kollaber ask "summarize today's alerts" > summary.txt`}</Code>
              <p className="mt-3">
                Conversations persist across commands, so follow-ups keep context. Run with no question to open an
                interactive multi-turn session (type <InlineCode>exit</InlineCode> to quit).
              </p>
              <Code>{`kollaber ask "what was the last alert?"
kollaber ask "yes, show its metadata"   # remembers the previous turn

kollaber ask --env production           # interactive session`}</Code>
              <div className="mt-2 space-y-1 text-sm">
                <div className="flex gap-3"><Badge>--env</Badge><span>Scope the question to an environment (optional)</span></div>
                <div className="flex gap-3"><Badge>--quiet</Badge><span>Suppress tool-lookup progress on stderr</span></div>
                <div className="flex gap-3"><Badge>--new</Badge><span>Start a fresh conversation, ignoring saved history</span></div>
                <div className="flex gap-3"><Badge>--no-save</Badge><span>One-off question; don&apos;t read or write saved history</span></div>
              </div>
            </SubSection>
          </Section>

          {/* ── Kubernetes ── */}
          <Section id="kubernetes" title="Kubernetes">
            <p>
              The <strong className="text-white">kube-watcher</strong> is a lightweight agent that watches your
              Kubernetes cluster and automatically fires events to your Kollaber timeline — no manual CLI calls or
              CI steps needed.
            </p>
            <ul className="mt-2 space-y-1 pl-4 text-sm">
              <li><span className="text-green-400 font-mono">DEPLOY</span> — fired when a Deployment completes a rollout, capturing the image tag and replica count</li>
              <li><span className="text-red-400 font-mono">ALERT</span> — fired when a pod enters <InlineCode>CrashLoopBackOff</InlineCode>, capturing the pod and container name</li>
            </ul>

            <SubSection title="Deploy with Helm">
              <p>
                The watcher runs as a <InlineCode>Deployment</InlineCode> inside your cluster using a{" "}
                <InlineCode>ServiceAccount</InlineCode> with read-only access to Deployments and Pods.
                Install one release per cluster:
              </p>
              <Code>{`helm install kollaber-watcher ./charts/kube-watcher \\
  --set kollaber.env=prod \\
  --set kollaber.api=https://kollaber.io \\
  --set kollaber.token=<cli-token>`}</Code>
              <p>
                Get your CLI token from the dashboard under <strong className="text-white">Settings → CLI Token</strong>.
              </p>
            </SubSection>

            <SubSection title="Helm values">
              <div className="mt-2 space-y-2 text-sm">
                <div className="flex gap-3"><Badge>kollaber.env</Badge><span>Kollaber environment name to map events to (required)</span></div>
                <div className="flex gap-3"><Badge>kollaber.api</Badge><span>Kollaber API base URL (required)</span></div>
                <div className="flex gap-3"><Badge>kollaber.token</Badge><span>CLI token — from Settings → CLI Token</span></div>
                <div className="flex gap-3"><Badge>kollaber.existingSecret</Badge><span>Use an existing <InlineCode>Secret</InlineCode> with key <InlineCode>token</InlineCode> instead of creating one</span></div>
                <div className="flex gap-3"><Badge>watchNamespace</Badge><span>Limit to a single namespace; empty = all namespaces</span></div>
              </div>
            </SubSection>

            <SubSection title="Multiple clusters">
              <p>
                Install a separate Helm release for each cluster, pointing each one at the matching Kollaber environment:
              </p>
              <Code>{`helm install kollaber-watcher-prod    ./charts/kube-watcher --set kollaber.env=prod    ...
helm install kollaber-watcher-staging ./charts/kube-watcher --set kollaber.env=staging ...`}</Code>
              <p>
                Events from each cluster will appear in their respective environment timelines in the dashboard.
              </p>
            </SubSection>

            <SubSection title="Using an existing secret">
              <p>
                If you manage secrets externally (Vault, Sealed Secrets, External Secrets Operator), skip secret
                creation by pointing at your own:
              </p>
              <Code>{`# Your secret must have a key named "token"
kubectl create secret generic my-kollaber-token --from-literal=token=<cli-token>

helm install kollaber-watcher ./charts/kube-watcher \\
  --set kollaber.env=prod \\
  --set kollaber.api=https://kollaber.io \\
  --set kollaber.existingSecret=my-kollaber-token`}</Code>
            </SubSection>

            <SubSection title="Running locally (out-of-cluster)">
              <p>
                The watcher binary also works outside the cluster using your local kubeconfig — useful for testing:
              </p>
              <Code>{`go install github.com/urbangeeks/kollaber/cmd/kube-watcher@latest

kube-watcher \\
  --kubeconfig ~/.kube/config \\
  --env prod \\
  --api https://kollaber.io \\
  --token <cli-token>`}</Code>
              <p className="text-sm">
                The binary tries in-cluster config first. If it is not running inside a pod it falls back to{" "}
                <InlineCode>--kubeconfig</InlineCode> (or <InlineCode>~/.kube/config</InlineCode> by default).
              </p>
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

          {/* ── Self-hosting ── */}
          <Section id="self-hosting" title="Self-hosting">
            <p>
              Kollaber ships as a single binary that embeds the frontend. The official Helm chart is the recommended
              way to deploy a self-hosted instance on Kubernetes.
            </p>

            <SubSection title="Prerequisites">
              <ul className="space-y-1 pl-4 text-sm">
                <li>Kubernetes cluster with Helm 3</li>
                <li>PostgreSQL 14+ (or use the in-cluster option below)</li>
              </ul>
            </SubSection>

            <SubSection title="Minimal install">
              <Code>{`helm install kollaber oci://ghcr.io/urbangeeks/charts/kollaber \\
  --namespace kollaber \\
  --create-namespace \\
  --set secret.jwtSecret=$(openssl rand -hex 32) \\
  --set externalDatabaseUrl=postgres://user:pass@your-postgres:5432/kollaber \\
  --set ingress.enabled=true \\
  --set ingress.host=kollaber.mycompany.com`}</Code>
              <p className="text-sm">
                Save the generated <InlineCode>jwtSecret</InlineCode> — it must stay the same across upgrades
                or existing sessions will be invalidated.
              </p>
            </SubSection>

            <SubSection title="In-cluster PostgreSQL">
              <p>If you don't have an external database, deploy one with Bitnami's chart:</p>
              <Code>{`helm install postgres oci://registry-1.docker.io/bitnamicharts/postgresql \\
  --namespace kollaber \\
  --set auth.username=kollaber \\
  --set auth.password=changeme \\
  --set auth.database=kollaber`}</Code>
              <p>Then use the service name as the hostname:</p>
              <Code>--set externalDatabaseUrl=postgres://kollaber:changeme@postgres-postgresql:5432/kollaber</Code>
            </SubSection>

            <SubSection title="All Helm values">
              <div className="mt-2 space-y-2 text-sm">
                <p className="text-white/40 uppercase tracking-widest text-xs font-semibold">Core</p>
                <div className="flex gap-3"><Badge>secret.jwtSecret</Badge><span>JWT signing secret — <InlineCode>openssl rand -hex 32</InlineCode> (required)</span></div>
                <div className="flex gap-3"><Badge>externalDatabaseUrl</Badge><span>Postgres connection string (required)</span></div>
                <div className="flex gap-3"><Badge>replicaCount</Badge><span>Number of API replicas (default: 1)</span></div>
                <div className="flex gap-3"><Badge>migrate.enabled</Badge><span>Run DB migrations on install/upgrade (default: true)</span></div>

                <p className="text-white/40 uppercase tracking-widest text-xs font-semibold pt-2">Image</p>
                <div className="flex gap-3"><Badge>image.repository</Badge><span>Image repository (default: <InlineCode>ghcr.io/urbangeeks/kollaber-api</InlineCode>)</span></div>
                <div className="flex gap-3"><Badge>image.tag</Badge><span>Image tag (default: <InlineCode>latest</InlineCode>)</span></div>
                <div className="flex gap-3"><Badge>image.pullPolicy</Badge><span>Image pull policy (default: <InlineCode>IfNotPresent</InlineCode>)</span></div>
                <div className="flex gap-3"><Badge>imagePullSecrets</Badge><span>Pull secrets for private registries</span></div>

                <p className="text-white/40 uppercase tracking-widest text-xs font-semibold pt-2">Ingress</p>
                <div className="flex gap-3"><Badge>ingress.enabled</Badge><span>Create a standard Kubernetes Ingress resource (default: false)</span></div>
                <div className="flex gap-3"><Badge>ingress.host</Badge><span>Hostname for the Ingress</span></div>
                <div className="flex gap-3"><Badge>ingress.className</Badge><span>Ingress class, e.g. <InlineCode>nginx</InlineCode></span></div>
                <div className="flex gap-3"><Badge>ingress.annotations</Badge><span>Annotations to add to the Ingress resource</span></div>
                <div className="flex gap-3"><Badge>ingress.tls</Badge><span>TLS configuration block</span></div>

                <p className="text-white/40 uppercase tracking-widest text-xs font-semibold pt-2">Istio</p>
                <div className="flex gap-3"><Badge>istio.enabled</Badge><span>Create Istio Gateway + VirtualService instead of Ingress (default: false)</span></div>
                <div className="flex gap-3"><Badge>istio.host</Badge><span>Hostname for the Gateway and VirtualService</span></div>
                <div className="flex gap-3"><Badge>istio.gatewaySelector</Badge><span>Label selector for the Istio ingress gateway pod (default: <InlineCode>istio: ingressgateway</InlineCode>)</span></div>
                <div className="flex gap-3"><Badge>istio.tls.mode</Badge><span>TLS mode, e.g. <InlineCode>SIMPLE</InlineCode></span></div>
                <div className="flex gap-3"><Badge>istio.tls.credentialName</Badge><span>Name of the TLS credential in <InlineCode>istio-system</InlineCode></span></div>

                <p className="text-white/40 uppercase tracking-widest text-xs font-semibold pt-2">Optional integrations</p>
                <div className="flex gap-3"><Badge>secret.githubClientId</Badge><span>GitHub OAuth client ID — disables GitHub login if unset</span></div>
                <div className="flex gap-3"><Badge>secret.githubClientSecret</Badge><span>GitHub OAuth client secret</span></div>
                <div className="flex gap-3"><Badge>secret.smtpHost</Badge><span>SMTP host — disables email if unset</span></div>
                <div className="flex gap-3"><Badge>secret.smtpPort</Badge><span>SMTP port (default: 587)</span></div>
                <div className="flex gap-3"><Badge>secret.smtpUser</Badge><span>SMTP username</span></div>
                <div className="flex gap-3"><Badge>secret.smtpPassword</Badge><span>SMTP password</span></div>
                <div className="flex gap-3"><Badge>secret.webhookSecret</Badge><span>HMAC secret for webhook verification — skips verification if unset</span></div>
              </div>
            </SubSection>

            <SubSection title="Optional: GitHub OAuth">
              <p>
                Create a GitHub OAuth App at <InlineCode>github.com/settings/developers</InlineCode>. Set the
                callback URL to <InlineCode>https://kollaber.mycompany.com/auth/github/callback</InlineCode>, then
                pass the credentials:
              </p>
              <Code>{`--set secret.githubClientId=your_client_id \\
--set secret.githubClientSecret=your_client_secret`}</Code>
              <p className="text-sm">If not set, GitHub OAuth is disabled and users log in with email/password only.</p>
            </SubSection>

            <SubSection title="Email delivery">
              <p>
                Kollaber uses email OTP for login — users receive a 6-digit code to sign in.
                Delivery is resolved in this order:
              </p>
              <div className="mt-3 space-y-2 text-sm">
                <div className="flex gap-3 rounded-lg border border-white/10 bg-white/5 p-3">
                  <span className="shrink-0 font-mono text-[#60a5fa]">1</span>
                  <span><strong className="text-white">Resend API</strong> — if <InlineCode>RESEND_API_KEY</InlineCode> is set (SaaS default)</span>
                </div>
                <div className="flex gap-3 rounded-lg border border-white/10 bg-white/5 p-3">
                  <span className="shrink-0 font-mono text-[#60a5fa]">2</span>
                  <span><strong className="text-white">SMTP</strong> — if <InlineCode>SMTP_HOST</InlineCode> is set (recommended for self-hosted)</span>
                </div>
                <div className="flex gap-3 rounded-lg border border-white/10 bg-white/5 p-3">
                  <span className="shrink-0 font-mono text-[#60a5fa]">3</span>
                  <span><strong className="text-white">Pod logs</strong> — fallback if neither is configured; retrieve with <InlineCode>kubectl logs -n kollaber deployment/kollaber-api</InlineCode></span>
                </div>
              </div>
              <p className="mt-4">For production installs, configure SMTP:</p>
              <Code>{`--set secret.smtpHost=smtp.yourprovider.com \\
--set secret.smtpPort=587 \\
--set secret.smtpUser=notifications@mycompany.com \\
--set secret.smtpPassword=your_password`}</Code>
              <p className="text-sm text-white/50">
                SMTP uses STARTTLS on port 587. Port 465 (implicit TLS) is not supported.
                Most providers — Gmail, SendGrid, Mailgun, Exchange — support port 587.
              </p>
            </SubSection>

            <SubSection title="Optional: Webhook HMAC verification">
              <Code>--set secret.webhookSecret=your_hmac_secret</Code>
              <p className="text-sm">If not set, webhook payloads are accepted without signature verification.</p>
            </SubSection>

            <SubSection title="Istio (Gateway + VirtualService)">
              <p>
                If your cluster uses Istio instead of a standard ingress controller, disable the Ingress resource
                and enable Istio routing:
              </p>
              <Code>{`helm install kollaber oci://ghcr.io/urbangeeks/charts/kollaber \\
  --namespace kollaber \\
  --create-namespace \\
  --set secret.jwtSecret=$(openssl rand -hex 32) \\
  --set externalDatabaseUrl=postgres://user:pass@your-postgres:5432/kollaber \\
  --set ingress.enabled=false \\
  --set istio.enabled=true \\
  --set istio.host=kollaber.mycompany.com`}</Code>
              <p>With TLS:</p>
              <Code>{`--set istio.tls.mode=SIMPLE \\
--set istio.tls.credentialName=kollaber-tls`}</Code>
              <p className="text-sm text-white/50">
                The gateway selector defaults to <InlineCode>istio: ingressgateway</InlineCode>. Override with{" "}
                <InlineCode>--set istio.gatewaySelector.istio=my-gateway</InlineCode> if your gateway pod uses
                a different label.
              </p>
            </SubSection>

            <SubSection title="Upgrading">
              <Code>{`helm upgrade kollaber oci://ghcr.io/urbangeeks/charts/kollaber \\
  --namespace kollaber \\
  --reuse-values`}</Code>
              <p className="text-sm">
                Use <InlineCode>--reuse-values</InlineCode> to keep your existing secrets and config.
                Migrations run automatically on every upgrade.
              </p>
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

            <SubSection title="Settings">
              <div className="space-y-2">
                <ApiRow method="GET" path="/settings/notifications" desc="Get email notification preferences (authenticated)" />
                <ApiRow method="PUT" path="/settings/notifications" desc="Update email notification preferences (authenticated)" />
                <ApiRow method="GET" path="/settings/slack"         desc="Get org Slack webhook URL (authenticated)" />
                <ApiRow method="PUT" path="/settings/slack"         desc="Set org Slack webhook URL (admin)" />
                <ApiRow method="POST" path="/settings/slack/test"   desc="Send a test Slack message (admin)" />
                <ApiRow method="GET" path="/settings/teams"         desc="Get org Teams webhook URL (authenticated)" />
                <ApiRow method="PUT" path="/settings/teams"         desc="Set org Teams webhook URL (admin)" />
                <ApiRow method="POST" path="/settings/teams/test"   desc="Send a test Teams message (admin)" />
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
