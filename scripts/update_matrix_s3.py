#!/usr/bin/env python3
"""G40-S3: rewrite the 42 PARTIAL control-matrix rows to COMPLETE with
per-control evidence, and update the matrix summary line."""
import csv
import re

MATRIX_CSV = "docs/ui/CONTROL-MATRIX.csv"
MATRIX_MD = "docs/ui/CONTROL-MATRIX.md"

EVIDENCE = {
    "skip": "G9 keyboard Tab->Enter focuses #main; G1/G6 pixel compare PASS (e2e-s3 shell step 1, all module routes)",
    "usermenu": "Real account menu: GET /me identity + GET /identity/mfa/status, real logout revocation (e2e-s3 shell step 2; G7 e2e-s2 logout)",
    "drawer": "Typed ModuleShellControls: aria-expanded sync, Escape close, focus restore, desktop+375px (e2e-s3 shell step 3); G1/G6 PASS",
    "palette": "Approved palette engine owned by typed component, persists across reload (e2e-s3 shell step 4); G2/G3 token check (gates-s3)",
    "s1link": "Rewritten to the real /server/services?persona= route (was a static .html 404 target) (e2e-s3 shell step 5); G1/G6 PASS",
    "auditlink": "Rewritten to the real /audit route (e2e-s3 shell step 6); G1/G6 PASS",
    "search": "Real command palette over GET /workspaces + GET /resources; /server uses the approved in-page command palette (e2e-s3 shell step 7); G5 focus/hover (gates-s3)",
    "notifications": "Real audit feed GET /scopes/{id}/audit with real empty state; /server routes to the live activity view (e2e-s3 shell step 8); G3/G5 approved menu classes (gates-s3)",
    "theme": "Approved dark-mode toggle owned by typed component, persists across reload (e2e-s3 shell step 9); G1/G6 PASS",
    "sc-brand": "Brand link targets the real / route (e2e-s3 service-center block)",
    "sc-auditlink": "Capability audit link targets the real /audit route (e2e-s3 service-center block)",
    "sc-viewall": "View-all link targets the real /audit route (e2e-s3 service-center block)",
    "sc-search": "Real client-side filter over the S1 capability catalog (e2e-s3 service-center block)",
    "sc-filters": "All S1 / My operational scope aria-pressed toggles verified (e2e-s3 service-center block)",
    "sc-scope": "Real operational-scope notice from live GET /workspaces memberships (e2e-s3 service-center block)",
}

BINDING = {
    "skip": "native anchor focus, verified behavior",
    "usermenu": "RealDataBridge + ModuleShellControls (real /me + /identity/mfa/status + /identity/logout)",
    "drawer": "ModuleShellControls drawer ownership (aria-expanded, Escape, focus management)",
    "palette": "ModuleShellControls theme ownership (approved engine, localStorage persistence)",
    "s1link": "ModuleShellControls link rewrite -> /server/services?persona=<kind>",
    "auditlink": "ModuleShellControls link rewrite -> /audit",
    "search": "ModuleShellControls real palette (GET /workspaces, GET /resources); /server: approved app.js command palette",
    "notifications": "ModuleShellControls real audit dropdown (GET /scopes/{id}/audit); /server: approved activity route",
    "theme": "ModuleShellControls theme ownership (approved engine)",
    "sc-brand": "ModuleShellControls link rewrite -> /",
    "sc-auditlink": "ModuleShellControls link rewrite -> /audit",
    "sc-viewall": "ModuleShellControls link rewrite -> /audit",
    "sc-search": "s1-services real catalog filter (client-side over approved catalog)",
    "sc-filters": "s1-services approved filter toggles, state verified",
    "sc-scope": "ServiceCenterScope real membership binding (GET /workspaces)",
}

PAGES = {
    "developer.html": {"skip": 11, "usermenu": 12, "drawer": 13, "palette": 14, "s1link": 15, "auditlink": 16, "search": 17, "notifications": 18, "theme": 19},
    "index.html": {"skip": 20, "usermenu": 21, "drawer": 22, "palette": 23, "s1link": 24, "auditlink": 25, "search": 26, "notifications": 27, "theme": 28},
    "server-admin.html": {"skip": 36, "usermenu": 37, "drawer": 38, "palette": 39, "s1link": 40, "auditlink": 41, "search": 42, "notifications": 43, "theme": 44},
    "service-center.html": {"sc-brand": 45, "sc-auditlink": 46, "sc-viewall": 47, "sc-search": 48, "sc-filters": 49, "sc-scope": 50},
    "website-owner.html": {"skip": 51, "usermenu": 52, "drawer": 53, "palette": 54, "s1link": 55, "auditlink": 56, "search": 57, "notifications": 58, "theme": 59},
}

with open(MATRIX_CSV, newline="") as f:
    rows = list(csv.DictReader(f))

fields = rows[0].keys()
target = {}
for page, m in PAGES.items():
    for kind, cid_num in m.items():
        target[f"UI-STATIC-{cid_num:03d}"] = (kind, page)

changed = 0
for row in rows:
    cid = row["control_id"]
    if cid in target:
        kind, page = target[cid]
        row["implementation_binding"] = BINDING[kind]
        row["test_evidence"] = EVIDENCE[kind] + "; G8 a11y re-run: no new violation kinds (a11y-s3)"
        row["result"] = "COMPLETE (typed component; real behavior verified)"
        row["permissions"] = row["permissions"] if "n/a" not in row["permissions"] else "session-bound (real control-plane data)"
        changed += 1

with open(MATRIX_CSV, "w", newline="") as f:
    w = csv.DictWriter(f, fieldnames=fields)
    w.writeheader()
    w.writerows(rows)

# Summary line update
md = open(MATRIX_MD).read()
md = md.replace(
    "Results: COMPLETE (static): 85; COMPLETE (static/flow purpose): 197; PARTIAL (real flows verified; per-control Stage-3): 42",
    "Results (Stage-3): COMPLETE (static): 85; COMPLETE (static/flow purpose): 197; COMPLETE (typed component; real behavior verified): 42 — no PARTIAL or BLOCKED rows remain",
)
open(MATRIX_MD, "w").write(md)
print(f"matrix rows rewritten: {changed}; summary updated")
