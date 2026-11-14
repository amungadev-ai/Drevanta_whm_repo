#!/usr/bin/env python3
"""G40-F1: extend the registers with a FUNCTIONAL coverage classification —
requirement ID, frontend component, API contract, permission, integration
test, browser test, outcome — kept strictly separate from the visual/static
control matrix (324 rows)."""
import csv

CSV_PATH = "docs/ui/FUNCTIONAL-COVERAGE.csv"
MD_PATH = "docs/ui/CONTROL-MATRIX.md"
LEDGER = "docs/ui/MIGRATION-LEDGER.md"
RESULTS = "docs/TEST-RESULTS.md"

# capability rows: requirement, capability, frontend component, api contract,
# permission, integration test, browser test, outcome classification
ROWS = [
    ("SUP-01", "Support tickets (create/list/answer/resolve + KB)",
     "PersonaRealData hosting support section (typed forms, real rows)",
     "GET/POST /api/v1/scopes/{id}/tickets; GET/POST …/tickets/{id}/messages; POST …/resolve; GET …/kb",
     "support.ticket.create:support ticket (hosting.owner, hosting.support_agent); requester-scoped reads otherwise",
     "internal/api: TestSupportTicketLifecycle (allow, strict-decode 422, staff scoping, foreign-scope 404, resolve)",
     "tools/ui-migration/e2e-s4.mjs (section rendering + real mutation flow)",
     "OPERATIONAL"),
    ("BIZ-01", "Plans, customers, hosting accounts",
     "PersonaRealData hosting customers/accounts/plans sections",
     "GET/POST …/plans; GET/POST …/customers; GET/POST …/accounts",
     "billing.adjust:billing entry (lists+creates); account.provision:hosting account (provision)",
     "internal/api: TestCommercialHostingLifecycle (lifecycle, grant denials 403, foreign-scope 404)",
     "tools/ui-migration/e2e-s4.mjs (hosting sections render; Go fixtures carry mutations)",
     "OPERATIONAL (external payment gateway blocked on owner credentials — BLK-001)"),
    ("BIZ-02", "Invoices, payments, credit ledger",
     "PersonaRealData hosting billing section (manual payment recording)",
     "GET/POST …/invoices; POST …/invoices/{id}/payments",
     "billing.adjust:billing entry",
     "internal/api: TestCommercialHostingLifecycle (invoice total, settle-once 409 on double payment)",
     "tools/ui-migration/e2e-s4.mjs (billing section renders real invoices)",
     "OPERATIONAL (manual gateway; fixture rates flagged non-production — DEC-018)"),
    ("APP-05/06/07", "Developer pipeline (source, builds, environments, promotions)",
     "PersonaRealData developer source/builds/environments sections",
     "GET/POST …/source-connections; GET/POST …/builds; GET/POST …/environments; POST …/environments/{id}/promote",
     "source.connect:source connection; environment.clone:environment; deployment.promote:release artifact",
     "internal/api: TestDeveloperPipelineLifecycle (connect, queue, premature-promote 409, promote after digest)",
     "tools/ui-migration/e2e-s4.mjs (developer sections render real state)",
     "OPERATIONAL (build execution worker is a later wave; CAS promotion state machine verified)"),
    ("MAIL-01", "Mail domains, mailboxes, aliases",
     "API-complete; no approved UI section slot exists in the prototype nav (recorded deviation candidate)",
     "GET/POST …/mail-domains; POST …/mail-domains/{id}/verify; GET/POST …/mailboxes; POST …/mail-aliases",
     "mail.domain.configure:mail domain; mailbox.provision:mailbox",
     "internal/api: TestWebsiteMailDomainLifecycle (pending-domain 409, verify, mailbox after verify, alias)",
     "API-level (browser section not rendered — no approved nav slot)",
     "OPERATIONAL-API (service + API + authz verified; UI slot pending owner-approved design)"),
    ("OBS-01", "Health observations + notifications",
     "PersonaRealData monitoring sections (server/website/developer); ModuleShellControls notifications; RealDataBridge MFA line",
     "GET …/observations[?resource_id]; GET …/notifications",
     "monitor.read:health observation (all personas)",
     "internal/api: TestObservationsNotificationsSchedulesBackups",
     "tools/ui-migration/e2e-s4.mjs (monitoring renders real state incl. empty; notifications real feed)",
     "OPERATIONAL (recording is system-side: operations and services write observations)"),
    ("WEB-09", "Site backups (manifests, verified restore)",
     "PersonaRealData website backups section; backup.apply intent verb mapped in the executor",
     "GET …/backups; intents: backup.restore on backup artifact resources",
     "backup.restore:backup artifact (website.owner)",
     "internal/domains: backup manifest verify + key gates (domains_test)",
     "tools/ui-migration/e2e-s4.mjs (backups section renders real manifests/empty)",
     "OPERATIONAL (creation is job-driven; restore gated on verified manifest + key)"),
    ("CORE-03/G10", "Resources + intent operations (file.write, database.provision, domain.bind, host.inspect)",
     "PersonaRealData website target sections with typed intent forms; RealOpsDock OperationTracker",
     "POST /api/v1/resources/{id}/intents; GET /api/v1/operations/{id}; POST /api/v1/scopes/{id}/resources",
     "resource.link + action grants per role (server.owner/website.owner/developer.owner matrices)",
     "internal/api: G10 contract suite + domains tests (denials, replay, generation fencing)",
     "tools/ui-migration/e2e-s4.mjs (real resource registered via UI form; intent forms render)",
     "OPERATIONAL (host effects execute through the privileged helper when a host is connected)"),
    ("WEB-08", "Schedules (arm/pause/list)",
     "PersonaRealData server jobs section",
     "GET …/schedules (list); POST …/schedules + pause (G40-S2 handlers)",
     "schedule.configure:schedule (server.owner/administrator/operator)",
     "internal/api: G10 suite (schedule lifecycle)",
     "tools/ui-migration/e2e-s4.mjs (jobs section renders real schedules)",
     "OPERATIONAL"),
    ("CORE-01", "Identity, MFA, sessions, recovery",
     "auth flows + RealDataBridge account menu (real MFA status) + RecoveryDeliveryNotice",
     "identity/login|mfa|logout|mfa/status|mfa/enroll|recovery-codes|recover-request|recover-confirm; install/*",
     "public + session-bound",
     "internal/api: G10 identity + MFA lifecycle tests; TestIdentityPasswordResetLifecycle",
     "tools/ui-migration/e2e-s2.mjs (claim, MFA enroll, verify, logout revocation)",
     "OPERATIONAL (recovery delivery: SMTP transport implemented + tested; auditable no-op without relay config)"),
    ("G34", "SMTP recovery delivery",
     "RecoveryDeliveryNotice (honest delivery state)",
     "Mailer seam: SmtpMailer via DREVANTA_SMTP_* env; LogMailer default",
     "system configuration",
     "internal/mailsmtp: delivery vs fake relay (envelope, token in body), config validation",
     "n/a (no live relay in the sandbox; transport verified against a relay fixture)",
     "OPERATIONAL (relay credentials are a deployment input)"),
    ("AUDIT", "Audit trail across capabilities",
     "PersonaRealData activity/audit sections; AuditFeed; notifications",
     "GET /api/v1/scopes/{id}/audit",
     "audit.read:audit event",
     "internal/api: audit allow+deny rows (G10); domain suites assert audit events on mutations",
     "tools/ui-migration/e2e-s2.mjs + e2e-s4.mjs (real audit feed rendering)",
     "OPERATIONAL"),
]

with open(CSV_PATH, "w", newline="") as f:
    w = csv.writer(f)
    w.writerow(["requirement_id", "capability", "frontend_component", "api_contract",
                "permission", "integration_test", "browser_test", "outcome"])
    w.writerows(ROWS)

md = open(MD_PATH).read()
addition = """

## Functional coverage register (G40-F1 — separate from visual/static control coverage)

The 324-row control matrix above records VISUAL/STATIC and interaction coverage of the
approved prototype controls. It does NOT certify functional completeness. The register
below classifies REAL functional coverage per capability; a mock-backed interactive
control is never classified production-complete here.

Full per-capability rows: FUNCTIONAL-COVERAGE.csv. Summary:
""" + "".join(
    f"- {r[0]} {r[1]} — **{r[7]}**\n" for r in ROWS
)
if "Functional coverage register" not in md:
    md = md.rstrip() + addition
open(MD_PATH, "w").write(md)

led = open(LEDGER).read()
led_add = """

## Full-persona functional wave (G40-F1, 2026-11-14)

The domain service layer (support, commercial hosting, developer pipeline, mail,
observability, templates, backups) was implemented but unexposed; this wave opened it
through 34 scope-pinned, grant-authorized endpoints and replaced the persona pages'
demonstration datasets with real sections in the approved markup (PersonaRealData).
The installer now provisions one workspace per persona kind at claim; workspace lists
order primary-first. SMTP recovery delivery (G34) is implemented and relay-tested.
Functional classification lives in FUNCTIONAL-COVERAGE.csv (12 capabilities: 10
OPERATIONAL, 1 OPERATIONAL-API, 1 OPERATIONAL with deployment-input caveat). Known
honest gaps: build execution worker, external payment gateway (BLK-001), mail UI slot
(no approved nav section), host-connected effects (require a connected host).
"""
if "Full-persona functional wave" not in led:
    led = led.rstrip() + led_add
open(LEDGER, "w").write(led)

res = open(RESULTS).read()
res_add = """
| G40-F1 domain capability API (support, commercial, pipeline, mail, observations, notifications, templates, backups, schedules — 34 routes) | API integration (isolated SQLite, httptest) | internal/api/domains_test.go | **PASS (6 suites: lifecycle + grant denial + tenant isolation + strict decode + state conflicts)** | this wave |
| G40-F1 SMTP recovery delivery (G34) | SMTP integration (fake relay) | internal/mailsmtp | **PASS (delivery, envelope, token, config validation)** | this wave |
| G40-F1 functional browser suite (persona sections real data, real mutations via UI forms, honest empty/denial states) | browser e2e (fresh install) | tools/ui-migration/e2e-s4.mjs | **PASS (12/12)** | this wave |
| G40-F1 regressions after persona wave | browser e2e (fresh installs) | e2e-s2.mjs, e2e-s3.mjs | **PASS (18/18, 45/45)** | this wave |
"""
if "G40-F1 domain capability API" not in res:
    res = res.rstrip() + "\n" + res_add
open(RESULTS, "w").write(res)
print("functional coverage register written; ledgers updated")
