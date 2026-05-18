#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE_URL:-http://localhost:3000}"

curl -s -X POST "$BASE/register" | tee /tmp/register_response.json
echo
TOKEN=$(cat /tmp/register_response.json | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "export TOKEN=$TOKEN"
