import { env } from "@/config/env"
import type { APIError } from "@/types/api"

export class ApiError extends Error {
  constructor(
    public readonly code: string,
    message: string,
    public readonly status: number
  ) {
    super(message)
    this.name = "ApiError"
  }
}

const ERROR_MESSAGES: Record<string, string> = {
  SESSION_NOT_FOUND:         "This session has ended.",
  TABLE_OCCUPIED:            "This table already has an active session.",
  PARTICIPANT_UNAUTHORIZED:  "You're not part of this session.",
  CART_EMPTY:                "Your cart is empty.",
  ORDER_ALREADY_PLACED:      "This order was already placed.",
  INSUFFICIENT_ROLE:         "You don't have permission to do this.",
  INVALID_PHONE:             "Please enter a valid 10-digit mobile number.",
}

export function friendlyErrorMessage(code: string): string {
  return ERROR_MESSAGES[code] ?? "Something went wrong. Please try again."
}

type RequestOptions = {
  participantId?: number
  staffToken?: string
  body?: unknown
  method?: string
}

async function request<T>(
  path: string,
  { participantId, staffToken, body, method = "GET" }: RequestOptions = {}
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  if (participantId != null) {
    headers["X-Participant-ID"] = String(participantId)
  }

  if (staffToken) {
    headers["Authorization"] = `Bearer ${staffToken}`
  }

  // In local dev, pass tenant slug via header so the backend can identify the tenant
  // without a real subdomain. Backend reads this only when BASE_DOMAIN is unset.
  if (env.tenantSlug) {
    headers["X-Tenant-Slug"] = env.tenantSlug
  }

  const res = await fetch(`${env.apiUrl}${path}`, {
    method,
    headers,
    body: body != null ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    let errBody: APIError = { code: "INTERNAL_ERROR", message: res.statusText }
    try {
      errBody = await res.json()
    } catch {}
    throw new ApiError(errBody.code, errBody.message, res.status)
  }

  if (res.status === 204) {
    return undefined as T
  }

  return res.json() as Promise<T>
}

export const api = {
  get: <T>(path: string, opts?: Omit<RequestOptions, "body" | "method">) =>
    request<T>(path, { ...opts, method: "GET" }),

  post: <T>(path: string, body: unknown, opts?: Omit<RequestOptions, "body" | "method">) =>
    request<T>(path, { ...opts, method: "POST", body }),

  patch: <T>(path: string, body: unknown, opts?: Omit<RequestOptions, "body" | "method">) =>
    request<T>(path, { ...opts, method: "PATCH", body }),

  delete: <T>(path: string, opts?: Omit<RequestOptions, "body" | "method">) =>
    request<T>(path, { ...opts, method: "DELETE" }),
}
