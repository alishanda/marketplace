#!/usr/bin/env bash
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8180}"

curl -s -X POST "$BASE/admin/providers/config" \
  -H 'Content-Type: application/json' \
  -d '{"a":{"error_rate":1,"timeout_rate":0},"b":{"error_rate":0,"timeout_rate":0}}' >/dev/null

ORDER=$(curl -s -X POST "$BASE/api/orders" -H 'Content-Type: application/json' -d '{"sku":"GIFT-XBOX-1500"}')
ID=$(printf '%s' "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
AMOUNT=$(printf '%s' "$ORDER" | python3 -c 'import json,sys; print(json.load(sys.stdin)["amount"])')

curl -s -X POST "$BASE/webhook/payment" \
  -H 'Content-Type: application/json' \
  -d "{\"event_id\":\"evt_fb_$ID\",\"order_id\":\"$ID\",\"status\":\"paid\",\"amount\":$AMOUNT,\"currency\":\"RUB\",\"created_at\":\"2025-01-01T12:00:00Z\"}" >/dev/null

for i in $(seq 1 40); do
  BODY=$(curl -s "$BASE/api/orders/$ID")
  STATUS=$(printf '%s' "$BODY" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')
  PROV=$(printf '%s' "$BODY" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("delivery_provider") or "")')
  echo "t=$i status=$STATUS provider=$PROV"
  if [ "$STATUS" = "delivered" ] && [ "$PROV" = "B" ]; then
    echo "ok: fallback A->B for $ID"
    exit 0
  fi
  sleep 0.25
done
echo "fallback failed"
exit 1
