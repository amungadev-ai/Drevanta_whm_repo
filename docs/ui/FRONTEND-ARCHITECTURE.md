# Frontend Architecture & Deployment Plan — Directive §5 + §8

Framework: pending DEC-020 (recommendation: SvelteKit + TypeScript — see FRAMEWORK-EVALUATION.md).
This plan is written for that recommendation; the Next.js variant differs only in build output and
server entry (noted inline). **No migration implementation begins before owner approval.**

## 1. Repository and application architecture

```
frontend/                 # new SvelteKit app (Svelte 5 + TypeScript, Vite)
  src/routes/             # 31 routes, 1:1 with approved pages (MIGRATION-MAP.md)
  src/lib/components/     # shared components extracted without visual change
  src/lib/api/            # typed Go API client (single boundary)
  src/lib/api/types.ts    # TS contract types mirroring internal/api Go types
  src/lib/stores/         # session, permissions, operations, notifications
  static/                 # prototype assets copied verbatim (icons/images)
  tests/                  # vitest unit + playwright e2e/fidelity
ui/                       # approved prototype — UNCHANGED reference artifact
```

- The Go backend is untouched (directive §7): the frontend is a client of `/api/v1`.
- Contract typing: `types.ts` is generated/curated from the Go request/response structs; a CI test
  renders a representative payload through Go's handler tests and validates it against the TS
  types (schema-drift gate). Any needed backend contract change is documented first (directive §7).

## 2. Sessions, auth, permissions (directive §5)

- Login → `POST /api/v1/identity/login`; MFA → `/identity/mfa`; the panel issues bearer tokens.
  The frontend stores the token **in memory only**; a HttpOnly session cookie (Go side, planned
  G40 cookie support with CSRF double-submit) is the durable channel for browser sessions —
  no token ever reaches localStorage.
- Route guards: server-side load functions call `GET /api/v1/me` + `GET /api/v1/workspaces`
  through the server proxy (cookie → Go), redirect unauthenticated traffic to `/login`,
  MFA-pending to `/auth/verify`. Navigation items render from the workspace role (authz role from
  `GET /api/v1/workspaces`) — permission-aware, per DEC-004 (UI never grants authority; it hides
  what the API would deny anyway).
- 401 handling: global interceptor → re-auth redirect; 403/404 render the approved error states.

## 3. Real backend data replacing mock state

- Every mock-data variable found in the inventory (`mock-data vars` column) is replaced by typed
  API calls in the same component slot; loading/empty/error/recovery states come from the
  approved prototype visuals.
- Mutations go through the intent pipeline: `POST /resources/{id}/intents` with
  `Idempotency-Key` (generated client-side, stored per-draft) → 202 `OperationAccepted` →
  `OperationTracker` polls `GET /operations/{id}` (1s) and upgrades to SSE
  (`GET /api/v1/operations/{id}/events` — Go-side endpoint planned with the G40 wave; poll-only
  until then). Terminal states render the approved success/failure visuals.
- Error taxonomy mapping: 422 → inline field errors; 409/412 → conflict/stale dialogs;
  503 → dependency-unavailable banner; all bodies safe-detail only (contract §API envelope).

## 4. Production build & deployment (directive §8)

- Build: `npm ci && npm run build` → `adapter-node` output (`frontend/build/`); SvelteKit server
  runs as `drevanta-web.service` (Node, unprivileged user `drevanta-web`), listening on
  `127.0.0.1:3100`. Next.js variant: `output: 'standalone'`, same service model.
- Integration: **nginx is the platform web server** (G30 wave): site roots for customer sites AND
  the panel vhost. The panel vhost reverse-proxies `/api/v1/*` → Go panel (`127.0.0.1:8080`) and
  `/*` → frontend server (`127.0.0.1:3100`). One coherent installation, one TLS certificate,
  no manual glue (directive §8).
- Installer: `infra/` gains `frontend` steps (node runtime pinned, build artifacts shipped or
  built on-target, systemd unit, vhost include via the helper's managed vhost path). Upgrade =
  new build artifact + digest + systemd restart + health check; rollback = previous artifact
  directory swap (same pattern as the Go binary drills).
- Resources: pinned Node LTS, `NODE_ENV=production`, memory cap in the unit (`MemoryMax=512M` as
  a policy default pending DEC-018 ratification); health endpoint monitored by the panel.
- SSR and API proxying keep cookies same-origin (no CORS exposure); `X-Frame-Options: DENY`,
  CSP (script-src 'self'), HSTS at nginx.

## 5. Rollout sequence

1. DEC-020 approved → scaffold `frontend/` with the 31-route tree and the **unchanged prototype
   CSS imported globally**; visual gates pass against the baseline with zero API wiring (pure
   fidelity proof of the shell).
2. Auth/session slice (G40 step 1) → real login/MFA/recovery.
3. Persona waves per MIGRATION-MAP.md sequencing, each closing only on G1–G9 gates.
4. Panel vhost + TLS wiring on the VPS drill target; deployment drill recorded in TEST-RESULTS.md.
5. `ui/` remains in the repo as the immutable reference; it is never served in production after
   cutover except as a read-only audit artifact.
