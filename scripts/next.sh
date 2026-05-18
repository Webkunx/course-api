#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:3000}"
TOKEN="${TOKEN:?set TOKEN=<access_token>}"

curl -s -X POST "$BASE/next" \
  -H "Authorization: Bearer $TOKEN" \
  --compressed | python3 -m json.tool
