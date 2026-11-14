#!/usr/bin/env python3
"""G40-S2: mount the real-data components into the persona routes.

Edits the GENERATED +page.svelte files (they are the source of truth since
Stage-1): RealDataBridge binds the real session (workspace identity, real
sign-out, workspace switching) and RealOpsDock exposes live operations
(real resources, real intents with operation tracking, real audit feed).
Idempotent: re-running does not duplicate mounts.
"""
import os
import re

FRONT = "/home/z/my-project/drevanta-repo/frontend"

# route dir -> (kind, dock)
MOUNTS = {
    ".": ("hosting", True),
    "server": ("server_admin", True),
    os.path.join("server", "services"): ("server_admin", True),
    "website": ("website", True),
    "developer": ("developer", True),
    "audit": ("hosting", False),  # session identity only on the meta screen
}

for rel, (kind, dock) in MOUNTS.items():
    path = os.path.join(FRONT, "src/routes", rel, "+page.svelte")
    src = open(path, encoding="utf-8").read()
    if "RealDataBridge" in src:
        print(f"already mounted: {rel}")
        continue
    # Imports inside the existing component script block.
    src = src.replace(
        "<script lang=\"ts\">",
        "<script lang=\"ts\">\n\timport RealDataBridge from '$components/RealDataBridge.svelte';"
        + ("\n\timport RealOpsDock from '$components/RealOpsDock.svelte';" if dock else ""),
        1,
    )
    # Mounts after the verbatim markup.
    mounts = f'\n<RealDataBridge preferredKind="{kind}" />'
    if dock:
        mounts += "\n<RealOpsDock />"
    src = src.replace("{@html markup}", "{@html markup}\n" + mounts, 1)
    with open(path, "w", encoding="utf-8") as f:
        f.write(src)
    print(f"mounted: {rel} kind={kind} dock={dock}")
