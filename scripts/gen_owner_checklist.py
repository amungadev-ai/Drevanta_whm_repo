#!/usr/bin/env python3
"""Generates docs/OWNER-ACCEPTANCE-TESTING.md — the owner manual audit
checklist across all 31 approved prototype pages and the four personas.
Statuses are honest implementation state, never aspirational."""
import csv

S = list(csv.reader(open('spec/registers/prototype-screens.csv')))
idx = {h: i for i, h in enumerate(S[0])}
screens = [(r[idx['route_id']], r[idx['title']], r[idx['relative_path']]) for r in S[1:]]

# status vocabulary
OPV = "OPERATIONAL-VERIFIED (implemented, VPS-verified)"
OP = "OPERATIONAL (implemented; owner verification pending)"
PART = "PARTIAL (backend or UI partially wired)"
UNIMPL = "UNIMPLEMENTED"
EXT = "EXTERNAL-CONFIG (requires owner-provided external inputs)"

# page -> (production route, persona, status, limitations, req ids)
PAGES = {
 "PAGE-001": ("/onboarding (check-email step)", "onboarding", OP, "email delivery requires configured mail/SMTP", "SEC-P1-012"),
 "PAGE-002": ("/onboarding (email recovery)", "onboarding", OP, "recovery delivery uses the same mail path", "SEC-P1-012"),
 "PAGE-003": ("/onboarding (forgot password)", "onboarding", OP, "—", "SEC-P1-015"),
 "PAGE-004": ("/onboarding (login success handoff)", "onboarding", OP, "—", "SEC-P1-032"),
 "PAGE-005": ("/login", "all", OPV, "session auth verified E2E (G32)", "SEC-P1-032"),
 "PAGE-006": ("/onboarding (reset password)", "onboarding", OP, "token epoch revocation enforced", "SEC-P1-015"),
 "PAGE-007": ("/onboarding (reset success)", "onboarding", OP, "—", "SEC-P1-015"),
 "PAGE-008": ("/onboarding (server recovery)", "onboarding", PART, "recovery-code path at claim; full server-recovery drill pending B1", "SEC-P1-015"),
 "PAGE-009": ("/onboarding (verify/MFA verify)", "onboarding", OP, "MFA verify endpoint live; enrollment UX in onboarding", "SEC-P1-032"),
 "PAGE-010": ("(internal audit page)", "admin", UNIMPL, "audit tooling, not a product surface; covered by registers", "—"),
 "PAGE-011": ("/developer", "developer", PART, "build/deploy/rollback engine VPS-verified; console wiring of projects/secrets/environments pending A6", "FP-P4-026, INT-01"),
 "PAGE-012": ("/ (workspace overview)", "all", OPV, "overview data via live API (G32 E2E)", "SEC-P2-029"),
 "PAGE-013": ("/login (alt entry)", "all", OPV, "—", "SEC-P1-032"),
 "PAGE-014": ("/onboarding/access-address", "owner", OP, "—", "install contract"),
 "PAGE-015": ("/onboarding (administrator creation)", "owner", OPV, "claim boundary verified: 0 pre-created principals", "install contract"),
 "PAGE-016": ("/onboarding (claim)", "owner", OPV, "single-use 24h claim token; wrong token 422 rejected", "install contract"),
 "PAGE-017": ("/onboarding (domain setup)", "owner", PART, "domain attach pending DEC-024 public domain", "FP-P3-020s"),
 "PAGE-018": ("/onboarding (handoff)", "owner", OP, "—", "install contract"),
 "PAGE-019": ("/onboarding (installation instructions)", "owner", OP, "docs/INSTALL.md shipped", "install contract"),
 "PAGE-020": ("/onboarding (account access)", "owner", OP, "—", "SEC-P1-032"),
 "PAGE-021": ("/onboarding (MFA enrollment)", "owner", PART, "TOTP verify live; enrollment UX completion pending A5", "SEC-P1-032"),
 "PAGE-022": ("/onboarding (workspace setup)", "owner", OP, "first workspace + membership created at claim", "SEC-P2-029"),
 "PAGE-023": ("/onboarding (recover)", "owner", PART, "as PAGE-008", "SEC-P1-015"),
 "PAGE-024": ("/onboarding/recovery-codes", "owner", OP, "issued at claim", "SEC-P1-015"),
 "PAGE-025": ("/onboarding (reset password)", "owner", OP, "—", "SEC-P1-015"),
 "PAGE-026": ("/onboarding (setup complete)", "owner", OP, "—", "install contract"),
 "PAGE-027": ("/onboarding (verify)", "owner", OP, "—", "SEC-P1-032"),
 "PAGE-028": ("/onboarding (first-time setup)", "owner", OP, "full flow ready for owner walk-through", "install contract"),
 "PAGE-029": ("/server", "server admin", PART, "host inventory/services live; Resource Intelligence Center, quotas, security center pending A7/A8", "ADM-05, SEC-01"),
 "PAGE-030": ("/server/services", "server admin", PART, "service lifecycle verbs live; specialized app categories pending A9", "FP-P2 services"),
 "PAGE-031": ("/website", "website owner", PART, "mail operations server-verified; DNS/cache/container/terminal/analytics cards pending A2 wiring", "MAIL-01, WEB-07"),
}

# directive item checklists per persona: (area, items)
SECTIONS = {
 "Server & Platform Administrator": [
  ("Dashboard", "/server", OP, "ADM-05"), ("Host inventory", "/server", PART, "ADM-05"),
  ("Resource Intelligence Center", "/server (RIC section)", UNIMPL, "ADM-05"),
  ("System resources", "/server (monitoring)", PART, "ADM-05"),
  ("Runtime management", "toolchain.apply (API)", OPV, "FP-P2 runtimes"),
  ("Service management", "/server/services", PART, "FP-P2 services"),
  ("Platform settings", "API (settings)", PART, "FP-P2 settings"),
  ("Integrations", "docs/INTEGRATION-READINESS.md", PART, "INT-01"),
  ("Security", "SEC-01 surfaces", UNIMPL, "SEC-01"),
  ("Firewall", "helper firewall verbs", PART, "SEC-01"),
  ("SSH", "ssh-key mgmt + scoped access", PART, "WEB-07"),
  ("Terminal", "exec.open/write/close (PTY)", OPV, "WEB-07"),
  ("Backups", "backup.apply", PART, "DR §8.1"),
  ("Logs", "helper journal + audit", OP, "ADM-05"),
  ("Audit trails", "/audit + GET /scopes/{id}/audit", OPV, "SEC-01"),
  ("Updates and rollback", "deploy.apply rollback", OPV, "deploy contract"),
 ],
 "Hosting Provider / Reseller / Agency": [
  ("Customers", "workspaces/memberships", PART, "FP-P2 orgs"),
  ("Organizations", "org hierarchies", UNIMPL, "FP-P2 orgs"),
  ("Resellers", "reseller allocation", UNIMPL, "FP-P2 orgs"),
  ("Hosting plans", "plans API", PART, "COMM"),
  ("Hosting accounts", "site provisioning", PART, "FP-P3"),
  ("Provisioning", "durable ops", OPV, "FP-P3"),
  ("Resource limits", "quotas/admission", UNIMPL, "ADM-05"),
  ("Billing", "commercial core (DEC-026)", PART, "COMM"),
  ("Invoices", "invoice engine", PART, "COMM"),
  ("Payments", "M-Pesa sandbox (DEC-025/M-Pesa)", PART, "COMM"),
  ("Subscriptions", "subscriptions", PART, "COMM"),
  ("Support", "tickets API", OP, "FP-P2 support"),
  ("Knowledge base", "kb API", OP, "FP-P2 kb"),
  ("Notifications", "notification service", PART, "FP-P2 notify"),
 ],
 "Website Owner": [
  ("Website creation", "site wizard", PART, "FP-P3 APP-01"),
  ("Website management", "site surfaces", PART, "FP-P3"),
  ("Domains and subdomains", "domain APIs", PART, "FP-P3 DNS"),
  ("DNS management", "dns.apply (zone synthesis verified)", PART, "WEB-07 dns"),
  ("TLS certificates", "tls.apply (ACME behind DEC-024)", PART, "TLS"),
  ("File manager", "sftp.apply", PART, "FP-P3 files"),
  ("Database management", "database.apply", OPV, "FP-P3 db"),
  ("Database users", "database.apply users", OP, "FP-P3 db"),
  ("Email management", "mail.apply", OPV, "MAIL-01"),
  ("Mailboxes", "mail.apply lifecycle", OPV, "MAIL-01"),
  ("Webmail", "maintained reader integration", UNIMPL, "MAIL-01 (A1)"),
  ("SSH and SFTP", "sftp.apply", OP, "WEB-07"),
  ("Browser terminal", "exec.open/write/close PTY", OPV, "WEB-07"),
  ("Scheduled tasks", "cron surfaces", UNIMPL, "FP-P3 cron"),
  ("Backups and restoration", "backup.apply", PART, "DR §8.1"),
  ("Analytics", "FP-P3-052 engine (API)", PART, "FP-P3-052"),
  ("Application installation", "APP-01 catalog", UNIMPL, "APP-01"),
  ("Security", "site security center", UNIMPL, "P3 §53"),
  ("Logs", "site logs", PART, "FP-P3 observe"),
 ],
 "Developer": [
  ("Projects", "project wizard", PART, "P4 dev"),
  ("Source connections", "source security + provenance", PART, "P4 §25"),
  ("Webhooks", "FP-P4-026 engine (HMAC/replay/dedup)", OPV, "FP-P4-026"),
  ("Builds", "build.execute (isolated, artifact)", OPV, "P4 builds"),
  ("Artifacts", "artifact tar.gz + digest", OPV, "P4 builds"),
  ("Deployments", "deploy.apply release/health", OPV, "deploy contract"),
  ("Environments", "deployment environments", UNIMPL, "P4 §28"),
  ("Secrets", "storage/injection/rotation", UNIMPL, "P4 secrets"),
  ("Runtime configuration", "env injection", PART, "P4 runtime"),
  ("Containers", "container.apply (idempotent run)", OPV, "containers"),
  ("Database and cache services", "database.apply + cache.apply AUTH", OPV, "cache/db"),
  ("Monitoring", "observability surfaces", PART, "P4 observe"),
  ("Logs", "build/deploy logs", PART, "P4 observe"),
  ("Promotion and rollback", "rollback + approval policies", PART, "deploy contract"),
 ],
}

def row(cells, tag="|"):
    return "| " + " | ".join(cells) + " |"

out = []
out.append("# Drevanta Owner Acceptance Testing (Manual Audit Checklist)\n")
out.append("Build 1.0.0-owneraudit.1 @ commit 2a125c9 (branch feat/g40-frontend-migration).")
out.append("Access: docs/OWNER-ACCESS.md (SSH tunnel, loopback-only). Statuses are the honest")
out.append("implementation state at handover: **OPERATIONAL-VERIFIED** (implemented + VPS-verified),")
out.append("**OPERATIONAL** (implemented, awaiting your verification), **PARTIAL**, **UNIMPLEMENTED**,")
out.append("**EXTERNAL-CONFIG**. Record observations in the per-item cells and defects in")
out.append("docs/OWNER-AUDIT-FINDINGS.md. No finding or acceptance result below is pre-filled.\n")

out.append("## Part 1 — Onboarding (your first walk-through)\n")
out.append("Follow docs/OWNER-ACCESS.md: tunnel → claim (single-use token) → recovery codes →")
out.append("access address → sign-in → MFA/security setup → dashboard. Expected first-run behavior")
out.append("and boundaries are listed there; the platform ships with zero pre-created accounts.\n")

out.append("## Part 2 — Approved prototype pages (31/31)\n")
out.append(row(["Page", "Title", "Production route", "Persona", "Status", "Known limitations / notes", "Requirement IDs", "Owner observations"]))
out.append(row(["---", "---", "---", "---", "---", "---", "---", "---"]))
for pid, title, rel in screens:
    route, persona, status, lim, req = PAGES.get(pid, ("—", "—", UNIMPL, "—", "—"))
    out.append(row([pid, title.replace("|", "/"), route, persona, status, lim, req, ""]))

for persona, items in SECTIONS.items():
    out.append(f"\n## Part 3 — {persona}: functional audit items\n")
    out.append(row(["#", "Capability", "Page / route / surface", "Prerequisites", "Manual procedure", "Expected result", "Status", "Requirement", "Owner observations"]))
    out.append(row(["---", "---", "---", "---", "---", "---", "---", "---", "---"]))
    for i, (cap, where, status, req) in enumerate(items, 1):
        proc = f"Sign in as {persona.split('/')[0].strip()}; open {where}; exercise the capability end-to-end; confirm real effects (no mock success states)."
        out.append(row([str(i), cap, where, "Active persona workspace", proc, "Real data/effects; accurate progress and error states; permission boundaries enforced", status, req, ""]))

out.append("""
## Part 4 — Persona test-account creation (after your onboarding)

Create additional accounts from inside the platform (workspace invitations /
team members) so each persona is evaluated with realistic permissions:
1. **Server & Platform Administrator** — your claim account.
2. **Hosting Provider / Reseller / Agency** — invite a second principal; grant
   provider-scope roles; verify scope isolation from your admin data.
3. **Website Owner** — third principal with a website-owner workspace.
4. **Developer** — fourth principal; connect a source repository; verify
   developer-scope boundaries.

Optional demonstration fixtures (clearly identified, separate from real data):
a disposable demo website, a demo database, a demo invoice — create them only
after onboarding, from the respective management surfaces.

## Part 5 — Ground rules

- Do not treat any item as accepted before performing it; the register ships
  with no pre-filled results.
- Mock-backed success states are prohibited by policy — if you observe one,
  file it as a HIGH finding.
- External-configuration items (TLS domain, gateway credentials) are expected
  to be non-functional until you supply the inputs; that is a status, not a
  defect.
""")

open('docs/OWNER-ACCEPTANCE-TESTING.md', 'w').write("\n".join(out) + "\n")
print("checklist written:", len(screens), "pages,", sum(len(v) for v in SECTIONS.values()), "audit items")
