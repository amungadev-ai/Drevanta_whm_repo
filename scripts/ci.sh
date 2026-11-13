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
# Fail on obvious credential artifacts ever entering the tree: GitHub token
# prefixes, generic long bearer tokens in .env-like files, private keys.
leaks=$(git ls-files -z | xargs -0 grep -lE 'ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|-----BEGIN (RSA|OPENSSH|EC) PRIVATE KEY-----|(?i)password\s*=\s*[A-Za-z0-9]{16,}' 2>/dev/null || true)
if [ -n "$leaks" ]; then
  echo "SECRET MATERIAL DETECTED in tracked files — refusing:"
  echo "$leaks"
  exit 1
fi

echo "== prototype provenance"
scripts/verify-prototype.sh

echo "== go test"
go test ./... -count=1

echo "CI OK"
