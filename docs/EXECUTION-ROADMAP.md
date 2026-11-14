# Drevanta Execution Roadmap — expanded (G32 §3)

Updated: 2026-11-14 · wave = one checkpoint cycle of implementation + drill +
evidence. Estimates are engineering effort, not dates; no invented percentages.

## Verified this wave (G32, commit f7e4c3d)
FP-P3-052 analytics + FP-P4-026 source webhooks implemented and unit-tested
(reconciliation now 241 IMPLEMENTED-TESTED / 0 IMPLEMENTED-UNTESTED); mail
client operations (IMAP full session, POP3, tenant isolation); Redis per-
instance AUTH; container run idempotency; interactive PTY sessions; REAL
application build -> artifact -> release -> health -> rollback through
Drevanta verbs (nginx-served, health TRUE x3); full-stack chain
(cookie -> BFF -> API -> authz -> durable op -> worker -> helper -> host
effect -> audit) verified live including 401 rejection.

## Phase A — full-scope engineering completion

| ID | Workstream (reconciliation rows) | Depends | Effort | Status |
| --- | --- | --- | --- | --- |
| A1 | Mail: webmail frontend integration (DEC-023) + mail card UI | A10 | 1 wave | OPEN |
| A2 | Frontend wiring of G31/G32 surfaces: DNS zones, cache instances, containers, terminal, analytics cards + fidelity baselines | — | 2 waves | OPEN (BLK-009) |
| A3 | Website mgmt: managed app abstraction (APP-01), site create/suspend/archive, onboarding, static upload journeys | A2 | 2 waves | OPEN |
| A4 | DB + DB-user mgmt surfaces complete; file manager; domain/subdomain; DNS mgmt UI | A2 | 1.5 waves | OPEN |
| A5 | Identity/orgs: workspace+persona switching, delegation/invitations/revocation, agency-managed sites, teams, org hierarchies, reseller allocation | — | 2.5 waves | OPEN |
| A6 | Developer platform: project wizard, source-connection security, dev webhooks UI (FP-P4-026 surface), source history/provenance, deployment environments, promotion approval, secrets storage/injection/rotation/reveal/break-glass, supply-chain, diagnostics, capacity pressure | W3 core | 3 waves | OPEN |
| A7 | Platform admin: multi-node inventory+placement (ADM-05), Resource Intelligence Center, quotas/admission, settings+integrations, monitoring analytics, service lifecycle mgmt | — | 3 waves | OPEN |
| A8 | Security: scans/WAF/incident management (SEC-01) | A7 | 2 waves | OPEN |
| A9 | Marketplace + specialized app categories (MKT-02) | A3 | 1 wave | OPEN |
| A10 | Commercial: merchant identity/jurisdiction config, tax enforcement UI, invoices, checkout, subscriptions/renewals, refunds/credits, failed-payment recovery, reconciliation, SLA enforcement (DEC-026 core in tree: validation+tax math+sandbox adapter) | — | 2.5 waves | OPEN |
| A11 | Cross-platform: error taxonomy, high-risk confirmations, journeys, acceptance matrices, handoff artifacts | — | 1 wave | OPEN |

## Phase B — integration & operational verification
| B1 | Full-stack chains per persona incl. revocation, quotas, exhaustion, interruption, replay | A2-A10 | 2 waves | OPEN (first chain DONE G32) |
| B2 | DR extension: second clean VPS restore + app boot (BLK-010/011) | owner input | 0.5 wave | WAITING |
| B3 | ACME public-domain validation/issuance/renewal | DEC-024 input | 0.5 wave | WAITING |
| B4 | Security regression suite across new surfaces | A2, A8 | 1 wave | OPEN |
| B5 | Runtime matrix lifecycle tests (every supported runtime) | — | 1.5 waves | OPEN |

## Phase C — certification · D — clean install plan · E — private beta (approval-gated) · F — stabilization · G — v1.0 launch · H — maintenance
Combined estimate: ~26-30 engineering waves of remaining scope (Phases A+B),
parallelizable across ~3 independent tracks (A3-A4 web, A5-A6 identity/dev,
A7-A9 admin/infra; A10-A11 commercial/cross).

## Genuine owner inputs outstanding
1. Second disposable VPS for independent off-host DR (BLK-010/011).
2. Public domain for ACME (DEC-024).
3. Gateway choice + TEST credentials (DEC-025, options register attached).
4. Tax/SLA/ops VALUES (OWNER-DECISIONS-REQUIRED.md) — mechanisms configurable; values before commercial acceptance.
