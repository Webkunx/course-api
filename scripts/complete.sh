#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:3000}"
TOKEN="${TOKEN:?set TOKEN=<access_token>}"

curl -s -X POST "$BASE/complete" \
  -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
