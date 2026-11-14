# RELEASE-READINESS — Drevanta full-platform release

Updated: 2026-11-14 (G30/G50 wave) · Head: see git log · CI: required check `test`

## Status: NOT release-certified — engineering waves in progress (owner gate + engineering gates both required)

The platform is feature-complete for the implemented scope with REAL executed
evidence at every layer (unit/integration suites, browser e2e, fidelity baselines,
and ten live VPS drills). Certification is blocked ONLY on owner-gated inputs —
no engineering blocker remains unowned.

## What changed in this wave (G30/G50)

- Host operations completed with live-VPS evidence: TLS lifecycle (issue/renew/
  recover; ACME fail-closed), mail provisioning + end-to-end real delivery,
  runtime toolchain provisioning (real apt), service reload/restart/revert.
- Developer builds now execute in a VERIFIED ISOLATION BOUNDARY (systemd sandbox:
  no network, non-root, cgroup limits, allowlisted env, concurrency + timeouts,
  fail-closed probe) — untrusted repository content never runs unsandboxed.
- Full DR chain proven live: encrypted off-host backups (incl. sqlite WAL),
  integrity verification, off-host push/pull, restore into a clean environment,
  DB recovery (46+44 tables, integrity ok), interrupted-operation recovery,
  failed-upgrade rollback, reboot convergence, firewall/SSH safety rollback.
- Release reconciliation for ALL 404 scope rows produced
  (`spec/registers/release-reconciliation.csv`); CI now gates on it.

## Blockers to certification (all owner-gated)

| # | Blocker | Owner input needed |
| --- | --- | --- |
| 1 | BLK-005/008: 13 AMBIGUOUS-OWNER scope rows (DEC-016) | batch ruling |
| 2 | BLK-007 + BLK-006b/009 (ENGINEERING-OWNED, not owner-deferrable): Resource Intelligence Center, platform settings/integrations, quotas/admission, org hierarchies; IMAP INBOX path (dovecot 2.4), PTY streaming, container run idempotency, POP3/webmail; frontend wiring of new surfaces (mail card per DEC-021, DNS/cache/terminal/containers) + fidelity baselines | engineering execution waves |
| 3 | BLK-001/DEC-015: gateway credentials (TEST-mode path documented, DEC-020) | provider + test key |
| 4 | BLK-003/DEC-013: gateway credentials prod + domain for TLS ACME (DEC-019) | production inputs |
| 5 | TAX/SLA/OPS values (docs/OWNER-DECISIONS-REQUIRED.md T-1..O-5) | policy values |
| 6 | MAIL-UI placement (DEC-021 proposal) + IMAP auth wiring (BLK-006) | UI ruling; engineering wave |
| 7 | BLK-004: credentials rotation at completion | revoke/rotate |

## Removed / corrected statuses

- Obsolete: "expired disposable-target" (drill VPS active and verified), "GitHub
  write credentials" (owner-supplied per BLK-004 policy), "nginx /api/v1 template
  defect" (fixed in G40-S2, verified on VPS), "builds never execute" (isolated
  build execution drilled live), "backup chain unproven" (full DR drill).
- Contradictions removed: release-review rows now carry row-level status derived
  from REAL code+test evidence; unexecuted specification tests are never counted.

## Reproducible RC build

`scripts/release_candidate.sh` runs gates (gofmt, vet, build, full race suite,
reconciliation gate, prototype provenance) and only then produces the archive.
It is wired into CI (reconciliation step). The RC archive is produced after gates
pass; certification itself remains an explicit owner action.
