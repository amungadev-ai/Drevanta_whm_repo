#!/usr/bin/env bash
# Verify inherited prototype files against docs/prototype-manifest.sha256 (TC-P01-01/02 fixture).
# Usage: scripts/verify-prototype.sh [repo-root]
set -euo pipefail
root="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$root/ui"
# Recompute digests of the 71 inherited files and diff against the manifest.
status=0
while IFS=' ' read -r expected path; do
  if [ ! -f "$path" ]; then
    echo "MISSING $path"
    status=1
    continue
  fi
  actual=$(sha256sum "$path" | cut -d' ' -f1)
  if [ "$actual" != "$expected" ]; then
    echo "MISMATCH $path"
    status=1
  fi
done < ../docs/prototype-manifest.sha256
# Any extra file that is not in the manifest is a provenance violation.
count_manifest=$(wc -l < ../docs/prototype-manifest.sha256)
count_actual=$(find . -type f | wc -l)
if [ "$count_manifest" != "$count_actual" ]; then
  echo "FILE COUNT drift: manifest=$count_manifest tree=$count_actual"
  status=1
fi
if [ "$status" -eq 0 ]; then
  echo "prototype provenance OK ($count_manifest files)"
fi
exit "$status"
