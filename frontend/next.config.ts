import type { NextConfig } from "next";

// Production CSP for the QR Dining PWA. Notes:
// - `script-src 'self' 'unsafe-inline'`: Next.js inlines a small bootstrap
//   script for hydration; the long-term fix is to migrate to per-request
//   nonces (Phase C). Today the inline script is shipped from our own
//   bundle, so the practical risk is bounded to a code-injection attack on
//   our own build pipeline.
// - `connect-src` allows the API host (NEXT_PUBLIC_API_BASE) plus the WS
//   origin (wss://...). For dev, plain ws:// and localhost are added.
// - `img-src` allows any https host: menu items and branding accept operator
//   supplied image URLs (R2-hosted uploads or pasted stock links), so images
//   may come from arbitrary https origins. Images-only; everything else stays
//   locked to 'self'.
const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";
const R2_PUBLIC_BASE = process.env.NEXT_PUBLIC_R2_PUBLIC_BASE ?? "";
const POSTHOG_HOST = process.env.NEXT_PUBLIC_POSTHOG_HOST ?? "";
const POSTHOG_ASSET_HOST = process.env.NEXT_PUBLIC_POSTHOG_ASSET_HOST ?? "";
const POSTHOG_REPLAY = process.env.NEXT_PUBLIC_POSTHOG_REPLAY === "1";

const connectSources = ["'self'"];
const imgSources = ["'self'", "data:", "blob:", "https:"];
const scriptSources = ["'self'"];
if (API_BASE) {
  connectSources.push(API_BASE);
  // The API host also accepts WebSocket upgrades; allow ws/wss to the same host.
  const url = new URL(API_BASE);
  connectSources.push(`wss://${url.host}`);
  connectSources.push(`ws://${url.host}`);
}
if (R2_PUBLIC_BASE) {
  imgSources.push(R2_PUBLIC_BASE);
}
if (POSTHOG_HOST) {
  connectSources.push(POSTHOG_HOST);
}
if (POSTHOG_REPLAY && POSTHOG_ASSET_HOST) {
  scriptSources.push(POSTHOG_ASSET_HOST);
}

const isDev = process.env.NODE_ENV !== "production";
if (isDev) {
  connectSources.push("ws://localhost:*", "http://localhost:*");
}

const csp = [
  "default-src 'self'",
  // Inline script directive is required by Next.js hydration today. Tracked
  // for Phase C nonce migration.
  `script-src ${scriptSources.join(" ")} ${isDev ? "'unsafe-eval'" : ""} 'unsafe-inline'`.trim(),
  "style-src 'self' 'unsafe-inline'",
  `img-src ${imgSources.join(" ")}`,
  "font-src 'self' data:",
  `connect-src ${connectSources.join(" ")}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
  "form-action 'self'",
  "object-src 'none'",
  ...(POSTHOG_REPLAY ? ["worker-src 'self' blob:"] : []),
  "upgrade-insecure-requests",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: csp },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "geolocation=(), microphone=(), camera=(), payment=()" },
  { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
];

if (!isDev) {
  securityHeaders.push({
    key: "Strict-Transport-Security",
    value: "max-age=63072000; includeSubDomains; preload",
  });
}

const nextConfig: NextConfig = {
  eslint: {
    // Pre-existing lint issues are tracked separately; don't block builds.
    ignoreDuringBuilds: true,
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: securityHeaders,
      },
    ];
  },
};

export default nextConfig;
