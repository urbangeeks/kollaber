# Kollaber

**Collaboration layer for infrastructure teams.**  
Capture deploys, alerts, and manual notes in a shared timeline your whole team can see and comment on.

🌐 **[kollaber.io](https://kollaber.io)** · 📖 **[Docs](https://kollaber.io/docs)**

---

## What it does

- **Timeline** — every deploy, alert, and note in one chronological view per environment
- **Comments** — annotate any event with root cause, rollback decisions, follow-ups
- **CLI** — send deploy events and notes from your terminal or CI pipeline
- **AI timeline assistant** — ask natural-language questions about your events, in the dashboard or from the CLI (Team plan and up)
- **Webhooks** — integrate GitHub Actions or any HTTP tool without installing anything
- **Alert ingestion** — point Prometheus Alertmanager at Kollaber and firing alerts land next to the deploys that caused them
- **Role-based access** — Owner / Admin / Member / Viewer tiers per organization

## Quick start (local)

**Prerequisites:** Podman, Go 1.26+, Node 20+, [Task](https://taskfile.dev)

```bash
# 1. Copy environment config
cp .env.example .env

# 2. Start PostgreSQL
task db:up

# 3. Start backend + frontend (hot reload)
task start
```

| URL | |
|---|---|
| Frontend | http://localhost:3000 |
| API | http://localhost:8080 |

Register at http://localhost:3000/register to create the first account.

## CLI

Install:

```bash
go install github.com/urbangeeks/kollaber/cmd/kollaber@latest
```

Usage:

```bash
# Authenticate — --api points the CLI at your Kollaber instance
kollaber login --api https://kollaber.io --email you@example.com   # emails a one-time code
kollaber login --api https://kollaber.io --token <cli-token>

# List environments
kollaber envs

# Send a deploy event
kollaber deploy --env production --service api --version v1.2.3

# Pass the commit time to power the DORA lead-time metric
kollaber deploy --env production --service api --version v1.2.3 \
  --committed-at "$(git show -s --format=%cI HEAD)"

# Add a note
kollaber note --env production "Rolling back — 5xx spike in us-east-1"

# View the timeline
kollaber timeline --env production --limit 20

# Incidents — group events, track status, link them together
kollaber incident list                       # all incidents
kollaber incident list --status open         # filter by status
kollaber incident open --title "5xx spike on api" --severity sev2
kollaber incident open --title "Deploy failed" --severity sev2 --event <event-id>
kollaber incident attach <incident-id> --event <event-id>   # repeatable
kollaber incident resolve <incident-id>      # defaults to resolved
kollaber incident resolve <incident-id> --status mitigated

# DORA metrics — deploy frequency, lead time, change failure rate, time to restore
kollaber dora                                # last 30 days, all environments
kollaber dora --days 7                       # last 7 days
kollaber dora --env production --days 90      # scope to one environment
# Lead time needs a commit timestamp on deploys (deploy --committed-at, or a
# committed_at field in webhook metadata). Time to restore is derived from
# incidents and is always org-wide; --env narrows the other three metrics only.

# Ask the AI assistant (Team plan and up); answer streams to stdout,
# tool lookups to stderr, so you can pipe the answer cleanly
kollaber ask --env production "what deployed in the last hour?"
kollaber ask "summarize today's alerts" > summary.txt

# Conversations persist across commands, so follow-ups keep context
kollaber ask "what was the last alert?"
kollaber ask "yes, show its metadata"   # remembers the previous turn

# Run with no question for an interactive multi-turn session
kollaber ask --env production
# --new starts fresh; --no-save makes a one-off call
```

The CLI stores its token at `~/.kollaber/config.json`.  
Defaults to `https://kollaber.io` (the hosted service); set `KOLLABER_API` (or `--api` on login) to point at a self-hosted instance.

## Webhooks

### Generic / GitHub Actions

No CLI install needed — POST directly from CI:

```yaml
# .github/workflows/deploy.yml
- name: Notify Kollaber
  run: |
    curl -sS -X POST https://kollaber.io/webhooks/events \
      -H "Content-Type: application/json" \
      -d '{
        "type": "deploy",
        "service": "${{ github.repository }}",
        "environment_id": "${{ secrets.KOLLABER_ENV_ID }}",
        "metadata": {
          "version": "${{ github.sha }}",
          "author": "${{ github.actor }}",
          "committed_at": "${{ github.event.head_commit.timestamp }}"
        }
      }'
```

### Prometheus Alertmanager

Point an Alertmanager receiver at `/webhooks/alertmanager` and firing alerts land
on the timeline next to your deploys. Each entry in the delivery's `alerts[]`
becomes one `alert` event — firing as `failure`, resolved as `success`.

The target environment comes from the `environment_id` query parameter, since a
receiver's only per-target field is its URL:

```yaml
# alertmanager.yml
receivers:
  - name: kollaber
    webhook_configs:
      - url: https://kollaber.io/webhooks/alertmanager?environment_id=<your-env-id>
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials: <your WEBHOOK_SECRET>
```

`send_resolved: true` is what closes the loop — without it the timeline shows
alerts firing but never recovering.

**Authentication.** When `WEBHOOK_SECRET` is set the endpoint accepts the secret
as a bearer token (above), as an `X-Kollaber-Secret` header, or as an
`X-Hub-Signature-256` HMAC. Alertmanager can only send the first of those; the
other two are there for non-Alertmanager clients posting the same payload shape.
When `WEBHOOK_SECRET` is unset the endpoint is open, so set it in any
internet-facing deployment.

**Field mapping.**

| Event field | Source |
|---|---|
| `service` | first non-empty of the `service`, `job`, `app`, `alertname` labels |
| `status` | `failure` when firing, `success` when resolved |
| `timestamp` | `startsAt`, or `endsAt` once resolved |
| `metadata` | `alertname`, `severity`, `summary`, `description`, `fingerprint`, `generator_url`, and all raw labels |

Events are timestamped when the alert actually started rather than when the
webhook arrived — grouping and `repeat_interval` can delay delivery by minutes,
which would otherwise misorder alerts against the deploys that caused them.

**Duplicate suppression.** Alertmanager re-sends firing alerts on every
`repeat_interval`. Kollaber skips a delivery whose `fingerprint` already sits at
the same status, so the timeline gets one entry when an alert fires and one when
it resolves — not one every four hours. The response reports both counts:

```json
{ "ingested": 1, "skipped": 0 }
```

This also makes retries safe: a delivery that failed mid-batch can be re-sent
without duplicating the alerts that already landed.

## Development

| Command | Description |
|---|---|
| `task dev` | Backend with hot reload (air) |
| `task ui:dev` | Frontend dev server |
| `task start` | Both in parallel |
| `task build` | Compile single binary (embeds frontend) |
| `task run` | Build and run the single binary |
| `task build:cli` | Compile CLI binary |
| `task install:cli` | Install CLI to `$GOPATH/bin` |
| `task build:kube-watcher` | Compile kube-watcher binary |
| `task install:kube-watcher` | Install kube-watcher to `$GOPATH/bin` |
| `task db:up` | Start PostgreSQL container |
| `task db:down` | Stop PostgreSQL container |
| `task db:reset` | Wipe volume and restart |
| `task generate` | Re-run sqlc codegen |
| `task migrate` | Run migrations only (MIGRATE_ONLY mode) |

### Tests

```bash
# Unit tests (no database needed)
go test ./...

# Include database-backed integration tests (store layer). Point at a
# throwaway Postgres — migrations are applied automatically, and the
# tests skip when the variable is unset.
TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/kollaber_test?sslmode=disable' go test ./...
```

## Deployment

### Railway / single binary

The app ships as a **single binary** — the Go server embeds the compiled Next.js frontend.

```bash
# Build the production binary
task build

# Required environment variables
DATABASE_URL=postgres://...
JWT_SECRET=...
PORT=8080              # optional, default 8080
CORS_ORIGINS=https://your-domain.com   # optional
```

Railway deployment is configured in `railway.toml`. The Dockerfile is at `docker/api/Dockerfile`.

### Self-hosting (Helm)

Kollaber ships as a **single binary** that embeds the frontend. The self-hosted image is published to `ghcr.io/urbangeeks/kollaber-api:latest` on every merge to `main`.

#### Prerequisites

- Kubernetes cluster with Helm 3
- PostgreSQL database (see below for an in-cluster option)

#### Minimal install

```bash
helm install kollaber oci://ghcr.io/urbangeeks/charts/kollaber \
  --namespace kollaber \
  --create-namespace \
  --set secret.jwtSecret=$(openssl rand -hex 32) \
  --set externalDatabaseUrl=postgres://user:pass@your-postgres:5432/kollaber \
  --set ingress.enabled=true \
  --set ingress.host=kollaber.mycompany.com
```

> Save the generated `jwtSecret` — it must stay the same across upgrades or existing sessions will be invalidated.

#### In-cluster PostgreSQL

If you don't have an external database:

```bash
helm install postgres oci://registry-1.docker.io/bitnamicharts/postgresql \
  --namespace kollaber \
  --set auth.username=kollaber \
  --set auth.password=changeme \
  --set auth.database=kollaber
```

Then use `postgres-postgresql.kollaber.svc.cluster.local` as the hostname:

```
--set externalDatabaseUrl=postgres://kollaber:changeme@postgres-postgresql:5432/kollaber
```

#### All Helm values

| Value | Required | Description |
|---|---|---|
| `secret.jwtSecret` | Yes | JWT signing secret — `openssl rand -hex 32` |
| `externalDatabaseUrl` | Yes | Postgres connection string |
| `secret.anthropicApiKey` | No | Anthropic API key — enables AI summaries, postmortems, and the timeline assistant |
| `ingress.enabled` | No | Create an Ingress resource (default: `false`) |
| `ingress.host` | No | Hostname for the Ingress |
| `ingress.className` | No | Ingress class (e.g. `nginx`) |
| `ingress.tls` | No | TLS config block |
| `migrate.enabled` | No | Run DB migrations on install/upgrade (default: `true`) |
| `replicaCount` | No | Number of API replicas (default: `1`) |

#### Istio (Gateway + VirtualService)

If your cluster uses Istio instead of a standard ingress controller, disable the Ingress resource and enable Istio routing:

```bash
--set ingress.enabled=false \
--set istio.enabled=true \
--set istio.host=kollaber.mycompany.com
```

With TLS:

```bash
--set istio.tls.mode=SIMPLE \
--set istio.tls.credentialName=kollaber-tls
```

The `gatewaySelector` defaults to `istio: ingressgateway`. Override if your gateway pod uses a different label:

```bash
--set istio.gatewaySelector.istio=my-gateway
```

#### Optional: GitHub OAuth

Create a GitHub OAuth App at `github.com/settings/developers`. Set the callback URL to `https://kollaber.mycompany.com/auth/github/callback`, then pass the credentials:

```bash
--set secret.githubClientId=your_client_id \
--set secret.githubClientSecret=your_client_secret
```

If not set, GitHub OAuth is disabled and users log in with email/password only.

#### Email delivery

Kollaber uses email OTP for login — users receive a 6-digit code to sign in. Email delivery is resolved in this order:

| Priority | Config | Behaviour |
|---|---|---|
| 1 | `RESEND_API_KEY` set | Sends via [Resend](https://resend.com) (SaaS default) |
| 2 | `SMTP_HOST` set | Sends via your own SMTP server |
| 3 | Neither set | OTP is printed to pod logs — `kubectl logs -n kollaber deployment/kollaber-api` |

For production self-hosted installs, configure SMTP:

```bash
--set secret.smtpHost=smtp.yourprovider.com \
--set secret.smtpPort=587 \
--set secret.smtpUser=notifications@mycompany.com \
--set secret.smtpPassword=your_password
```

Without email configured, users can still log in by retrieving the OTP from the pod logs.

> **Note:** SMTP uses STARTTLS (port 587). Port 465 (implicit TLS) is not supported.

#### Optional: Webhook HMAC verification

```bash
--set secret.webhookSecret=your_hmac_secret
```

If not set, webhook payloads are accepted without signature verification.

#### Optional: AI features

AI event summaries, postmortems, and the timeline assistant call the Anthropic API. Provide a key to enable them:

```bash
--set secret.anthropicApiKey=sk-ant-...
```

If not set, the AI features return an error and the rest of the app works normally. (Self-hosted installs run with `SELF_HOSTED=true`, so plan entitlements are unlocked — the API key is the only thing the AI features need.)

#### kube-watcher

Watches a cluster for Deployment/StatefulSet/DaemonSet rollouts (and teardowns, with `reportDeletes`) and `CrashLoopBackOff` pods, and fires events to your Kollaber timeline. Deploy one release per cluster — the pod uses its `ServiceAccount` token, no kubeconfig needed.

```bash
helm install kollaber-watcher oci://ghcr.io/urbangeeks/charts/kube-watcher \
  --set kollaber.env=prod \
  --set kollaber.api=https://api.kollaber.example.com \
  --set kollaber.token=<cli-token>
```

| Value | Description |
|---|---|
| `kollaber.env` | Kollaber environment name to map events to (required) |
| `kollaber.api` | Kollaber API base URL (required) |
| `kollaber.token` | CLI token — from `kollaber login` then `Settings → CLI token` |
| `kollaber.existingSecret` | Name of an existing `Secret` with key `token` (skips creating one) |
| `watchNamespace` | Limit watching to one namespace; empty = all namespaces |
| `reportDeletes` | Also fire a `teardown` event when a Deployment is removed (default: `false`) |

**Multi-cluster** — one install per environment:

```bash
helm install kollaber-watcher-prod    oci://ghcr.io/urbangeeks/charts/kube-watcher --set kollaber.env=prod    ...
helm install kollaber-watcher-staging oci://ghcr.io/urbangeeks/charts/kube-watcher --set kollaber.env=staging ...
```

The watcher image is built and pushed to `ghcr.io/urbangeeks/kollaber/kube-watcher:latest` automatically on every merge to `main`.

## Stack

| Layer | Technology |
|---|---|
| Backend | Go, [Echo](https://echo.labstack.com), [sqlc](https://sqlc.dev), pgx |
| Frontend | Next.js 16, React 19, Tailwind CSS, shadcn/ui |
| Database | PostgreSQL 16 |
| CLI | Go, [Cobra](https://github.com/spf13/cobra) |
| Deploy | Railway (backend), single-binary embed |
