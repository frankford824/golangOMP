# V1.3-A1 架构师独立 Verify 报告

- **日期**: 2026-04-27 PT
- **被 verify 对象**: V1.3-A1 Frontend Integration Bugfix Diagnosis（codex 自报终止符 `V1_3_A1_FRONTEND_BUGFIX_DIAGNOSED_PENDING_LOG_AND_VERIFY`，未自签 PASS）
- **verify 范围**: commit `bee7383`、5 evidence 文件、ROADMAP v55、RETRO V1.3-T1
- **verify 方式**: 100% 独立证据，逐 issue 复核 codex 静态分析结论

---

## 1. 总裁决

**`V1.3-A1 PASS` — 诊断质量高、根因定位准确、严守 readonly 边界、Issue 3 拒绝 guess work 守住 trace log 调度。**

CONDITIONAL 部分仅在 **Issue 3 待用户提供后端 log 后才能进入修复阶段**，这是设计上必须的卡点，不是 codex 的工作不完整。

---

## 2. 独立 verify 矩阵（10 项）

### V1: commit 范围严格

```text
bee7383 docs(diagnosis): V1.3-A1 frontend integration bugfix root cause analysis
 7 files changed, 202 insertions(+)

docs/iterations/V1_3_A1_FRONTEND_INTEGRATION_BUGFIX_REPORT.md  (+60)
docs/iterations/V1_RETRO_REPORT.md                              (+1)
prompts/V1_ROADMAP.md                                           (+1)
tmp/v1_3_a1_issue1_evidence.md                                  (+26)
tmp/v1_3_a1_issue2_evidence.md                                  (+32)
tmp/v1_3_a1_issue3_evidence.md                                  (+40)
tmp/v1_3_a1_issue4_evidence.md                                  (+42)
```

**0 业务 Go / 0 OpenAPI / 0 frontend docs / 0 transport/http.go**。完全符合 prompt §3.4 硬门。

### V2: 3 锚 SHA 不漂移

| 文件 | 当前 | 期望 | 结果 |
|---|---|---|---|
| `docs/api/openapi.yaml` | `fe1b3a26…` | `fe1b3a26…` | OK |
| `transport/http.go` | `9a6d194b…` | `9a6d194b…` | OK |
| `cmd/server/main.go` | `61a52019…` | `61a52019…` | OK |

### V3: 5 evidence 文件全部存在 + 内容充实

| 文件 | 字节 | 状态 |
|---|---|---|
| `docs/iterations/V1_3_A1_FRONTEND_INTEGRATION_BUGFIX_REPORT.md` | 3112 B | OK |
| `tmp/v1_3_a1_issue1_evidence.md` | 2186 B | OK |
| `tmp/v1_3_a1_issue2_evidence.md` | 1251 B | OK |
| `tmp/v1_3_a1_issue3_evidence.md` | 2826 B | OK |
| `tmp/v1_3_a1_issue4_evidence.md` | 2352 B | OK |

### V4: Issue 1 诊断质量复核

codex 静态分析定位:
- `transport/handler/task_draft.go` 读 raw body → `service/task_draft/service.go::cloneRaw(raw)` → `repo/mysql/task_draft_repo.go` 写入 `task_drafts.payload` blob
- OpenAPI `TaskDraftPayload` 用 `additionalProperties: true`,允许任意字段含 `download_url`
- `GET /v1/assets/{asset_id}/download` 路径 `globalSvc.DownloadLatest` → `asset_center/download.go::PresignDownloadURL(key)` 每次重签
- 给前端主修 + 后端 V1.3-A1.1 可选硬化方案(scrub `download_url`/`url`/`download_url_expires_at` from draft payload before persist)

**架构师复核**:链路追踪准确,符合 OpenAPI line 3507 "default 15 minutes, consumers SHOULD refresh" 设计意图。前端把 draft `download_url` 当 display cache 而非 authority 是正确架构。**采纳**。

### V5: Issue 2 诊断质量复核

codex 已证实 `/v1/reports/l1/overview` 既不在 `transport/http.go` 挂载也不在 OpenAPI 定义。给 Option A(前端改)+ Option B(V2 新 feature),推荐 A。

**架构师复核**:与我之前独立 grep 结论完全一致(transport/http.go:134~139 仅 cards/throughput/module-dwell,OpenAPI line 14723/14761/14831 同步 3 path)。Option A **采纳**为 V1.3-A1 决策。Option B 转入 V2 feature backlog,本轮不动。

### V6: Issue 3 诊断质量复核(关键)

codex 准确定位:
- `service/task_service.go::mapTaskCreateTxError` 仅把 MySQL `1062` 转 `400`,其它一律 `INTERNAL_ERROR` 加固定 message `internal error during create task tx`
- 错误日志会携带 `create_task_tx_failed err=...` 行,这是真正的 root cause 来源
- 列出 3 个最可能假设:(1) reference_file_refs FK/unique 失败 (2) `defer_local_product_binding=true` + ERP 桥接产生 `product_id=NULL` 触发下游 schema mismatch (3) code-rule/task-no/SKU 序列 / SKU item 失败
- **明确说**: "Without that error string or stack, code changes would be guesswork"
- 没尝试本地复现(因 cmd/server 需 MySQL+Redis+特定 fixture,faithful repro 不可得)

**架构师复核**:这是 V1.3-A1 最关键的判断 — codex **拒绝在没有 trace log 的情况下擅自修码**,严守"先取证、再下结论"原则。3 个假设均基于 payload 字段 + 静态代码分析,合理。**采纳**等待 log 调度。

**用户必须提供**:

```bash
grep '83ba7d26-385b-4bea-99b7-db0925be2975' /生产或测试环境的服务日志路径
```

需要的关键行:
- `create_task_tx_failed err=...`(必须)
- 上下文 ±5 行,含 `create_task_entry` / `create_task_product_selection_*` / 任何 stack trace 或 SQL error code

提供后我直接起草 `V1.3-A1.1 fix` prompt(后端单点修),不再启额外诊断轮。

### V7: Issue 4 诊断质量复核

codex 准确发现:
- `GET /v1/assets` 列表 item schema = `DesignAsset` (root 不带 `download_url`/`preview_url`,仅嵌套 `current_version` 等里有)
- `GET /v1/tasks/{id}/assets` + `GET /v1/tasks/{id}/asset-center/assets` 同 `DesignAsset`
- 单 GET `/v1/assets/{asset_id}/download` 返 `AssetDownloadInfo` 含 `download_url` + `expires_at`
- canonical/global list (`service/asset_center.Search`) 不 presign list items
- 给"列表 metadata-only · 单 GET 重签"trade-off 论证:列表 presign fan-out 性能开销 + 短期 URL 缓存难度 + draft 持久化误存风险 三重劣势

**架构师复核**:trade-off 分析完整,与 Issue 1 的草稿 URL 持久化问题形成闭环(列表若返 fresh URL,前端把整个列表存草稿就会再次踩 Issue 1)。**采纳** "Do not add root-level list URLs in V1.3-A1"。

### V8: ROADMAP v55 + RETRO V1.3-T1 治理同步

ROADMAP v55 已落(2026-04-27 V1.3-A1 entry,定位 4 issue + 报告路径)。

RETRO 治理债区 V1.3-T1 已落(MED · OPEN · onboarding sha 锚组过期 + CLAUDE.md L7 指向归档文件)。

### V9: 工作树清洁

工作区只剩 4 个本轮开始前已有的未跟踪 prompt 文件:
- `prompts/V1_1_A2_CONTRACT_DRIFT_PURGE.md`
- `prompts/V1_2_C_AUDIT_TOOL_REWORK.md` / `V1_2_C_GIT_COMMIT_LANDING.md`
- `prompts/V1_2_D_DRIFT_TRIAGE.md` / `V1_2_D_1_TASK_DETAIL_FALLBACK_REMOVAL.md` / `V1_2_D_2_DRIFT_RESIDUAL_TRIAGE.md`

无新增未跟踪文件,无 staged 残留。

### V10: codex 自报与终止符遵守

codex 严格 "不自签 PASS",输出终止符 `V1_3_A1_FRONTEND_BUGFIX_DIAGNOSED_PENDING_LOG_AND_VERIFY`,等待架构师 verify + 用户提供后端日志,完全符合 prompt §3.4。

---

## 3. 4 个 issue 的最终决策(架构师拍板)

| Issue | 优先级 | 决策 | 责任方 | 下一步 |
|---|---|---|---|---|
| 1 草稿 download_url 404 | P1 | 前端主修(从 `asset_id` 调 `/v1/assets/{id}/download` 拿 fresh URL);后端 V1.3-A1.1 可选 scrub | 前端主 + 后端可选 | 通知前端落实;后端 scrub 工作排进 V1.3-A1.1 后端硬化批 |
| 2 reports/l1/overview 404 | P1 | **采纳 Option A**:前端改用 cards/throughput/module-dwell 组合 · 后端不动 | 前端 | 通知前端改造;V2 backlog 登记"是否引入 overview 聚合 endpoint" |
| 3 POST /v1/tasks 500 | **P0** | 等待用户提供 trace log → 起草 V1.3-A1.1 后端单点修 | 后端 | **用户提供 trace log 是阻塞项** |
| 4 列表 download_url 一致性 | P2 | **不补**列表 root URL · 列表保持 metadata-only · 前端按需调单 GET 重签 | 前端 | 文档/CHEATSHEET 已在 frontend docs 充分说明,无需后端动作 |

---

## 4. V1.3 后续路线

按上面决策矩阵,V1.3 后续 prompt 拆分:

- **V1.3-A1.1 (P0)**: Issue 3 后端单点修 — 触发条件:用户提供 trace log
- **V1.3-A1.2 (P1 可选)**: Issue 1 后端 draft payload `download_url` scrub 硬化(防御性) — 优先级低于 A1.1
- **V1.3-T1 (MED)**: onboarding sha 锚 + CLAUDE.md handoff 路径回写到 V1.2-D-2 head — 与 A1.1 / A1.2 解耦,可并行
- **V1.3-T2 (从 V1.2-D-2 verify report 继承)**: known_gap class 字段写入 audit JSON + CI gate
- **V1.3-T3 (从 V1.2-D-2 verify report 继承)**: 4 deprecated `/v1/assets/:id` 决策(OpenAPI 删 vs transport 重挂)

---

## 5. 终止符

`V1_3_A1_DIAGNOSIS_ARCHITECT_VERIFIED_AWAITING_USER_TRACE_LOG`

诊断 PASS · 4 决策已拍板 · 阻塞项是 Issue 3 的后端 trace log,**请用户立刻 grep 服务日志的 trace_id `83ba7d26-385b-4bea-99b7-db0925be2975` 把 `create_task_tx_failed` 那一行连同 ±5 行上下文贴出来**,我即刻起草 V1.3-A1.1 后端修复 prompt。
