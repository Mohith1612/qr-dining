// OpenNext adapter configuration for Cloudflare.
// Defaults are correct for this app (no ISR cache/queue bindings needed yet —
// all dynamic data comes from the backend API at request time from the client).
import { defineCloudflareConfig } from "@opennextjs/cloudflare";

export default defineCloudflareConfig();
