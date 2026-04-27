#!/usr/bin/env bash
# ERP/JST real-link verification checklist (run on production host).
set -euo pipefail
BASE="${BASE:-/root/ecommerce_ai}"
cd "$BASE" || exit 1

echo "=== Phase 1: listeners / health ==="
ss -tlnp | grep -E '8080|8081|8082' || true
curl -sS "http://127.0.0.1:8080/health" && echo " :8080"
curl -sS "http://127.0.0.1:8081/health" && echo " :8081"
curl -sS "http://127.0.0.1:8082/health" && echo " :8082"

echo "=== Phase 1: config (grep) ==="
grep -h 'ERP_REMOTE_MODE\|ERP_REMOTE_SKU\|ERP_SYNC_SOURCE' shared/bridge.env shared/.env 2>/dev/null || true

echo "=== Phase 3: Bridge product search sample ==="
curl -sS "http://127.0.0.1:8081/v1/erp/products?page=1&page_size=5" | head -c 2000
echo

echo "=== Phase 4: log hints (last 15 lines each) ==="
for pat in remote_erp_openweb jst_sku_query erp_bridge_hybrid; do
  echo "--- $pat ---"
  grep -h "$pat" logs/*.log 2>/dev/null | tail -15 || true
done

echo "Done. Phase 2 (SQL) run manually against MySQL — see docs/archive/ERP_REAL_LINK_VERIFICATION.md"
