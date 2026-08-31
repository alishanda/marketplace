#!/usr/bin/env bash
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8180}"
SKU="${SKU:-STEAM-TOPUP-500}"

curl -s -X POST "$BASE/admin/providers/config" \
  -H 'Content-Type: application/json' \
  -d '{"a":{"error_rate":0,"timeout_rate":0},"b":{"error_rate":0,"timeout_rate":0}}' >/dev/null

ORDER=$(curl -s -X POST "$BASE/api/orders" -H 'Content-Type: application/json' -d "{\"sku\":\"$SKU\"}")
ID=$(printf '%s' "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
AMOUNT=$(printf '%s' "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["amount"])')
echo "order $ID"

seq 0 49 | xargs -P 50 -I{} curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST "$BASE/webhook/payment" \
  -H 'Content-Type: application/json' \
  -d "{\"event_id\":\"evt_race_${ID}_{}\",\"order_id\":\"$ID\",\"status\":\"paid\",\"amount\":$AMOUNT,\"currency\":\"RUB\",\"created_at\":\"2025-01-01T12:00:00Z\"}"

for i in $(seq 1 40); do
  BODY=$(curl -s "$BASE/api/orders/$ID")
  STATUS=$(printf '%s' "$BODY" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')
  CODE=$(printf '%s' "$BODY" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("delivery_code") or "")')
  echo "t=$i status=$STATUS code=$CODE"
  if [ "$STATUS" = "delivered" ]; then
    echo "ok: one delivery for $ID -> $CODE"
    exit 0
  fi
  sleep 0.25
done
echo "failed to deliver"
exit 1
