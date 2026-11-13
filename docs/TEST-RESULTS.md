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
