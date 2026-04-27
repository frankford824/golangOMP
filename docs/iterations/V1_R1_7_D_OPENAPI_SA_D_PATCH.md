# R1.7-D · OpenAPI SA-D 前置补丁

> 轮次:R1.7-D(SA-D 前置 OpenAPI 对齐)
> 触发:R4-SA-D 起草前 explore 扫描发现 4 条 owner 路径全量残留 `501` · `SearchResultGroup` / `throughput` / `module-dwell` schema 未固化 · RBAC 占位缺失 · `L1Card.description` 误引 IA §6
> 原则:本轮 **不** 修改 `transport/http.go`(501 占位保留至 SA-D 实现轮清理) · 本轮 **只** 冻结契约 schema + RBAC 占位 + 描述引用修正 · openapi-validate 必须 0/0
> 签字门槛:架构师 10 问裁决 + openapi-validate 0/0 + 路径清单对齐

---

## 1. Gap 来源

- `docs/iterations/V1_R1_7_OPENAPI_SA_A_PATCH.md` §5 已列 2 条 block(`/v1/search` limit + SearchResultGroup 固化)
- explore 再扫发现 5 条扩展 gap:
 - 两条 report 详情路径 `data[]` 完全开放
 - 4 条路径全部缺 `x-rbac-placeholder`
 - 4 条路径全部缺 401/403 ErrorResponse 契约
 - `L1Card.description` 误引 IA §6(实际 §6=组织架构)
 - 4 条路径全部残留 dangling `501`(本轮**不动**,由 SA-D 实现轮清)

---

## 2. 架构师 10 问裁决(2026-04-24)

| 编号 | 问题 | 决议 | 下游 |
|---|---|---|---|
| Q1 | `/v1/search` 权限行级过滤 | **A1** = IA §4.2 原方案:tasks/assets/products 全量命中 · users 低权空数组 | 无 actor scope 过滤 · SA-D 实现只按 Q2 处理 users |
| Q2 | `users[]` 在低权角色形态 | **U1** = 始终返回空数组(仅 SuperAdmin/HRAdmin 填充) | `SearchResultGroup.users[]` items 固化 · 低权调用返回 `[]` |
| Q3 | 搜索后端技术栈 | **B1** = v1 MySQL LIKE(IA §4.4) | SA-D 实现不引 ES/Meilisearch · 不加相关性字段 |
| Q4 | `SearchResultGroup` items 最小集 | **S1** = 接受提案 | 见 §3.1 固化字段 |
| Q5 | L1 报表 API RBAC | **E1** = 仅 SuperAdmin | 4 条 SA-D 路径 `x-rbac-placeholder.allowed_roles=[super_admin]` |
| Q6 | L1 报表数据源 | **C1** = v1 直查 `task_module_events` + `tasks`(Module §13 表述) | SA-D 实现不建物化表 · R6+ 视情况再升级 |
| Q7 | throughput / module-dwell 字段集 | **T1** = 接受提案 | 见 §3.3 / §3.4 固化字段 |
| Q8 | 报表导出/异步 | **F1** = v1 不做导出 | 不新增 `/v1/reports/.../export` · 不扩 export-jobs |
| Q9 | Module §12 把 L1 挂 R5 vs OpenAPI 标 R4-SA-D 命名张力 | **R3** = 保持现状 · 不改文档(记录张力,不影响本轮契约) | 文档张力留作 R5 时再评估 |
| Q10 | `L1Card.description` 锚点修正 | **AGREE** = `Source: V1_INFORMATION_ARCHITECTURE §1 一级菜单「报表」 + V1_MODULE_ARCHITECTURE §12 U 表` | 本轮直接改 |

---

## 3. OpenAPI Diff 计划(本轮实际落地动作)

### 3.1 `SearchResultGroup` schema 固化(`components/schemas/SearchResultGroup`)

```yaml
SearchResultGroup:
 type: object
 description: |
 Source: V1_INFORMATION_ARCHITECTURE §4.2.
 Decision (R1.7-D): all four arrays are item-schema fixed; `users[]` returns `[]`
 for roles other than super_admin / hr_admin regardless of match count (IA §4.3).
 properties:
 tasks:
 type: array
 items:
 type: object
 required: [id, task_no]
 properties:
 id: { type: integer, format: int64 }
 task_no: { type: string }
 title: { type: string, nullable: true }
 task_status: { type: string, nullable: true }
 priority: { type: string, nullable: true }
 highlight: { type: string, nullable: true }
 assets:
 type: array
 items:
 type: object
 required: [asset_id, file_name]
 properties:
 asset_id: { type: integer, format: int64 }
 file_name: { type: string }
 source_module_key: { type: string, nullable: true }
 task_id: { type: integer, format: int64, nullable: true }
 products:
 type: array
 items:
 type: object
 required: [erp_code, product_name]
 properties:
 erp_code: { type: string }
 product_name: { type: string }
 category: { type: string, nullable: true }
 users:
 type: array
 description: |
 Always `[]` unless caller is super_admin or hr_admin (IA §4.3 Q1/Q2 A1+U1).
 items:
 type: object
 required: [user_id, username]
 properties:
 user_id: { type: integer, format: int64 }
 username: { type: string }
 department_name: { type: string, nullable: true }
```

### 3.2 `/v1/search` operation 补丁

- 加 query param `limit`(integer · 1~50 · default 20 · IA §4.2)
- 加 `x-rbac-placeholder.auth_mode: session_token_authenticated`(所有登录用户可调 · users[] 按 Q2 U1 处理)
- 加 401 ErrorResponse
- **保留** 501(SA-D 实现轮清)

### 3.3 `L1Card.description` 修正(Q10)

```yaml
L1Card:
 type: object
 description: |
 Source: V1_INFORMATION_ARCHITECTURE §1 一级菜单「报表」 + V1_MODULE_ARCHITECTURE §12 U 表.
 R1.7-D decision: v1 直查 task_module_events + tasks,不建物化表。
 ...
```

### 3.4 `/v1/reports/l1/throughput` schema 固化

- query 参数:
 - `from`: date(ISO 8601 · required)
 - `to`: date(required)
 - `department_id`: integer(optional · R5+ 可能扩)
 - `task_type`: string(optional · enum 暂不收紧)
- response `data[]` items:
 - `date`: string(date)
 - `created`: integer
 - `completed`: integer
 - `archived`: integer

### 3.5 `/v1/reports/l1/module-dwell` schema 固化

- query 参数同 throughput(`from` / `to` / `department_id?` / `task_type?`)
- response `data[]` items:
 - `module_key`: string(enum `task_detail/design/audit/customization/warehouse`)
 - `avg_dwell_seconds`: number
 - `p95_dwell_seconds`: number
 - `samples`: integer(样本量)

### 3.6 三条报表路径 RBAC & ErrorResponse(Q5 E1)

- `x-rbac-placeholder:`
 - `auth_mode: session_token_authenticated`
 - `allowed_roles: [super_admin]`
- 响应追加:401、403(`deny_code: reports_super_admin_only`)

### 3.7 不动项

- 4 条路径 `501` **保留**(等待 SA-D 实现轮清)
- `transport/http.go` 不改
- Module §12 / IA §1 报表菜单章节不改(Q9 R3)

---

## 4. 验证门槛

- `openapi-validate` 必须 0 error 0 warning
- `grep -E 'x-owner-round:[[:space:]]*R4-SA-D' openapi.yaml` 仍为 4 条
- `grep -c "'501'" openapi.yaml` 在 SA-D 4 条 path 段内仍为 4 条(占位未被误删)
- `SearchResultGroup` 四数组 `items` 字段必须包含 §3.1 列出的必选项
- `L1Card.description` 不再出现 `§6.`(引用校正已生效)

---

## 5. 路径对齐清单

| Path | Method | 状态 | RBAC | 501 | query |
|---|---|---|---|---|---|
| `/v1/search` | GET | schema 固化 + limit + RBAC + 401 | `session_token_authenticated`(全登录) | 保留 | q,scope,limit |
| `/v1/reports/l1/cards` | GET | RBAC + 401/403 | `super_admin` | 保留 | (无) |
| `/v1/reports/l1/throughput` | GET | data[] 固化 + RBAC + 401/403 | `super_admin` | 保留 | from,to,department_id?,task_type? |
| `/v1/reports/l1/module-dwell` | GET | data[] 固化 + RBAC + 401/403 | `super_admin` | 保留 | from,to,department_id?,task_type? |

---

## 6. 未纳入本轮

- export(Q8 F1):v1 不做
- 物化表 task_metrics_l1(Q6 C1):v1 不建
- ES/相关性(Q3 B1):v1 不接
- `/v1/admin/*reports*`(explore §1.2):未规划
- Module §12 L1→R5 vs R4-SA-D 命名张力(Q9 R3):记录不改
- SA-D **实现**(Handler/Service):由 R4-SA-D 主轮承接

---

## 7. 后续动作

1. 本轮结束后立即起草 `prompts/V1_R4_FEATURES_SA_D.md`
2. R4-SA-D 主轮落地时:4 条路径移除 `501` + 实现 MySQL LIKE + task_module_events 聚合查询
3. R4-SA-D.1 补丁轮(预期):I1~In 专项 integration 测试

---

**签字(架构师)**:本文件落盘 + openapi-validate 0/0 + `V1_ROADMAP.md` 更新后签 R1.7-D 门槛。
