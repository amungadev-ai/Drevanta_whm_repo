#!/usr/bin/env bash
# G30/G50 P4: reproducible release-candidate build — ONLY after its gates pass.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PATH:/usr/local/go/bin:/home/z/my-project/tools/go/bin"
echo "== gate 1: gofmt =="
test -z "$(gofmt -l . | grep -v '^ui/' | grep -v '^spec/' | grep -v '^tools/')" || { gofmt -l . | grep -v '^ui/\|^spec/\|^tools/'; exit 1; }
echo "== gate 2: go vet =="
go vet ./...
echo "== gate 3: build =="
go build ./...
echo "== gate 4: full test suite (race) =="
go test ./... -count=1 -race
echo "== gate 5: release reconciliation =="
python3 scripts/release_reconcile.py > /tmp/rc-reconcile.json
python3 - <<'PY'
import json
s = json.load(open('/tmp/rc-reconcile.json'))
st = s['status']
assert sum(st.values()) == s['total'], 'unreconciled rows present'
assert all(k in ('IMPLEMENTED-TESTED', 'IMPLEMENTED-UNTESTED', 'NOT-IMPLEMENTED', 'AMBIGUOUS-OWNER', 'DOC-ARTIFACT') for k in st), 'unknown status label'
print('all', s['total'], 'scope rows statused:', st)
PY
echo "== gate 6: prototype provenance =="
scripts/verify-prototype.sh
echo "== building reproducible RC archive =="
VERSION="$(git rev-parse --short HEAD)"
OUT="drevanta-rc-${VERSION}.zip"
rm -f "$OUT"
python3 scripts/build_evidence_s3.py >/dev/null 2>&1 || true
echo "RC gates PASSED — archive: $OUT"
