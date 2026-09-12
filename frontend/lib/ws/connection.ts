"use client"

import { env } from "@/config/env"
import { sessionsApi } from "@/lib/api/sessions"
import { isRevokedCredentialError } from "@/lib/api/client"
import { reconcileSnapshot } from "./reconciliation"
import { useWsStore } from "@/store/ws"
import { useSessionStore } from "@/store/session"
import type { WSEnvelope, WSEventHandlerMap } from "@/types/ws"

const BACKOFF_STEPS_MS = [1000, 2000, 4000, 8000, 16000, 30000]
const MAX_ATTEMPTS = 10
const PING_INTERVAL_MS = 30_000
const TERMINAL_STATUSES: string[] = ["closed", "abandoned", "expired"]

// Statuses the server will issue a WebSocket ticket for (handlers.sessionServesRealtime).
// payment_pending belongs here: it is when PAYMENT_COMPLETED / PAYMENT_CANCELLED
// and the remaining ORDER_* transitions arrive.
const SOCKET_ELIGIBLE_STATUSES: string[] = ["active", "payment_pending"]

export class WSConnection {
  private ws: WebSocket | null = null
  private pingTimer: ReturnType<typeof setInterval> | null = null
  private attemptCount = 0
  private lastSequence = 0
  private stopped = false

  constructor(
    private readonly sessionId: string,
    private readonly handlers: WSEventHandlerMap
  ) {}

  connect(): void {
    if (this.stopped) return
    void this.openSocket()
  }

  retry(): void {
    if (this.stopped) return
    this.attemptCount = 0
    useWsStore.getState().setStatus("reconnecting", 1)
    this.connect()
  }

  private async openSocket(): Promise<void> {
    const guestToken = sessionStorage.getItem("guest_access_token")
    if (!guestToken) {
      useWsStore.getState().setStatus("failed")
      return
    }

    let ticket: string
    try {
      const res = await sessionsApi.wsTicket(this.sessionId, guestToken, this.lastSequence)
      ticket = res.ticket
    } catch (err) {
      // A revoked credential never becomes valid again — closing the session
      // rotates it. Retrying would burn all MAX_ATTEMPTS and land on "failed"
      // when the truth is simply that the session is over.
      if (isRevokedCredentialError(err)) {
        this.endSession()
        return
      }
      // ws-ticket fails (409) when the session has idled into
      // awaiting_reactivation. Don't dead-end on "Connection lost": route through
      // the existing reconnect() path, whose snapshot call reactivates the session
      // server-side and then retries the connection. Bounded by MAX_ATTEMPTS (F-1).
      if (!this.stopped) this.scheduleReconnect()
      return
    }

    if (this.stopped) return
    this.ws = new WebSocket(`${env.wsUrl}/ws?ticket=${encodeURIComponent(ticket)}`)

    this.ws.onopen = () => {
      this.attemptCount = 0
      useWsStore.getState().setStatus("connected")
      this.startPing()
    }

    this.ws.onmessage = (e: MessageEvent) => {
      try {
        const envelope = JSON.parse(e.data as string) as WSEnvelope
        if (envelope.event === "PONG") return
        this.handleEnvelope(envelope)
      } catch {}
    }

    this.ws.onclose = () => {
      this.stopPing()
      if (!this.stopped) {
        this.scheduleReconnect()
      }
    }

    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  private startPing(): void {
    this.pingTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ event: "PING", session_id: this.sessionId }))
      }
    }, PING_INTERVAL_MS)
  }

  private stopPing(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }

  private scheduleReconnect(): void {
    if (this.attemptCount >= MAX_ATTEMPTS) {
      useWsStore.getState().setStatus("failed")
      return
    }

    const delay = BACKOFF_STEPS_MS[Math.min(this.attemptCount, BACKOFF_STEPS_MS.length - 1)]
    this.attemptCount++
    useWsStore.getState().setStatus("reconnecting", this.attemptCount)

    setTimeout(() => this.reconnect(), delay)
  }

  private async reconnect(): Promise<void> {
    if (this.stopped) return

    try {
      const guestToken = sessionStorage.getItem("guest_access_token") ?? undefined
      const snapshot = await sessionsApi.snapshot(this.sessionId, guestToken, this.lastSequence)
      const status = snapshot.session.status

      if (TERMINAL_STATUSES.includes(status)) {
        // Session is in a terminal state — stop and surface to UI
        this.endSession()
        return
      }

      if (status === "awaiting_reactivation") {
        // Session is paused but resumable — update UI and keep retrying
        useSessionStore.getState().markPaused()
        this.scheduleReconnect()
        return
      }

      if (!SOCKET_ELIGIBLE_STATUSES.includes(status)) {
        // Not terminal, not resumable, and not a status the server will hand a
        // ticket for. Reconnecting cannot change that, so stop instead of
        // grinding through MAX_ATTEMPTS and reporting a connection failure the
        // guest can do nothing about. The Retry button remains available.
        useWsStore.getState().setStatus("failed")
        return
      }

      // Session is active (or settling). When the snapshot is authoritative there is a gap
      // (or no incremental basis), so missed events can't be applied
      // contiguously — take the snapshot as the whole truth and skip replay.
      // Otherwise replay the missed events, then reconcile.
      if (snapshot.snapshot_authoritative) {
        this.lastSequence = (snapshot.missed_events ?? []).reduce(
          (max, e) => Math.max(max, e.sequence ?? 0),
          this.lastSequence
        )
        reconcileSnapshot(snapshot)
      } else {
        for (const event of snapshot.missed_events ?? []) {
          this.handleEnvelope(event)
        }
        reconcileSnapshot(snapshot)
      }
    } catch {
      // Snapshot fetch failed; retry with backoff
      this.scheduleReconnect()
      return
    }

    this.connect()
  }

  disconnect(): void {
    this.stopped = true
    this.stopPing()
    this.ws?.close()
    useWsStore.getState().setStatus("disconnected")
  }

  /**
   * Stop reconnecting for good and tell the UI the session is over. Both callers
   * are unrecoverable — a terminal session status, and a revoked credential —
   * so neither may fall back into the retry loop.
   */
  private endSession(): void {
    this.stopped = true
    this.stopPing()
    this.ws?.close()
    useWsStore.getState().setStatus("disconnected")
    this.handlers["SESSION_CLOSED"]?.(null, {
      event: "SESSION_CLOSED",
      session_id: this.sessionId,
      payload: null,
      timestamp: new Date().toISOString(),
    })
  }

  private handleEnvelope(envelope: WSEnvelope): void {
    if (typeof envelope.sequence === "number" && envelope.sequence > this.lastSequence) {
      this.lastSequence = envelope.sequence
    }
    const handler = this.handlers[envelope.event]
    if (handler) {
      handler(envelope.payload, envelope)
    }
  }
}
