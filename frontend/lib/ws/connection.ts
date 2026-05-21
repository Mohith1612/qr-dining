"use client"

import { env } from "@/config/env"
import { sessionsApi } from "@/lib/api/sessions"
import { reconcileSnapshot } from "./reconciliation"
import { useWsStore } from "@/store/ws"
import type { WSEnvelope, WSEventHandlerMap } from "@/types/ws"

const BACKOFF_STEPS_MS = [1000, 2000, 4000, 8000, 16000, 30000]
const MAX_ATTEMPTS = 10
const PING_INTERVAL_MS = 30_000

export class WSConnection {
  private ws: WebSocket | null = null
  private pingTimer: ReturnType<typeof setInterval> | null = null
  private attemptCount = 0
  private stopped = false

  constructor(
    private readonly sessionId: string,
    private readonly participantId: number,
    private readonly handlers: WSEventHandlerMap
  ) {}

  connect(): void {
    if (this.stopped) return
    const url = `${env.wsUrl}/ws?session_id=${this.sessionId}&participant_id=${this.participantId}`
    this.ws = new WebSocket(url)

    this.ws.onopen = () => {
      this.attemptCount = 0
      useWsStore.getState().setStatus("connected")
      this.startPing()
    }

    this.ws.onmessage = (e: MessageEvent) => {
      try {
        const envelope = JSON.parse(e.data as string) as WSEnvelope
        if (envelope.event === "PONG") return
        const handler = this.handlers[envelope.event]
        if (handler) {
          handler(envelope.payload, envelope)
        }
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
      useWsStore.getState().setStatus("disconnected")
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
      const snapshot = await sessionsApi.snapshot(this.sessionId)

      if (snapshot.session.status !== "active") {
        // Session ended while disconnected — stop and let UI handle it
        useWsStore.getState().setStatus("disconnected")
        this.handlers["SESSION_CLOSED"]?.(null, {
          event: "SESSION_CLOSED",
          session_id: this.sessionId,
          payload: null,
          timestamp: new Date().toISOString(),
        })
        return
      }

      reconcileSnapshot(snapshot)
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
}
