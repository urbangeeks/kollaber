/**
 * Content for the /integrations landing pages.
 *
 * Every setup step, field name and behavioural note here is taken from the
 * shipped handlers (internal/api/*.go) and the docs page. Nothing on these
 * pages describes a capability Kollaber does not have — they are indexed
 * marketing pages, so a wrong claim is a wrong claim in public.
 */

export type Step = {
  title: string
  body: string
  code?: string
}

export type Capture = {
  label: string
  detail: string
}

export type Faq = {
  q: string
  a: string
}

export type Integration = {
  slug: string
  name: string
  /** Short label for the index grid. */
  tagline: string
  /** Grouping on the index page. */
  category: "Deployments" | "Infrastructure as code" | "Alerting"
  /** <title> — the root layout appends " — Kollaber". */
  title: string
  description: string
  h1: string
  intro: string
  captures: Capture[]
  steps: Step[]
  notes: string[]
  faqs: Faq[]
  /** Anchor on /docs with the full reference. */
  docsAnchor: string
}

export const INTEGRATIONS: Integration[] = [
  {
    slug: "github-actions",
    name: "GitHub Actions",
    tagline: "Record every workflow deploy from a single curl step.",
    category: "Deployments",
    title: "GitHub Actions deployment tracking",
    description:
      "Record every GitHub Actions deploy on a shared timeline alongside alerts and incidents. One curl step, no runner plugin, no agent to install.",
    h1: "Track GitHub Actions deployments on a shared timeline",
    intro:
      "GitHub gives you a list of workflow runs, one repository at a time. What it cannot tell you is whether the alert that fired this afternoon followed a deploy from a different service. Kollaber records each workflow deploy as an event on one timeline, so the deploy and the alert that followed it sit next to each other.",
    captures: [
      { label: "Service", detail: "The repository the workflow ran in." },
      { label: "Version", detail: "The commit SHA that was deployed." },
      { label: "Author", detail: "The actor who triggered the workflow." },
      { label: "Ref", detail: "The branch or tag the deploy came from." },
    ],
    steps: [
      {
        title: "Store your environment ID as a secret",
        body: "Find the environment UUID on the dashboard under your environment settings, then add it to the repository as a secret named KOLLABER_ENV_ID.",
      },
      {
        title: "Add a step to the end of your deploy job",
        body: "The webhook endpoint takes a plain JSON body and needs no authentication, so a single curl step is the whole integration.",
        code: `- name: Notify Kollaber
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
      }'`,
      },
      {
        title: "Run the workflow",
        body: "The deploy appears on the environment timeline immediately over SSE — no refresh, no polling.",
      },
    ],
    notes: [
      "The metadata object is free-form. Anything you add is stored on the event and shown on the timeline, so build numbers, changelog links, and approver names all work.",
      "Run the step with `if: always()` and send a status field if you want failed deploys recorded too — failures are what DORA change failure rate is computed from.",
    ],
    faqs: [
      {
        q: "Does this need a GitHub App or marketplace action?",
        a: "No. The endpoint accepts a plain JSON POST, so a curl step is enough. There is nothing to install and no OAuth flow to approve.",
      },
      {
        q: "Can one repository deploy to several environments?",
        a: "Yes. The environment is chosen by the environment_id in the body, so pass a different secret per job or per matrix leg to split staging from production.",
      },
      {
        q: "Do these deploys count toward DORA metrics?",
        a: "Yes. Events of type deploy feed deployment frequency and lead time, and failed deploys feed change failure rate.",
      },
    ],
    docsAnchor: "webhooks",
  },
  {
    slug: "argo-cd",
    name: "Argo CD",
    tagline: "Turn sync notifications into a durable deployment history.",
    category: "Deployments",
    title: "Argo CD deployment history and tracking",
    description:
      "Give Argo CD a deployment history that outlives the UI. Record every sync as an event on a shared timeline, with revision, health, and sync status attached.",
    h1: "Argo CD deployment history that outlives the UI",
    intro:
      "Argo CD knows what is running right now, and keeps a short window of what came before. It is not built to answer what changed in this cluster three weeks ago, or which sync preceded last month's incident. Kollaber records each sync as a timeline event, so the history stays after Argo has moved on.",
    captures: [
      { label: "Service", detail: "The Argo CD application name." },
      { label: "Revision", detail: "The Git revision the sync applied." },
      { label: "Sync status", detail: "Synced, out of sync, and the operation phase." },
      { label: "Health", detail: "Application health at the time of the sync." },
      { label: "Placement", detail: "The Argo project and destination namespace." },
    ],
    steps: [
      {
        title: "Add a webhook service and template",
        body: "Argo CD builds the request body from a template you write, so the body below is the contract. Add both blocks to argocd-notifications-cm.",
        code: `service.webhook.kollaber: |
  url: https://kollaber.io/webhooks/argocd?environment_id=<uuid>
  headers:
  - name: X-Kollaber-Secret
    value: $kollaber-webhook-secret

template.kollaber-sync: |
  webhook:
    kollaber:
      method: POST
      body: |
        {
          "app": "{{.app.metadata.name}}",
          "revision": "{{.app.status.sync.revision}}",
          "sync_status": "{{.app.status.sync.status}}",
          "health_status": "{{.app.status.health.status}}",
          "operation_phase": "{{.app.status.operationState.phase}}",
          "project": "{{.app.spec.project}}",
          "namespace": "{{.app.spec.destination.namespace}}"
        }`,
      },
      {
        title: "Subscribe an application",
        body: "Annotate the Application you want recorded. Any trigger works; on-sync-succeeded is the usual starting point.",
        code: `notifications.argoproj.io/subscribe.on-sync-succeeded.kollaber: ""`,
      },
      {
        title: "Sync and check the timeline",
        body: "The next successful sync lands on the environment timeline with the revision and health attached.",
      },
    ],
    notes: [
      "Only the app field is required. Everything else enriches the event, so a trimmed template still works.",
      "Event status comes from operation_phase first and health_status second. A sync that succeeded onto a degraded app is recorded as a successful change, with the health left in the metadata — the change did land, and conflating the two would distort change failure rate.",
      'Add "type": "teardown" to the body on an on-app-deleted subscription. It defaults to deploy.',
    ],
    faqs: [
      {
        q: "Does this replace Argo CD's own notifications?",
        a: "No. It is an additional service in the same notifications config, so your existing Slack or email subscriptions keep working unchanged.",
      },
      {
        q: "How do I send different applications to different environments?",
        a: "Define one webhook service per environment, each with its own environment_id in the URL, and subscribe applications to the matching one.",
      },
      {
        q: "What happens when a sync fails?",
        a: "Subscribe on-sync-failed to the same service. The operation phase drives the event status, so a failed sync is recorded as a failed change and feeds change failure rate.",
      },
    ],
    docsAnchor: "webhooks",
  },
  {
    slug: "hcp-terraform",
    name: "HCP Terraform",
    tagline: "An audit trail of applies that actually touched infrastructure.",
    category: "Infrastructure as code",
    title: "HCP Terraform run history and audit trail",
    description:
      "Record every HCP Terraform apply on a shared timeline. Workspace notifications become a durable audit trail of infrastructure changes, with plans deliberately filtered out.",
    h1: "An audit trail of HCP Terraform runs",
    intro:
      "A workspace's run list tells you what Terraform did in that workspace. It does not put the apply next to the alert that followed it, or next to the application deploy that went out at the same time. Kollaber records completed runs as timeline events so infrastructure changes sit alongside everything else that changed.",
    captures: [
      { label: "Service", detail: "The workspace name." },
      { label: "Status", detail: "Applied as a success, errored as a failure." },
      { label: "Run detail", detail: "The run that produced the notification." },
    ],
    steps: [
      {
        title: "Open the workspace notification settings",
        body: "In HCP Terraform, go to Settings, then Notifications, then Create a notification, and choose Webhook.",
      },
      {
        title: "Point it at Kollaber",
        body: "Use your environment UUID in the URL and your WEBHOOK_SECRET as the token.",
        code: `URL:      https://kollaber.io/webhooks/terraform?environment_id=<uuid>
Token:    your WEBHOOK_SECRET
Triggers: Completed, Errored`,
      },
      {
        title: "Apply something",
        body: "The next completed run appears on the timeline, with the workspace as the service name.",
      },
    ],
    notes: [
      "The token signs the body with HMAC-SHA512, which Kollaber verifies against WEBHOOK_SECRET. Deliveries that fail verification are rejected.",
      "Only runs that reached your infrastructure are recorded: applied as a success, errored as a failure. Plan, cancel, and discard notifications are accepted and skipped, so enabling extra triggers is harmless — a plan is not a change, and recording one would add a timeline marker for a run that touched nothing and count it as a deployment in DORA.",
    ],
    faqs: [
      {
        q: "Is it safe to enable every notification trigger?",
        a: "Yes. Triggers that do not represent a change — plan, cancel, discard — are accepted and skipped rather than recorded, so nothing pollutes the timeline or the DORA figures.",
      },
      {
        q: "How do workspaces map to environments?",
        a: "The environment comes from the environment_id in the notification URL, so create one notification per workspace pointing at whichever environment it manages.",
      },
      {
        q: "Does this work with self-managed Terraform Enterprise?",
        a: "The webhook format is the same, so a Terraform Enterprise workspace configured with the same URL and token behaves identically.",
      },
    ],
    docsAnchor: "webhooks",
  },
  {
    slug: "atlantis",
    name: "Atlantis",
    tagline: "Every atlantis apply, with the pull request that caused it.",
    category: "Infrastructure as code",
    title: "Atlantis apply tracking and audit trail",
    description:
      "Record every atlantis apply on a shared timeline, with the pull request, branch, commit, and the engineer who ran it kept on the event.",
    h1: "Track every Atlantis apply, with the PR that caused it",
    intro:
      "Atlantis leaves its history in pull request comments, spread across repositories. Six weeks later, working out which apply changed a security group means searching GitHub. Kollaber records each apply as a timeline event with the pull request attached, so the change and its rationale stay together.",
    captures: [
      { label: "Service", detail: "The project from atlantis.yaml, falling back to the directory and then the repository." },
      { label: "Pull request", detail: "The PR number the apply ran from." },
      { label: "Commit", detail: "The branch and commit that were applied." },
      { label: "Actor", detail: "The user who ran atlantis apply." },
    ],
    steps: [
      {
        title: "Add an apply webhook to your server-side config",
        body: "Atlantis posts webhooks from repos.yaml. Point the apply event at Kollaber with your environment UUID.",
        code: `# repos.yaml
webhooks:
  - event: apply
    kind: http
    url: https://kollaber.io/webhooks/atlantis?environment_id=<uuid>`,
      },
      {
        title: "Send the secret as a header",
        body: "Atlantis cannot sign the body, so a shared secret in a static header is what it can offer. Set it on the server flag or the equivalent environment variable.",
        code: `--webhook-http-headers='{"Authorization":"Bearer $WEBHOOK_SECRET"}'`,
      },
      {
        title: "Run an apply",
        body: "Atlantis posts only after an apply has run, so every delivery is a real change to your infrastructure.",
      },
    ],
    notes: [
      "Use workspace-regex and branch-regex on the webhook to point different workspaces at different Kollaber environments.",
      "Because Atlantis fires after the apply completes, there is no plan noise to filter — every event on the timeline corresponds to infrastructure that actually changed.",
    ],
    faqs: [
      {
        q: "Will this record plans as well as applies?",
        a: "No. The webhook is registered on the apply event only, and Atlantis posts it after the apply has run, so plans never reach the timeline.",
      },
      {
        q: "How is the service name chosen?",
        a: "The project name from atlantis.yaml is used when present. Without one it falls back to the directory, and then to the repository name.",
      },
      {
        q: "Can one Atlantis server feed several environments?",
        a: "Yes. Register multiple webhooks with different environment_id values and use workspace-regex or branch-regex to decide which applies go where.",
      },
    ],
    docsAnchor: "webhooks",
  },
  {
    slug: "prometheus-alertmanager",
    name: "Prometheus Alertmanager",
    tagline: "Alerts on the same timeline as the deploys that caused them.",
    category: "Alerting",
    title: "Prometheus Alertmanager alerts with deploy context",
    description:
      "Send Prometheus Alertmanager alerts to a timeline that already holds your deploys, so the change that caused an alert is visible next to it. Repeat deliveries are deduplicated by fingerprint.",
    h1: "Alertmanager alerts, next to the deploys that caused them",
    intro:
      "Alertmanager tells you something is wrong. It cannot tell you that a deploy went out four minutes earlier, because it has never heard of your deploys. Kollaber puts both on one timeline, which turns the usual question — what changed just before this fired? — into something you can read off the screen.",
    captures: [
      { label: "Service", detail: "Resolved from the service, job, or app label, falling back to alertname." },
      { label: "Severity", detail: "The severity label, kept with the full label set." },
      { label: "Summary", detail: "The summary and description annotations." },
      { label: "Source", detail: "The generator URL back to the firing rule." },
    ],
    steps: [
      {
        title: "Add a receiver",
        body: "Point a webhook receiver at Kollaber with your environment UUID, and send the shared secret as a bearer token.",
        code: `receivers:
  - name: kollaber
    webhook_configs:
      - url: https://kollaber.io/webhooks/alertmanager?environment_id=<uuid>
        send_resolved: true
        http_config:
          authorization:
            type: Bearer
            credentials: <WEBHOOK_SECRET>`,
      },
      {
        title: "Route alerts to it",
        body: "Add the receiver to your route tree. Alerts can go to Kollaber as well as to your existing paging receiver.",
        code: `route:
  receiver: pagerduty
  routes:
    - receiver: kollaber
      continue: true`,
      },
      {
        title: "Wait for something to fire",
        body: "Each entry in the delivery becomes one alert event, timestamped when the alert started rather than when the webhook arrived.",
      },
    ],
    notes: [
      "Keep send_resolved: true. A firing alert is recorded as a failure and its resolution as a success, so without it the timeline shows problems that never end.",
      "Alertmanager re-delivers firing alerts every repeat_interval. Deliveries whose fingerprint already sits at the same status are skipped, so a long-running alert does not bury the timeline. A firing-to-resolved transition differs, so it still lands.",
      "Events are timestamped from startsAt, and from endsAt once resolved. Grouping and repeat_interval can delay a delivery by minutes, which would otherwise scramble the ordering against deploys.",
      "Alertmanager cannot compute an HMAC over the body, so the bearer token is the strongest authentication it can offer. Kollaber also accepts the secret in an X-Kollaber-Secret header.",
    ],
    faqs: [
      {
        q: "Does this replace PagerDuty or Opsgenie?",
        a: "No. Kollaber does not page anyone and evaluates no alert rules. Route alerts to it alongside your pager so the alert is recorded next to the changes around it.",
      },
      {
        q: "Will a noisy alert flood the timeline?",
        a: "No. Repeat deliveries are deduplicated by fingerprint, so an alert that fires for six hours is one event, not one event per repeat_interval.",
      },
      {
        q: "Which label becomes the service name?",
        a: "The first of service, job, or app that is set, falling back to alertname. Setting a service label on your rules gives the cleanest grouping.",
      },
    ],
    docsAnchor: "webhooks",
  },
  {
    slug: "kubernetes",
    name: "Kubernetes",
    tagline: "Rollouts, rollbacks, scales, and pod failures, automatically.",
    category: "Deployments",
    title: "Kubernetes deployment and rollback tracking",
    description:
      "Run a read-only watcher in your cluster and get every rollout, rollback, scale, and pod failure on a shared timeline automatically — no CI changes, no manual events.",
    h1: "Kubernetes rollouts and rollbacks, recorded automatically",
    intro:
      "Not every change to a cluster comes from a pipeline. Someone scales a deployment by hand, an HPA reacts to load, a rollout gets reverted at two in the morning. The kube-watcher observes the cluster directly, so those changes reach the timeline whether or not anything told CI about them.",
    captures: [
      { label: "Deploy", detail: "A Deployment, StatefulSet, or DaemonSet completed a rollout, with image tag, replica count, and duration." },
      { label: "Rollback", detail: "A workload's image reverted to a previously seen tag, recorded as a rollback rather than a generic deploy." },
      { label: "Scale", detail: "A replica count changed, manually or via HPA, with direction and old and new counts." },
      { label: "Teardown", detail: "A workload was removed. Requires reportDeletes." },
      { label: "Alert", detail: "Pod failures: CrashLoopBackOff, image pull errors, OOMKilled, and unschedulable pods." },
    ],
    steps: [
      {
        title: "Get a CLI token",
        body: "On the dashboard, go to Settings, then CLI Token.",
      },
      {
        title: "Install the watcher",
        body: "It runs as a Deployment using a ServiceAccount with read-only access to Deployments, StatefulSets, DaemonSets, and Pods. Install one release per cluster.",
        code: `helm install kollaber-watcher oci://ghcr.io/urbangeeks/charts/kube-watcher \\
  --set kollaber.env=prod \\
  --set kollaber.api=https://kollaber.io \\
  --set kollaber.token=<cli-token>`,
      },
      {
        title: "Scale something",
        body: "Change a replica count and the scale event appears on the timeline within seconds.",
      },
    ],
    notes: [
      "For several clusters, install one release each and point them at matching environments with kollaber.env.",
      "If you manage secrets externally with Vault, Sealed Secrets, or External Secrets Operator, set kollaber.existingSecret to a secret with a token key instead of letting the chart create one.",
      "Set watchNamespace to limit the watcher to a single namespace. Empty means all namespaces.",
      "The binary also runs outside the cluster against your kubeconfig, which is useful for trying it before deploying anything.",
    ],
    faqs: [
      {
        q: "What access does the watcher need?",
        a: "Read-only access to Deployments, StatefulSets, DaemonSets, and Pods through a ServiceAccount. It never writes to the cluster.",
      },
      {
        q: "Does this collect metrics or logs?",
        a: "No. It watches workload and pod state changes and sends events. Kollaber collects no metrics, logs, or traces.",
      },
      {
        q: "How does it tell a rollback from a deploy?",
        a: "It tracks image tags it has already seen for a workload. When a tag reverts to a previous one, the event is recorded as a rollback instead of a deploy.",
      },
    ],
    docsAnchor: "kubernetes",
  },
]

export function getIntegration(slug: string): Integration | undefined {
  return INTEGRATIONS.find((i) => i.slug === slug)
}
