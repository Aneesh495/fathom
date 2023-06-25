#!/usr/bin/env bash
# Build, test, and vet Fathom. Exits nonzero if any step fails.
set -euo pipefail
cd "$(dirname "$0")/.."

echo "gofmt"
fmt_out="$(gofmt -l .)"
if [[ -n "$fmt_out" ]]; then
  echo "these files need gofmt:"
  echo "$fmt_out"
  exit 1
fi

echo "vet"
go vet ./...

echo "test"
go test ./...

echo "examples"
go run ./cmd/fathom run examples/fact.scala >/tmp/fathom-fact.out
grep -q 720 /tmp/fathom-fact.out
go run ./cmd/fathom check examples/list.scala >/dev/null

echo "ok"
