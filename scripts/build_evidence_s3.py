#!/usr/bin/env python3
"""G40-S3 §5: build the versioned Stage-3 evidence archive directly from the
pushed commit, with per-member digests and commit verification.

- Repository members are taken from `git archive HEAD` — byte-identical to the
  pushed commit, so the ZIP cannot contain stale or divergent copies.
- Run-evidence members come from the verification runs and are separated under
  evidence/ with their own digests.
- MANIFEST.sha256 covers every member; the script verifies the archive after
  writing (unzip -t equivalent + digest re-check).
"""
import hashlib
import io
import os
import subprocess
import sys
import tarfile
import zipfile

ROOT = "/home/z/my-project/drevanta-repo"
OUT = "/home/z/my-project/download"

REPO_MEMBERS = [
    "docs/TEST-RESULTS.md",
    "docs/RELEASE-READINESS.md",
    "docs/BLOCKERS.md",
    "docs/ui/CONTROL-MATRIX.csv",
    "docs/ui/CONTROL-MATRIX.md",
    "docs/ui/MIGRATION-LEDGER.md",
    "docs/ui/FUNCTIONAL-COVERAGE.csv",
    "docs/ui/compare-report.json",
    "docs/ui/a11y-report.json",
    "docs/ui/baseline/manifest.json",
    "docs/DEPLOY-DRILL.md",
    "docs/DEPLOY-DRILL-G30-G50.md",
    "docs/DEPLOY-DRILL-G31-G32.md",
    "docs/PAYMENT-GATEWAY-OPTIONS.md",
    "docs/EXECUTION-ROADMAP.md",
    "docs/DECISIONS.md",
    "docs/IMPLEMENTATION-LEDGER.md",
    "docs/OWNER-DECISIONS-REQUIRED.md",
    "spec/registers/release-reconciliation.csv",
    "infra/nginx/drevanta-panel-vhost.conf.example",
    "infra/systemd/drevanta-panel.service",
    "infra/systemd/drevanta-web.service",
    "infra/systemd/drevanta-helper.service",
    "tools/deploy/nginx-guard.sh",
    "tools/deploy/vps_run.py",
    "tools/deploy/vps_put.py",
    "tools/ui-migration/e2e-s2.mjs",
    "tools/ui-migration/e2e-s3.mjs",
    "tools/ui-migration/e2e-s4.mjs",
    "tools/ui-migration/gates-s3.mjs",
    "tools/ui-migration/a11y-s3.mjs",
    "tools/ui-migration/security-test.mjs",
]

EVIDENCE = {
    "evidence/e2e-s3-results.txt": "/tmp/drevanta-dev/e2e-s3.log",
    "evidence/e2e-s4-results.txt": "/tmp/drevanta-dev/e2e-s4.log",
    "evidence/e2e-s2-regression-results.txt": "/tmp/drevanta-dev/e2e-s2-reg.log",
    "evidence/e2e-s2-regression-results.txt": "/tmp/drevanta-dev/e2e-s2-regression.log",
    "evidence/e2e-vps-results.txt": "/home/z/my-project/download/ui-g40-s2/e2e-vps-results.txt",
    "evidence/security-results.txt": "/home/z/my-project/download/ui-g40-s2/security-results.txt",
}


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def main() -> int:
    head = subprocess.run(["git", "rev-parse", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    branch = subprocess.run(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=ROOT, capture_output=True, text=True).stdout.strip()
    if not head:
        print("cannot resolve HEAD", file=sys.stderr)
        return 1
    name = f"drevanta-fullplatform-g32-evidence-{head[:7]}.zip"
    path = os.path.join(OUT, name)

    # Repository members: from git archive of the exact pushed commit.
    archive = subprocess.run(["git", "archive", "HEAD", "--"] + REPO_MEMBERS,
                             cwd=ROOT, capture_output=True)
    if archive.returncode != 0:
        print("git archive failed: " + archive.stderr.decode(), file=sys.stderr)
        return 1
    members: dict[str, bytes] = {}
    with tarfile.open(fileobj=io.BytesIO(archive.stdout)) as tar:
        for m in tar.getmembers():
            if m.isfile():
                members[m.name] = tar.extractfile(m).read()

    commit_txt = (
        f"commit: {head}\nbranch: {branch}\n"
        f"stage: G40-S3 (frontend migration closed; 324/324 controls accounted)\n"
        f"generated: 2026-11-14\n"
        f"repo_members_source: git archive {head} (byte-identical to the pushed commit)\n"
    )
    members["commit.txt"] = commit_txt.encode()

    # Build digests: panel binary + frontend build tree (from the verified runs).
    build_lines = []
    panel = "/tmp/drevanta-dev/panel"
    if os.path.exists(panel):
        build_lines.append(f"panel-binary sha256={sha256(open(panel, 'rb').read())} path=/tmp/drevanta-dev/panel")
    fbuild = os.path.join(ROOT, "frontend/build")
    for dirpath, _, files in os.walk(fbuild):
        for fn in sorted(files):
            fp = os.path.join(dirpath, fn)
            rel = os.path.relpath(fp, fbuild)
            build_lines.append(f"frontend/build/{rel} sha256={sha256(open(fp, 'rb').read())}")
    members["evidence/build-digests.txt"] = ("\n".join(build_lines) + "\n").encode()

    for arc, src in EVIDENCE.items():
        if os.path.exists(src):
            members[arc] = open(src, "rb").read()

    manifest = "\n".join(f"{sha256(data)}  {n}" for n, data in sorted(members.items())) + "\n"
    members["MANIFEST.sha256"] = manifest.encode()

    os.makedirs(OUT, exist_ok=True)
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        for n in sorted(members):
            z.writestr(n, members[n])

    # Verification pass: reopen, check every digest + zip integrity.
    with zipfile.ZipFile(path) as z:
        bad = z.testzip()
        assert bad is None, f"corrupt member {bad}"
        for line in z.read("MANIFEST.sha256").decode().splitlines():
            digest, n = line.split("  ", 1)
            actual = sha256(z.read(n))
            assert actual == digest, f"digest mismatch {n}"
        # Commit verification: repo members byte-identical to the commit blobs.
        for n in REPO_MEMBERS:
            blob = subprocess.run(["git", "show", f"HEAD:{n}"], cwd=ROOT, capture_output=True).stdout
            assert z.read(n) == blob, f"member differs from commit: {n}"
    print(f"archive: {path} ({os.path.getsize(path)} bytes, {len(members)} members)")
    print("verified: zip integrity OK, MANIFEST.sha256 digests OK, repo members identical to commit", head[:7])
    return 0


if __name__ == "__main__":
    sys.exit(main())
