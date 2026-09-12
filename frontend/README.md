# frontend

The QR Dining web app: Next.js (App Router) + TypeScript, deployed as an OpenNext Cloudflare Worker.

Three route groups under `app/` — `(guest)`, `(staff)`, `(platform)` — plus a public `pricing` page and a tenant-resolving `middleware.ts`.

```bash
npm install
npm run dev          # http://localhost:3000
npm run lint
npm run typecheck
npm run build
```

Point it at a backend with `NEXT_PUBLIC_API_BASE`. It must equal the API origin **exactly**, or the CSP blocks API and WebSocket calls.

A `prebuild` guard refuses production builds that target localhost or plain HTTP. Set `ALLOW_LOCALHOST_BUILD=true` for a local production build — this is what CI does.

Do not run `build:cf` or `deploy:cf` while `next dev` is serving this checkout: the production build rewrites `.next/` underneath the dev server and corrupts it.

- Setup, commands and conventions: [../docs/DEVELOPMENT.md](../docs/DEVELOPMENT.md)
- Cloudflare deployment: [../docs/DEPLOYMENT.md §7](../docs/DEPLOYMENT.md#7-frontend-cloudflare)
- Product analytics: [docs/product-analytics.md](docs/product-analytics.md)
- Realtime client contract: [../docs/reference/reconnect-guide.md](../docs/reference/reconnect-guide.md) and [../docs/reference/websocket-events.md](../docs/reference/websocket-events.md)
