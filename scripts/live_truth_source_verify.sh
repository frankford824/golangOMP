#!/usr/bin/env bash
# live 真相源收口专项 — 8081 OpenWeb + 8080 products JST 驱动验收
# 在服务器 root@223.4.249.11 上执行: bash scripts/live_truth_source_verify.sh
set -euo pipefail

BASE="${BASE:-/root/ecommerce_ai}"
cd "$BASE" || exit 1

MAIN_ENV="${MAIN_ENV:-$BASE/shared/main.env}"
BRIDGE_ENV="${BRIDGE_ENV:-$BASE/shared/bridge.env}"
LOG_DIR="${LOG_DIR:-$BASE/logs}"
OUTPUT_FILE="${OUTPUT_FILE:-}"

log() { echo "[$(date -Iseconds)] $*" | tee -a "${OUTPUT_FILE:-/dev/stdout}"; }
section() { echo ""; log "========== $* =========="; }

# --- Phase 1: 服务器运行态与环境确认 ---
section "Phase 1: 服务器运行态与环境确认"

log "pwd=$(pwd) whoami=$(whoami) hostname=$(hostname) date=$(date)"
log ""

log "--- 端口监听 ---"
ss -tlnp | grep -E '8080|8081|8082' || true

log ""
log "--- 进程 ---"
ps -ef | grep -E 'ecommerce-api|erp_bridge|workflow|server' | grep -v grep || true

log ""
log "--- 各端口 PID 详情 ---"
for pid in $(ss -tlnp 2>/dev/null | grep -E '8080|8081|8082' | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | sort -u); do
  echo "===== PID=$pid ====="
  ls -l /proc/$pid/exe 2>/dev/null || true
  echo "--- cmdline ---"
  tr '\0' ' ' < /proc/$pid/cmdline 2>/dev/null || true
  echo ""
done

log "--- Health ---"
curl -sS http://127.0.0.1:8080/health 2>/dev/null && echo " :8080" || log "8080 health failed"
curl -sS http://127.0.0.1:8081/health 2>/dev/null && echo " :8081" || log "8081 health failed"
curl -sS http://127.0.0.1:8082/health 2>/dev/null && echo " :8082" || log "8082 health failed"

log ""
log "--- 环境变量来源 (main.env / bridge.env) ---"
for f in "$MAIN_ENV" "$BRIDGE_ENV"; do
  if [ -f "$f" ]; then
    log "--- $(basename "$f") ---"
    grep -E '^ERP_|^JST_' "$f" 2>/dev/null | sed 's/\(APP_SECRET\|PASSWORD\|TOKEN\)=.*/\1=***/' || true
  else
    log "文件不存在: $f"
  fi
done

log ""
log "--- 8080/8081 进程环境 (关键变量) ---"
for pid in $(ss -tlnp 2>/dev/null | grep -E ':8080|:8081' | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' | sort -u); do
  echo "===== PID=$pid ====="
  tr '\0' '\n' < /proc/$pid/environ 2>/dev/null | grep -E '^ERP_|^JST_|^SERVER_PORT=' | sed 's/\(APP_SECRET\|PASSWORD\|TOKEN\)=.*/\1=***/' || true
done

log ""
log "--- 关键结论 ---"
ERP_MODE=$(grep -h '^ERP_REMOTE_MODE=' "$BRIDGE_ENV" "$MAIN_ENV" 2>/dev/null | tail -1 | cut -d= -f2)
ERP_SYNC_MODE=$(grep -h '^ERP_SYNC_SOURCE_MODE=' "$MAIN_ENV" 2>/dev/null | tail -1 | cut -d= -f2)
ERP_SYNC_EN=$(grep -h '^ERP_SYNC_ENABLED=' "$MAIN_ENV" 2>/dev/null | tail -1 | cut -d= -f2)
log "ERP_REMOTE_MODE=${ERP_MODE:-未配置}"
log "ERP_SYNC_SOURCE_MODE=${ERP_SYNC_MODE:-未配置}"
log "ERP_SYNC_ENABLED=${ERP_SYNC_EN:-未配置}"
log "8081 OpenWeb 条件: remote/hybrid + ERP_REMOTE_BASE_URL + openweb"
log "8080 JST sync 条件: ERP_SYNC_SOURCE_MODE=jst + ERP_SYNC_ENABLED=true"

# --- Phase 2: 只读实测 ---
section "Phase 2: 数据库现状 (需手动执行 SQL)"

log "请连接 MySQL 执行以下 SQL，并将结果附上："
cat << 'EOSQL'
SELECT COUNT(*) AS products_count FROM products;
SELECT COUNT(*) AS jst_inventory_count FROM jst_inventory;

SELECT id, erp_product_id, sku_code, product_name, updated_at
FROM products ORDER BY id DESC LIMIT 20;

SELECT * FROM products WHERE sku_code='HQT21413' OR erp_product_id='HQT21413' LIMIT 20;

-- jst_inventory 若存在
SELECT * FROM jst_inventory WHERE sku_code='HQT21413' OR sku_id='HQT21413' LIMIT 20;
EOSQL

section "Phase 2.2: 8081 商品搜索只读验收"

log "先获取 Admin Token (8080 登录接口)，然后执行："
log ""
log "# 搜索 HQT21413"
log 'curl -sS "http://127.0.0.1:8081/v1/erp/products?q=HQT21413&page=1&page_size=20" -H "Authorization: Bearer $TOKEN"'
log ""
log "# 或经 8080 转发 (若 8080 转发到 8081)"
log 'curl -sS "http://127.0.0.1:8080/v1/erp/products?q=HQT21413&page=1&page_size=20" -H "Authorization: Bearer $TOKEN"'
log ""
log "执行后立刻 grep 8081 日志："
log 'grep -E "erp_bridge_product_search|remote_ok|fallback_local_products|8081_remote|8081_local_only" '"$LOG_DIR"'/*.log | tail -30'

section "Phase 2.3: sync 只读验收"

log "# 查看 sync 状态"
log 'curl -sS "http://127.0.0.1:8080/v1/products/sync/status" -H "Authorization: Bearer $TOKEN"'
log ""
log "# 手动触发 sync"
log 'curl -sS -X POST "http://127.0.0.1:8080/v1/products/sync/run" -H "Authorization: Bearer $TOKEN"'
log ""
log "执行后 grep sync 日志："
log 'grep -E "erp_sync_run|source_mode|JSTOpenWeb|sync_role" '"$LOG_DIR"'/*.log | tail -20'

# --- Phase 5: 复验指引 ---
section "Phase 5: 复验检查清单"

log "A 组 (8081 OpenWeb):"
log "  - 日志出现 erp_bridge_product_search result=remote_ok fallback_used=false => 通过"
log "  - 日志出现 fallback_local_products => 未通过 (fallback)"
log "  - 日志出现 8081_local_only => 未通过 (未走 remote)"
log ""
log "B 组 (products JST 驱动):"
log "  - sync/status 返回 source_mode=jst => 配置正确"
log "  - sync/run 返回 status=success, total_upserted>0 => 有写入"
log "  - products 表有 spec_json 含 sync_role=8080_products_replica_from_openweb => 来自 JST"

log ""
log "=== 脚本结束。请将 Phase 1 输出 + SQL 结果 + curl 响应 + 日志 grep 结果汇总为验收报告 ==="
