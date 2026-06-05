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
  type: z.enum(["deploy", "alert", "note", "teardown"]),
  service: z.string(),
  environment_id: z.string(),
  timestamp: z.string(),
  metadata: z.record(z.string(), z.unknown()),
  status: z.enum(["success", "failure", "in_progress"]).default("success"),
  created_at: z.string(),
})

export const commentResponseSchema = z.object({
  id: z.string(),
  event_id: z.string(),
  user_id: z.string(),
  body: z.string(),
  created_at: z.string(),
})

export type LoginInput = z.infer<typeof loginSchema>
export type CommentInput = z.infer<typeof commentSchema>
