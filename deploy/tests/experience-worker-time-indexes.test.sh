#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
fixture="${1:?pass an isolated yongbo-mysql-time-index-test-* container}"
[[ "$fixture" == yongbo-mysql-time-index-test-* ]] || exit 2
[[ "$(docker inspect "$fixture" --format '{{.HostConfig.NetworkMode}}')" == none ]] || exit 2
sql() { docker exec -i "$fixture" mysql --batch --raw --skip-column-names codex_time_index_test "$@"; }
migration="$root/db/migrations/149_experience_worker_run_time_indexes.sql"
forward() { sed '/^-- ROLLBACK-BEGIN/,$d' "$migration" | sql >/dev/null; }
fingerprint() { sql -e 'SELECT COUNT(*), SUM(id), SUM(CRC32(CONCAT_WS("|",worker_name,source_name,started_at,status,created_at))) FROM experience_worker_runs'; }
indexes() { sql -e "SELECT INDEX_NAME,GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='experience_worker_runs' AND INDEX_NAME IN ('idx_experience_worker_runs_created','idx_experience_worker_runs_started') GROUP BY INDEX_NAME ORDER BY INDEX_NAME"; }
awk '/^CREATE TABLE IF NOT EXISTS experience_worker_runs \(/{copy=1} copy{print} copy && /^\);/{exit}' "$root/db/migrations/100_v1_14_experience_phase2_closed_loop.sql" | sql
sql <<'SQL'
SET SESSION cte_max_recursion_depth=20001;
INSERT INTO experience_worker_runs(worker_name,source_name,started_at,status,created_at)
WITH RECURSIVE seq AS (SELECT 1 n UNION ALL SELECT n+1 FROM seq WHERE n<20000)
SELECT 'fixture','fixture',TIMESTAMPADD(SECOND,n,'2026-01-01'),'success',TIMESTAMPADD(SECOND,n,'2026-01-01') FROM seq;
SQL
before="$(fingerprint)"
expected=$'idx_experience_worker_runs_created\tcreated_at,id\nidx_experience_worker_runs_started\tstarted_at,id'
forward
[[ "$(indexes)" == "$expected" && "$(fingerprint)" == "$before" ]]
echo 'PASS initial application and data preservation'
forward
[[ "$(indexes)" == "$expected" && "$(fingerprint)" == "$before" ]]
echo 'PASS repeat application'
sql -e 'ALTER TABLE experience_worker_runs DROP INDEX idx_experience_worker_runs_started, ALGORITHM=INPLACE, LOCK=NONE'
forward
[[ "$(indexes)" == "$expected" ]]
echo 'PASS resume from one installed index'
sed -n '/^-- ROLLBACK-BEGIN/,$p' "$migration" | sql
[[ -z "$(indexes)" && "$(fingerprint)" == "$before" ]]
forward
[[ "$(indexes)" == "$expected" ]]
echo 'PASS rollback and reapply'
sql -e "EXPLAIN DELETE FROM experience_worker_runs WHERE created_at < '2026-01-01 00:16:40' LIMIT 1000" |
  awk -F '\t' '$5=="range" && $7=="idx_experience_worker_runs_created" && $10<=1100 {ok=1} END{exit !ok}'
echo 'PASS bounded retention range scan'
sql -e 'EXPLAIN SELECT id FROM experience_worker_runs ORDER BY started_at DESC,id DESC LIMIT 12' |
  awk -F '\t' '$7=="idx_experience_worker_runs_started" && $10<=12 && $12 !~ /filesort/ {ok=1} END{exit !ok}'
echo 'PASS recent-run ordered index scan'
[[ "$(fingerprint)" == "$before" ]]
