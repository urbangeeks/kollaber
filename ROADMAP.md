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

### 2. Full-text search across events and comments — **shipped**

`GET /search?q=…` over event text and comment bodies, with a `/search` page scoped to one
environment or the whole org. Generated `tsvector` columns and GIN indexes in migration 021.

Event vectors index metadata *values* only, never keys — indexing keys would make a search for
"version" match every deploy ever shipped.

Known limitation: `websearch_to_tsquery` matches whole stemmed words, so "check" does not find
"checkout" and "rollback" does not find "rolling back". Prefix matching would need a trigram index
on top; worth adding if users hit it.

### 3. Postmortem generator from a timeline range — **shipped**

`POST /postmortems` takes an environment and a time window and returns a markdown document: the
event sequence, every comment thread grouped under the event it belongs to, the participants, and
an AI narrative summary. Reachable from the timeline header.

The factual half is assembled from data the org already owns and is returned on every plan, with or
without an Anthropic key; only the narrative section is gated on the Pro entitlement. A
`narrative_status` field says which of those held, so a missing summary reads as an explained gap
rather than a failed request.

The narrative prompt deliberately asks for prose only. The timeline and discussion are rendered
deterministically from the rows, so having the model restate them would add nothing but a chance of
restating them wrongly.

Comments are selected by their *event's* timestamp, not their own — analysis written a week after an
outage is exactly the considered thinking a postmortem wants, and filtering on comment time would
drop it.

Known limitation: one document caps at 500 events and 1000 comments, and the narrative works from
the most recent 120 events. The response sets `truncated` when the window overflows the event cap;
the UI suggests narrowing the range.

---

## Tier 2 — distribution and stickiness

### 4. Grafana annotations endpoint — **shipped**

`/annotations` serves change and alert events as Grafana annotations, so Kollaber markers render as
vertical lines on dashboards teams already have. Both verbs return the same array: `POST` speaks the
simple-json datasource contract, `GET` takes the window on the query string for Infinity and for
curl.

Auth reuses the existing 90-day CLI token rather than introducing a datasource key. Grafana sends a
static `Authorization` header, the token already carries the org, and every new credential type is
one more thing to revoke, audit, and get wrong.

Grafana gives a dashboard author one free-text box per annotation track, so its contents are read as
a query string. A `POST` filters through exactly the same code as a `GET` and the two cannot drift.
A bare word in that box degrades to the defaults rather than erroring, because a Grafana user cannot
see our error body.

Notes are excluded by default and the set is derived by exclusion from `store.ValidEventTypes` — a
new event type shows up on dashboards automatically instead of being silently missing until someone
notices. Naming a type explicitly still returns it, notes included.

Empty results marshal to `[]`, never `null`: Grafana reads a null body as a datasource failure and
paints the panel as broken.

Known limitation: 1000 markers per query, applied to the *newest* events in the window. A panel is
read at its recent end, so truncating the other way would leave the right-hand side bare while the
far left stayed dense.

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
