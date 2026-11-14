#!/usr/bin/env bash
# Builds the release artifact from the CURRENT verified tree (methods B/C and
# the asset behind Method A):
#   drevanta-install-<version>-<commit>.tar.gz
# Contains: install.sh (shared engine from engine/), binaries (linux/amd64),
# console bundle, migrations, infra templates, docs, VERSION + SHA256SUMS.
# Gates: gofmt/vet/tests must pass. Used by scripts/cut_dev_release.sh.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PATH:/home/z/my-project/tools/go/bin"

echo "== gate: gofmt =="
test -z "$(gofmt -l cmd internal 2>/dev/null)" || { gofmt -l cmd internal; exit 1; }
echo "== gate: go vet =="
go vet ./...
echo "== gate: tests =="
go test ./... -count=1

COMMIT=$(git rev-parse --short HEAD)
VERSION="1.0.0-owneraudit.1"
OUT="drevanta-install-${VERSION}-${COMMIT}"
echo "== assembling $OUT =="

rm -rf "/tmp/$OUT" && mkdir -p "/tmp/$OUT/bin" "/tmp/$OUT/web" "/tmp/$OUT/docs" "/tmp/$OUT/infra"
cp engine/install.sh "/tmp/$OUT/"
chmod 755 "/tmp/$OUT/install.sh"
printf '%s+%s\nbuilt %s\n' "$VERSION" "$(git rev-parse HEAD)" "$(date -u +%Y-%m-%dT%H:%MZ)" > "/tmp/$OUT/VERSION"

GOOS=linux GOARCH=amd64 go build -o "/tmp/$OUT/bin/helper" ./cmd/helper
GOOS=linux GOARCH=amd64 go build -o "/tmp/$OUT/bin/panel" ./cmd/panel
cp -r frontend/build "/tmp/$OUT/web/build"
cp frontend/package.json "/tmp/$OUT/web/"
[[ -d frontend/node_modules ]] && cp -r frontend/node_modules "/tmp/$OUT/web/node_modules"
cp -r migrations "/tmp/$OUT/migrations"
cp -r infra/. "/tmp/$OUT/infra/"
for f in docs/INSTALL.md docs/OWNER-ACCESS.md docs/OWNER-ACCEPTANCE-TESTING.md docs/OWNER-AUDIT-FINDINGS.md docs/INTEGRATION-READINESS.md docs/MPESA-SANDBOX.md; do
  [[ -f "$f" ]] && cp "$f" "/tmp/$OUT/docs/"
done
mkdir -p "/tmp/$OUT/spec/registers"
cp spec/registers/release-reconciliation.csv "/tmp/$OUT/spec/registers/" 2>/dev/null || true

(cd "/tmp/$OUT" && find . -type f ! -name SHA256SUMS -exec sha256sum {} \; > SHA256SUMS)
tar -C /tmp -czf "$OUT.tar.gz" "$OUT"
sha256sum "$OUT.tar.gz" > "$OUT.tar.gz.sha256"
echo "package: $PWD/$OUT.tar.gz"
cat "$OUT.tar.gz.sha256"
