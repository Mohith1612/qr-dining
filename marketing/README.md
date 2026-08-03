# QR Dining marketing site

Standalone Astro + Tailwind site for `qrdining.app`. It builds to static files for Cloudflare Pages; the only dynamic route is `functions/api/demo.ts`.

## Local development

```sh
npm install
npm run dev
npm run build
```

`npm run build` type-checks both Astro and the Cloudflare Pages Function before producing `dist/`.

## Cloudflare Pages

- Build command: `npm run build`
- Build output directory: `dist`
- Root directory: `marketing`
- Set `PUBLIC_APP_URL` at build time.
- Add secrets `RESEND_API_KEY`, `DEMO_TO_EMAIL`, and `DEMO_FROM_EMAIL`.
- Create a KV namespace and replace the placeholder ID in `wrangler.jsonc` before deployment. The `RATE_LIMIT` binding limits each IP to five demo requests per hour.
- Verify the sending domain used by `DEMO_FROM_EMAIL` with Resend.

The form honeypot silently accepts bot submissions. Valid requests are checked again at the edge, rate-limited, HTML-escaped, and delivered by Resend.

## Content that needs founder sign-off

- Plan prices are centralized in `src/data/site.ts`. ₹1,499 and ₹2,999 follow the INR values used in backend billing tests; confirm them before launch because the main seed file still contains older placeholder values.
- Privacy and terms contain clearly marked operational drafts. Counsel must review them and add the final company entity and governing terms.
- Product visuals are faithful DOM vignettes because the repository has no clean marketing screenshots. Replace them with exports from the Saffron demo tenant when those are available.
- Replace the KV placeholder and confirm `PUBLIC_APP_URL`.
