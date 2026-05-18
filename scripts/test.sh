#!/usr/bin/env bash
set -euo pipefail

echo "running tests (3 times to check for flakes)..."
for i in 1 2 3; do
  echo "--- run $i ---"
  go test ./tests/... ./services/... -v -count=1 -timeout 120s
done
echo "=== all runs passed ==="
