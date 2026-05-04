# Kollaber

Collaboration layer for infrastructure teams. Capture, annotate, and query infrastructure events (deployments, alerts, manual notes) in a shared timeline.

## Quick start

```bash
# 1. Start the database
task db:up

# 2. Start backend + frontend
task start

# 3. Install the CLI
task install:cli
```

Frontend: http://localhost:3000  
API: http://localhost:8080

Default credentials: `admin@kollaber.io` / `password`

## CLI

```bash
# Authenticate
kollaber login --email admin@kollaber.io --password password

# List environments
kollaber envs

# View timeline
kollaber timeline --env prod
kollaber timeline --env prod --limit 50

# Add a note
kollaber note --env prod "Investigating latency spike"

# Send a deploy event
kollaber deploy --env prod --service api --version v1.2.3
```

The CLI stores its auth token at `~/.kollaber/config.json`.  
Override the API URL with the `KOLLABER_API` environment variable.

## Development

| Task | Command |
|---|---|
| Start backend (hot reload) | `task dev` |
| Start frontend | `task ui:dev` |
| Start both | `task start` |
| Build CLI | `task build:cli` |
| Install CLI | `task install:cli` |
| Start DB | `task db:up` |
| Stop DB | `task db:down` |
| Re-run seed data | `task seed` |

## Stack

- **Backend** — Go, Echo, sqlc, pgx
- **Frontend** — Next.js, Tailwind, shadcn/ui
- **Database** — PostgreSQL
- **CLI** — Go, Cobra
