import posthog from "posthog-js"
import { env } from "@/config/env"
import { scrubEvent } from "@/lib/product-analytics/scrub"

let initialized = false

export function isEnabled(): boolean {
  return typeof window !== "undefined" && Boolean(env.posthogKey && env.posthogHost)
}

export function ensureInit() {
  if (!isEnabled()) return null
  if (!initialized) {
    posthog.init(env.posthogKey!, {
      api_host: env.posthogHost!,
      person_profiles: "identified_only",
      autocapture: false,
      capture_pageview: false,
      capture_pageleave: true,
      disable_session_recording: !env.posthogReplay,
      disable_surveys: true,
      advanced_disable_feature_flags: true,
      persistence: "localStorage",
      respect_dnt: true,
      before_send: scrubEvent,
      session_recording: {
        maskAllInputs: true,
        maskTextSelector: "[data-ph-mask]",
        blockSelector: ".ph-no-capture",
        captureCanvas: { recordCanvas: false },
      },
    })
    posthog.register({
      env: env.posthogEnv,
      ...(env.appVersion ? { app_version: env.appVersion } : {}),
    })
    initialized = true
  }
  return posthog
}
