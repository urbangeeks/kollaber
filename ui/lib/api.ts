const API = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"

export type Environment = {
  id: string
  name: string
  cluster_name: string
  created_at: string
}

export type Event = {
  id: string
  type: "deploy" | "alert" | "note"
  service: string
  environment_id: string
  timestamp: string
  metadata: Record<string, unknown>
  created_at: string
}

export type Comment = {
  id: string
  event_id: string
  user_id: string
  body: string
  created_at: string
}

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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
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
  return res.json()
}

export async function login(email: string, password: string): Promise<string> {
  const data = await request<{ token: string }>("/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  })
  return data.token
}

export function getEnvironments(): Promise<Environment[]> {
  return request("/environments")
}

export function getEvents(environmentId: string, limit = 50, offset = 0): Promise<Event[]> {
  return request(`/events?environment_id=${environmentId}&limit=${limit}&offset=${offset}`)
}

export function getComments(eventId: string): Promise<Comment[]> {
  return request(`/events/${eventId}/comments`)
}

export function createComment(eventId: string, body: string): Promise<Comment> {
  return request(`/events/${eventId}/comments`, {
    method: "POST",
    body: JSON.stringify({ body }),
  })
}
