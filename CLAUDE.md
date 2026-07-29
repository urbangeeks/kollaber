# Kollaber – Claude Code Spec

## 1. Product Overview

**Name:** Kollaber
**Tagline:** Collaboration layer for infrastructure teams

**Core function:**
Capture, annotate, and query infrastructure events (deploys, alerts, manual notes) in a shared
timeline.

**Category:** change intelligence + operational memory. Kollaber is *not* an observability tool —
it collects no metrics, logs, or traces, and evaluates no alert rules. It sits downstream of the
observability stack and answers a different question: *what changed, and what did we decide about
it?* See `ROADMAP.md` for the features this positioning does and does not admit.

## 2. Scope

Kollaber is past MVP. The list below is what exists today — treat it as the baseline, not a
build list.

**Shipped:**

- Auth: JWT, email OTP, GitHub OAuth, SAML/OIDC SSO
- Multi-tenancy: orgs, members, invites, RBAC (Owner / Admin / Member / Viewer)
- Environments and services
- Event ingestion: manual, CLI, generic webhook, GitHub Actions, Prometheus Alertmanager,
  Kubernetes watcher
- Timeline UI with comments
- Incidents: group events, track status, AI postmortems
- DORA metrics
- AI: timeline assistant (`/ai/chat`), event summaries, postmortem generation
- MCP server (`kollaber mcp`) exposing the timeline to coding agents
- Realtime: server-sent events (`/events/stream`) — not polling, not websockets
- Notifications: Slack, Microsoft Teams, email (Resend)
- Billing: Stripe, plan entitlements (free / team / pro / enterprise)
- Audit logs
- CLI (`kollaber`)

**Deliberately excluded** (see `ROADMAP.md` § "What we say no to"):

- Metric, log, or trace collection
- Uptime checks or alert evaluation rules
- Generic project management / kanban

## 3. System Architecture

```text
[ CLI ] ----------\
[ Webhooks ] ------\
[ Kube watcher ] ---> [ Go API (Echo) ] ---> [ PostgreSQL ]
[ MCP server ] ----/         |
[ Frontend ] -----/          +--> [ Slack / Teams / Resend ]
                             +--> [ Anthropic API ]
                             +--> [ Stripe ]
```

## 4. Backend (Go)

- **Framework:** Echo v4 (`github.com/labstack/echo/v4`)
- **DB access:** pgx v5 + sqlc. Generated code lives in `internal/store/*.sql.go`; hand-written
  queries live alongside it in plain `.go` files (e.g. `events_extra.go`, `dora.go`).
- **Migrations:** numbered SQL in `migrations/`, embedded via `migrations/embed.go`, applied by
  `internal/db/migrate.go`.

```text
cmd/
  app/            # API server
  kollaber/       # CLI (+ MCP server)
  kube-watcher/   # Kubernetes event watcher
internal/
  ai/             # Anthropic-backed agent, summaries, postmortems
  api/            # HTTP handlers + router
  billing/        # Stripe, plans, entitlements
  db/             # connection, migrations
  middleware/     # auth, org context, rate limiting
  resend/         # transactional email
  slack/          # Slack + Teams notifications
  store/          # sqlc output + hand-written queries
  teams/
db/queries/       # sqlc source queries
migrations/
ui/               # Next.js frontend
charts/           # Helm chart for self-hosted
```

**When adding a query:** prefer adding to `db/queries/*.sql` and regenerating with sqlc
(`sqlc.yaml`). Drop to a hand-written method in `internal/store/` only when the query needs
dynamic SQL that sqlc can't express (see `events_filter.go` for the existing pattern).

## 5. Database Schema (PostgreSQL)

Core tables from `migrations/001_init.sql`: `users`, `orgs`, `org_members`, `environments`,
`events`, `invites`, `comments`. Later migrations add incidents, billing, SSO, audit logs,
notification prefs, AI cache/usage, and DORA support.

`events` carries `type`, `service`, `environment_id`, `timestamp`, `metadata` (jsonb), `status`,
`ai_summary`, `ai_postmortem`. Event types are validated in `internal/store/event_types.go` —
add new types there *and* in the corresponding migration's CHECK constraint.

**Tenancy rule:** `events` has no `org_id`. Every event query MUST join
`environments env ON env.id = e.environment_id` and filter `env.org_id = $n`. Skipping this is a
cross-tenant data leak. Follow the existing queries in `db/queries/events.sql`.

## 6. API Design

Routes are registered in `internal/api/router.go` — read it rather than trusting a list here.
Broad shape:

| Group | Routes |
| --- | --- |
| Auth | `/auth/login`, `/auth/register`, `/auth/otp/*`, `/auth/github/*`, `/auth/sso/*`, `/token` |
| Orgs | `/orgs`, `/switch`, `/members`, `/invites` |
| Environments | `/environments`, `/environments/stats`, `/services` |
| Events | `/events`, `/events/:id`, `/events/stream`, `/events/:id/comments`, `/events/:id/summary`, `/events/:id/postmortem` |
| Incidents | `/incidents`, `/incidents/:id`, `/incidents/:id/events`, `/incidents/:id/postmortem` |
| Metrics | `/metrics/dora` |
| Search | `/search?q=…&environment_id=…` |
| Postmortems | `/postmortems` (POST: environment + time window → markdown) |
| AI | `/ai/chat` |
| Webhooks | `/webhooks/events`, `/webhooks/alertmanager`, `/webhooks/stripe` |
| Settings | `/settings/{notifications,slack,teams,sso}`, `/audit-logs`, `/billing` |

Create event body:

```json
{
  "type": "deploy",
  "service": "api-service",
  "environment_id": "uuid",
  "metadata": { "version": "v1.2.3", "author": "jerome" }
}
```

## 7. CLI (`kollaber`)

```bash
kollaber login --api https://kollaber.io --email you@example.com
kollaber envs
kollaber timeline --env prod
kollaber note --env prod "Investigating latency spike"
kollaber deploy --env prod --service api --version v1.2.3
kollaber mcp          # MCP server over stdio
```

## 8. Frontend (Next.js)

App Router, under `ui/app/`. Routes: `/login`, `/register`, `/onboarding`, `/dashboard`,
`/env/[id]`, `/incidents`, `/metrics`, `/docs`, `/download`, `/pricing`, `/admin`, and
`/settings/{members,teams,billing,notifications,slack,sso,kubernetes,audit-logs}`.

Timeline updates over SSE (`/events/stream`), not polling.

**Lint is a CI gate.** Run it before declaring frontend work done.

## 9. Deployment

- **SaaS:** kollaber.io
- **Self-hosted:** Helm chart in `charts/`; `NEXT_PUBLIC_SELF_HOSTED=true` switches landing-page
  behavior
- **Local:** `task db:up && task start` (Podman, not Docker)

## 10. Conventions

- Containers: **Podman**. Never invoke `docker`.
- Commits: conventional-commit prefixes (`feat(scope):`, `fix(scope):`). No AI attribution
  trailers.
- Go style: follow the `golang-*` skills available in this session — naming, error handling
  (wrap with `%w`), table-driven tests.
- Tests live next to the code (`*_test.go`). New store queries and webhook normalizers should
  come with tests; see `alertmanager_test.go` and `dora_test.go` for the house pattern.
