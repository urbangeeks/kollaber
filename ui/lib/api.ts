import { z } from "zod"
import { environmentSchema, eventSchema, commentResponseSchema } from "./schemas"

const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

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
}

export function getEvents(environmentId: string, limit = 50, offset = 0, filters: EventFilters = {}): Promise<Event[]> {
  const params = new URLSearchParams({ environment_id: environmentId, limit: String(limit), offset: String(offset) })
  if (filters.type)    params.set("type",    filters.type)
  if (filters.service) params.set("service", filters.service)
  if (filters.status)  params.set("status",  filters.status)
  if (filters.before)  params.set("before",  filters.before)
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

export async function acceptInvite(token: string, email: string, password: string): Promise<string> {
  const data = await request(`/invites/${token}/accept`, z.object({ token: z.string() }), {
    method: "POST",
    body: JSON.stringify({ email, password }),
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
