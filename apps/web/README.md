# TestForge web

Next.js 16 (App Router, Cache Components) + Tailwind CSS 4 + shadcn/ui (Radix, `nova` preset).

## Commands

```bash
pnpm dev          # http://localhost:3000, proxies /api and /healthz to $API_URL
pnpm build        # production build
pnpm lint         # ESLint
pnpm typecheck    # next typegen + tsc --noEmit
pnpm test         # Vitest unit tests
pnpm gen:api      # regenerate src/lib/api/schema.d.ts from backend/api/openapi.yaml
```

`API_URL` (default `http://localhost:8080`) is the Go API. The browser always calls
same-origin `/api/...`, so the HttpOnly `tf_session` cookie stays first-party; the
API's configured app origin must be this app's origin (cookie-auth writes send `Origin`).

## Layout

```text
src/app/               routes only (thin pages)
src/proxy.ts           redirects to /login when there is no session cookie
src/features/<name>/   components, api (TanStack Query hooks), lib per feature
src/components/        shared components; ui/ is shadcn
src/lib/api/           generated schema, browser client, server client (forwards cookies)
```
