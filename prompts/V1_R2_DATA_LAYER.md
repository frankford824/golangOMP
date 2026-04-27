# V1 · R2 · 数据层迁移 + Backfill(v3 · 真生产对齐)

> 版本:**v3**(2026-04-17)—— 本文替代 v2;v2 因 R1.6 真生产探测发现 2 处口径偏差 + 性能假设不成立而重写。
> 状态:**待 Codex 执行**
> 依赖:R1(OpenAPI + 501 骨架)已合并;R1.5 + R1.6 权威文档 v1.2 已签字;仓库 `db/migrations/` 最大号段 058。
> 执行环境:**SSH 直连 `jst_ecs` 真生产 `jst_erp` 库**(理由见 §4);本地 Docker 不用。
> 禁止前置:不能在 R1.6 未签字前启动 Codex。

## 0. 本轮目标(一句话)

> **建表 + 回填 + 回滚三件套落地到真生产 `jst_erp`**,把 R1.6 签字版权威文档里的数据模型映射到生产 MySQL;**不动 service / transport 业务代码**;所有新路由保持 R1 冻结的 `501 Not Implemented` 不变。

## 1. 必读(Read-Only)输入

Codex 启动时**必须先读**以下文件。

1. `docs/V1_MODULE_ARCHITECTURE.md` **v1.2**
   - §8 数据模型 · §10.1 derived_status · §11.2 backfill 规则
   - §17 Q7.5 行(priority **4 值**:`low | normal | high | critical`)
2. `docs/V1_ASSET_OWNERSHIP.md` **v1.2**
   - §2.1 `task_assets` 新增 7 列拆 061 + 066
   - §3 `reference_file_refs` 展平表方案
   - §6.1 + §6.1a(asset_type **5 值**:`reference | source | delivery | design_thumb | preview`)
   - §6.3 JSON 解析规则
3. `docs/V1_CUSTOMIZATION_WORKFLOW.md` v1.1
   - §3.1.1.1 R2 新建 `task_customization_orders`
4. `docs/V1_INFORMATION_ARCHITECTURE.md` v1.1
   - §3.5.9 `task_drafts` · §5 / §6 `org_move_requests` · §8.2 `notifications`
5. `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` v1.0
6. `docs/iterations/V1_R1_6_PROD_ALIGN.md` **v1.0** · §1 真生产数据快照 · §2 三项决策
7. `domain/enums_v7.go`(`TaskStatus` 真实 24 值)
8. `domain/reference_file_ref.go`(JSON ref 结构)
9. `db/migrations/001_v7_tables.sql` · `004_v7_task_assets_*.sql` · `020_v7_asset_storage_upload_boundary.sql` · `036_v7_design_asset_flow_semantics.sql` · `042_v7_task_note_reference_file_refs.sql` · `054_v7_customization_schema_closure.sql`
10. `tmp/release_v09_schema_guarded_patch.sql`(只读)
11. 当前 `docs/api/openapi.yaml`(确认 R2 路径全部 `x-owner-round: R2`,响应仍 501)
12. `deploy/run-org-master-convergence.sh`(SSH 执行模板,Codex 仿照其风格)
13. `tmp/r2_probe_readonly.sh`(R1.6 只读探测脚本,Codex 跑前后各跑一次做对比)

禁止引用:已删除的 R2 v1 遗迹;`docs/archive/*`;R1.5 之前的 V1 权威文档版本。

## 2. 交付范围(精确列表)

### 2.1 10 份迁移 SQL

所有迁移**必须**包含 `-- ROLLBACK-BEGIN` / `-- ROLLBACK-END` 注释块。

| 号段 | 文件 | 内容 |
| --- | --- | --- |
| 059 | `db/migrations/059_v1_0_task_modules.sql` | `CREATE TABLE task_modules`(主 §8.1)+ `UNIQUE(task_id, module_key)` + pool/claim 索引 |
| 060 | `db/migrations/060_v1_0_task_module_events.sql` | `CREATE TABLE task_module_events` + `INDEX(task_module_id, created_at)` + `INDEX(event_type, created_at)` |
| 061 | `db/migrations/061_v1_0_task_assets_source_module_key.sql` | `ALTER TABLE task_assets` ADD `source_module_key VARCHAR(32) NOT NULL DEFAULT 'design'` + `source_task_module_id BIGINT NULL` + `is_archived TINYINT(1) NOT NULL DEFAULT 0` + `archived_at DATETIME NULL` + `archived_by BIGINT NULL`;加 `INDEX(source_task_module_id)`、FK 到 `task_modules.id` |
| 062 | `db/migrations/062_v1_0_reference_file_refs_flat.sql` | **`CREATE TABLE reference_file_refs`** 展平表(资产 §3.2):`id / task_id / sku_item_id NULL / ref_id / owner_module_key / context NULL / attached_at`;`UNIQUE(task_id, ref_id, sku_item_id)`;FK 到 `tasks.id` 和 `asset_storage_refs.ref_id`。**全新建表,不是 ALTER** |
| 063 | `db/migrations/063_v1_0_task_drafts.sql` | `CREATE TABLE task_drafts`(IA §3.5.9 DDL 一比一)+ `INDEX(owner_user_id, task_type, expires_at)` |
| 064 | `db/migrations/064_v1_0_notifications.sql` | `CREATE TABLE notifications`(IA §8.2 DDL 一比一)+ `INDEX(user_id, is_read, created_at)` |
| 065 | `db/migrations/065_v1_0_org_move_requests.sql` | `CREATE TABLE org_move_requests`(IA §5.2):`id / source_department / target_department NULL / user_id / state(pending_super_admin_confirm / approved / rejected) / requested_by / resolved_by NULL / reason / resolved_at NULL / created_at` |
| 066 | `db/migrations/066_v1_0_task_assets_lifecycle.sql` | `ALTER TABLE task_assets` ADD `cleaned_at DATETIME NULL` + `deleted_at DATETIME NULL` + `INDEX(is_archived, deleted_at)` |
| 067 | `db/migrations/067_v1_0_tasks_priority_constraint.sql` | **不新建列**:仅 `ALTER TABLE tasks` ADD CHECK **`priority IN ('low', 'normal', 'high', 'critical')`**(R1.6 · Z1 决策,4 值,对齐真生产分布 `low 56 / high 20 / normal 19`)+ ADD `INDEX idx_tasks_priority_created(priority, created_at)`。Rollback 段:DROP INDEX + DROP CHECK;**严禁** DROP COLUMN priority |
| 068 | `db/migrations/068_v1_0_task_customization_orders.sql` | `CREATE TABLE task_customization_orders`(定制 §3.1.1.1)+ `KEY(online_order_no)` + `KEY(erp_product_code)` + FK `task_id` 到 `tasks.id` |

**约定**:

- 新表字符集 `utf8mb4_unicode_ci`,引擎 `InnoDB`
- 时间列统一 `DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP` / `ON UPDATE CURRENT_TIMESTAMP`
- `ALTER TABLE ... ADD COLUMN` 用 MySQL 8.x 语法;幂等由 forward 工具在执行前做 `SELECT COLUMN_NAME FROM information_schema.COLUMNS` 判断,不依赖 `IF NOT EXISTS`
- **严禁**:读或写 `tasks.status` / `tasks.is_urgent` / `tasks.task_priority` / `task_assets.flow_stage`(全部不存在)

### 2.2 三个工具包(Go)

| 包 | 路径 | 入口 |
| --- | --- | --- |
| Forward | `cmd/tools/migrate_v1_forward/main.go` | 按号段顺序执行 059~068,幂等;读取 `--dsn` 参数 |
| Backfill | `cmd/tools/migrate_v1_backfill/{main.go, phases.go, mapping.go, query.go, helpers.go}` | Phase A~E,支持 `--dry-run` / `--batch-size=1000` / `--dsn` / **`--cleanup-partial`**(R1.6 新增,清空 task_modules / task_module_events / reference_file_refs / task_customization_orders,便于失败后人工重跑);日志前缀 `[R2-BACKFILL]` |
| Rollback | `cmd/tools/migrate_v1_rollback/main.go` | 按号段逆序提取 `-- ROLLBACK-BEGIN/END` 块执行;读取 `--dsn`;**新增 `--dry-run`** 模式(R1.6 新增,生产默认先跑 dry-run) |
| 共享库 | `cmd/tools/internal/v1migrate/migrate.go` | 迁移列表常量 + 通用 MySQL 连接 / 事务辅助;不可被业务代码 import |

### 2.3 Backfill Phase 语义

按主 §11.2 + 资产 §6.1(v1.2) + §6.3 严格实现。

#### Phase A — `task_modules` / `task_module_events` 初始化

- 扫 `tasks`,按 `task_type` 查 blueprint(硬编码 7 种 task_type 对应的模块列表,见主 §4.1)
- **SELECT 源列永远是 `tasks.task_status`**
- 对每条任务:先 INSERT `basic_info` 模块(state=`active`);再按主 §11.2 推其他模块 state / claimed_by / pool_team_code;为每条 INSERT 写 `task_module_events.event_type='migrated_from_v0_9'`,payload 含原 `task_status`
- **幂等**:`INSERT IGNORE` + `UNIQUE(task_id, module_key)`;事件表可追加 `rebackfill_noop`

#### Phase B — `task_assets.source_module_key` / `source_task_module_id` 回填(v1.2 · 5 值)

- 启动前必跑自检:`SELECT asset_type, COUNT(*) FROM task_assets GROUP BY asset_type`
  - 若出现 **不在 {`reference`, `source`, `delivery`, `design_thumb`, `preview`} 5 值集合** 的 asset_type → **立即 abort**,写日志 `unknown asset_type: <value>, count=<n>`,退出码 3,要求更新资产 §6.1 再重跑
  - 若出现 036 之前的旧值(`original / draft / revised / final / outsource_return`)→ 提示先跑 036 再 abort
- 推断规则(按资产 §6.1 v1.2 表格,优先级从上到下):

```
asset_type = 'reference'                         → basic_info
asset_type IN ('design_thumb', 'preview')        → design     -- R1.6 新增
asset_type IN ('source','delivery') ∧ customization_required=1 → customization
asset_type IN ('source','delivery') ∧ task_type LIKE '%retouch%' → retouch
asset_type IN ('source','delivery') 其它          → design
```

- 用 `(task_id, source_module_key)` 在 `task_modules` 查 id,写 `source_task_module_id`;模块不存在则创建 `state='closed', data.backfill_placeholder=true` 占位,写 `task_module_events.event_type='backfill_placeholder'`
- **真生产预期产出**(对 `jst_erp` 264 条 task_assets):

  | source_module_key | 预期行数 | 来源 |
  | --- | --- | --- |
  | `basic_info` | 9 | `reference` |
  | `design` | 70(design_thumb + preview)+ ≈168(source/delivery 非定制非精修) | |
  | `customization` | ≈17 | 9 条定制任务的 source/delivery |
  | `retouch` | 0 | |

  Phase B 完成后 `COUNT(*) FROM task_assets WHERE source_module_key IS NULL` 必须 = 0,否则 exit code 2

#### Phase C — `reference_file_refs` 展平插入

按资产 §6.3 算法:

1. 流式 SELECT `task_id, reference_file_refs_json FROM task_details`(`task_details.reference_file_refs_json` 非空的行在生产为 80/95,批大小 1000)
2. JSON parse;对每个元素:
   - 取 `ref_id`(兼容 `asset_id` 旧字段)
   - 查 `asset_storage_refs.owner_type` 推 `owner_module_key`(按资产 §3.3 映射表)
   - `INSERT IGNORE INTO reference_file_refs (task_id, sku_item_id=NULL, ref_id, owner_module_key, context=ref.source, attached_at=NOW())`
3. 同样处理 `task_sku_items.reference_file_refs_json`(生产 1/78 非空,sku_item_id 取 `task_sku_items.id`)
4. JSON 解析失败 → 写 `task_module_events.event_type='backfill_error'` 事件,跳过不中断
5. 完成后一致性 SELECT:`COUNT(*)` FROM 展平表 vs JSON 中 unique ref 总数,允差 < 0.5%,否则 exit code 2

#### Phase D — `tasks.priority` 枚举校验(v1.2 · 4 值)

- **不加新列**
- 本 phase 扫 `tasks` 中 `priority NOT IN ('low','normal','high','critical')` 的行
  - **v1.2 真生产分布**:`low=56 / high=20 / normal=19`,共 95 行,**全部命中** 4 值枚举 → 预期修正数 = 0
  - 若出现第 5 值 → 写日志 + `task_module_events.event_type='backfill_priority_out_of_range'`,payload 含原值;**不改写**(与 Phase B 一致的 abort 风格),退出码 3 要求人工确认
- CHECK 约束由 067 加;本 phase 只保证数据满足约束
- **严禁**:读或写 `is_urgent` / `task_priority`

#### Phase E — `task_customization_orders` / 其他新表初始化

- `task_customization_orders`:对 `tasks.customization_required=1 AND task_type='customer_customization'` 插入空壳行
  - **真生产**:`customer_customization` 任务数 = 0(`original_product_development + customization_required=1` 为 9 条,不属 customer_customization)→ 预期插入 0 行
- `task_drafts` / `notifications` / `org_move_requests`:v1 新功能,无历史数据,不回填

### 2.4 Smoke Test(Go integration test)

`cmd/tools/migrate_v1_backfill/smoke_test.go`(build tag `integration`):

最低 5 个断言:

1. 每条 `tasks` 行 backfill 后存在 `task_modules` 中 `module_key='basic_info'` 一条(100% 覆盖 · 真生产 95 条 → 95 条)
2. 每条 `task_status='PendingAuditA'` 的任务存在 `audit` 模块(真生产 9 条)
3. 所有 `task_assets.source_module_key` 非空(真生产 264 条)
4. `reference_file_refs` 展平表 `COUNT(*)` ≥ JSON 长度之和 × 0.995(真生产 ≈80+ 条)
5. 重跑 backfill 一次,所有表行数不变(幂等验证)

smoke test 接受 `MYSQL_DSN`;缺失时 `t.Skip`,退出码 0。

### 2.5 OpenAPI 瘦身

v2 已完成;**v3 不再处理 OpenAPI**。

## 3. 严禁触碰(DO NOT TOUCH)

- `service/**` `transport/**` `repo/**` `domain/**` 任何现有代码(R3 负责)
- `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` `docs/api/openapi.yaml`
- `db/migrations/001~058`
- R1 冻结 501 响应
- 生产数据 UPDATE(除 Phase A~E 明确定义的 INSERT;**严禁** 归一化 UPDATE 历史任务的 `priority` / `asset_type`)

## 4. 执行环境(R1.6 · 真生产 SSH 模式)

### 4.1 目标库

- 主机:`jst_ecs`(`~/.ssh/config` alias,已配免密)
- 库:`jst_erp`
- 用户 / 密码 / DSN:**从服务器 `/root/ecommerce_ai/shared/main.env` 里读 `DB_HOST / DB_PORT / DB_USER / DB_PASS / DB_NAME`**,不要在本地或提交代码里写出
- MySQL 版本:`8.0.45`(满足 067 CHECK 约束的 ≥ 8.0.16)

### 4.2 为什么不用 Docker(R1.6 决策链)

- R2 v2 最初想用本地 Docker,但 Codex / 本机 Docker 环境不稳(daemon 未启动 / pull 卡住)
- R1.6 直接在 `jst_ecs` 探测确认 · 数据规模仅 95 条 · 真实口径比合成种子准得多
- 决策 P1:生产规模小,Docker 合成种子的价值 < SSH 直连;直连效率、对齐度、说服力都更强

### 4.3 执行模板(Codex 必须仿照)

Codex 所有 DB 执行走两种形态之一,**禁止** 把 DSN / 密码硬编码到 Go 源码:

**形态 1:服务器执行 Go 工具**(forward / backfill / rollback 正式跑)

```bash
# 1. 本地 build linux 二进制
GOOS=linux GOARCH=amd64 go build -o /tmp/r2_forward  ./cmd/tools/migrate_v1_forward
GOOS=linux GOARCH=amd64 go build -o /tmp/r2_backfill ./cmd/tools/migrate_v1_backfill
GOOS=linux GOARCH=amd64 go build -o /tmp/r2_rollback ./cmd/tools/migrate_v1_rollback

# 2. scp 三份二进制 + 对应 SQL 到 jst_ecs
scp /tmp/r2_forward /tmp/r2_backfill /tmp/r2_rollback jst_ecs:/root/ecommerce_ai/r2/bin/
scp db/migrations/059_*.sql ... db/migrations/068_*.sql jst_ecs:/root/ecommerce_ai/r2/sql/

# 3. 服务器上跑
ssh jst_ecs 'cd /root/ecommerce_ai && . ./shared/main.env && \
  DSN="${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/${DB_NAME}?parseTime=true&multiStatements=true" && \
  /root/ecommerce_ai/r2/bin/r2_forward --dsn="$DSN" --sql-dir=/root/ecommerce_ai/r2/sql'
```

**形态 2:纯只读 SQL 查询**(Phase 自检 / 最终对账)

```bash
ssh jst_ecs 'bash -s' <<'EOF'
cd /root/ecommerce_ai && . ./shared/main.env
export MYSQL_PWD="$DB_PASS"
mysql -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" "$DB_NAME" -N -B -e "SELECT ..."
EOF
```

### 4.4 强制备份前置

- forward 执行前:`ssh jst_ecs 'mysqldump --single-transaction --databases $DB_NAME > /root/ecommerce_ai/backups/<ts>_r2_pre_forward.sql.gz'`
- backfill 执行前:同上,路径 `<ts>_r2_pre_backfill.sql.gz`
- dump 文件 > 0 字节 + gzip 可解 → 才允许进入下一步;失败即 abort

### 4.5 生产不跑 rollback

- **真生产只验证 forward + backfill 正向链**
- rollback **默认 `--dry-run`**:SSH 跑一次 dry-run 打印"将要执行的逆序 SQL 列表",不实际 DROP
- 若真要回退:必须人工 + 全量 dump 前置 + 明确书面确认;**不在本轮 Codex 流程内完成**

## 5. 验收脚本(10 步,Codex 必须跑通)

```bash
# 0. 环境检查
ssh jst_ecs 'hostname && mysql --version'
#   断言:hostname 含 jst,MySQL ≥ 8.0.16

# 1. 编译 (本地)
go build ./...
#   断言:退出 0

# 2. 生产现状只读探测 (R1.6 script 复跑)
scp tmp/r2_probe_readonly.sh jst_ecs:/tmp/r2_probe_pre.sh
ssh jst_ecs 'bash /tmp/r2_probe_pre.sh' > docs/iterations/r2_probe_pre.log
#   断言:log 中 asset_type 5 值分布 / priority 3 值分布 / 目标表 count=0 与 R1.6 §1 一致

# 3. 全量备份
ssh jst_ecs 'cd /root/ecommerce_ai && . ./shared/main.env && \
  mkdir -p backups && \
  mysqldump --single-transaction --databases "$DB_NAME" | gzip > backups/$(date +%s)_r2_pre_forward.sql.gz'
#   断言:文件 > 1MB

# 4. Forward: 059~068
# (按 §4.3 形态 1 执行)
#   断言:SHOW TABLES 包含 task_modules / task_module_events / reference_file_refs / task_drafts / notifications / org_move_requests / task_customization_orders;task_assets 有 5 个新列 + cleaned_at / deleted_at;tasks 有 idx_tasks_priority_created 索引 + CHECK(priority IN ...)

# 5. Backfill · 第一次
#   断言:Phase A~E stats 打印,errors=0,Phase B unknown_asset_type=0,Phase D out_of_range=0

# 6. Backfill · 第二次(幂等)
#   断言:所有表行数 ± 0

# 7. Smoke test (在本地开发机用同一 DSN 跑 integration test)
MYSQL_DSN='...' go test ./cmd/tools/migrate_v1_backfill/... -tags=integration -count=1 -v
#   断言:5 断言 PASS

# 8. Rollback dry-run (仅打印,不执行)
ssh jst_ecs '/root/ecommerce_ai/r2/bin/r2_rollback --dsn="$DSN" --dry-run'
#   断言:打印出 068 → 059 的逆序 SQL,全部非空;退出 0

# 9. 生产现状探测(对比 pre/post)
ssh jst_ecs 'bash /tmp/r2_probe_pre.sh' > docs/iterations/r2_probe_post.log
diff docs/iterations/r2_probe_pre.log docs/iterations/r2_probe_post.log
#   断言:R2 目标表从 0 变非 0;其他分布不变(tasks / task_assets / task_details 行数一致;asset_type / priority 分布完全一致)
```

**性能门槛**(R1.6 · P1):

- 生产规模 95 tasks / 264 task_assets / 95 task_details
- Phase A~E 总 duration ≤ **10s**;> 10s 视为 fail
- **不需要** 10w 合成种子

## 6. 交付物清单

1. 10 份 migration SQL(§2.1)
2. 3 个 tool Go 包 + 1 个共享 internal lib(§2.2 · 含 `--cleanup-partial` 和 `--dry-run`)
3. 1 份 integration smoke test(§2.4)
4. **不再** 交付 `docs/testdata/r2_seed.sql`(v2 要求;P1 后取消 · 生产真数据即权威)
5. 1 份 `docs/iterations/V1_R2_REPORT.md`

**报告强制章节**:

- `## Backfill Stats`:Phase A~E 各自 processed / generated / warnings / errors / duration,真生产数据
- `## Performance`:真生产 95 tasks 的 total_duration(秒级)
- `## Rollback Verification`:dry-run 打印的逆序 SQL 清单;**不做** 真 rollback
- `## Smoke Results`:5 断言 PASS/FAIL 证据
- `## Probe Diff`:pre/post 两次 probe 对比,证明非目标表零改动
- `## Backup Evidence`:两次 mysqldump 文件路径 + 大小 + sha256

## 7. 对下游的交接约束

### 7.1 给 R3

- `task_modules.state` / `task_module_events.event_type` 取值域必须在 R2 报告附录枚举,R3 1:1 使用
- `reference_file_refs` 展平表 + 两个 JSON 列双写由 R3 落地
- `tasks.priority` CHECK 已上线 4 值,R3 写入必须落枚举

### 7.2 给 R4

- `task_customization_orders`:R4 客户定制创建 handler INSERT 本表
- `notifications`:R4 订阅 `task_module_events` 推送
- `org_move_requests`:R4 落地端点

## 8. 已知非目标(明确不做)

- 任何 R3 业务逻辑
- frontend 文件(R5)
- 改 R1 冻结的 501 路径
- 改现有 service / repo
- 动 `customization_jobs` / `customization_pricing_rules`
- 删两个 JSON 参考图列(保留到 R6-slim)
- 10w 合成种子压测(R1.6 · P1 取消)
- **UPDATE 历史 `priority` 值**(R1.6 · Z1 不做归一化)
- **UPDATE 历史 `asset_type` 值**(R1.6 · Y1 不做归一化)

## 9. 失败终止条件(Codex 必须主动 abort)

- 无法 SSH 到 `jst_ecs`(`ssh -o BatchMode=yes jst_ecs true` 退出非 0)
- `go build ./...` 首次失败 → R1 回归,R2 无权修
- `task_assets.asset_type` 出现 5 值集合外的值(R1.6 · Y1 abort)
- `tasks.priority` 出现 4 值集合外的值(R1.6 · Z1 abort)
- 036 之前的旧 asset_type 残留
- 067 CHECK 在当前版本不支持(< 8.0.16)
- 备份 dump < 1MB 或 sha256 失败
- 任一 Phase 耗时 > 60s(性能门槛 10x 上限)
- smoke test 任一断言 FAIL

---

## 10. 给 Codex 的最后一句话

> **老老实实在真生产 `jst_erp` 上跑**。
> v1 被字段幻觉回退;v2 想用 Docker 合成种子被生产规模打回;
> v3 的路径是:**真库 + 真数据 + 只加结构 + 零 UPDATE + 强备份 + 强探测**。
> 遇到与本文冲突的点,**立即 abort**,写日志,报告。不要猜,不要 fallback,不要在生产上 UPDATE 历史业务字段。

---

## 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1 | 2026-04-23 | 初版(已被全量回退) |
| v2 | 2026-04-17 | R1.5 签字后重写(Docker 执行 + 10w 性能门槛) |
| v3 | 2026-04-17 | **R1.6 真生产对齐**:改真库 `jst_erp` + SSH 执行模板(`jst_ecs`)+ `mysqldump` 强制备份前置;067 CHECK 改 **4 值** `low/normal/high/critical`(Z1);Phase B 加 **5 值** asset_type 规则(Y1 · `design_thumb`+`preview` → `design`);未知值统一 abort,不静默 fallback;性能门槛降为生产 95 行 ≤ 10s(P1),移除 10w seed 要求;backfill 加 `--cleanup-partial`,rollback 加 `--dry-run` 默认策略;验收脚本从 8 步 → 10 步,加前后两次 probe 对比 + 备份证据。依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md` · 配套文档 `V1_MODULE_ARCHITECTURE.md` v1.2、`V1_ASSET_OWNERSHIP.md` v1.2 |
