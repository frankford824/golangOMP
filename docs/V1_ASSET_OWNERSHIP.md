# V1 资产归属、版本锚定与生命周期

> 状态: **v1.2 · 真生产对齐已签字生效**(2026-04-17,依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md` v1.0);前置版本 v1.1 已签字生效(2026-04-17)
> 最终签字版本:Draft v1(资产归属 / 版本锚定 / 生命周期 5 态机 / 跨任务资产中心契约)
> v1.2 变更(R1.6):§6.1 `source_module_key` 推断规则追加 2 条 `design_thumb / preview → design` 映射(真生产 `jst_erp` 这两个 asset_type 占 26.5%,v1.1 未覆盖);未知 asset_type 从"兜底到 design"改为"**abort + 写 error 事件**",避免未来新枚举值被悄悄兜底。依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md`。
> v1.1 变更:§3 整章重写(`reference_file_refs` 不是表,是 JSON TEXT 列;本轮新建展平表);§6.1 不再依赖幻觉字段 `task_assets.flow_stage`,改为基于 `asset_type × tasks.task_type × customization_required` 的推断;§6.3 重写为 JSON 解析 → 展平表插入。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md`。
> 主次关系: 本文与 `V1_MODULE_ARCHITECTURE.md` 冲突时以主文档为准;主文档未覆盖的资产领域细节由本文补齐。
> 读者: 后端研发、前端研发、运维(OSS 生命周期部分)

---

## 1. 核心约定

1. **全局 `task_assets` + `source_module_key`**:不拆分子表,每个资产通过 `source_module_key` 标识它属于哪个模块。
2. **云仓无独立资产**:`warehouse` 模块只引用上游模块(`design` / `retouch` / `customization`)的资产,不产生新资产。
3. **审核引用版本 = always latest**:`audit` 模块**不冻结** `task_assets` 的版本号;但审核事件 `task_module_events.payload` **必须冻结审核当时的 `asset_version_id` 快照**,用于审计追溯。
4. **OSS 生命周期**:任务终态后 **365 天自动清理**;SuperAdmin 可通过**资产管理中心**手动归档/删除。
5. **参考图归属模块(v1.1 重构)**:v0.9 里 `reference_file_refs` 不是独立表,而是两个 JSON TEXT 列(`task_details.reference_file_refs_json` / `task_sku_items.reference_file_refs_json`)+ `asset_storage_refs` 表(020 引入,owner_type 已含 `task_create_reference | audit_reference | customization_reference` 等分类)。**R2 新建 `reference_file_refs` 展平表**,每条 JSON 元素对应一行,带 `owner_module_key`;挂载点按 §3.3 规则映射为 `basic_info | audit | customization`。两个 JSON 列保留到 R6-slim 作兼容。
6. **资产管理中心跨任务可见**:所有登录用户可浏览、下载,但**删除权限仅 SuperAdmin**。

---

## 2. `task_assets` 表扩展

### 2.1 新增列

| 列 | 类型 | 说明 | 默认 |
| --- | --- | --- | --- |
| `source_module_key` | varchar(32) NOT NULL | `design` / `audit` / `warehouse` / `customization` / `procurement` / `retouch` / `basic_info` | backfill 期间按 §6.1 推断规则填充 |
| `source_task_module_id` | bigint NULL | 指向 `task_modules.id`,强关联 | backfill 期间按 `(task_id, source_module_key)` 唯一键填充 |
| `is_archived` | tinyint(1) NOT NULL | 是否被"归档"(不等于物理删除) | 0 |
| `archived_at` | datetime NULL | — | NULL |
| `archived_by` | bigint NULL | 执行归档的 SuperAdmin user_id | NULL |
| `cleaned_at` | datetime NULL | 365 天清理 job 执行的时刻(OSS 对象已删) | NULL |
| `deleted_at` | datetime NULL | 软删除标记 | NULL |

> v1.1 注:共新增 7 列。R2 迁移拆成两份:`061_v1_0_task_assets_source_module_key.sql`(前 5 列 + FK)+ `066_v1_0_asset_lifecycle_state.sql`(cleaned_at / deleted_at + 索引);号段及落地顺序见 `prompts/V1_R2_DATA_LAYER.md` v2。

### 2.2 保留现有列语义

- `version_no`:模块内版本号,保持单调递增
- `storage_key`:OSS 对象 key
- `download_url_expires_at`:预签名缓存(read-enricher 逻辑保留)
- `uploader_id` / `uploader_name`:上传人快照,沿用 Round W-2 产物

### 2.3 模块版本线(每模块独立)

| module_key | 版本线存在? | 版本号范围 | 说明 |
| --- | --- | --- | --- |
| `basic_info` | 否 | — | `basic_info` 本身不产出 asset,只挂 reference 类资产(见 §3) |
| `design` | 是 | 1..N | 设计师每次上传新版本 +1 |
| `retouch` | 是 | 1..N | 精修组独立版本线 |
| `customization` | 是 | 1..N | 定制美工独立版本线,含沟通稿→终稿 |
| `audit` | 否 | — | audit 不上传资产(v0.9 约定继续生效),只引用 |
| `warehouse` | 否 | — | 无独立资产 |
| `procurement` | 否 | — | v1 内无资产产出(采购单是业务数据不是"资产");若未来需要入库凭证,列为 R7+ ADR |

---

## 3. 参考图归属(`reference_file_refs` 展平表,v1.1 重构)

### 3.1 v0.9 现状(审计结论)

参考图在 v0.9 并无独立表,分布在三处:

| 位置 | 角色 | 字段 |
| --- | --- | --- |
| `task_details.reference_file_refs_json` TEXT(migration 042) | 任务级参考图 JSON 数组 | 每元素为 `domain.ReferenceFileRef`(`asset_id / ref_id / filename / mime_type / file_size / source / storage_key / ...`) |
| `task_sku_items.reference_file_refs_json` TEXT(`tmp/release_v09_schema_guarded_patch.sql`) | SKU 级参考图 JSON 数组 | 同上 |
| `asset_storage_refs`(表,migration 020) | 参考图真正的 OSS 元数据 | `ref_id PK / owner_type / owner_id / ref_key / file_name / mime_type / file_size / status / ...`;`owner_type` 取值已涵盖 `task_create_reference | customization_reference | audit_reference | ...` |

因此"把参考图归属到某个 `module_key`"的真实承载点是 `asset_storage_refs.owner_type`,但它与 v1 的 `module_key` 枚举不完全对齐(前者按业务场景,后者按模块)。v1.1 选定方案:**新建展平表**,把两个 JSON 列内容与 `asset_storage_refs` 元数据组合成一行一条引用。

### 3.2 新表定义(R2 新建)

```sql
CREATE TABLE reference_file_refs (
  id                BIGINT      NOT NULL AUTO_INCREMENT,
  task_id           BIGINT      NOT NULL COMMENT 'FK tasks.id',
  sku_item_id       BIGINT      NULL     COMMENT 'FK task_sku_items.id; NULL = 任务级引用',
  ref_id            VARCHAR(64) NOT NULL COMMENT 'FK asset_storage_refs.ref_id',
  owner_module_key  VARCHAR(32) NOT NULL COMMENT 'basic_info | audit | customization',
  context           VARCHAR(64) NULL     COMMENT '保留原 JSON 元素的 source 字段语义,用于审计追溯',
  attached_at       DATETIME    NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uq_reference_file_refs_task_ref_sku (task_id, ref_id, sku_item_id),
  KEY idx_reference_file_refs_owner_task (owner_module_key, task_id),
  KEY idx_reference_file_refs_ref_id (ref_id),
  CONSTRAINT fk_reference_file_refs_task FOREIGN KEY (task_id) REFERENCES tasks (id),
  CONSTRAINT fk_reference_file_refs_ref  FOREIGN KEY (ref_id)  REFERENCES asset_storage_refs (ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**不重复存储** `filename / mime_type / file_size / storage_key`,这些数据继续由 `asset_storage_refs` 作为权威源提供;展平表仅持有"这条引用挂在哪个任务 / 哪个 SKU / 归属哪个模块"的路由信息。

### 3.3 `owner_module_key` 映射规则

R2 backfill 按如下优先级推断(逐行参考图展平后):

| 来源组合 | owner_module_key |
| --- | --- |
| 来自 `task_details.reference_file_refs_json` ∧ `asset_storage_refs.owner_type='task_create_reference'` | `basic_info` |
| 来自 `task_sku_items.reference_file_refs_json` | `basic_info`(SKU 级也归 basic_info,由 `sku_item_id` 区分) |
| `asset_storage_refs.owner_type='customization_reference'`(含客户定制附件)| `customization` |
| `asset_storage_refs.owner_type='audit_reference'`(审核员挂图) | `audit` |
| `asset_storage_refs.owner_type` 其它值(如 `task_create_reference` 以外) ∧ 任务定制:`tasks.customization_required=1` | `customization` |
| 以上都不命中 | `basic_info` + `backfill_warning`(写 `task_module_events`) |

> v1.1 更正:Draft v1 里的"`context=audit_reference`(v0.9 `056` migration 引入)"是幻觉,migration 056 实际是定制评审参考价字段(`review_reference_unit_price` DECIMAL),与审核参考图无关。审核参考图真身在 `asset_storage_refs.owner_type='audit_reference'` 的记录中(由 service 层下发,无独立 DDL)。

### 3.4 双写与读策略(R3 handler 必须遵守)

- **R2 完成后**:两个 JSON 列 + `reference_file_refs` 展平表**并存**;展平表由 backfill 初始化。
- **R3 handler 写入策略**:新增 / 删除参考图时,必须**同时**更新两个 JSON 列和展平表(事务内双写),保证 R4/R5 前端切到展平表 API 时数据一致。
- **R3 handler 读取策略**:列表 / 详情 API **仅查展平表**(按 owner_module_key 分组),不再解析 JSON 列。
- **R6-slim**:删除两个 JSON 列 + 展平表对应的填充代码,仅留展平表。

### 3.5 可见性

- 前端详情页按 `owner_module_key` 分组渲染到对应模块卡片
- Layer 2 作用域:参考图**可见性等同于所在模块**(`basic_info` 的参考图全员可见;`audit` 参考图按 audit 模块 scope 判定;`customization` 同理)
- 下载仍走 read-time presigning,从 `asset_storage_refs.ref_id` 派生预签名 URL(沿用现有 `service/reference_file_refs_download_enricher.go` 逻辑,enricher 新增按 `reference_file_refs.ref_id` 批量 enrich 的路径)

---

## 4. 审核引用版本的锚定规则(关键)

### 4.1 Why always-latest

- 设计师在审核阶段仍可能临时追加一版修复
- 审核员看到的**永远是最新资产版本**,符合业务直觉
- 若选 snapshot,会导致"审核页看到的图和设计列表不一致"的误解(v0.9 已有 UAT 反馈)

### 4.2 审计追溯靠事件不靠主表

`task_module_events` 针对 audit 模块强制记录:

```
event_type = claimed | approved | rejected | ...
payload = {
  "asset_versions_snapshot": [
    { "asset_id": 123, "version_id": 456, "version_no": 3, "storage_key": "oss://..." },
    ...
  ],
  "snapshot_taken_at": "2026-04-15T10:00:00Z"
}
```

- **每次** audit 动作触发时都必须写入 `asset_versions_snapshot`(不只是 approve/reject)
- 事件流只追加不修改,作为法律级追溯证据
- 归档 / 删除资产时**不得**触及已经发生的事件 payload

### 4.3 前端呈现

- 当前审核页:显示"当前资产版本 = vN(latest)"
- 审核历史记录页:每条 audit 事件旁显示"审核当时快照 = vM",点击可下载该 version
- 如果 vM 已被清理(见 §7 OSS 生命周期),返回 410 GONE + 前端降级显示"该版本资产已清理"

---

## 5. 资产管理中心(全局跨任务)

### 5.1 定位

一级菜单"资产管理中心",**所有登录用户可见**。本轮新建跨任务 API,取代现有仅任务级的 `/v1/tasks/{id}/asset-center/*`(**后者保留不下线**,作为任务详情页内入口)。

### 5.2 新增 API(R2)

| 端点 | 说明 | 最低角色 |
| --- | --- | --- |
| `GET /v1/assets/search?keyword=&module_key=&owner_team_code=&created_from=&created_to=&page=&size=` | 跨任务检索资产 | 登录用户 |
| `GET /v1/assets/{asset_id}` | 资产详情(含版本列表) | 登录用户 |
| `GET /v1/assets/{asset_id}/download` | 最新版本下载(预签名) | 登录用户 |
| `GET /v1/assets/{asset_id}/versions/{version_id}/download` | 特定版本下载 | 登录用户 |
| `POST /v1/assets/{asset_id}/archive` | 归档(is_archived=1,保留 OSS 对象) | SuperAdmin |
| `POST /v1/assets/{asset_id}/restore` | 取消归档 | SuperAdmin |
| `DELETE /v1/assets/{asset_id}` | 硬删除(清除 OSS + 逻辑删除 DB 行) | SuperAdmin |

### 5.3 过滤参数

- `keyword`:模糊匹配 `file_name`、任务号、任务标题(联合索引)
- `module_key`:限定模块
- `owner_team_code`:限定归属组
- `is_archived`:默认 0,可选 true/false/all
- `task_status`:仅 `open` / `closed` / `archived` / `all`

### 5.4 权限策略

- **浏览 / 下载**:所有登录用户(与任务列表 Layer 1 一致)
- **归档 / 恢复 / 删除**:仅 `SuperAdmin` 角色
- 删除动作必须带 `reason`(写入 `task_module_events.event_type=asset_deleted_by_admin`)

---

## 6. 旧数据迁移

### 6.1 `source_module_key` 回填(v1.2 · R1.6 扩规则)

> **Draft v1 里基于 `task_assets.flow_stage` 的规则已废**:该列在仓库 004/020/034/035/036/049 全部迁移中从未出现,是幻觉字段。真实信号组合是 `task_assets.asset_type × tasks.task_type × tasks.customization_required`。
>
> **v1.2 真生产对齐**:在 `jst_erp` 生产库上 `SELECT asset_type, COUNT(*) FROM task_assets GROUP BY asset_type` 发现 5 种值:`delivery(138) / source(47) / design_thumb(35) / preview(35) / reference(9)`。`design_thumb + preview` 共 26.5%,在 v1.1 规则里未映射。v1.2 将其补入规则,并把未知值从"兜底"改为"abort",防止未来新枚举被悄悄吞掉。

推断规则(按行扫描 `task_assets`,优先级从上到下取首条命中):

| 条件 | source_module_key | 归属理由 |
| --- | --- | --- |
| `asset_type = 'reference'` | `basic_info` | 任务创建参考图 |
| **`asset_type IN ('design_thumb','preview')`** | **`design`** | 设计环节产出的缩略图 / 预览(v1.2 新增) |
| `asset_type IN ('source','delivery')` ∧ `tasks.customization_required = 1` | `customization` | 定制成品 |
| `asset_type IN ('source','delivery')` ∧ `tasks.task_type LIKE '%retouch%'` | `retouch` | 精修任务 |
| `asset_type IN ('source','delivery')` ∧ 其它 | `design` | 常规设计 |
| **其它未知 asset_type** | **abort backfill + 写 `task_module_events.event_type='asset_backfill_unknown_type'`** | v1.2 加硬:**不做兜底**,强制人工确认后再跑 |

> R2 backfill 执行前必须先 `SELECT DISTINCT asset_type` 校验生产现状:命中 {`reference`, `source`, `delivery`, `design_thumb`, `preview`} 5 值以内则继续;出现第 6 值立即 early-abort,要求更新本规则表再重跑。

### 6.1a 生产真值快照(v1.2 · R1.6 固化)

`jst_erp` 2026-04-17 真值分布(用于 Codex 执行前自检):

```
SELECT asset_type, COUNT(*) FROM task_assets GROUP BY asset_type;
delivery       138
source          47
design_thumb    35
preview         35
reference        9
```

Phase B 执行后,基于上述分布预期生成:

| source_module_key | 预期行数(生产) | 来源 |
| --- | --- | --- |
| `basic_info` | 9 | `reference` |
| `design` | 70 + ≈170 | `design_thumb` + `preview` + `source`/`delivery` 中非定制非精修部分 |
| `customization` | ≈9 × 2 = 18 | 基于 9 条 `customization_required=1` 任务的 source/delivery |
| `retouch` | 0 | 生产至今无 retouch 任务 |

若 Phase B 完成后 `COUNT(*) FROM task_assets WHERE source_module_key IS NULL` > 0,视为失败。

### 6.2 `source_task_module_id` 回填

- backfill 阶段先建好 `task_modules` 实例(参见主文档 §11.2)
- 再用 `(task_id, source_module_key)` 关联回 `task_modules.id`
- 若找不到对应模块实例(例如旧任务有 design asset 但无设计阶段),降级创建一条 `module_key=design, state=closed` 的占位模块,标 `data.backfill_placeholder=true`

### 6.3 参考图 backfill(v1.1 重写 · 核心)

由于 `reference_file_refs` 在 v0.9 不是表(§3.1),backfill 必须解析 JSON 并展平插入新表。算法:

```
-- 阶段 C(主文档 §11.2 的 Phase C,已重写)
-- 输入:task_details.reference_file_refs_json,task_sku_items.reference_file_refs_json
-- 输出:reference_file_refs 展平表
-- 依赖:asset_storage_refs 已存在(020 建表),必读其 owner_type 推断 owner_module_key

FOR EACH row IN task_details:
    arr = JSON_EXTRACT(reference_file_refs_json, '$[*]')
    FOR EACH ref IN arr:
        ref_id = ref.ref_id || ref.asset_id           -- 兼容旧记录用 asset_id
        storage_ref = SELECT owner_type FROM asset_storage_refs WHERE ref_id = ?
        owner_module_key = MAP_OWNER_TYPE(storage_ref.owner_type, tasks.customization_required)
        INSERT IGNORE INTO reference_file_refs
          (task_id, sku_item_id, ref_id, owner_module_key, context, attached_at)
          VALUES (task_details.task_id, NULL, ref_id, owner_module_key, ref.source, ref.attached_at || NOW())

FOR EACH row IN task_sku_items WHERE reference_file_refs_json IS NOT NULL:
    -- 同上,但 sku_item_id = task_sku_items.id

-- MAP_OWNER_TYPE() 映射表见 §3.3
```

校验规则:

- 所有展平行的 `ref_id` 必须能在 `asset_storage_refs` 找到,否则写 `backfill_warning` 事件并跳过该行
- JSON 解析失败(非法 JSON)写 `backfill_error` 事件并跳过整条 task_details 行,不中断批次
- 迁移后**必跑**一致性校验:`COUNT(DISTINCT ref_id) FROM reference_file_refs` 应 ≈ `COUNT` of unique refs in both JSON columns(允差 < 0.5%,差额归因于非法 JSON 或孤儿 ref_id)

R4/R5/R6 handler 必须按 §3.4 双写规则维护两侧数据,直到 R6-slim 删除 JSON 列。

---

## 7. OSS 生命周期

### 7.1 状态机

```
active ──(task.closed)──▶ closed_retained ──(+365d)──▶ auto_cleaned
  │                           │
  │                           │
  └─(SuperAdmin archive)──▶ archived ──(SuperAdmin delete)──▶ deleted
```

| 状态 | DB 标记 | OSS 对象 | 下载 |
| --- | --- | --- | --- |
| `active` | `is_archived=0, archived_at=null` | 存在 | 可下载 |
| `closed_retained` | 任务已 closed/archived 但资产未到 365 天 | 存在 | 可下载 |
| `archived` | `is_archived=1` | 存在 | 可下载(但搜索默认过滤,需显式 `is_archived=true`) |
| `auto_cleaned` | `is_archived=1, storage_key=null, cleaned_at=NOW()` | 已删除 | 410 GONE |
| `deleted` | 软删除 `deleted_at IS NOT NULL` | 已删除 | 404 |

### 7.2 自动清理 Job(R6 落地)

- 每日 03:00 扫描 `task_assets` 中所属任务满足:
  - `task.status in (closed, archived, cancelled)` ∧ `task.terminal_at < NOW() - INTERVAL 365 DAY`
- 批量执行:
  1. 调用 OSS DeleteObject
  2. 更新 `task_assets.storage_key=NULL, is_archived=1, cleaned_at=NOW()`
  3. 写事件 `task_module_events.event_type=asset_auto_cleaned`(归属在资产 `source_task_module_id`)
- 参考图同步处理

### 7.3 手动归档 / 删除

- 资产管理中心 SuperAdmin 入口
- 归档:仅标记,不删除 OSS(为了紧急"取消归档"可恢复)
- 删除:同时清 OSS 和 DB 软删除
- 两者都必填 `reason`,写审计事件

### 7.4 事件 payload 里引用已清理资产的兼容

- `task_module_events.payload.asset_versions_snapshot` 不受清理影响(只是 storage_key 失效)
- 前端收到 410 时降级展示"该版本资产已按保留策略清理,归档时间 XXX"

---

## 8. 下载权限策略(不变但明确)

- **任务内部**:通过任务详情页进入,按 Layer 2 模块作用域判断可见性,但**下载不门控**(Layer 1 可见即可下载,与浏览一致)
- **资产管理中心**:直接下载,与 §5.4 一致
- **外部链接分享**:通过预签名 URL 实现,URL 有效期默认 1 小时(沿用 Round T 语义),到期自动失效

---

## 9. 验收标准(本子文档追加)

| 编号 | 验收项 | 关联章节 |
| --- | --- | --- |
| AS-A1 | `task_assets` 存在 `source_module_key` 列,所有资产 NOT NULL | §2.1 / §6 |
| AS-A2 | `reference_file_refs` 展平表已创建;两个 JSON 列里的 ref 100% 展平入表(允差 < 0.5%),每行 `owner_module_key` 按 §3.3 规则非空 | §3 / §6.3 |
| AS-A2b | R3 handler 对展平表和两个 JSON 列执行事务内双写;读路径仅查展平表 | §3.4 |
| AS-A3 | audit 的每条事件 payload 均含 `asset_versions_snapshot` | §4.2 |
| AS-A4 | 资产管理中心 `GET /v1/assets/search` 对所有登录用户返回 200,`DELETE /v1/assets/{id}` 对非 SuperAdmin 返回 403 | §5 |
| AS-A5 | 365 天清理 job 在 staging 演练:生成 1000 条过期资产,job 执行后 OSS 对象全部删除,DB `storage_key=NULL` | §7.2 |
| AS-A6 | 已清理资产的下载请求返回 410 GONE | §7.4 |
| AS-A7 | audit 事件 payload 中 `asset_versions_snapshot` 在资产清理后仍可读取原 snapshot 字段 | §7.4 |

---

## 10. 变更记录

| 版本 | 日期 | 变更 | 签字 |
| --- | --- | --- | --- |
| Draft v1 | 2026-04-17 | 初稿;合并 Q3 + C5 + C6 决策;FE Plan 评审未触及本文(资产契约前后端一致) | **已签字**(2026-04-17) |
| v1.1 | 2026-04-17 | **R1.5 DDL 对齐**:§1.5 更正"参考图现状";§2.1 补 `cleaned_at / deleted_at` 列 + 说明迁移拆分;§3 整章重写(`reference_file_refs` 展平表方案 · 方案 B)+ §3.4 双写双读规则;§6.1 `flow_stage` 规则废除,改为 `asset_type × task_type × customization_required`;§6.3 重写为 JSON 解析 → 展平表插入算法 + 一致性校验。依据 `docs/iterations/V1_R1_5_DDL_ALIGNMENT.md` | **已签字**(2026-04-17) |
| v1.2 | 2026-04-17 | **R1.6 真生产对齐**:§6.1 追加 `design_thumb / preview → design` 2 条映射规则(生产 26.5% 资产原 v1.1 未覆盖);未知 asset_type 从"兜底到 design + warning"改为"**abort + error 事件**",杜绝静默 fallback;新增 §6.1a 固化 `jst_erp` 真值快照供 Codex 执行前自检。依据 `docs/iterations/V1_R1_6_PROD_ALIGN.md` | **已签字**(2026-04-17) |

---

**签字**:

- 产品: **已确认**(2026-04-17)
- 后端: **已确认**(2026-04-17)
- 前端: **已确认**(2026-04-17)

> 本文于 2026-04-17 随主文档 v4 一并签字生效。OSS 对象生命周期、审核版本锚定、资产管理中心 API 契约均以本文为准。
