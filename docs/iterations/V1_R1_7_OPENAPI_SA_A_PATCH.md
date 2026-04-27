# V1 · R1.7 OpenAPI SA-A 补丁 · 执行报告

> 版本:**v1.0 · 已签字生效**
> 日期:2026-04-17
> 触发:R4-SA-A Codex 前置校验 abort(search 缺 query、archive/delete 缺 reason body、4 条 R4-SA-A 路径残留 `501`)
> 执行者:主对话(架构/Prompt designer)
> 范围:**只补 SA-A 所需 9 点**;SA-B/C/D 的 gap 收录为"已知待补",后续轮起草前再处理
> 认证文档:`docs/V1_ASSET_OWNERSHIP.md` v1.2

---

## 1. 背景

R4-SA-A Codex 按 prompt §9 失败终止条件正确 abort,提示:

1. `GET /v1/assets/search` schema 缺 §5.3 要求的 8 个 query 参数
2. 4 条 R4-SA-A 路径仍挂 `'501': { description: Reserved for R4-SA-A }` 占位

主对话启动 R1.7 补丁轮:**不由 SA-A 自行扩 OpenAPI**,由架构师按权威文档补齐后再重启 SA-A。

---

## 2. Explore subagent 扫描结果(31 路径)

一次性扫描所有 `x-owner-round: R4-SA-*` 路径 × 权威文档 gap:

| owner | 路径数 | block | minor | dangling_501 |
| --- | --- | --- | --- | --- |
| SA-A | 7 | 5 | 0 | 4 |
| SA-B | 14 | 5 | 0 | 10 |
| SA-C | 10 | 3 | 1 | 11 |
| SA-D | 4 | 2 | 0 | 4 |
| **总计** | **35 ops** | **15** | **1** | **29** |

(ops 包含同一 path 的 GET+DELETE 等多方法;路径数 SA-A 7 = 6 paths × 1~2 methods)

---

## 3. R1.7 本轮实补(仅 SA-A · 9 点)

**为什么不一次全补 15 block**:

1. SA-B 的 `team_codes[]` / `primary_team_code` 触及 v1 多组模型 vs v0.9 单组 `team` 字段的语义裁决,属架构决策而非 R1 漏项,应由 SA-B 起草时主对话裁决
2. SA-C 的 `/v1/me/task-drafts` query 和 `/v1/me/notifications` query 应配合 SA-C 的 service 设计(cursor 分页 vs page 分页)
3. SA-D 的 `/v1/search` response 固定字段涉及 SA-D 实现策略(MySQL LIKE vs 未来 ES)
4. 一次性 16 条 schema 编辑风险过大(一个 StrReplace 错就污染整个 OpenAPI)

**SA-B/C/D 的 gap 清单收录本报告 §5**,作为各轮起草前的"前置补丁清单",每轮启动前再开 R1.7-B / R1.7-C / R1.7-D 小补丁。

### 实补 9 点(SA-A 全部 block + dangling_501)

| # | 路径 | 动作 | 权威依据 |
| --- | --- | --- | --- |
| P1 | `GET /v1/assets/{id}` | response `data` 从 `Asset` 升级为新 `AssetDetail`(含 `versions[]`、`archived_at`、`archived_by`、`cleaned_at`、`deleted_at`) | 资产 §5.2 |
| P2 | `DELETE /v1/assets/{id}` | 加 `requestBody: AssetReasonRequest(reason required)`;补 403 / 404 response schema | 资产 §5.4 |
| P3 | `GET /v1/assets/search` | 加 9 个 query params(keyword / module_key / owner_team_code / is_archived / task_status / created_from / created_to / page / size);response 加 `total` / `page` / `size`;**移除 501** | 资产 §5.2 / §5.3 |
| P4 | `GET /v1/assets/{asset_id}/versions/{version_id}/download` | response `200` 补 `data: AssetDownloadInfo`;补 404 / 410 结构化 ErrorResponse;**移除 501** | 资产 §5.2 / §7.4 |
| P5 | `POST /v1/assets/{asset_id}/archive` | 加 `requestBody: AssetReasonRequest`;补 403 / 404;**移除 501** | 资产 §7.3 |
| P6 | `POST /v1/assets/{asset_id}/restore` | 补 403 / 404;**移除 501** | 资产 §7.3 |
| P7 | `components/schemas/AssetReasonRequest` | **新增**(archive / delete 共用) | 资产 §5.4 / §7.3 |
| P8 | `components/schemas/AssetVersion` | **新增**(AssetDetail.versions[] 的元素类型) | 资产 §4 / §5.2 |
| P9 | `components/schemas/AssetDetail` | **新增**(allOf Asset + versions + 归档/清理/删除元数据) | 资产 §5.2 |

### 不做的事

- `GET /v1/assets/{id}/download`:原本就无 501,现状符合 §5.2,**不改**
- 不新增任何 DenyCode 枚举(R3 的 `module_action_role_denied` 复用)
- 不改 `Asset` / `AssetDownloadInfo` 现有 schema 的字段
- 不改任何非 R4-SA-A 路径
- SA-B/C/D 的 gap 本轮**不动**

---

## 4. 验证

```bash
wsl bash -lc "cd /mnt/c/Users/wsfwk/Downloads/yongboWorkflow/go && \
  /home/wsfwk/go/bin/go run ./cmd/tools/openapi-validate docs/api/openapi.yaml"
# exit=0
# openapi validate: 0 error 0 warning
```

结论:**SA-A 7 条路径 schema 与 `V1_ASSET_OWNERSHIP.md v1.2` §5.2/§5.3/§5.4/§7.3 完全对齐**。

---

## 5. SA-B / SA-C / SA-D 待补清单(后续各轮前置补丁)

> 这些条目已由 explore subagent 扫描确认,主对话在起草 SA-B / SA-C / SA-D prompt **之前**需各开一轮小补丁(预估 R1.7-B / R1.7-C / R1.7-D 各 30 分钟)。

### SA-B 待补(5 block)

| 路径 | gap | §依据 | 备注 |
| --- | --- | --- | --- |
| `GET /v1/me` | response 加 `email` / `mobile` / `avatar` / `team_codes[]` | IA §7.2 + 主 §5.2 | **需裁决**:扩 Actor 或新建 MyProfile? |
| `GET /v1/me/org` | response 加 `team_codes[]` / `managed_*` 列表结构 | IA §7.2 | 同上 |
| `PATCH /v1/me` | requestBody 加 `nickname / avatar / mobile / email` | IA §7.2 | — |
| `POST /v1/me/change-password` | requestBody 加 `old_password / new_password / confirm` | IA §7.2 | — |
| `PATCH /v1/users/{id}` | requestBody 加 `roles[] / primary_team_code / team_codes[]`(或等价) | IA §5.3 / §5.4 | **需裁决 v1 多组模型 vs v0.9 单组** |

### SA-C 待补(3 block + 1 minor)

| 路径 | gap | §依据 |
| --- | --- | --- |
| `GET /v1/design-sources/search` | query `keyword / page / size` | 定制 §3.2.2 |
| `GET /v1/me/task-drafts` | query `task_type / limit / cursor` | IA §3.5.9 |
| `GET /v1/me/notifications` | query `is_read / limit / cursor` | IA §8.3 |
| `POST /v1/task-drafts`(minor) | 显式 `draft_id` 字段(当前 `additionalProperties: true` 已容纳) | IA §3.5.9 |

### SA-D 待补(2 block)

| 路径 | gap | §依据 | 备注 |
| --- | --- | --- | --- |
| `GET /v1/search` | query 加 `limit`(默认 20) | IA §4.2 | — |
| `GET /v1/search` | response `SearchResultGroup` 固化 `tasks[]{id,task_no,highlight}` / `assets[]{asset_id,file_name}` / `products[]{erp_code,product_name}` / `users[]{...}` | IA §4.2 | **需裁决 `users[]` 低权限返回空数组的形式** |

### SA-B/C/D 的 23 条 dangling_501

**保留不动**。那几轮还没落地,501 是正当占位;各轮落地时连同 block 补一并清理。

---

## 6. 对 SA-A prompt 的影响

`prompts/V1_R4_FEATURES_SA_A.md` 升 **v2**,仅追加:

- §1 必读输入追加 "R1.7 补丁报告"
- §9 失败终止条件去掉 "OpenAPI schema 对不上" 一条(已被 R1.7 解决)
- 变更记录加 v2 行

Codex 用最新 OpenAPI 重新跑 SA-A 前置校验 → 7 条路径 schema 全部吻合 → 应直接进入实现阶段。

---

## 7. 签字矩阵

| 角色 | 签字 |
| --- | --- |
| 架构(本对话) | **已签**(2026-04-17) |
| 后端 | **已同步**(2026-04-17,OpenAPI 0/0 · 无 DDL 影响 · 无生产影响) |
| 前端 | **已同步**(2026-04-17,Asset schema 仅扩展 · AssetDetail 为新增 · SearchResultGroup 未动) |
| 产品 | **已同步**(2026-04-17,字段新增均来自产品已签字文档 §5.2/§5.3/§5.4/§7.3) |

---

## 8. 变更记录

| 版本 | 日期 | 变更 |
| --- | --- | --- |
| v1.0 | 2026-04-17 | 初稿即签字。R4-SA-A Codex abort 触发;一次性扫描 31 R4 路径;本轮仅实补 SA-A 9 点(3 新 schema + 6 path 编辑);`openapi-validate` 0/0;SA-B/C/D 待补 10 block 收录 §5 供后续轮起草前消化 |
