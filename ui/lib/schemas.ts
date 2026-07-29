import { z } from "zod"

// --- form schemas ---

export const loginSchema = z.object({
  email: z.email("Enter a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
})

export const otpEmailSchema = z.object({
  email: z.email("Enter a valid email"),
})

export const otpCodeSchema = z.object({
  code: z.string().length(6, "Enter the 6-digit code"),
})

export const registerSchema = z.object({
  orgName: z.string().min(2, "Organization name must be at least 2 characters"),
  email: z.email("Enter a valid email"),
  password: z.string().min(8, "Password must be at least 8 characters"),
  confirm: z.string(),
}).refine((d) => d.password === d.confirm, {
  message: "Passwords do not match",
  path: ["confirm"],
})

export const otpRegisterSchema = z.object({
  orgName: z.string().min(2, "Organization name must be at least 2 characters"),
  email: z.email("Enter a valid email"),
})

export const createEnvSchema = z.object({
  name: z.string().min(1, "Environment name is required").max(50, "Name is too long"),
  clusterName: z.string().max(100, "Cluster name is too long").optional().default(""),
})

export const commentSchema = z.object({
  body: z.string().min(1, "Comment cannot be empty").max(2000, "Comment is too long"),
})

export const createEventSchema = z.object({
  type: z.enum(["deploy", "alert", "note"]),
  service: z.string().min(1, "Service name is required").max(100, "Too long"),
  version: z.string().max(100, "Too long").optional().default(""),
  message: z.string().max(500, "Too long").optional().default(""),
  status: z.enum(["success", "failure", "in_progress"]).default("success"),
})

// --- API response schemas ---

export const environmentSchema = z.object({
  id: z.string(),
  name: z.string(),
  cluster_name: z.string(),
  created_at: z.string(),
})

export const eventSchema = z.object({
  id: z.string(),
  type: z.enum(["deploy", "alert", "note", "teardown", "rollback", "scale"]),
  service: z.string(),
  environment_id: z.string(),
  timestamp: z.string(),
  metadata: z.record(z.string(), z.unknown()),
  status: z.enum(["success", "failure", "in_progress"]).default("success"),
  created_at: z.string(),
})

// A change that preceded an event and might explain it. The score is a
// heuristic ranking, not a causal claim — reasons carries the terms that fired
// so the UI can show its working rather than asking for blind trust.
export const suspectSchema = z.object({
  event: eventSchema,
  score: z.number(),
  confidence: z.enum(["high", "medium", "low"]),
  reasons: z.array(z.string()),
  lag_seconds: z.number(),
  lag_display: z.string(),
})

export const suspectsResponseSchema = z.object({
  event_id: z.string(),
  window_minutes: z.number(),
  candidates: z.number(),
  // The API always sends an array, but tolerate null so an empty result can
  // never surface as a parse error in front of someone mid-incident.
  suspects: z.array(suspectSchema).nullish().transform((s) => s ?? []),
})

// One search match. The event is present for both kinds: a comment match
// without its event is a quote with no context.
export const searchHitSchema = z.object({
  kind: z.enum(["event", "comment"]),
  event: eventSchema,
  comment: z
    .object({
      id: z.string(),
      body: z.string(),
      user_id: z.string(),
      created_at: z.string(),
    })
    .optional(),
  rank: z.number(),
})

export const searchResponseSchema = z.object({
  query: z.string(),
  count: z.number(),
  results: z.array(searchHitSchema).nullish().transform((r) => r ?? []),
})

// A generated postmortem document. The markdown is always present; only the
// narrative section depends on plan and model availability, which
// narrative_status explains.
export const postmortemSchema = z.object({
  markdown: z.string(),
  environment_name: z.string(),
  from: z.string(),
  to: z.string(),
  event_count: z.number(),
  comment_count: z.number(),
  participants: z.array(z.string()).nullish().transform((p) => p ?? []),
  narrative_status: z.enum([
    "included",
    "not_requested",
    "upgrade_required",
    "unavailable",
    "failed",
  ]),
  truncated: z.boolean(),
})

export const commentResponseSchema = z.object({
  id: z.string(),
  event_id: z.string(),
  user_id: z.string(),
  body: z.string(),
  created_at: z.string(),
  // Decision fields. Nullish rather than required because a comment arriving
  // over SSE is serialised from the create path, which does not carry them.
  is_decision: z.boolean().nullish().transform((v) => v ?? false),
  decided_by: z.string().nullish(),
  decided_at: z.string().nullish(),
})

// A comment promoted to a decision, with the event it was written on. The
// event context is what makes it readable months later: "we're rolling back"
// means nothing without the deploy it was said about.
export const decisionSchema = z.object({
  id: z.string(),
  event_id: z.string(),
  body: z.string(),
  author: z.string(),
  created_at: z.string(),
  decided_by: z.string().nullish(),
  decided_at: z.string().nullish(),
  event_type: z.string(),
  event_service: z.string(),
  event_timestamp: z.string(),
  environment_id: z.string(),
  environment_name: z.string(),
})

export const decisionsResponseSchema = z.object({
  decisions: z.array(decisionSchema).nullish().transform((d) => d ?? []),
  total: z.number(),
})

export const incidentSchema = z.object({
  id: z.string(),
  title: z.string(),
  severity: z.enum(["sev1", "sev2", "sev3", "sev4"]),
  status: z.enum(["open", "mitigated", "resolved"]),
  // owner_id and resolved_at are omitted by the API when unset.
  owner_id: z.string().optional().default(""),
  opened_at: z.string(),
  resolved_at: z.string().optional().default(""),
  created_at: z.string(),
  event_count: z.number().optional().default(0),
})

export const createIncidentSchema = z.object({
  title: z.string().min(1, "Title is required").max(200, "Title is too long"),
  severity: z.enum(["sev1", "sev2", "sev3", "sev4"]).default("sev3"),
})

export type LoginInput = z.infer<typeof loginSchema>
export type CommentInput = z.infer<typeof commentSchema>
