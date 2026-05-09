import { z } from "zod"
import { environmentSchema, eventSchema, commentResponseSchema } from "./schemas"

const API = process.env.NEXT_PUBLIC_API_URL ?? ""

export type Environment = z.infer<typeof environmentSchema>
export type Event = z.infer<typeof eventSchema>
export type Comment = z.infer<typeof commentResponseSchema>

export function getToken(): string | null {
  if (typeof window === "undefined") return null
  return localStorage.getItem("token")
}

export function setToken(token: string) {
  localStorage.setItem("token", token)
}

export function removeToken() {
  localStorage.removeItem("token")
}

async function request<T>(path: string, schema: z.ZodType<T>, init?: RequestInit): Promise<T>
async function request(path: string, schema: null, init?: RequestInit): Promise<unknown>
async function request<T>(path: string, schema: z.ZodType<T> | null, init?: RequestInit): Promise<T | unknown> {
  const token = getToken()
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error(body.error ?? res.statusText)
  }
  if (res.status === 204 || res.headers.get("content-length") === "0") return undefined
  const data = await res.json()
  return schema ? schema.parse(data) : data
}

export async function register(email: string, password: string, orgName: string): Promise<string> {
  const data = await request("/auth/register", z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ email, password, org_name: orgName }),
  })
  return (data as { token: string }).token
}

export async function login(email: string, password: string): Promise<string> {
  const data = await request("/auth/login", z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ email, password }),
  })
  return (data as { token: string }).token
}

export function getEnvironments(): Promise<Environment[]> {
  return request("/environments", z.array(environmentSchema)) as Promise<Environment[]>
}

export type EventFilters = {
  type?: string
  service?: string
  status?: string
  before?: string
  after?: string
}

export function getEvents(environmentId: string, limit = 50, offset = 0, filters: EventFilters = {}): Promise<Event[]> {
  const params = new URLSearchParams({ environment_id: environmentId, limit: String(limit), offset: String(offset) })
  if (filters.type)    params.set("type",    filters.type)
  if (filters.service) params.set("service", filters.service)
  if (filters.status)  params.set("status",  filters.status)
  if (filters.before)  params.set("before",  filters.before)
  if (filters.after)   params.set("after",   filters.after)
  return request(`/events?${params}`, z.array(eventSchema)) as Promise<Event[]>
}

export function createEnvironment(name: string, clusterName: string): Promise<Environment> {
  return request("/environments", environmentSchema, {
    method: "POST",
    body: JSON.stringify({ name, cluster_name: clusterName }),
  }) as Promise<Environment>
}

const orgStatSchema = z.object({
  id: z.string(),
  name: z.string(),
  slug: z.string(),
  member_count: z.number(),
  env_count: z.number(),
  created_at: z.string(),
})

export type OrgStat = z.infer<typeof orgStatSchema>

export async function createInvite(role: "admin" | "member" | "viewer" = "member"): Promise<string> {
  const data = await request("/invites", z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ role }),
  })
  return (data as { token: string }).token
}

export async function getInvite(token: string): Promise<{ org_name: string; expires_at: string }> {
  return request(
    `/invites/${token}`,
    z.object({ org_name: z.string(), expires_at: z.string() }),
  ) as Promise<{ org_name: string; expires_at: string }>
}

export async function sendOTP(email: string): Promise<void> {
  await request("/auth/otp/send", null, {
    method: "POST",
    body: JSON.stringify({ email }),
  })
}

export async function verifyOTP(email: string, code: string, orgName?: string): Promise<{ token: string; new_user: boolean }> {
  return request(
    "/auth/otp/verify",
    z.object({ token: z.string(), new_user: z.boolean() }),
    {
      method: "POST",
      body: JSON.stringify({ email, code, org_name: orgName ?? "" }),
    },
  ) as Promise<{ token: string; new_user: boolean }>
}

export async function acceptInvite(token: string, email: string, code: string): Promise<string> {
  const data = await request(`/invites/${token}/accept`, z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ email, code }),
  })
  return (data as { token: string }).token
}

export type OrgItem = { id: string; name: string; slug: string; role: string }

export function getMyOrgs(): Promise<OrgItem[]> {
  return request("/auth/orgs", z.array(z.object({
    id: z.string(), name: z.string(), slug: z.string(), role: z.string(),
  }))) as Promise<OrgItem[]>
}

export async function createOrg(name: string): Promise<{ token: string; org_id: string; org_name: string }> {
  return request("/auth/orgs", z.object({ token: z.string(), org_id: z.string(), org_name: z.string() }), {
    method: "POST",
    body: JSON.stringify({ name }),
  }) as Promise<{ token: string; org_id: string; org_name: string }>
}

export async function renameOrg(id: string, name: string): Promise<{ id: string; name: string; slug: string }> {
  return request(`/auth/orgs/${id}`, z.object({ id: z.string(), name: z.string(), slug: z.string() }), {
    method: "PUT",
    body: JSON.stringify({ name }),
  }) as Promise<{ id: string; name: string; slug: string }>
}

export async function switchOrg(orgId: string): Promise<string> {
  const data = await request("/auth/switch", z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ org_id: orgId }),
  })
  return (data as { token: string }).token
}

export async function joinInvite(token: string): Promise<string> {
  const data = await request(`/invites/${token}/join`, z.object({ token: z.string() }), {
    method: "POST",
  })
  return (data as { token: string }).token
}

export type Member = {
  id: string
  email: string
  role: "owner" | "admin" | "member" | "viewer"
  joined_at: string
}

export type PendingInvite = {
  token: string
  role: "admin" | "member" | "viewer"
  created_by_email: string
  created_at: string
  expires_at: string
}

const memberSchema = z.object({
  id: z.string(),
  email: z.string(),
  role: z.enum(["owner", "admin", "member", "viewer"]),
  joined_at: z.string(),
})

const pendingInviteSchema = z.object({
  token: z.string(),
  role: z.enum(["admin", "member", "viewer"]),
  created_by_email: z.string(),
  created_at: z.string(),
  expires_at: z.string(),
})

export function getMembers(): Promise<Member[]> {
  return request("/members", z.array(memberSchema)) as Promise<Member[]>
}

export function updateMemberRole(userID: string, role: "admin" | "member" | "viewer"): Promise<void> {
  return request(`/members/${userID}`, null, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  }) as Promise<void>
}

export function removeMember(userID: string): Promise<void> {
  return request(`/members/${userID}`, null, { method: "DELETE" }) as Promise<void>
}

export function getPendingInvites(): Promise<PendingInvite[]> {
  return request("/members/invites", z.array(pendingInviteSchema)) as Promise<PendingInvite[]>
}

export function revokeInvite(token: string): Promise<void> {
  return request(`/members/invites/${token}`, null, { method: "DELETE" }) as Promise<void>
}

export function getAdminOrgs(): Promise<OrgStat[]> {
  return request("/admin/orgs", z.array(orgStatSchema)) as Promise<OrgStat[]>
}

function decodeToken(): Record<string, unknown> | null {
  if (typeof window === "undefined") return null
  const token = getToken()
  if (!token) return null
  try {
    return JSON.parse(atob(token.split(".")[1]))
  } catch {
    return null
  }
}

export function isAdmin(): boolean {
  return !!decodeToken()?.is_admin
}

export function getCurrentEmail(): string {
  return (decodeToken()?.email as string) ?? ""
}

export function getCurrentRole(): "owner" | "admin" | "member" | "viewer" | null {
  const role = decodeToken()?.role
  if (role === "owner" || role === "admin" || role === "member" || role === "viewer") return role
  return null
}

export function createEvent(
  environmentId: string,
  type: "deploy" | "alert" | "note",
  service: string,
  metadata: Record<string, string>,
  status: "success" | "failure" | "in_progress" = "success",
): Promise<Event> {
  return request("/events", eventSchema, {
    method: "POST",
    body: JSON.stringify({ type, service, environment_id: environmentId, metadata, status }),
  }) as Promise<Event>
}

export async function generateCLIToken(): Promise<string> {
  const data = await request("/auth/token", z.object({ token: z.string() }), { method: "POST" })
  return (data as { token: string }).token
}

export function updateEnvironment(id: string, name: string, clusterName: string): Promise<Environment> {
  return request(`/environments/${id}`, environmentSchema, {
    method: "PUT",
    body: JSON.stringify({ name, cluster_name: clusterName }),
  }) as Promise<Environment>
}

export async function deleteEnvironment(id: string): Promise<void> {
  await request(`/environments/${id}`, null, { method: "DELETE" })
}

export type EnvStat = {
  environment_id: string
  deploys: number
  alerts: number
  notes: number
  last_event_at: string | null
}

export function getEnvStats(): Promise<EnvStat[]> {
  return request(
    "/environments/stats",
    z.array(z.object({
      environment_id: z.string(),
      deploys: z.number(),
      alerts: z.number(),
      notes: z.number(),
      last_event_at: z.string().nullable(),
    })),
  ) as Promise<EnvStat[]>
}

export function getServices(environmentId: string): Promise<string[]> {
  return request(`/services?environment_id=${environmentId}`, z.array(z.string())) as Promise<string[]>
}

export function getComments(eventId: string): Promise<Comment[]> {
  return request(`/events/${eventId}/comments`, z.array(commentResponseSchema)) as Promise<Comment[]>
}

export function createComment(eventId: string, body: string): Promise<Comment> {
  return request(`/events/${eventId}/comments`, commentResponseSchema, {
    method: "POST",
    body: JSON.stringify({ body }),
  }) as Promise<Comment>
}

export async function getEventSummary(eventId: string): Promise<string> {
  const data = await request(`/events/${eventId}/summary`, null, { method: "POST" })
  return (data as { summary: string }).summary
}

export async function getEventPostmortem(eventId: string): Promise<string> {
  const data = await request(`/events/${eventId}/postmortem`, null, { method: "POST" })
  return (data as { postmortem: string }).postmortem
}

const notificationPrefsSchema = z.object({
  notify_on: z.array(z.string()),
  notification_email: z.string(),
})

export type NotificationPrefs = { notifyOn: string[]; notificationEmail: string }

export async function getNotificationPrefs(): Promise<NotificationPrefs> {
  const data = await request("/settings/notifications", notificationPrefsSchema) as z.infer<typeof notificationPrefsSchema>
  return { notifyOn: data.notify_on, notificationEmail: data.notification_email }
}

export async function updateNotificationPrefs(notifyOn: string[], notificationEmail: string): Promise<void> {
  await request("/settings/notifications", null, {
    method: "PUT",
    body: JSON.stringify({ notify_on: notifyOn, notification_email: notificationEmail }),
  })
}

const slackSettingsSchema = z.object({ webhook_url: z.string() })

export async function getSlackSettings(): Promise<string> {
  const data = await request("/settings/slack", slackSettingsSchema)
  return (data as { webhook_url: string }).webhook_url
}

export async function updateSlackSettings(webhookUrl: string): Promise<void> {
  await request("/settings/slack", null, {
    method: "PUT",
    body: JSON.stringify({ webhook_url: webhookUrl }),
  })
}

export async function testSlackSettings(): Promise<void> {
  await request("/settings/slack/test", null, { method: "POST" })
}

const teamsSettingsSchema = z.object({ webhook_url: z.string() })

export async function getTeamsSettings(): Promise<string> {
  const data = await request("/settings/teams", teamsSettingsSchema)
  return (data as { webhook_url: string }).webhook_url
}

export async function updateTeamsSettings(webhookUrl: string): Promise<void> {
  await request("/settings/teams", null, {
    method: "PUT",
    body: JSON.stringify({ webhook_url: webhookUrl }),
  })
}

export async function testTeamsSettings(): Promise<void> {
  await request("/settings/teams/test", null, { method: "POST" })
}

const billingStatusSchema = z.object({
  plan: z.string(),
  subscription_status: z.string(),
  seats_used: z.number(),
  seats_limit: z.number(),
  environments_used: z.number(),
  environments_limit: z.number(),
  history_days: z.number(),
  has_stripe_customer: z.boolean(),
  entitlements: z.object({
    kubernetes_ingestion: z.boolean(),
    ai_summaries: z.boolean(),
    ai_postmortems: z.boolean(),
    sso: z.boolean(),
    audit_logs: z.boolean(),
  }),
})

export type BillingStatus = z.infer<typeof billingStatusSchema>

export function getBillingStatus(): Promise<BillingStatus> {
  return request("/billing", billingStatusSchema) as Promise<BillingStatus>
}

export async function createCheckoutSession(plan: string): Promise<string> {
  const data = await request("/billing/checkout", z.object({ url: z.string() }), {
    method: "POST",
    body: JSON.stringify({ plan }),
  })
  return (data as { url: string }).url
}

export async function createPortalSession(): Promise<string> {
  const data = await request("/billing/portal", z.object({ url: z.string() }), { method: "POST" })
  return (data as { url: string }).url
}

export type AuditLogEntry = {
  id: string
  actor_email: string
  action: string
  target_type: string
  target_id: string
  metadata: Record<string, unknown>
  created_at: string
}

export async function getAuditLogs(limit = 50, offset = 0): Promise<AuditLogEntry[]> {
  const data = await request(`/audit-logs?limit=${limit}&offset=${offset}`, null)
  return (data as AuditLogEntry[]) ?? []
}

export type SSOConfig = {
  issuer_url: string
  client_id: string
  client_secret: string
  domain: string
  enabled: boolean
  callback_url: string
}

export function getSSOConfig(): Promise<SSOConfig> {
  return request("/settings/sso", null) as Promise<SSOConfig>
}

export async function saveSSOConfig(cfg: Omit<SSOConfig, "callback_url">): Promise<void> {
  await request("/settings/sso", null, { method: "PUT", body: JSON.stringify(cfg) })
}
