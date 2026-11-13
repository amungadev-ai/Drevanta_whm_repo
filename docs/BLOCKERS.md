# Blocker Register

Living register. Source input: `spec/registers/handoff-blockers.csv` (476 inherited BLOCKED rows —
tracked wholesale; not duplicated row-by-row here). This file records *engineering* blockers
encountered during implementation, their affected requirements, options, recommendation, and
consequences. A blocker is closed only with linked evidence.

| ID | Raised | Kind | Affected | Description | Options | Recommendation | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| BLK-001 | 2026-11-13 | Authority | DEC-015, BIZ-01..03, release gates | Payment gateway credentials, tax jurisdiction and merchant-of-record rules are owner-specific inputs; native billing module cannot pass commercial acceptance tests without them | (a) develop billing core with gateway adapter behind seam and fixture gateways in tests; (b) defer BIZ wave | (a) — adapters + fixture provider now, real credentials before release certification | OPEN (release-gate) |
| BLK-002 | 2026-11-13 | Authority | DEC-018, phase 22 | Operational policy values (session TTLs, rate limits, retention, RPO/RTO) require owner-approved table; fixture values are test-only | (a) implement policy engine with conservative defaults flagged non-production; (b) block on input | (a) — engine + defaults, marked pending ratification | OPEN (release-gate) |
| BLK-003 | 2026-11-13 | Authority | TC-V4-* browser gates | Browser render-state evidence (DEC-017) requires an environment permitting Chromium process sockets; sandbox denied process socket in handoff attempt | (a) run browser tests on VPS test env; (b) request sandbox policy change | (a) — VPS is authorized test environment | OPEN |
| BLK-004 | 2026-11-13 | Credentials | All remote work | Owner-issued GitHub token + VPS root password are long-lived credentials; handoff protocol requires short-lived scoped credentials | (a) continue with owner-issued credentials stored sandbox-only per owner instruction, rotate/revoke at completion; (b) request scoped tokens | (a) per owner's explicit instruction | OPEN (revocation scheduled at completion) |
| BLK-005 | 2026-11-13 | Scope | DEC-016 sign-off | 13 scope rows carry "source optionality or inclusion boundary requires owner ruling"; 339 rows need row-level normative attestation | (a) implement all retained scope; flag ambiguous rows in ledger; (b) request batch owner ruling | (a) with explicit flagging; batch ruling requested in final report | OPEN (release-gate) |
