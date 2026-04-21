#!/usr/bin/env bash
# Ripple Go backend · 端到端冒烟测试
# 用法：
#   ./scripts/smoke.sh                   # 默认 http://localhost:8000
#   BASE=http://host:8000 ./scripts/smoke.sh
#
# 流程：
#   1. POST /auth/register
#   2. POST /auth/login → 提取 token
#   3. POST /lakes
#   4. sleep 3 等 outbox dispatcher 处理 Lake 写到 Neo4j
#   5. GET  /lakes/{id}
#   6. POST /nodes
#   7. POST /nodes/{id}/evaporate
#   8. POST /nodes/{id}/restore
#
# 依赖：curl、jq
set -euo pipefail

BASE="${BASE:-http://localhost:8000}"
EMAIL="smoke-$(date +%s)@ripple.test"
PASS="smoke-password-123"
NAME="SmokeTester"

echo "== health =="
curl -fsS "$BASE/healthz" | jq .

echo "== register =="
curl -fsS -X POST "$BASE/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"display_name\":\"$NAME\"}" | jq .

echo "== login =="
TOKEN=$(curl -fsS -X POST "$BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\"}" | jq -r '.access_token')
echo "token: ${TOKEN:0:40}..."

auth="-H Authorization:Bearer\ $TOKEN"

echo "== create lake =="
LAKE=$(curl -fsS -X POST "$BASE/api/v1/lakes" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Smoke Lake","description":"e2e","is_public":false}' | jq .)
echo "$LAKE"
LAKE_ID=$(echo "$LAKE" | jq -r '.id')

echo "== wait outbox dispatcher (3s) =="
sleep 3

echo "== get lake =="
curl -fsS "$BASE/api/v1/lakes/$LAKE_ID" \
  -H "Authorization: Bearer $TOKEN" | jq .

echo "== create node =="
NODE=$(curl -fsS -X POST "$BASE/api/v1/nodes" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"lake_id\":\"$LAKE_ID\",\"content\":\"hello ripple\",\"type\":\"TEXT\",\"position\":{\"x\":0,\"y\":0,\"z\":0}}" | jq .)
echo "$NODE"
NODE_ID=$(echo "$NODE" | jq -r '.id')

echo "== evaporate =="
curl -fsS -X POST "$BASE/api/v1/nodes/$NODE_ID/evaporate" \
  -H "Authorization: Bearer $TOKEN" | jq '.state, .deleted_at, .ttl_at'

echo "== restore =="
curl -fsS -X POST "$BASE/api/v1/nodes/$NODE_ID/restore" \
  -H "Authorization: Bearer $TOKEN" | jq '.state'

echo
echo "OK · all smoke checks passed"
