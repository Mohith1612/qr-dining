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
  SESSION_CLOSED:            "Your table session has paused. Please refresh to continue.",
  PAYMENT_IN_PROGRESS:       "A payment is being processed — ordering is paused until it's settled.",
  MENU_ITEM_UNAVAILABLE:     "One of your items just became unavailable. Please review your cart.",
  TABLE_OCCUPIED:            "This table already has an active session.",
  PARTICIPANT_UNAUTHORIZED:  "You're not part of this session.",
  CART_EMPTY:                "Your cart is empty.",
  ORDER_ALREADY_PLACED:      "This order was already placed.",
  INSUFFICIENT_ROLE:         "You don't have permission to do this.",
  INVALID_PHONE:             "Please enter a valid 10-digit mobile number.",
  NOT_SESSION_HOST:          "Only the table host can do this. Ask the host to send the order or request the bill.",
  FEATURE_DISABLED:          "This restaurant doesn't offer saved preferences.",
  MODIFIER_CONFLICT:         "Only one option may be chosen from this group.",
}

export function friendlyErrorMessage(code: string): string {
  return ERROR_MESSAGES[code] ?? "Something went wrong. Please try again."
}

type RequestOptions = {
  staffToken?: string
  guestToken?: string
  platformToken?: string
  body?: unknown
  method?: string
}

async function request<T>(
  path: string,
  { staffToken, guestToken, platformToken, body, method = "GET" }: RequestOptions = {}
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  }

  // Platform is a separate trust domain; its token uses the same Bearer scheme
  // but is never mixed with staff/guest tokens by callers.
  if (staffToken || guestToken || platformToken) {
    headers["Authorization"] = `Bearer ${staffToken ?? guestToken ?? platformToken}`
  }

  // In local dev, pass tenant slug via header so the backend can identify the tenant
  // without a real subdomain. Backend reads this only when BASE_DOMAIN is unset.
  if (env.tenantSlug) {
    headers["X-Tenant-Slug"] = env.tenantSlug
  }

  const res = await fetch(`${env.apiUrl}${path}`, {
    method,
    credentials: "include",
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

type CallOptions = Omit<RequestOptions, "body" | "method">

export const api = {
  get: <T>(path: string, opts?: CallOptions) =>
    request<T>(path, { ...opts, method: "GET" }),

  post: <T>(path: string, body: unknown, opts?: CallOptions) =>
    request<T>(path, { ...opts, method: "POST", body }),

  patch: <T>(path: string, body: unknown, opts?: CallOptions) =>
    request<T>(path, { ...opts, method: "PATCH", body }),

  put: <T>(path: string, body: unknown, opts?: CallOptions) =>
    request<T>(path, { ...opts, method: "PUT", body }),

  delete: <T>(path: string, opts?: CallOptions) =>
    request<T>(path, { ...opts, method: "DELETE" }),
}
