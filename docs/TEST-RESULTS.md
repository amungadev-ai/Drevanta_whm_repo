# Test-Results Register

Every result binds to the exact commit (short SHA + build artifact digest when applicable).
Gate values: PASS / FAIL / BLOCKED / NOT_RUN / reviewed N/A. **No stage receives passing credit
for an unexecuted case.** Baseline truth: 744 parametrized cases in
`spec/specification/contracts/test-specifications.jsonl` and 761 in
`spec/contracts/verification-specifications.jsonl` — all NOT_RUN at baseline (per handoff VALIDATION.md).

## Summary

| Date (UTC) | Commit | Suite | Result | Notes |
| --- | --- | --- | --- | --- |
| 2026-11-13 | baseline | handoff archive manifest verification | PASS | 107/107 files hash-verified (scripts/verify_handoff.py) |
| 2026-11-13 | baseline | prototype provenance digest (71 files) | PASS | docs/prototype-manifest.sha256 matches inherited zip |
| 2026-11-13 | baseline | all 744+761 parametrized cases | NOT_RUN | inherited from handoff VALIDATION.md |

## Automated suites (added as code lands)

| Suite | Layer | First commit | Latest result | Commit |
| --- | --- | --- | --- | --- |
| G10 contract suite (11 tests: unknown-field 422, stale-generation 412, hidden-resource 404, stable replay, altered replay 409, revoked replay 403, 401 taxonomy, me/workspaces, operation visibility/concealment, audit allow+deny, MFA login lifecycle) | API integration (httptest, isolated per-test SQLite) | f20b08a | PASS (11/11) | f20b08a |
| G21 identity/permission enforcement suite (7 tests: password-reset lifecycle w/ single-use token + epoch revocation + MFA preservation (TC-P17-01/02 analog), recovery-code single-use, logout, membership grant/list/revoke + dispatch-recheck cancel of revoked actor's queued operation (TC-P18-01/02 analog), last-owner protection 409, role validation 422, support-lease exact-action semantics) | API integration (httptest, isolated per-test SQLite) | 284ae24 | PASS (7/7) | 284ae24 |
| G22 durability suite (4 tests: worker success with observed generation + outbox terminal event; crash reconciliation via expired lease re-dispatch with fence increment and journaled observation; failed/uncertain outcomes with unsafe-retry blocking; revoked-actor cancellation through the worker) | worker integration (isolated SQLite) | this commit | PASS (4/4) | see git log |
| G22 API surface suite (2 tests: scope-pinned API key issue/use/concealment/revocation; interval schedule arm/fire/coalesce with owner-bound operations) | API integration (httptest, isolated per-test SQLite) | bc84f7f | PASS (2/2) | bc84f7f |
| G20 helper protocol suite (7 tests: profile host fixtures 7–10 — wrong host rejected before effects, stale fence rejected before finalization, lost-reply replay returns journaled observation with one effect, changed digest rejected with journal retained; unknown verb/expiry/version rejection; typed-argument discipline; preflight fail-closed on unsupported OS) | helper protocol (in-process) | this commit | PASS (7/7) | see git log |
| G20 installer + wire suite (3 tests: claim lifecycle — bootstrap token, uniform wrong-token denial, atomic consume, replay denial, admin sign-in, checkpoint advance, resume; panel↔helper over real Unix socket incl. wire-level replay; intent→dispatch-guard→helper flow for host verbs) | API + socket integration | 7eb51cf | PASS (3/3) | 7eb51cf |
| G31/G30 domain verb suite (6 tests: file.apply site-root confinement + traversal rejection + content bounds + typed discipline; database.apply binding/engine validation; sftp.apply account/key validation; vhost.apply deterministic render + loopback-only proxy + injection rejection) | helper protocol (in-process) | this commit | PASS (6/6) | see git log |
| G31 domain integration (1 test: file.write intent → worker → helper over real socket → file written in bound site root; acceptance idempotency; exactly one helper journal row) | worker + socket integration | a25ec5d | PASS (1/1) | a25ec5d |
| Domain service suite (10 tests, 23 assertions: commercial lifecycle incl. underpayment rejection + invoice settles once + foreign-scope rejection; ticket SLA/state machine/terminal resolved; pipeline build-concurrency + CAS promotion + outbox events; mail domain verify-before-mailbox + alias validation; observations latest-per-name; template catalog→install on application resources only; backup manifest verify+key gates) | service integration (isolated SQLite) | a15c8cd | PASS (10/10) | a15c8cd |
| Cumulative automated suite | all layers | — | PASS (53 tests, race in CI) | a15c8cd |
| G40 Stage-1 auth e2e (browser, real Go panel) | auth+onboarding | tools/ui-migration/e2e-auth.mjs | **PASS 11/11** (protection gate, real claim/address/MFA-skip/session/error-state/workspace/clean console) | facd209+ |
| G40 fidelity comparison vs 62-shot baseline | all 29 routes × 2 viewports | tools/ui-baseline/compare.js + docs/ui/compare-report.json | **PASS 58/58 within tolerance** (57 exact ≤0.5%, 1 chart-tolerance; worst 0.579%) | this wave |
| G40 a11y scan (axe-core) | 9 routes | docs/ui/a11y-report.json | Recorded — violations inherited from approved design tokens; no unapproved visual change | this wave |
| G40-S2 real-flow e2e (browser, live Go panel): onboarding claim, MFA enrollment, TOTP confirm, 10 real recovery codes, address write, MFA login, workspace routing, real resource registration (permission-checked), real intent + operation tracking, real audit, logout revocation | auth+personas | tools/ui-migration/e2e-s2.mjs | **PASS 18/18** | 472cf7c |
| G40-S2 security suite: cookie flags (HttpOnly/Strict/Secure/Path/Max-Age), token isolation, CSRF, direct-API denial, forged cookies, cross-tenant uniform denial, logout revocation + replay denial | BFF chain | tools/ui-migration/security-test.mjs | **PASS 16/16** | fd8bba6 |
| G40-S2 control matrix | all controls | docs/ui/CONTROL-MATRIX.csv | 324 accounted (197 complete / 42 partial-real / 85 static) | this wave |
| G40-S2 fidelity re-run after wiring | 29 routes x 2 viewports | tools/ui-baseline/compare.js + compare-report.json | desktop 27/29 within tolerance; mobile 22/29 (7 baseline transient-width artifacts documented) | this wave |
| G40-S2 a11y fix (nonvisual) | / | axe re-run | label-content-name-mismatch eliminated; contrast items remain owner-approval class | this wave |
| G40-S2 VPS re-verification of the updated build (2026-11-14): full real-flow e2e 18/18 through nginx TLS -> SvelteKit BFF -> Go on a fresh disposable installation; security spot-checks through nginx (cookie flags HttpOnly/Strict/Secure, token stripped from login body, forged cookie 401, API without credentials 401); drill pair torn down after evidence | VPS drill | tools/deploy/vps_run.py + e2e-s2.mjs (E2E_BASE) | **PASS** | this wave |
| G40-S2 remote push | git | branch feat/g40-frontend-migration | **DONE** — 5651d8c..524bab9 pushed (credentials supplied per-task, used transiently in the process environment only) | this wave |
| G40 deployment integration drill (2026-11-13, VPS): Node 22 + /opt/drevanta-web (adapter-node artifact, digest-packaged) + drevanta-web.service (MemoryMax 512M, hardening) + panel vhost include — nginx proxies /api/v1 to the Go panel and / to the frontend server; verified through nginx: / -> 302 session gate, /api/v1/install/status -> live Go JSON, /auth/login -> 200. One coherent installation, no manual glue. | VPS drill | infra/systemd/drevanta-web.service + infra/nginx/drevanta-panel-vhost.conf.example | PASS (end-to-end) | this commit |
| G30 web-server integration drill (2026-11-13, VPS): nginx installed on the disposable target; managed include `/etc/drevanta/vhosts/*.conf` wired into nginx.conf http scope; helper policy extended (file/database/sftp/vhost verbs, root-owned site bindings); API flow — vhost resource registration → vhost.apply intent (Idempotency-Key) → worker dispatch → helper wrote deterministic managed server block (root resolved through binding, server_name injected safely) → operation succeeded with journaled observation → nginx reload → `curl -H Host: drill.drevanta.test` returns the drill site. Operational note recorded honestly: platform's systemd reload did not propagate once; direct `nginx -s reload` verified — service-reload integration hardening queued for the G30 remainder. | VPS drill | this commit | PASS (end-to-end) | commit + drill transcript |
| Protocol fix verified: journaled effect verdict (Outcome) now rides HostResult — a protocol-ok response whose effect failed surfaces "failed", never "succeeded"; intent payload arguments flow to verbs (executor previously dropped them — caught by this test) | regression evidence | this commit | covered by suite | see git log |
| G20 VPS installation drill (authorized disposable target, exact supported tuple ubuntu 26.04): binary digests verified on target; systemd services (unprivileged panel user + root helper); bootstrap → claim token (single-use) → API claim → installation ACTIVE; login → resource registration → host.inspect intent → worker dispatch → privileged helper over Unix socket → helper journal (host.inspect succeeded fence=1) → operation succeeded. Failure-path evidence recorded honestly: unreachable-socket attempt → failed (no effect delivered); pre-fix ambiguous attempt → uncertain (blocked from unsafe retry per DEC-009). | VPS drill (SSH, systemd, curl) | this commit | PASS (end-to-end) | commit + docs/DEPLOY-DRILL.md |

Environment coverage: unit + isolated integration locally; API/security/tenant-isolation via
httptest; browser via VPS test environment; VPS lifecycle/backup/upgrade/rollback on disposable
VPS targets per graph G20+. CI adds `-race`.

## Registration protocol

1. Suite lands with code in same commit; CI runs `scripts/ci.sh`.
2. Results recorded here with commit SHA; failures recorded as FAIL with failing case IDs, never omitted.
3. Parametrized case IDs from the JSONL specs are marked covered only when a test binds the exact
   fixture, procedure and oracle from the specification row.

| G40-S3 per-control shell e2e (42 controls: skip link, account menu w/ real MFA status, mobile drawer a11y, palette/theme persistence, real-route links, real-data search palette, real audit notifications, service-center scope/filter controls) | browser e2e (fresh install, real claim+MFA, real API data) | tools/ui-migration/e2e-s3.mjs | **PASS (45/45)** | this wave |
| G40-S3 G2/G3/G5 gates (typography/token inheritance, focus/hover/disabled states) | browser computed-style + state forcing | tools/ui-migration/gates-s3.mjs | **PASS (6/6)** | this wave |
| G40-S3 a11y re-run (6 routes + mobile drawer state) | axe-core WCAG 2.1 AA | tools/ui-migration/a11y-s3.mjs | PASS — no new violation kinds vs Stage-2; /audit counts grew because the crash fix renders the real catalog | this wave |
| G40-S3 nginx security regression (template pins BFF-only routing) | Go unit (CI) | internal/api/nginx_template_test.go | **PASS** | this wave |
| G40-S3 deployed-config guard (loaded nginx -T, no direct Go routing/token exposure) | VPS verification | tools/deploy/nginx-guard.sh | **PASS** (after regenerating the helper-managed drill vhost that still carried the pre-BFF split routing) | this wave |
| G40-S2/S3 security suite (cookie flags from Set-Cookie, token isolation in localStorage/sessionStorage/DOM, cross-site CSRF denial, direct Go 401, forged cookie rejection, foreign-scope 404 uniformity, logout revocation + replay denial) — re-run MFA-aware (password -> verify -> cookie ceremony) | browser + raw-header probes | tools/ui-migration/security-test.mjs | **PASS (16/16)** | this wave (MFA-aware re-run) |

| G40-F1 domain capability API (support, commercial, pipeline, mail, observations, notifications, templates, backups, schedules — 34 routes) | API integration (isolated SQLite, httptest) | internal/api/domains_test.go | **PASS (6 suites: lifecycle + grant denial + tenant isolation + strict decode + state conflicts)** | this wave |
| G40-F1 SMTP recovery delivery (G34) | SMTP integration (fake relay) | internal/mailsmtp | **PASS (delivery, envelope, token, config validation)** | this wave |
| G40-F1 functional browser suite (persona sections real data, real mutations via UI forms, honest empty/denial states) | browser e2e (fresh install) | tools/ui-migration/e2e-s4.mjs | **PASS (12/12)** | this wave |
| G40-F1 regressions after persona wave | browser e2e (fresh installs) | e2e-s2.mjs, e2e-s3.mjs | **PASS (18/18, 45/45)** | this wave |
| G40-F1 VPS deployment-integration (new panel binary + full-persona web build deployed to the disposable VPS; helper-managed vhost verified again) | deployed-environment verification | tools/deploy/vps_run.py + nginx-guard.sh | **PASS** — services active; unauthenticated /api/v1/workspaces AND the new /api/v1/scopes/{id}/tickets answer 401 through the vhost (BFF session gate); bad-credential login 401; guard: no direct Go routing, no token exposure | this wave |
