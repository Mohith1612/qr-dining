// Guest credential persistence.
//
// The app reads guest credentials from sessionStorage (per-tab). That is lost
// when the tab closes, which used to strand a returning guest on the landing
// page when they reopened a /session/<id> link. We additionally mirror the
// credentials to localStorage keyed by session id so a rejoin can recover them.
// The token still expires server-side, so a stale copy simply fails auth.

const LS_PREFIX = "guest_creds:"

interface GuestCreds {
  participantId: number
  token: string
}

/** Persist credentials to sessionStorage (current tab) and localStorage (rejoin). */
export function persistGuestCreds(sessionId: string, participantId: number, token: string) {
  // sessionStorage is the per-tab identity and is always the fresh one.
  sessionStorage.setItem("session_id", sessionId)
  sessionStorage.setItem("participant_id", String(participantId))
  sessionStorage.setItem("guest_access_token", token)
  try {
    // localStorage is the cross-tab rejoin slot, keyed only by session id. When
    // two participants share one browser (host tab + guest tab) it would collide,
    // so the FIRST participant to claim the session keeps the slot; a later,
    // different participant does not clobber it. The same participant may refresh
    // its token. This makes reopening a /session/<id> link restore the original
    // (host) identity rather than the most recent joiner.
    const raw = localStorage.getItem(LS_PREFIX + sessionId)
    let existing: Partial<GuestCreds> | null = null
    if (raw) { try { existing = JSON.parse(raw) as Partial<GuestCreds> } catch { existing = null } }
    if (!existing || existing.participantId === participantId) {
      localStorage.setItem(LS_PREFIX + sessionId, JSON.stringify({ participantId, token }))
    }
  } catch {
    /* localStorage unavailable (private mode quota) — sessionStorage still works this tab */
  }
}

/**
 * Return the credentials for a session, preferring the current tab's
 * sessionStorage and falling back to the localStorage copy. On a localStorage
 * hit it rehydrates sessionStorage so the rest of the app (WS, API client) works
 * unchanged. Returns null when nothing is found — the caller should redirect.
 */
export function recoverGuestCreds(sessionId: string): GuestCreds | null {
  const ss = sessionStorage.getItem("session_id")
  const sp = sessionStorage.getItem("participant_id")
  const st = sessionStorage.getItem("guest_access_token")
  if (ss === sessionId && sp && st) {
    return { participantId: Number(sp), token: st }
  }
  try {
    const raw = localStorage.getItem(LS_PREFIX + sessionId)
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<GuestCreds>
      if (parsed && typeof parsed.participantId === "number" && parsed.token) {
        sessionStorage.setItem("session_id", sessionId)
        sessionStorage.setItem("participant_id", String(parsed.participantId))
        sessionStorage.setItem("guest_access_token", parsed.token)
        return { participantId: parsed.participantId, token: parsed.token }
      }
    }
  } catch {
    /* malformed/unavailable — fall through to null */
  }
  return null
}

/** Forget the persisted rejoin copy (e.g. once the session has ended). */
export function clearGuestCreds(sessionId: string) {
  try {
    localStorage.removeItem(LS_PREFIX + sessionId)
  } catch {
    /* ignore */
  }
}
