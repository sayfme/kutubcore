#!/usr/bin/env bash
# KutubCore CI (Milestone F): format, vet, test (unit + e2e), build.
# Used by both .github/workflows/ci.yml and .gitea/workflows/ci.yaml.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "== gofmt =="
UNFMT=$(gofmt -l .)
if [ -n "$UNFMT" ]; then
  echo "Unformatted files:"; echo "$UNFMT"; exit 1
fi

echo "== go vet =="
go vet ./...

echo "== go test =="
go test ./... -count=1

echo "== build =="
go build -o /tmp/opencode/kutubcore ./cmd/kutubcore
echo "ALL CI CHECKS PASSED"
