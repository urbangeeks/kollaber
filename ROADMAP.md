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

### 5. ArgoCD / Terraform / Atlantis ingestion — **shipped**

`/webhooks/argocd`, `/webhooks/terraform` and `/webhooks/atlantis`, each normalizing its source onto
the existing event shape. No data model change; the target environment comes from
`?environment_id=`, since a destination URL is the only per-target setting any of these tools has.

Argo CD is the odd one: its notification service builds the body from a Go template the operator
writes, so the payload is a contract we publish rather than one we are handed. The docs carry the
template. Only `app` is required, because a template renders an unset field as an empty string and
an app that has never synced would otherwise 400 on its first notification.

Terraform records terminal outcomes only — `applied` and `errored`. A plan is not a change:
recording one would put a marker on the timeline for a run that touched nothing, count it as a
deployment in DORA, and hand suspect detection a change that never happened. Other statuses are
accepted and skipped, so enabling extra triggers is harmless, and the verification payload sent when
a notification config is saved falls through the same path.

Terraform signs with HMAC-SHA512 in `X-TFE-Notification-Signature`, bare hex with no algorithm
prefix — a different hash and a different framing from the SHA-256 the other webhooks use, so it
cannot share `verifyHMAC`. Atlantis and Argo CD sign nothing and send static headers, so both use
the shared secret.

Atlantis posts only after an apply, so every delivery is a real change with no plan-stage noise to
filter. Its payload is a Go struct marshalled without json tags, which makes the wire format
capitalised field names — the one shape a lowercase tag reads as empty.

Known limitation: Terraform's notification payload carries no destroy flag, so a `terraform destroy`
run is recorded as a deploy rather than a teardown. Argo CD can send a teardown because its template
names the type explicitly.

### 6. Weekly digest email — **shipped**

A Monday recap of the week that ended: deploys and failures per environment, rollbacks, alerts,
incidents opened and resolved, and the events that drew the most discussion. Opt-in through the
existing `notification_prefs.notify_on` array, so there is one place to unsubscribe from Kollaber's
mail rather than three.

Threads are ranked by comments written *during* the week rather than by the event's own age, so a
months-old event the team argued about on Tuesday still surfaces — which is the conversation someone
would otherwise miss.

The scheduler runs in-process rather than as a cron job, so a digest arrives on a plain `docker run`
install with nothing configured. Correctness under more than one replica comes from the claim in
`digest_sends`, not from there being a single scheduler: every pod runs the same schedule and the
`INSERT ... ON CONFLICT DO NOTHING` decides which one sends. A failed send releases its claim, so a
transient error costs a retry rather than the whole week.

A week with nothing in it sends nothing, and environments with no activity are dropped from the
email — a wall of zeroes every Monday is how a digest teaches people to filter it.

Deliberately no AI summary. The digest is a nudge to open the timeline, and the numbers plus the
busiest threads already do that; a per-org model call every week would add cost and a failure mode
to an email whose whole job is to be reliable and cheap. The postmortem generator is where the
narrative belongs, and it runs when someone asks for it.

---

## Tier 3 — collaboration depth

### 7. Action items on comments

Promote a comment to a tracked follow-up with owner and due date. Open-items view per environment.

Postmortem action items are famously where good intentions go to die. Chasing them is a real,
unglamorous job no tool does well.

### 8. Decision log — **shipped**

`PATCH /comments/:id` promotes a comment to a decision; `GET /decisions` returns the org's log,
newest first, optionally scoped to one environment. A `/decisions` page lists them and the timeline
grows a **Mark as decision** action on each comment.

Each decision carries the event it was written on. "We're rolling back" is a sentence with no
subject without the deploy it was said about, and the whole point is being readable six months
later.

Ordered by when the comment was written rather than when it was marked, so someone tidying up old
threads on a Friday does not reshuffle the history. Marking is curation and never touches the body;
`decided_by` records who promoted it, which need not be the author. Unmarking clears the
attribution so a re-marked comment cannot carry a stale one. Viewers cannot mark.

While building this, `GET /events/:id/comments` was found to filter on `event_id` alone — no org
scope at all, so any authenticated user could read any org's discussion by guessing a uuid, and
`POST` wrote into another tenant's thread the same way. Both now resolve the event through
`environments` first. `internal/store/decisions.go` carries the replacement query and the tests
assert isolation in both directions.

### 9. Service version inventory — **shipped**

`GET /inventory?environment_id=…&at=…` returns what every service in an environment was running at a
moment, defaulting to now, derived entirely from deploy events. An `/inventory` page keeps both the
environment and the instant in the URL, so the answer is a link you can paste into an incident
thread.

Only successful deploys and rollbacks count. A failed deploy did not change what is running, and
treating it as current is how an inventory claims a build is in production that never got there; an
in-progress one has not landed either. A rollback counts and is flagged, because what is running is
then not the newest thing anyone shipped.

The version is read from a key chain — `version`, `image_tag`, `revision`, `head_commit`, `to` —
because every ingestion path names it differently, the same problem DORA already solves for commit
timestamps. When the deploy that landed carried none of them the service reports *unknown* rather
than the previous version, which would name a build that is not running.

Known limitation: point-in-time only. "When did this service last change?" is answerable from the
timeline but has no dedicated endpoint yet.

---

## Tier 4 — guardrails

### 10. Change freeze windows — **shipped**

`/freezes` declares a period when the org would rather nothing changed, scoped to one environment or
left org-wide (`environment_id` null), which is what a company-wide Black Friday freeze actually is.
Admins only: anyone who can declare a freeze can make every deploy in the company report a
violation.

Kollaber does not block. A change that lands inside a window is stamped `frozen` and
`freeze_reason` in its metadata, the timeline shows a badge, and `kollaber deploy` exits **2** —
distinct from 1 so a pipeline can tell a freeze apart from a network failure — with
`--allow-frozen` for a release meant to go out anyway. Blocking would put Kollaber on the critical
path of every deploy, which is a promise a tool that sits beside the stack should not make.

The mark is written at ingest rather than derived at read time, so it survives the window being
edited or deleted. What matters six months later is that this deploy went out during a declared
freeze, not what the freeze calendar says today. Deleting a window therefore leaves past marks
standing.

Only changes are flagged — deploy, rollback, scale, teardown. A freeze is a statement about
changing things, so an alert firing or a note being written during one violates nothing. Every
ingest path is covered: the API, the generic webhook, Terraform, Atlantis and Argo CD. Terraform is
asked against the run's own timestamp, so a delivery arriving after the freeze lifted is still
judged on when the apply happened.

Overlapping windows resolve to the one ending latest — the one still in force after the others
lapse, and the honest one to name.

While building this, `POST /events` was found to accept any `environment_id` with no org check,
letting an authenticated user write into another tenant's timeline. The freeze lookup needs the
environment resolved anyway, so ownership is now verified in the same step.

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
