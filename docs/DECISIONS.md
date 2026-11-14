# Decision Register

Authoritative source: `spec/approval-v4/decision-dossier.md` and
`spec/registers/decision-register.csv` (19 decisions, all with `human_choice` open).

Per handoff rule, an agent cannot convert a recommended default into owner approval. Accordingly
every decision below is dispositioned as **`ADOPTED_RECOMMENDED_DEFAULT_FOR_DEVELOPMENT`** — the
documented recommended candidate governs development work; each remains **pending owner
ratification** before release certification. Items that materially affect product scope are
additionally recorded in `docs/BLOCKERS.md` as release-gate blockers.

| ID | Topic | Development disposition | Affects scope? | Owner ratification |
| --- | --- | --- | --- | --- |
| DEC-001 | Backend language & service architecture | Modular single unprivileged Go process + separate privileged helper; per REC-02 source candidate (Go/SQLite/static UI) | No (implementation strategy) | Pending |
| DEC-002 | Database engine & migrations | SQLite WAL, forward-only migrations, restore drill before release | No | Pending |
| DEC-003 | VPS privileged execution protocol | Local Unix socket helper, root-owned typed allowlist, peer/host/target proof, fences, journals | No | Pending |
| DEC-004 | Tenant ownership & authorization | Explicit per-workspace membership, ownership edges, default deny everywhere; no wildcard grants | No | Pending |
| DEC-005 | Supported distributions | Single verified tuple: Debian/Ubuntu apt+systemd (dev target Ubuntu 26.04); preflight fails closed on unsupported | No | Pending |
| DEC-006 | Web server & runtime matrix | Nginx-first vhost adapter; static hosting milestone; per-site principals | No | Pending |
| DEC-007 | Mail transport | Separate SMTP recovery transport; native mailbox hosting behind feature boundary | **Yes (MAIL-02 breadth)** | Pending |
| DEC-008 | Recovery through SSH/console | Installation-local single-use recovery grant preserving MFA | No | Pending |
| DEC-009 | Jobs, idempotency, reconciliation | Durable transactional jobs + outbox; leases; monotonic fences; observation before replay | No | Pending |
| DEC-010 | Backup encryption & restore | Encrypted independently restorable backups; off-host copy; staged restore + cutover gates | No | Pending |
| DEC-011 | Secrets & credential storage | Secret references in data; local scoped broker (envelope encryption) with separate key custody | No | Pending |
| DEC-012 | API versioning & errors | `/api/v1` canonical namespace; historic 178 paths registered as candidates, not published; strict schema | No | Pending |
| DEC-013 | Production readiness & support boundaries | Exact tuples, immutable promotion, verified rollback, named ownership before production | No | Pending |
| DEC-014 | Domain DNS/TLS verification | Domain control proof + DNS target observation + independent HTTPS probe before activation | No | Pending |
| DEC-015 | Scope & billing module status | Develop BIZ-01..03 (customers/plans/orders/invoices/gateways/tickets) as native modules per full-platform direction; **payment provider credentials and tax rules need owner input** | **Yes (commercial breadth)** | **Required before release** |
| DEC-016 | Capability commitment | Full-platform directed; all 404 scope rows preserved; duplicates/optional/conflicting items dispositioned explicitly, none silently discarded | **Yes** | Required at scope sign-off |
| DEC-017 | Atomic source and runtime UI reconciliation | Static prototype exercised in isolation during development; render-state evidence collected against running platform | No | Pending |
| DEC-018 | Measurable policy targets | Fixture values used as **test** values only; no operating-limit claims without measurement | No | Pending |
| DEC-020 | Production frontend framework | Directive 2026-11-13: static prototype is the approved UI/UX (100% fidelity, non-negotiable); production frontend must be Next.js+TS or SvelteKit+TS. Evaluation complete (docs/ui/FRAMEWORK-EVALUATION.md): **SvelteKit + TypeScript recommended** (fidelity mechanics: global-CSS-first + page-per-route mapping; smallest footprint on shared VPS; simplest deploy; no capability gap). Honest tradeoffs recorded (ecosystem/hiring). **OWNER APPROVED 2026-11-13 (SvelteKit + TypeScript)** — G40 authorized; Stage-1 migration implemented and verified (docs/ui/MIGRATION-LEDGER.md) | **Yes (frontend architecture)** | Executed 2026-11-13 |
| DEC-022 | Production deployment shape (frontend) | adapter-node build served by `drevanta-web.service` (127.0.0.1:3100) behind the platform nginx vhost which proxies /api/v1 to the Go panel — one coherent installation, artifact-swap upgrade/rollback | **Yes (deployment)** | Recorded 2026-11-13 (docs/ui/FRONTEND-ARCHITECTURE.md §4) |
| DEC-021 | UI fidelity gates | Directive §6: 62-screenshot baseline captured (31 pages × 2 viewports, 0 failures, Playwright chromium — sandbox browser available, BLK-003 gap closed for baseline); gates G1–G9 with measurable tolerances defined (docs/ui/FIDELITY-PLAN.md); a page closes only when gates actually pass and all its registered controls are accounted for | No (verification process) | Recorded 2026-11-13 (owner directive) |
| DEC-019 | Autonomous agent authority | Granted for this assignment in writing by owner: repo `amungadev-ai/Drevanta_whm_repo`, dev branches, tests, CI, PR prep, isolated test environments. NOT granted: protected merges, production infra, production data, destructive host ops, production deploy. Credentials are owner-issued, stored sandbox-only, to be rotated/revoked at completion. | **Yes** | Recorded 2026-11-13 (owner assignment message); revocation at project completion |

| DEC-016-RULING | 2026-11-14 | Owner | BLK-005/008, scope register | The 13 AMBIGUOUS-OWNER capabilities are APPROVED for Drevanta v1.0: ADM-05 (multi-node inventory/placement), WEB-07 (scoped shell/browser terminal), DB-03 (Redis/Valkey), APP-08 (OCI containers/images), MAIL-01 (mail server/mailboxes/aliases/forwarding), MAIL-02 (mail DNS/deliverability/webmail), DNS-01 (authoritative DNS zones), BIZ-02 (invoices/taxes/credits/refunds), BIZ-03 (payment gateways/webhooks), SUP-02 (KB/notifications), SEC-01 (security scans/WAF/IR), MKT-02 (specialized app categories), INT-01 (API keys/SSO/webhooks). Optional INFRASTRUCTURE ACTIVATION is optional; implementation is NOT optional. An approved scope ruling is not implementation/test completion | full implementation wave ordered by roadmap | ACCEPTED (owner, this directive) |
| DEC-021-RULING | 2026-11-14 | Owner | Mail UI placement | Mail card APPROVED inside Website Owner -> Domain Detail -> below DNS/TLS, reusing approved components only; full mail management scope (domains, mailboxes, aliases/forwarding, password mgmt incl. secure generation, quotas, usage/delivery status, DNS/DKIM/SPF/DMARC, diagnostics, client settings, webmail via maintained integration, logs/audit); plaintext passwords never persisted in payloads/db/logs/audit/evidence; scope-pinned grants | implement per mail wave | ACCEPTED (owner, this directive) |
| DEC-022 | 2026-11-14 | Owner | Commercial architecture | Platform is a REUSABLE self-hosted platform: tax/SLA/operational policies configurable at platform/organization/provider/plan scope; development defaults must be labeled non-production; production config never silently falls back to fixtures; config changes require validation + audit | commercial configuration wave | ACCEPTED (owner, this directive) |
