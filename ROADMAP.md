# Kollaber Roadmap

## Positioning

Kollaber is **change intelligence + operational memory** for infrastructure teams — not an
observability tool. We collect no metrics, logs, or traces, and we evaluate no alert rules. We sit
downstream of the observability stack.

The question observability answers: *what is happening?*
The question Kollaber answers: *what changed, and what did we decide about it?*

Grafana can show you a 5xx spike. It can't tell you someone rolled back at 10:35 because the auth
flow change broke token refresh. That gap is the product.

Public positioning stays "collaboration layer for infrastructure teams" — broad enough to cover
SRE, platform, and DevOps without excluding the 20-person team that has none of those titles but
needs this most. "SRE" is fine in docs and comparison pages, where the search traffic and
credibility signal live.

Every feature below is judged against one test: **does it deepen change→impact correlation, or the
durability of human context around it?** If not, it doesn't ship.

---

## Tier 1 — deepen the core wedge

### 1. Suspect change detection — **shipped**

`GET /events/:id/suspects` returns the change events that preceded an event in the same
environment, ranked and scored 0–100 with the reasons that produced the score. Surfaced in the
timeline behind a "Suspect changes" action on alerts and failed events.

Weights live in `internal/api/suspects.go`; the candidate query is
`store.ListChangesBefore`. Scores are heuristics for *ordering*, never a causal claim — which is
why every response shows its working.

This is the feature that justifies the product. Today we co-locate deploys and alerts on a timeline
and leave the correlation to the human. Doing it for them is the difference between a nice UI and a
tool people can't work without.

No new ingestion — pure query work over data we already have. `store.GetEventsAroundTime` already
exists as a building block.

### 2. Full-text search across events and comments

Postgres `tsvector` over event service/type/metadata and comment bodies.

Operational memory that can't be searched isn't memory. The question users will have is "didn't we
hit this before?" and today the answer requires scrolling.

### 3. Postmortem generator from a timeline range

Select a time range or incident → markdown doc with event sequence, comment threads, participants,
and an AI narrative summary.

`internal/ai/summarize.go` and the `events.ai_postmortem` column already exist. This turns the AI
from a nice-to-have query box into the thing that saves two hours after every incident. Also the
most demo-able feature we could build.

---

## Tier 2 — distribution and stickiness

### 4. Grafana annotations endpoint

Serve deploy/alert events in Grafana's JSON annotations format so Kollaber markers render as
vertical lines on dashboards teams already have.

Every graph in the company becomes a Kollaber billboard, and "sits alongside your observability
stack" becomes literally true rather than a positioning claim. Low effort, disproportionate reach.

### 5. ArgoCD / Terraform / Atlantis ingestion

We cover GitHub Actions and Alertmanager. ArgoCD is where GitOps shops actually experience "a change
happened." Terraform/Atlantis covers infra changes — the ones that cause outages nobody can explain,
because the app deploy log shows nothing.

Broadens the event surface without changing the data model.

### 6. Weekly digest email

Per-environment recap via Resend: deploy/alert/incident counts, notable comment threads, AI summary.

Retention hook — pulls back people who stopped opening the dashboard.

---

## Tier 3 — collaboration depth

### 7. Action items on comments

Promote a comment to a tracked follow-up with owner and due date. Open-items view per environment.

Postmortem action items are famously where good intentions go to die. Chasing them is a real,
unglamorous job no tool does well.

### 8. Decision log

Mark a comment as a *decision* ("we're rolling back", "accepting this risk until Q3"). Filtered
decisions view.

The highest-value subset of our comment data, currently indistinguishable from chatter.

### 9. Service version inventory

Derive from deploy events: what version of each service was running at any given timestamp.

Answers "what was in prod when this broke?" — today that requires archaeology in CI logs. Free,
given data we already collect.

---

## Tier 4 — guardrails

### 10. Change freeze windows

Declare freeze periods per environment (Black Friday, quarter end). Flag deploys that land inside
one; CLI warns or exits non-zero.

Makes Kollaber *participate* in the change process rather than only observing it.

---

## What we say no to

- Custom metric collection
- Log search / aggregation
- Uptime checks
- Alert evaluation rules
- Generic project management / kanban

Each drags us into a fight with a much larger incumbent on their terms, and dilutes the one thing
we're uniquely good at.

---

## Sequencing

Build **1 → 2 → 3** first. They compound: correlation makes the timeline smart, search makes it
durable, the postmortem generator makes it produce something a manager can see. All three run on
data we already collect, so there's no new ingestion surface to maintain.
