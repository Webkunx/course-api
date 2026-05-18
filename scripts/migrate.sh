#!/usr/bin/env bash
set -euo pipefail

DB="${DB_URL:-root:root@tcp(localhost:3306)/course_api}"
SCALE="${SCALE:-0.01}"
SEED="${SEED:-42}"

go run ./cmd/migrate \
  --db="$DB" \
  --scale="$SCALE" \
  --seed="$SEED" \
  --generator-dir=./data-generator \
  --data-dir=./data \
  "$@"
