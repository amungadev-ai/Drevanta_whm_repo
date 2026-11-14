# Drevanta Execution Roadmap (dependency-ordered, honest estimates)

Updated: 2026-11-14 · Statuses from release-reconciliation.csv + registers.
No invented deadlines; effort in engineering-wave units (one wave ≈ one
checkpoint cycle of implementation + drill + evidence).

## Phase A — Full-scope engineering completion (IN PROGRESS)

| ID | Workstream | Dependencies | Effort | Status |
| --- | --- | --- | --- | --- |
| A1 | Mail: IMAP INBOX path (dovecot 2.4), POP3, webmail integration | — | 1.5 waves | OPEN (BLK-006b) |
| A2 | Frontend wiring: mail card (DEC-021), DNS zones, cache, terminal, containers | A1 partial | 2 waves | OPEN (BLK-009) |
| A3 | Multi-node inventory + placement (ADM-05) | — | 2 waves | OPEN |
| A4 | Resource Intelligence Center + quotas/admission | A4a schema | 2 waves | OPEN |
| A5 | Platform settings/integrations surfaces | A2 | 1 wave | OPEN |
| A6 | Org hierarchies/reseller allocation (BIZ scope) | A4 | 1.5 waves | OPEN |
| A7 | Security scans/WAF/incident response (SEC-01) | — | 2 waves | OPEN |
| A8 | Terminal PTY streaming + container run idempotency | — | 1 wave | OPEN |
| A9 | Two IMPLEMENTED-UNTESTED rows → tested | — | 0.5 wave | OPEN |

## Phase B — Integration & operational verification

| ID | Workstream | Depends | Effort | Status |
| --- | --- | --- | --- | --- |
| B1 | Full-stack browser→host chains (fresh panel E2E per persona) | A2 | 1.5 waves | OPEN (H wave) |
| B2 | DR extension: second clean VPS restore + full app boot | owner input (BLK-010) | 0.5 wave | WAITING |
| B3 | ACME staging verification (public domain) | owner input (DEC-024) | 0.5 wave | WAITING |
| B4 | Security regression suite across new surfaces | A2, A8 | 1 wave | OPEN |

## Phase C — Release certification
Gates: 404-row reconciliation green for all non-owner rows; all drills re-run
on the final commit; RC build (gates pass) + checksum archive; owner sign-off.
## Phase D — Clean production install plan (docs + installer hardening)
## Phase E — Private beta prep (approval-gated) · F — Beta stabilization · G — v1.0 launch · H — maintenance

## Genuine owner inputs outstanding
1. Second disposable VPS for independent off-host DR verification (BLK-010).
2. Public domain for ACME staging/prod TLS (DEC-024).
3. Gateway provider choice + TEST credentials (DEC-025).
4. Tax/SLA/ops VALUES (docs/OWNER-DECISIONS-REQUIRED.md) — configurable now, values needed before commercial acceptance.
