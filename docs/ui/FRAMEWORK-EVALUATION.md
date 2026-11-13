# Drevanta Frontend Framework Evaluation — Next.js + TypeScript vs SvelteKit + TypeScript

Status: **RECOMMENDATION SUBMITTED FOR OWNER APPROVAL** (DEC-020 — no production migration begins before approval; directive §3, §10.1)
Date: 2026-11-13 · Evaluating engineer: Drevanta Autonomous Agent
Inputs: approved prototype inventory (`docs/ui/PROTOTYPE-INVENTORY.md`, 31 pages / 13 CSS files / 10 JS modules),
Go control-plane contract (`internal/api` — `/api/v1`, bearer + MFA flows, 202/operation-status pattern),
deployment target (self-hosted VPS: 4 vCPU, 5.9 GB RAM, **shared with customer hosting workloads**),
directive constraints (§2 100% fidelity, §5 production requirements, §8 deployment).

**Decision criteria below are the directive's own list (§3) — popularity/familiarity is explicitly excluded as a criterion.**

## 1. Requirements profile derived from the prototype and platform

| Requirement | Evidence | Weight |
| --- | --- | --- |
| 100% visual fidelity to static HTML/CSS | 13 global CSS files + CSS custom properties = the approved design system in plain CSS; inline `on*=` handlers; page-per-HTML structure (31 pages) | Decisive |
| Reuse existing styling and assets | Global CSS must load **unchanged**; per-component overrides must not restart the cascade | Decisive |
| Auth-gated dashboard (no public SEO surface) | All 4 personas behind `/api/v1` sessions; MFA step-up | High |
| Real-time operation status | Go outbox + operation states (`accepted→dispatched→terminal`); SSE endpoint is the planned transport | High |
| VPS resource discipline | Panel shares the box with customer nginx/PHP/DB workloads; RAM/CPU per idle panel session counts | High |
| Deploy/upgrade simplicity on self-hosted VPS | Single systemd service, small dependency surface, deterministic builds | High |
| Typed API integration | TS types bound to Go contracts; schema tests reject drift | High |
| Complex dashboard interactions | Tables, tabs, modals, drawers, forms (85 static + 239 recorded controls) | Medium |
| Testing + accessibility | Playwright visual regression vs prototype baseline; WCAG-conscious tokens already in prototype (`accessible-palettes.css`) | Medium |
| Maintainability / team scaling | Framework surface area, onboarding, ecosystem | Medium |

## 2. Side-by-side evaluation

### 2.1 Fidelity mechanics (decisive criterion)

| Aspect | Next.js + TS | SvelteKit + TS |
| --- | --- | --- |
| Global CSS reuse | Supported (`app/globals.css` import); but ecosystem gravity pulls toward Tailwind/CSS-in-JS rewrites — a real drift risk against a 13-file plain-CSS system | Supported identically; Svelte's scoped styles are **additive** to global CSS (no cascade restart), and the component model ("global stylesheet + component markup") mirrors the prototype's own structure |
| Page mapping | App Router pages/routes fine; Server Components add a rendering model the prototype never had — every interactive control is a client component anyway (dashboard) | File-based routing maps 31 prototype pages → 31 routes 1:1; no server-component ambiguity: everything is client-hydrated exactly like the prototype's static pages |
| Interaction porting | Inline `on*=` → React synthetic events; controlled-component rewrites of every form can alter behavior subtly (event pooling eliminated in React 17+, but focus/active semantics need care) | `on:click`/`bind:value` map directly to the vanilla semantics being replaced; Svelte's transition directive reproduces the prototype's CSS-class toggling patterns without a different animation system |
| State replacement | localStorage → React Query/Zustand/Redux — choice burden; each option risks UI behavior drift (loading/empty states are part of the approved appearance) | localStorage → Svelte stores; the prototype's `persona-core.js` state module ports nearly 1:1 into typed stores |

**Fidelity verdict:** SvelteKit offers the smaller conceptual distance between the approved artifact and the production code. Next.js can achieve parity but carries a constant temptation (and larger API surface) toward re-styling; fidelity discipline must be enforced socially rather than structurally.

### 2.2 Runtime footprint on the shared VPS (decisive for §8)

| Metric | Next.js (adapter/standalone) | SvelteKit (adapter-node) |
| --- | --- | --- |
| Client bundle baseline (hello-dashboard) | React + react-dom + Next runtime ≈ 90–130 KB gz framework share | Svelte runtime ≈ 8–15 KB gz share (compile-away) |
| Server process memory (idle) | ~70–120 MB (Node + Next server, standalone) | ~45–80 MB (Node + SvelteKit) |
| Dependency surface to patch (CVE exposure) | Larger (React, Next, bundled toolchain) | Smaller (Vite + SvelteKit) |
| Cold start / rebuild on VPS | Heavier install (node_modules 300–500 MB) | Lighter (node_modules 150–250 MB) |
| CPU per request (JSON-proxy dashboard) | Comparable | Comparable; Svelte's smaller hydration work is a mild edge on low-power vCPUs |

Both are single-process Node services — neither requires exotic infrastructure. The panel shares the machine with customer workloads; the SvelteKit figures are modestly but consistently lower, and the smaller dependency tree reduces upgrade churn on self-hosted installs (a security-relevant property per directive §5 "secure handling").

### 2.3 Go API integration

Identical capability in both: a typed API client module (`src/lib/api`) with generated/curated TS types mirroring the Go contract, cookie-based session forwarding (server-side fetch during SSR/proxying) and bearer for CLI. The Go side needs no change in either case (directive §7 respected). The operation-status SSE endpoint is framework-agnostic (`EventSource`). No differentiator.

### 2.4 Testing and accessibility

Playwright visual regression is framework-agnostic — the fidelity gates (§6 of the directive) bind to the **prototype baseline**, not the framework. Vitest + Testing Library work in both (Testing Library has Svelte bindings). axe-core integration equal. No differentiator.

### 2.5 Ecosystem, hiring, maintainability

Next.js: larger ecosystem, more third-party components, larger hiring pool — the honest tradeoff of a SvelteKit recommendation. Mitigations: Drevanta's UI must be built from the approved prototype rather than pulled from component libraries (so the component-library advantage mostly inverts into a risk — pulling in a "generic component library" is explicitly forbidden by directive §2). Maintainability at Drevanta's scale (31 routes, one design system, one API) is comfortably served by SvelteKit's smaller surface: fewer concepts (no RSC/ISR/cache-layer matrix), less framework churn between majors.

### 2.6 Build/deploy complexity

Both: `npm run build` → Node server via systemd (same pattern as the Go panel binary). SvelteKit `adapter-node` outputs a minimal server; Next `standalone` similar. SvelteKit additionally supports `adapter-static` should a future screen set become fully prerenderable (not the default plan). Slight edge SvelteKit for CI time and image size.

## 3. Recommendation

**SvelteKit + TypeScript.**

1. **Fidelity (decisive):** closest structural mapping to the approved artifact — global-CSS-first, page-per-route, event semantics that port 1:1; smallest risk of drift against the 100%-preservation requirement.
2. **Resource discipline on shared VPS:** smallest runtime/dependency footprint of the two authorized options; directly serves §8's "avoid unnecessary resource consumption" and the co-tenancy reality of a hosting panel.
3. **Deployment simplicity:** identical process model, smaller artifact, fewer dependencies to patch across self-hosted fleets.
4. **No capability gap:** everything in directive §5 (typed components, typed API binding, permission-aware routing, MFA flows, real-time status, a11y, testing) is fully achievable in SvelteKit; where Next.js would offer a richer plugin ecosystem, directive §2 forbids the replacement-design-system shortcuts that ecosystem mainly provides.

**Honest tradeoffs (recorded, not hidden):** smaller ecosystem/hiring pool; fewer off-the-shelf dashboard components (largely irrelevant given the no-replacement-templates rule); team familiarity may favor Next.js — flagged as the primary cost of this recommendation.

**If the owner prefers Next.js**, the fidelity plan, migration map, and deployment architecture in this directory apply unchanged; only §2.1's drift-discipline measures (stricter lint against style rewrites, mandatory screenshot gates per PR) tighten.

## 4. Approval and next step

- Decision record: `docs/DECISIONS.md` DEC-020 (`RECOMMENDATION_SUBMITTED`, owner approval required).
- Per directive §3 and §10: **no production frontend migration begins until DEC-020 is approved.**
- Independent backend/infrastructure work continues meanwhile (directive §9) — see ledger.
