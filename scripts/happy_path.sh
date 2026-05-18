#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:3000}"

echo "=== happy path smoke test ==="

echo -n "health: "
curl -sf "$BASE/health" | python3 -m json.tool

echo -n "register: "
TOKEN=$(curl -sf -X POST "$BASE/register" | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "$TOKEN"

echo -n "next (ex_0000000001): "
EX=$(curl -sf -X POST "$BASE/next" -H "Authorization: Bearer $TOKEN" --compressed)
echo "$EX" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['exercise_id'])"

echo -n "complete: "
curl -sf -X POST "$BASE/complete" -H "Authorization: Bearer $TOKEN" | python3 -m json.tool

echo -n "next (ex_0000000002): "
EX2=$(curl -sf -X POST "$BASE/next" -H "Authorization: Bearer $TOKEN" --compressed)
echo "$EX2" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['exercise_id'])"

echo "=== PASS ==="
