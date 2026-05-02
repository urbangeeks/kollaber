Kollaber – Claude Code Spec
1. Product Overview

Name: Kollaber
Tagline: Collaboration layer for infrastructure teams

Core Function:
Capture, annotate, and query infrastructure events (deployments, alerts, manual notes) in a shared timeline.

2. MVP Scope (STRICT)

Claude should ONLY build:

Included:
Auth (simple)
Environments
Event ingestion (manual + webhook)
Timeline UI
Comments on events
CLI tool
Excluded (for now):
RBAC complexity
Billing
Realtime websockets (polling is fine)
Slack integration
AI features
3. System Architecture
High Level
[ CLI ] ----\
             \
[ Webhooks ] ---> [ Go API ] ---> [ PostgreSQL ]
             /
[ Frontend ]/
4. Backend (Go)
Framework
Use gin or fiber
Structure:
cmd/
internal/
  api/
  db/
  models/
  services/
  middleware/
pkg/
5. Database Schema (PostgreSQL)
users
id (uuid, pk)
email (text, unique)
created_at
environments
id (uuid, pk)
name (text) -- prod, staging
cluster_name (text)
created_at
events
id (uuid, pk)
type (text) -- deploy, alert, note
service (text)
environment_id (fk)
timestamp (timestamptz)
metadata (jsonb)
created_at
comments
id (uuid, pk)
event_id (fk)
user_id (fk)
body (text)
created_at
6. API Design
Auth
POST /auth/login
POST /auth/register
Environments
GET /environments
POST /environments
Events
Create Event (manual or webhook)
POST /events

Body:

{
  "type": "deploy",
  "service": "api-service",
  "environment_id": "uuid",
  "metadata": {
    "version": "v1.2.3",
    "author": "jerome"
  }
}
Get Timeline
GET /events?environment_id=xxx&limit=50
Comments
POST /events/:id/comments
GET /events/:id/comments
7. CLI Tool (kollaber)
Language: Go (same repo or separate)
Commands
Login
kollaber login
List environments
kollaber envs
View timeline
kollaber timeline --env prod
Add note
kollaber note --env prod "Investigating latency spike"
Send deploy event
kollaber deploy \
  --env prod \
  --service api \
  --version v1.2.3
8. Frontend (Next.js)
Pages
/login
Simple email login
/dashboard
List environments
/env/[id]
Timeline view
Timeline Component

Each event shows:

Type (icon)
Service name
Timestamp
Metadata (version, etc.)

Under each event:

Comment thread
Example UI
[ Deploy ] api-service v1.2.3
10:32 AM

💬 "Added new auth flow"

-----------------------

[ Alert ] 5xx spike
10:35 AM

💬 "Investigating"
💬 "Rolling back"
9. Event Ingestion
Webhook Endpoint
POST /webhooks/events

Accept:

GitHub Actions
Generic JSON

Claude should:

Normalize into events table
10. Polling Strategy (MVP)

Frontend:

Poll /events every 5–10 seconds
11. Auth (Simple)
JWT-based
No OAuth yet
12. Deployment
MVP Infra:
Backend: Fly.io or Railway
DB: Managed Postgres
Frontend: Vercel
13. Seed Data (IMPORTANT)

Claude should generate:

1 user
1 environment (prod)
Sample events:
Deploy
Alert
Note
14. Success Criteria

MVP is “done” when:

You can:
Run kollaber deploy
See it in UI
Comment on it
Timeline updates via polling
Data persists
15. Future Hooks (Don’t Build Yet)

Just leave placeholders:

Slack integration
Kubernetes watcher
AI summaries