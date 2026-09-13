# Frontend

This package is a Next.js 15 application with React 19
(`frontend/package.json:16-29`). It contains guest table/session pages, staff
login and dashboards, and a separately authenticated platform console
(`frontend/app/(guest)/table/[token]/page.tsx:1-112`,
`frontend/app/(staff)/staff/login/page.tsx:18-51`,
`frontend/app/(platform)/platform/login/page.tsx:1-72`).

```bash
npm ci
npm run dev
npm run lint
npm run typecheck
npm run build
```

Those commands map to the checked-in package scripts
(`frontend/package.json:5-15`). The default development API and WebSocket origins
are `localhost:8080` (`frontend/config/env.ts:1-5`). Production builds require
explicit `NEXT_PUBLIC_API_URL` and `NEXT_PUBLIC_WS_URL`; the prebuild guard rejects
localhost and insecure schemes unless an explicit bypass is set
(`frontend/scripts/check-prod-env.mjs:37-43,55-95,100-112`). Set
`NEXT_PUBLIC_API_BASE` to the API origin as well, because it populates CSP
`connect-src`; the guard warns when its host differs or is absent
(`frontend/next.config.ts:15-30`, `frontend/scripts/check-prod-env.mjs:109-124`).

The Cloudflare build, preview, and deploy commands exist in `package.json`, but
their presence is not evidence that a deployment ran
(`frontend/package.json:7-12`). The current repository contains no retained
frontend deployment verification, so deployment remains **unverified**. See
[docs/OPERATIONS.md](../docs/OPERATIONS.md).

- Setup and checks: [docs/DEVELOPMENT.md](../docs/DEVELOPMENT.md)
- Realtime recovery: [reconciliation invariants](../docs/reference/realtime-reconciliation-invariants.md)
- Event shapes: [WebSocket events](../docs/reference/websocket-events.md)
- Product analytics integration: [product-analytics.md](docs/product-analytics.md)

Do not run a production build in the same checkout while `next dev` is serving;
both commands write the package's `.next` directory. The manual launcher avoids
that collision by using a copied second frontend and clearing each output before
starting development servers (`scripts/manual-testing-up.sh:87-109`).
