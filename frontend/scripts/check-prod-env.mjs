#!/usr/bin/env node
/**
 * Production build guard.
 *
 * Runs before `next build` (see package.json). Fails the build when the public
 * endpoint configuration would ship a broken or insecure production bundle —
 * most importantly localhost API/WS URLs, which is the #1 way a prod build goes
 * out pointing at a developer's machine.
 *
 * Bypass for local production-build testing:
 *   ALLOW_LOCALHOST_BUILD=true npm run build
 *   (or NEXT_PUBLIC_ENV=development)
 *
 * Allow plain http/ws (e.g. TLS terminated elsewhere and you accept the risk):
 *   ALLOW_INSECURE_URLS=true npm run build
 */

// `next build` loads .env.production itself, but this guard runs standalone
// under node — load the same file (already-set env vars win, matching Next's
// precedence) so the documented `cp .env.production.example .env.production &&
// npm run deploy:cf` flow works without exporting vars by hand.
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

try {
  const envFile = join(dirname(fileURLToPath(import.meta.url)), "..", ".env.production");
  for (const line of readFileSync(envFile, "utf8").split("\n")) {
    const m = line.match(/^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*?)\s*$/);
    if (!m || m[1] in process.env) continue;
    process.env[m[1]] = m[2].replace(/^(["'])(.*)\1$/, "$2");
  }
} catch {
  // no .env.production — vars must come from the environment, as before
}

const BYPASS =
  process.env.ALLOW_LOCALHOST_BUILD === "true" ||
  process.env.NEXT_PUBLIC_ENV === "development";

const ALLOW_INSECURE = process.env.ALLOW_INSECURE_URLS === "true";

const LOCAL_HOSTS = new Set(["localhost", "127.0.0.1", "0.0.0.0", "::1", "[::1]"]);

const errors = [];
const warnings = [];

/**
 * @param {string} name env var name
 * @param {string|undefined} value
 * @param {"http"|"ws"} kind expected scheme family
 * @param {boolean} required
 * @returns {URL|null}
 */
function checkUrl(name, value, kind, required) {
  if (!value) {
    if (required) {
      errors.push(
        `${name} is not set. In production it must be an explicit public URL ` +
          `(it otherwise defaults to localhost, which ships a broken build).`,
      );
    }
    return null;
  }

  let url;
  try {
    url = new URL(value);
  } catch {
    errors.push(`${name}="${value}" is not a valid URL.`);
    return null;
  }

  const host = url.hostname.toLowerCase();
  if (LOCAL_HOSTS.has(host)) {
    errors.push(
      `${name}="${value}" points at localhost. A production build must use a ` +
        `public hostname.`,
    );
  }

  const secureScheme = kind === "http" ? "https:" : "wss:";
  const plainScheme = kind === "http" ? "http:" : "ws:";
  const validSchemes = kind === "http" ? ["http:", "https:"] : ["ws:", "wss:"];

  if (!validSchemes.includes(url.protocol)) {
    errors.push(
      `${name}="${value}" has scheme "${url.protocol}"; expected ${validSchemes.join(" or ")}.`,
    );
  } else if (url.protocol === plainScheme && !ALLOW_INSECURE) {
    errors.push(
      `${name}="${value}" is insecure (${plainScheme}). Use ${secureScheme} in ` +
        `production, or set ALLOW_INSECURE_URLS=true if TLS terminates upstream.`,
    );
  }

  return url;
}

if (BYPASS) {
  console.log(
    "[check-prod-env] bypass active (ALLOW_LOCALHOST_BUILD/NEXT_PUBLIC_ENV=development) — skipping production endpoint checks.",
  );
  process.exit(0);
}

const apiUrl = checkUrl("NEXT_PUBLIC_API_URL", process.env.NEXT_PUBLIC_API_URL, "http", true);
// Validated for its side effect (errors[]); the parsed URL itself is not needed here.
checkUrl("NEXT_PUBLIC_WS_URL", process.env.NEXT_PUBLIC_WS_URL, "ws", true);
// NEXT_PUBLIC_API_BASE drives the CSP connect-src in next.config.ts. Optional,
// but if present it must be valid and consistent with the runtime API URL or
// the browser will block API/WS calls.
const apiBase = checkUrl("NEXT_PUBLIC_API_BASE", process.env.NEXT_PUBLIC_API_BASE, "http", false);

if (apiUrl && apiBase && apiUrl.host !== apiBase.host) {
  warnings.push(
    `NEXT_PUBLIC_API_URL host (${apiUrl.host}) differs from NEXT_PUBLIC_API_BASE host ` +
      `(${apiBase.host}); the CSP connect-src may block API/WS requests.`,
  );
}
if (apiUrl && !apiBase) {
  warnings.push(
    "NEXT_PUBLIC_API_BASE is not set; the CSP connect-src in next.config.ts will not " +
      "include your API host. Set it to the same origin as NEXT_PUBLIC_API_URL.",
  );
}
if (process.env.NEXT_PUBLIC_POSTHOG_KEY && !process.env.NEXT_PUBLIC_POSTHOG_HOST) {
  warnings.push(
    "NEXT_PUBLIC_POSTHOG_KEY is set without NEXT_PUBLIC_POSTHOG_HOST; the CSP connect-src will block analytics ingest.",
  );
}
if (
  process.env.NEXT_PUBLIC_POSTHOG_REPLAY === "1" &&
  process.env.NEXT_PUBLIC_POSTHOG_ENV === "production"
) {
  warnings.push(
    "Session replay is enabled for the production analytics environment; policy limits replay to beta builds.",
  );
}

for (const w of warnings) console.warn(`[check-prod-env] warning: ${w}`);

if (errors.length > 0) {
  console.error("\n[check-prod-env] production build blocked:\n");
  for (const e of errors) console.error(`  ✗ ${e}`);
  console.error(
    "\nFix the env vars above, or bypass for local testing with ALLOW_LOCALHOST_BUILD=true.\n",
  );
  process.exit(1);
}

console.log("[check-prod-env] production endpoint configuration OK.");
