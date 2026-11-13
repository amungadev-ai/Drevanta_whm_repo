#!/usr/bin/env bash
# Drevanta CI: fmt, vet, build, test, prototype provenance, secret redaction scan.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"

echo "== gofmt"
test -z "$(gofmt -l . | grep -v '^ui/' | grep -v '^spec/' )" || { gofmt -l . | grep -v '^ui/' | grep -v '^spec/'; exit 1; }

echo "== go vet"
go vet ./...

echo "== go build"
go build ./...

echo "== secret redaction scan (staged/tracked files)"
# Crude but effective: fail on obvious credential artifacts ever entering the tree.
if git ls-files -z | xargs -0 grep -lE 'ghp_[A-Za-z0-9]{20,}|UGaPmN3Kss9ZrEOH|BEGIN (RSA|OPENSSH) PRIVATE KEY' 2>/dev/null; then
  echo "SECRET MATERIAL DETECTED in tracked files — refusing"
  exit 1
fi

echo "== prototype provenance"
scripts/verify-prototype.sh

echo "== go test"
go test ./... -count=1

echo "CI OK"
