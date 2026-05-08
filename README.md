# Kollaber

**Collaboration layer for infrastructure teams.**  
Capture deploys, alerts, and manual notes in a shared timeline your whole team can see and comment on.

🌐 **[kollaber.io](https://kollaber.io)** · 📖 **[Docs](https://kollaber.io/docs)**

---

## What it does

- **Timeline** — every deploy, alert, and note in one chronological view per environment
- **Comments** — annotate any event with root cause, rollback decisions, follow-ups
- **CLI** — send deploy events and notes from your terminal or CI pipeline
- **Webhooks** — integrate GitHub Actions or any HTTP tool without installing anything
- **Role-based access** — Owner / Admin / Member / Viewer tiers per organization

## Quick start (local)

**Prerequisites:** Docker, Go 1.22+, Node 20+, [Task](https://taskfile.dev)

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
kollaber login --api https://kollaber.io --email you@example.com --password yourpassword
kollaber login --api https://kollaber.io --token <cli-token>

# List environments
kollaber envs

# Send a deploy event
kollaber deploy --env production --service api --version v1.2.3

# Add a note
kollaber note --env production "Rolling back — 5xx spike in us-east-1"

# View the timeline
kollaber timeline --env production --limit 20
```

The CLI stores its token at `~/.kollaber/config.json`.  
Set `KOLLABER_API` to point at a self-hosted instance (default: `http://localhost:8080`).

## Webhook

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
          "author": "${{ github.actor }}"
        }
      }'
```

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
| `task db:up` | Start PostgreSQL container |
| `task db:down` | Stop PostgreSQL container |
| `task db:reset` | Wipe volume and restart |
| `task generate` | Re-run sqlc codegen |
| `task migrate` | Run migrations only (MIGRATE_ONLY mode) |

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

### Kubernetes (Helm)

Two charts live under `charts/`.

#### Kollaber (API + frontend)

```bash
helm install kollaber ./charts/kollaber \
  --set secret.jwtSecret=$(openssl rand -hex 32) \
  --set secret.dbPassword=changeme \
  --set externalDatabaseUrl=postgres://user:pass@postgres:5432/kollaber \
  --set urls.api=https://api.kollaber.example.com \
  --set urls.frontend=https://kollaber.example.com \
  --set ingress.enabled=true \
  --set ingress.host=kollaber.example.com
```

| Value | Description |
|---|---|
| `secret.jwtSecret` | JWT signing secret — generate with `openssl rand -hex 32` |
| `externalDatabaseUrl` | Postgres connection string |
| `urls.api` | Public API URL (used by the frontend) |
| `ingress.enabled` | Set to `true` to create an Ingress resource |
| `ingress.host` | Hostname for the Ingress |
| `migrate.enabled` | Runs DB migrations as a pre-install Job (default: `true`) |

#### kube-watcher

Watches a cluster for Deployment rollouts and `CrashLoopBackOff` pods, and fires events to your Kollaber timeline. Deploy one release per cluster — the pod uses its `ServiceAccount` token, no kubeconfig needed.

```bash
helm install kollaber-watcher ./charts/kube-watcher \
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

**Multi-cluster** — one install per environment:

```bash
helm install kollaber-watcher-prod    ./charts/kube-watcher --set kollaber.env=prod    ...
helm install kollaber-watcher-staging ./charts/kube-watcher --set kollaber.env=staging ...
```

The watcher image is built and pushed to `ghcr.io/urbangeeks/kollaber/kube-watcher:latest` automatically on every merge to `main`.

## Stack

| Layer | Technology |
|---|---|
| Backend | Go, [Echo](https://echo.labstack.com), [sqlc](https://sqlc.dev), pgx |
| Frontend | Next.js 15, Tailwind CSS, shadcn/ui |
| Database | PostgreSQL 16 |
| CLI | Go, [Cobra](https://github.com/spf13/cobra) |
| Deploy | Railway (backend), single-binary embed |
