# G40-S2 Control completion matrix (register-faithful)

Every control from the handoff registers is accounted: 324 rows (239 recorded + 85 static). Results (Stage-3): COMPLETE (static): 85; COMPLETE (static/flow purpose): 197; COMPLETE (typed component; real behavior verified): 42 — no PARTIAL or BLOCKED rows remain

Per-control rows are in CONTROL-MATRIX.csv. Full per-control component attribution, G4/G5 state gates and G9 accounting close with Stage-3 componentization (tracked in MIGRATION-LEDGER.md); nothing pending is claimed complete.

## Functional coverage register (G40-F1 — separate from visual/static control coverage)

The 324-row control matrix above records VISUAL/STATIC and interaction coverage of the
approved prototype controls. It does NOT certify functional completeness. The register
below classifies REAL functional coverage per capability; a mock-backed interactive
control is never classified production-complete here.

Full per-capability rows: FUNCTIONAL-COVERAGE.csv. Summary:
- SUP-01 Support tickets (create/list/answer/resolve + KB) — **OPERATIONAL**
- BIZ-01 Plans, customers, hosting accounts — **OPERATIONAL (external payment gateway blocked on owner credentials — BLK-001)**
- BIZ-02 Invoices, payments, credit ledger — **OPERATIONAL (manual gateway; fixture rates flagged non-production — DEC-018)**
- APP-05/06/07 Developer pipeline (source, builds, environments, promotions) — **OPERATIONAL (build execution worker is a later wave; CAS promotion state machine verified)**
- MAIL-01 Mail domains, mailboxes, aliases — **OPERATIONAL-API (service + API + authz verified; UI slot pending owner-approved design)**
- OBS-01 Health observations + notifications — **OPERATIONAL (recording is system-side: operations and services write observations)**
- WEB-09 Site backups (manifests, verified restore) — **OPERATIONAL (creation is job-driven; restore gated on verified manifest + key)**
- CORE-03/G10 Resources + intent operations (file.write, database.provision, domain.bind, host.inspect) — **OPERATIONAL (host effects execute through the privileged helper when a host is connected)**
- WEB-08 Schedules (arm/pause/list) — **OPERATIONAL**
- CORE-01 Identity, MFA, sessions, recovery — **OPERATIONAL (recovery delivery: SMTP transport implemented + tested; auditable no-op without relay config)**
- G34 SMTP recovery delivery — **OPERATIONAL (relay credentials are a deployment input)**
- AUDIT Audit trail across capabilities — **OPERATIONAL**
