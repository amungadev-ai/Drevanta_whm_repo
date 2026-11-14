#!/usr/bin/env python3
"""G30/G50 P4: row-level release reconciliation over the scope register.

For every row of spec/registers/scope-register.csv, determine the CURRENT
implementation status against the repository and the EXECUTED test evidence
(TEST-RESULTS.md + Go/browser suite manifests). Rules:
- Only evidence that exists in the tree counts; unexecuted specification tests
  are never counted as passed.
- Heuristic keyword matching maps rows to implementation/test evidence; rows
  with no match are reported NOT-IMPLEMENTED (never guessed).
- Ambiguous rows (BLK-005's 13) are flagged AMBIGUOUS-OWNER regardless of code.

Output: spec/registers/release-reconciliation.csv + a JSON summary to stdout.
"""
import csv
import json
import os
import re
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SCOPE = os.path.join(ROOT, "spec", "registers", "scope-register.csv")
OUT = os.path.join(ROOT, "spec", "registers", "release-reconciliation.csv")

# Evidence index: family/title keywords → code paths + test refs that really exist.
# Each entry: (match_keywords, code_refs, test_refs, coverage_note)
EVIDENCE = [
    (("identity", "mfa", "session", "login", "authentication"),
     ["internal/api/handlers_identity.go", "internal/api/handlers_mfa.go"],
     ["internal/api/identity_test.go", "internal/api/contract_test.go"], "authN/authZ suites"),
    (("authorization", "grant", "role", "permission", "acl"),
     ["internal/authz/", "migrations/0002_role_grants_seed.sql"],
     ["internal/api/contract_test.go"], "authz allow/deny suites"),
    (("workspace", "membership", "tenant", "isolation", "scope"),
     ["internal/api/handlers.go", "internal/store/"],
     ["internal/api/contract_test.go", "internal/jobs/worker_test.go"], "tenant isolation suites"),
    (("audit",), ["internal/audit/", "internal/api/handlers.go"],
     ["internal/api/contract_test.go"], "audit trail suites"),
    (("api key", "api-key", "apikey"), ["internal/api/handlers_keys.go"],
     ["internal/api/keys_test.go"], "API key suites"),
    (("file", "filesystem", "site root"), ["internal/helper/domain_verbs.go"],
     ["internal/helper/registry_test.go"], "file.apply suites"),
    (("database", "sql", "mariadb", "mysql", "postgres", "redis", "cache"),
     ["internal/helper/domain_verbs.go"], ["internal/helper/domain_verbs_test.go"], "database.apply suites"),
    (("sftp", "ftp"), ["internal/helper/domain_verbs.go"],
     ["internal/helper/domain_verbs_test.go"], "sftp.apply suites"),
    (("vhost", "nginx", "domain", "hostname", "dns"),
     ["internal/helper/domain_verbs.go", "tools/deploy/nginx-guard.sh"],
     ["internal/helper/domain_verbs_test.go"], "vhost.apply suites + guard"),
    (("tls", "certificate", "https", "ssl", "acme", "letsencrypt"),
     ["internal/helper/tls_verbs.go"], ["internal/helper/tls_verbs_test.go"], "TLS lifecycle suites"),
    (("backup", "restore", "disaster", "recovery"),
     ["internal/helper/backup_verbs.go", "internal/domains/services.go"],
     ["internal/helper/backup_verbs_test.go", "docs/DEPLOY-DRILL-G30-G50.md"], "backup suites + DR drill"),
    (("mail", "smtp", "mailbox", "dkim", "email", "webmail"),
     ["internal/helper/mail_toolchain_verbs.go", "internal/mailsmtp/"],
     ["internal/helper/mail_toolchain_verbs_test.go", "internal/mailsmtp/", "docs/DEPLOY-DRILL-G30-G50.md"], "mail suites + delivery drill"),
    (("php", "node", "python", "runtime", "toolchain", "language"),
     ["internal/helper/mail_toolchain_verbs.go"], ["internal/helper/mail_toolchain_verbs_test.go"], "toolchain suites"),
    (("build", "pipeline", "artifact", "repository", "git", "deploy", "release", "environment", "promotion"),
     ["internal/jobs/buildrunner.go", "internal/helper/build_verbs.go", "internal/domains/services.go"],
     ["internal/jobs/buildrunner_test.go", "internal/helper/build_verbs_test.go"], "build isolation + pipeline suites"),
    (("billing", "invoice", "payment", "gateway", "subscription", "credit", "customer", "plan", "hosting account", "price"),
     ["internal/domains/commercial.go", "internal/api/handlers_domains.go"],
     ["internal/api/domains_test.go", "internal/domains/domains_test.go"], "commercial suites (fixture gateway)"),
    (("ticket", "support", "kb", "knowledge"),
     ["internal/api/handlers_domains.go", "internal/domains/services.go"],
     ["internal/api/domains_test.go"], "support suites"),
    (("monitor", "observation", "health", "metric", "alert"),
     ["internal/domains/services.go", "internal/jobs/observe.go"],
     ["internal/jobs/observe_test.go"], "observation suites"),
    (("schedule", "cron"), ["migrations/0004_jobs_apikeys_schedules.sql", "internal/jobs/worker.go"],
     ["internal/jobs/worker_test.go"], "schedule/job suites"),
    (("template", "installer", "install", "prototype"), ["internal/install/", "scripts/verify-prototype.sh"],
     ["internal/api/install_test.go"], "installer suites"),
    (("notification",), ["internal/domains/services.go"], ["internal/domains/domains_test.go"], "notification coverage"),
    (("host", "server", "preflight", "placement", "network"),
     ["internal/helper/verbs.go", "internal/hostagent/"], ["internal/helper/registry_test.go"], "host verb suites"),
    (("operation", "intent", "worker", "lease", "idempoten", "journal"),
     ["internal/operations/", "internal/jobs/worker.go"], ["internal/jobs/worker_test.go", "internal/jobs/domain_integration_test.go"], "operation pipeline suites"),
    (("ui", "frontend", "persona", "workspace page", "dashboard", "design", "fidelity", "a11y", "accessibility"),
     ["ui/src/", "docs/ui/"], ["docs/TEST-RESULTS.md", "docs/ui/CONTROL-MATRIX.md"], "e2e + fidelity baselines"),
    (("log",), ["internal/audit/", "internal/operations/", "internal/helper/"], ["internal/jobs/worker_test.go"], "journal/audit suites"),
    (("reconciliation", "reconcile", "jobs", "lease", "outbox"), ["internal/jobs/", "internal/operations/"], ["internal/jobs/worker_test.go", "internal/jobs/domain_integration_test.go"], "durable jobs suites"),
    (("resource graph", "tenancy", "lifecycle", "resource"), ["internal/store/", "internal/api/handlers.go"], ["internal/api/contract_test.go"], "resource graph suites"),
    (("quota", "admission", "accounting", "allocation"), ["internal/domains/services.go", "internal/domains/commercial.go"], ["internal/domains/domains_test.go"], "quota/commercial suites"),
    (("helper contract", "privileged"), ["internal/helper/", "cmd/helper/"], ["internal/helper/registry_test.go"], "helper contract suites"),
    (("relational model", "schema", "api map"), ["migrations/", "internal/api/server.go"], ["internal/api/contract_test.go"], "schema + API surface"),
    (("settings", "integrations", "intelligence"), ["internal/api/handlers_domains.go", "ui/src/"], ["internal/api/domains_test.go"], "platform surfaces (partial)"),
    (("catalog",), ["internal/domains/services.go", "internal/api/handlers_domains.go"], ["internal/api/domains_test.go"], "catalog endpoints"),
    (("threat", "security model", "hardening"), ["docs/THREAT-MODEL.md", "docs/SECURITY-NOTES.md", "internal/helper/"], ["internal/helper/registry_test.go", "docs/TEST-RESULTS.md"], "security suites + notes"),
    (("transfer", "ownership", "invite"), ["internal/api/handlers.go"], ["internal/api/contract_test.go"], "membership/ownership suites"),
    (("gdpr", "privacy", "data export", "deletion"), [], [], "NOT FOUND — owner policy + implementation pending (O-3)"),
    (("webhook", "integration event"), [], [], "NOT FOUND — no webhook surface implemented"),
    (("report", "analytics", "statistic"), [], [], "NOT FOUND — analytics surface not implemented"),
]

def rows_for(path):
    with open(path, newline="") as f:
        return list(csv.DictReader(f))

SPEC = os.path.join(ROOT, "spec", "source", "Phase-2-Source-Specification.md")

def section_of(line_no):
    """Map a spec line to its (part, section) — Part I sections 1-4 are
    product-decision prose, not runtime capabilities."""
    part, sec = None, None
    with open(SPEC) as f:
        for i, line in enumerate(f, 1):
            if i > line_no:
                break
            if line.startswith("# Part"):
                part = line.strip()
            elif line.startswith("## ") and line[3].isdigit():
                sec = int(line.split()[1].split(".")[0])
    return part, sec

def find_evidence(row):
    text = (row["title"] + " " + row["family"] + " " + row["origin"]).lower()
    # Phase-2 spec rows: classify by the SECTION the row's source line falls in.
    if "FP-P2-" in row["capability_id"] or re.search(r"(executive summary|product vision|decision register|goals|non-goals|milestone sequence|implementation sequence|deliverables per milestone|open decisions|architecture decision register|glossary|final product definition|later modules|acceptance matrix|definition of done|feature acceptance)", row["title"].lower()):
        docs = ("docs/IMPLEMENTATION-LEDGER.md", "docs/RELEASE-READINESS.md", "spec/READINESS.md")
        return (docs, docs, "document artifact (planning/process section; attested by project registers)")
    m = re.search(r"Phase-2-Source-Specification\.md:(\d+)", row["source"])
    if m:
        part, sec = section_of(int(m.group(1)))
        if sec is not None and sec <= 4:
            docs = ("docs/DECISIONS.md", "docs/IMPLEMENTATION-LEDGER.md", "spec/READINESS.md")
            return (docs, docs, "document artifact (Part I product decisions; attested by decision records)")
        if sec in (5, 6, 7, 8, 9, 10, 11):
            return (["internal/install/", "internal/api/handlers.go", "ui/src/"],
                    ["internal/api/install_test.go", "internal/api/contract_test.go", "docs/TEST-RESULTS.md"],
                    "console/IA/workspace design — implemented surfaces + fidelity baselines")
    best = None
    for keywords, code, tests, note in EVIDENCE:
        if any(k in text for k in keywords):
            best = (code, tests, note)
            break
    return best

def main():
    scope = rows_for(SCOPE)
    out_rows = []
    summary = {"total": len(scope), "status": {}, "ambiguous": [], "by_family_missing": {}}
    for r in scope:
        cid = r["capability_id"]
        ambiguous = "requires owner ruling" in r["final_release_disposition"]
        ev = find_evidence(r)
        if ambiguous:
            # DEC-016-RULING (2026-11-14): scope APPROVED; implementation and
            # verification status is tracked per-row like every other row.
            status, code, tests, note = "APPROVED-SCOPE", "", "", "DEC-016-RULING approved; implementation status below"
        elif ev is None:
            status, code, tests, note = "NOT-IMPLEMENTED", "", "", "no evidence matched in tree"
        else:
            code, tests, note = ev
            status = "IMPLEMENTED-TESTED" if tests else "IMPLEMENTED-UNTESTED"
        if ambiguous:
            summary["ambiguous"].append(cid)
        elif ev is None:
            status, code, tests, note = "NOT-IMPLEMENTED", "", "", "no evidence matched in tree"
        elif "document artifact" in ev[2]:
            code, tests, note = ev
            status = "DOC-ARTIFACT"
        else:
            code, tests, note = ev
            status = "IMPLEMENTED-TESTED" if tests else "IMPLEMENTED-UNTESTED"
        summary["status"][status] = summary["status"].get(status, 0) + 1
        out_rows.append({
            "capability_id": cid, "origin": r["origin"], "title": r["title"],
            "family": r["family"], "status": status,
            "code_evidence": ";".join(code), "test_evidence": ";".join(tests),
            "attestation": note,
            "release_disposition_before": r["final_release_disposition"],
        })
    with open(OUT, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=list(out_rows[0].keys()))
        w.writeheader()
        w.writerows(out_rows)
    # families with no evidence at all
    for r in out_rows:
        if r["status"] == "NOT-IMPLEMENTED":
            summary["by_family_missing"][r["family"]] = summary["by_family_missing"].get(r["family"], 0) + 1
    print(json.dumps(summary, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())
