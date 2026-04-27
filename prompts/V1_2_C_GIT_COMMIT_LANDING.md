# V1.2-C-LANDING · git stage + commit V1.2 / V1.2-C 全部产物 · 落版本控制

> 状态:V1.2 + V1.2-C 已经架构师签字 `V1_2_DONE_ARCHITECT_VERIFIED`(2026-04-27)。
> 当前 working tree 有 102 项 dirty(M/A/R/D),都是已签字治理产物,但未 commit。
> 范围:**只做 git 落地** · 0 **业务** Go 改动 · 0 OpenAPI 改动 · 0 治理文件改动。
> **允许提交的 Go 文件白名单**(V1.2-C 工具及其测试 / fixture):
> - `tools/contract_audit/main.go`
> - `tools/contract_audit/main_test.go`
> - `tools/contract_audit/testdata/**/*.go`
>
> v1.0 ABORT 改判:codex 严守 §0 #1 字面"任何 *.go 文件 staged 或修改 → ABORT"是正确动作 · 该规则与 C-4 commit `tools/contract_audit/main.go` 字面冲突。本 v1.1 prompt 把 #1 改为"业务 Go 文件" · 显式给 V1.2-C 工具 Go 路径开白。

## §0 硬门(任意一项 trigger 立即 ABORT)

| # | 触发条件 | 行为 |
|---|---|---|
| 1 | 任何 **业务** `*.go` 文件 staged 或修改(业务 = 白名单**之外**的所有 Go 文件,即 `transport/**/*.go` / `service/**/*.go` / `repo/**/*.go` / `domain/**/*.go` / `cmd/**/*.go` / `internal/**/*.go` / `migrations/**/*.go` / `_test.go` 含在内 · **白名单仅** `tools/contract_audit/main.go` + `tools/contract_audit/main_test.go` + `tools/contract_audit/testdata/**/*.go`)| ABORT |
| 2 | `docs/api/openapi.yaml` 内容 SHA 改变 | ABORT |
| 3 | `tools/contract_audit/main.go` 内容 SHA 改变(校验等于 §1 锚 SHA · 不是"不允许 commit" · 是"内容必须等于已 verify 终态")| ABORT |
| 4 | `transport/http.go` 内容 SHA 改变 | ABORT |
| 5 | 任何 commit 失败 | ABORT |
| 6 | 任何 `git push` 命令 | ABORT(本轮严禁 push 到远程,只本地 commit) |
| 7 | git status 在最终 commit 之后仍含 ***业务 Go*** dirty(允许仍有未跟踪 prompts/tmp/*.py)| ABORT |
| 8 | 任何白名单**之外**的 `*.go` 路径出现在任一 commit 的 file list | ABORT |

## §1 baseline 锚 SHA(P0 校验)

```
docs/api/openapi.yaml                          80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f
transport/http.go                              9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396
transport/handler/task_detail.go               b8636965bda71004143bb968263080c0d737047db84f953a2a721c3d77a1d603
domain/task.go                                 658a8cdf65c09335ab74176efb4057eff68440537e50ce0d9e550c57413e6e6b
domain/task_detail_aggregate.go                315aef20dc7e34ad3233bf8f3e6bf8ae8e7477103586856d494a8c9e62bb82f0
service/task_aggregator/detail_aggregator.go   6e10c7e6d3f8096538015385fd317e94715a24568122159154538be17e347c7e
service/identity_service.go                    00ec340a81738a75a88d3b0d32d834b49879bea7df6ac1baa0eb1932d1d47644
repo/mysql/task_detail_bundle.go               c6518daef3db588525c6cada3f366118c21483643c9241e81b1e6a13a81b70ba
repo/mysql/identity_actor_bundle.go            d8c135221fc8c6745b6863521230a0a39ba43cc6420c713cc836e474fc1e8a6a
tools/contract_audit/main.go                   fc86c550622c3fcdbcd59beca8fe08e7a44b1fecd33c3c9f42dc116ac9f6455d
```

P0 不一致 → ABORT。

## §2 commit 分组(6 个 commit · 顺序执行 · 每组之后做 git status 自检)

### C-1 archive moves(纯 git mv 类 R 状态)

```
docs/ASSET_ACCESS_POLICY.md                       -> docs/archive/legacy_handoffs/ASSET_ACCESS_POLICY.md
docs/ASSET_STORAGE_AND_FLOW_RULES.md              -> docs/archive/legacy_handoffs/ASSET_STORAGE_AND_FLOW_RULES.md
docs/COMPATIBILITY_ROUTES_INVENTORY.md            -> docs/archive/legacy_handoffs/COMPATIBILITY_ROUTES_INVENTORY.md
docs/ERP_SEARCH_CAPABILITY.md                     -> docs/archive/legacy_handoffs/ERP_SEARCH_CAPABILITY.md
docs/FRONTEND_CUSTOMIZATION_HANDOFF.md            -> docs/archive/legacy_handoffs/FRONTEND_CUSTOMIZATION_HANDOFF.md
docs/FRONTEND_DIST_PUBLISH_SOP.md                 -> docs/archive/legacy_handoffs/FRONTEND_DIST_PUBLISH_SOP.md
docs/FRONTEND_MAIN_FLOW_CHECKLIST.md              -> docs/archive/legacy_handoffs/FRONTEND_MAIN_FLOW_CHECKLIST.md
docs/FRONTEND_OSS_DIRECT_HANDOFF.md               -> docs/archive/legacy_handoffs/FRONTEND_OSS_DIRECT_HANDOFF.md
docs/FRONTEND_REFERENCE_URL_REFRESH.md            -> docs/archive/legacy_handoffs/FRONTEND_REFERENCE_URL_REFRESH.md
docs/THREE_ENDPOINT_CONTROL_PLANE.md              -> docs/archive/legacy_handoffs/THREE_ENDPOINT_CONTROL_PLANE.md
docs/TRUTH_SOURCE_ALIGNMENT.md                    -> docs/archive/legacy_handoffs/TRUTH_SOURCE_ALIGNMENT.md
docs/V0_9_MODEL_HANDOFF_MANIFEST.md               -> docs/archive/legacy_handoffs/V0_9_MODEL_HANDOFF_MANIFEST.md
docs/V1_0_FRONTEND_INTEGRATION_GUIDE.md           -> docs/archive/legacy_handoffs/V1_0_FRONTEND_INTEGRATION_GUIDE.md
docs/V7_FRONTEND_INTEGRATION_ORDER.md             -> docs/archive/legacy_handoffs/V7_FRONTEND_INTEGRATION_ORDER.md
prompts/STEP_01.md                                -> prompts/archive_pre_v1_2/STEP_01.md
prompts/STEP_02.md                                -> prompts/archive_pre_v1_2/STEP_02.md
prompts/STEP_03.md                                -> prompts/archive_pre_v1_2/STEP_03.md
prompts/STEP_04.md                                -> prompts/archive_pre_v1_2/STEP_04.md
prompts/V1_R1_CONTRACT_FREEZE.md                  -> prompts/archive_pre_v1_2/V1_R1_CONTRACT_FREEZE.md
prompts/V1_R2_DATA_LAYER.md                       -> prompts/archive_pre_v1_2/V1_R2_DATA_LAYER.md
prompts/V1_R3_5_INTEGRATION_VERIFICATION.md       -> prompts/archive_pre_v1_2/V1_R3_5_INTEGRATION_VERIFICATION.md
prompts/V1_R3_ENGINE.md                           -> prompts/archive_pre_v1_2/V1_R3_ENGINE.md
prompts/V1_R4_FEATURES_SA_A.md                    -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_A.md
prompts/V1_R4_FEATURES_SA_B.md                    -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_B.md
prompts/V1_R4_FEATURES_SA_B_1_I1_I11_PATCH.md     -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_B_1_I1_I11_PATCH.md
prompts/V1_R4_FEATURES_SA_C.md                    -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_C.md
prompts/V1_R4_FEATURES_SA_C_1_I1_I11_PATCH.md     -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_C_1_I1_I11_PATCH.md
prompts/V1_R4_FEATURES_SA_D.md                    -> prompts/archive_pre_v1_2/V1_R4_FEATURES_SA_D.md
prompts/V1_R4_RETRO.md                            -> prompts/archive_pre_v1_2/V1_R4_RETRO.md
prompts/V1_R4_SA_A_PATCH_A2.md                    -> prompts/archive_pre_v1_2/V1_R4_SA_A_PATCH_A2.md
prompts/V1_R4_SA_A_PATCH_A3.md                    -> prompts/archive_pre_v1_2/V1_R4_SA_A_PATCH_A3.md
prompts/V1_R5_BATCH_SKU.md                        -> prompts/archive_pre_v1_2/V1_R5_BATCH_SKU.md
prompts/V1_R6_A_1_RUN_CLEANUP_CLI.md              -> prompts/archive_pre_v1_2/V1_R6_A_1_RUN_CLEANUP_CLI.md
prompts/V1_R6_A_2_CRON_INFRA.md                   -> prompts/archive_pre_v1_2/V1_R6_A_2_CRON_INFRA.md
prompts/V1_R6_A_3_AUTO_ARCHIVE.md                 -> prompts/archive_pre_v1_2/V1_R6_A_3_AUTO_ARCHIVE.md
prompts/V1_R6_A_4_RETRO_AND_HANDOFF.md            -> prompts/archive_pre_v1_2/V1_R6_A_4_RETRO_AND_HANDOFF.md
prompts/V1_RELEASE_v1_21_AND_FRONTEND_DOCS.md     -> prompts/archive_pre_v1_2/V1_RELEASE_v1_21_AND_FRONTEND_DOCS.md
docs/定制系统需求-winnie2026-04-09.xlsx           DELETE
```

```
git add docs/archive/legacy_handoffs/ prompts/archive_pre_v1_2/
git add -u docs/ prompts/
git commit -m "chore(archive): move pre-v1.2 legacy handoffs and round prompts to archive"
```

### C-2 V1.1-A2 contract drift purge(OpenAPI + 16 frontend doc + V1.1-A2 reports)

```
docs/api/openapi.yaml                              (M)  · V1.1-A2 schema 修订(detail 转 5 段)
docs/frontend/INDEX.md                             (M)
docs/frontend/V1_API_ASSETS.md                     (M)
docs/frontend/V1_API_AUTH.md                       (M)
docs/frontend/V1_API_BATCH.md                      (M)
docs/frontend/V1_API_CHEATSHEET.md                 (M)
docs/frontend/V1_API_DRAFTS.md                     (M)
docs/frontend/V1_API_ERP.md                        (M)
docs/frontend/V1_API_ME.md                         (M)
docs/frontend/V1_API_NOTIFICATIONS.md              (M)
docs/frontend/V1_API_ORG.md                        (M)
docs/frontend/V1_API_REPORTS.md                    (M)
docs/frontend/V1_API_SEARCH.md                     (M)
docs/frontend/V1_API_TASKS.md                      (M)
docs/frontend/V1_API_TASK_ASSETS.md                (M)
docs/frontend/V1_API_USERS.md                      (M)
docs/frontend/V1_API_WS.md                         (M)
docs/iterations/V1_1_A2_DRIFT_INVENTORY.md         (A)
docs/iterations/V1_1_A2_FIX_PLAN.md                (A)
docs/iterations/V1_1_A2_RETRO_REPORT.md            (A)
prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md       (M)
```

```
git add docs/api/openapi.yaml docs/frontend/ docs/iterations/V1_1_A2_*.md prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md
git commit -m "feat(contract): V1.1-A2 contract drift purge - detail switched to 5-section aggregate"
```

### C-3 V1.2 OpenAPI GC + governance + V1 SoT + frontend INDEX V1.2 markers

```
docs/V1_BACKEND_SOURCE_OF_TRUTH.md                 (A)  · 新建 V1 SoT
docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md            (M)
docs/iterations/INDEX.md                           (A)
docs/iterations/V1_2_ABORT_REPORT.md               (A)
docs/iterations/V1_2_AUTHORITY_INVENTORY.md        (A)
docs/iterations/V1_2_OPENAPI_GC_REPORT.md          (A)
prompts/INDEX.md                                   (A)
prompts/V1_NEXT_MODEL_ONBOARDING.md                (M)
CLAUDE.md                                          (M)
```

```
git add docs/V1_BACKEND_SOURCE_OF_TRUTH.md docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md docs/iterations/INDEX.md docs/iterations/V1_2_ABORT_REPORT.md docs/iterations/V1_2_AUTHORITY_INVENTORY.md docs/iterations/V1_2_OPENAPI_GC_REPORT.md prompts/INDEX.md prompts/V1_NEXT_MODEL_ONBOARDING.md CLAUDE.md
git commit -m "feat(governance): V1.2 authority purge - V1 SoT + OpenAPI 15 unreachable schemas dropped"
```

### C-4 V1.2 contract guard infra(scripts/contract-guard + .cursor/hooks + tools/contract_audit + v1 reports)

```
scripts/contract-guard.ps1                         (A)
scripts/contract-guard.sh                          (A)
.cursor/hooks/contract-guard.json                  (A)
tools/contract_audit/                              (full)
docs/iterations/V1_2_CONTRACT_AUDIT_v1.json        (A)
docs/iterations/V1_2_CONTRACT_AUDIT_v1.md          (A)
```

```
git add scripts/contract-guard.* .cursor/hooks/contract-guard.json tools/contract_audit/ docs/iterations/V1_2_CONTRACT_AUDIT_v1.*
git commit -m "feat(audit): V1.2 contract guard infra - tools/contract_audit + scripts/contract-guard + .cursor/hooks"
```

### C-5 V1.2-C audit tool rework(v2 outputs · 工具源码已在 C-4)

> 注意:`tools/contract_audit/main.go` 在 C-4 已 add 进 staging,V1.2-C 重写后会被覆盖到当前 SHA `fc86c550...`。
> 验证:C-4 commit 后 `tools/contract_audit/main.go` 内容必须等于本 prompt §1 所列 `fc86c550...`(即 V1.2-C 终态)。
> 若 C-4 commit 之后 main.go 是 V1.2-C v1 旧版,`git diff HEAD -- tools/contract_audit/main.go` 应为空。

```
docs/iterations/V1_2_CONTRACT_AUDIT_v2.json        (A · 但 working tree mtime 改过 → AM)
docs/iterations/V1_2_CONTRACT_AUDIT_v2.md          (AM)
docs/iterations/V1_2_C_RETRO_REPORT.md             (A)
```

```
git add docs/iterations/V1_2_CONTRACT_AUDIT_v2.* docs/iterations/V1_2_C_RETRO_REPORT.md
git commit -m "feat(audit): V1.2-C contract_audit rework - real three-way diff engine + 6 integration tests"
```

### C-6 V1.2 + V1.2-C governance final close(retro/SoT/ROADMAP 终态)

```
docs/iterations/V1_RETRO_REPORT.md                 (MM · 架构师补 §17 CLOSED + §18 V1.2-D)
docs/iterations/V1_2_RETRO_REPORT.md               (AM · 架构师补 verdict CLOSED + Closed Debt)
docs/V1_BACKEND_SOURCE_OF_TRUTH.md                 (M · 架构师补 contract state V1.2 CLOSED + §4.1)
prompts/V1_ROADMAP.md                              (MM · 追加 v42 v43)
prompts/V1_2_AUTHORITY_AND_OPENAPI_PURGE.md        (untracked · prompt v1)
prompts/V1_2_RESUME_FROM_P2.md                     (untracked · prompt v2 from-P2)
prompts/V1_2_C_AUDIT_TOOL_REWORK.md                (untracked · V1.2-C 子轮 prompt)
prompts/V1_2_C_GIT_COMMIT_LANDING.md               (untracked · 本 prompt 自身)
```

```
git add docs/iterations/V1_RETRO_REPORT.md docs/iterations/V1_2_RETRO_REPORT.md docs/V1_BACKEND_SOURCE_OF_TRUTH.md prompts/V1_ROADMAP.md prompts/V1_2_AUTHORITY_AND_OPENAPI_PURGE.md prompts/V1_2_RESUME_FROM_P2.md prompts/V1_2_C_AUDIT_TOOL_REWORK.md prompts/V1_2_C_GIT_COMMIT_LANDING.md
git commit -m "docs(governance): V1.2 + V1.2-C closed - architect verdict V1_2_DONE_ARCHITECT_VERIFIED"
```

## §3 verify 矩阵(每 commit 后跑)

| # | check | 期望 |
|---|---|---|
| 1 | `git status --short` 后剩余 dirty 数 | ≤ 上一步预期残留(允许 tmp/、未跟踪 .py 脚本) |
| 2 | `git log --oneline` 头部 commit message | 匹配本 prompt §2 模板 |
| 3 | `git diff HEAD -- transport/ service/ repo/ domain/ cmd/ internal/ migrations/` | 空(业务 Go 路径 0 改动) |
| 4 | `git diff HEAD -- docs/api/openapi.yaml` (C-2 之后) | 空 |
| 5 | `git diff HEAD -- transport/http.go` | 空 |
| 6 | `git diff HEAD -- tools/contract_audit/main.go` (C-4 之后) | 空(content SHA 等于 `fc86c550...` 锚) |
| 7 | `git log -p HEAD~5..HEAD -- '*.go' \| Select-String '^\+\+\+ '`(C-4 之后)的 Go 文件路径全部命中白名单 | PASS · 仅 `tools/contract_audit/` |

## §4 最终 verify(C-6 完成后)

```powershell
# 业务 Go 0 改动(仅 V1.2-C 工具白名单允许)
$businessGo = git log 207f9a1..HEAD --name-only --pretty=format: -- '*.go' | Where-Object { $_ -ne '' } | Where-Object { $_ -notmatch '^tools/contract_audit/' } | Sort-Object -Unique
if ($businessGo) { Write-Host "[FAIL] business Go modified:`n$($businessGo -join "`n")"; exit 1 } else { Write-Host "[OK] business Go 0 changes" }

# 应有 6 个新 commit
$commitCount = (git log --oneline 207f9a1..HEAD | Measure-Object).Count
if ($commitCount -ne 6) { Write-Host "[FAIL] expected 6 commits, got $commitCount"; exit 1 } else { Write-Host "[OK] 6 commits" }

# 工作树关键文件 SHA 仍等于本 prompt §1 baseline
@{
  'docs/api/openapi.yaml'              = '80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f';
  'transport/http.go'                  = '9a6d194b54aa8d49dbff3d10f6d91283e07d68f21e117d2ffb9c2f99a72eb396';
  'tools/contract_audit/main.go'       = 'fc86c550622c3fcdbcd59beca8fe08e7a44b1fecd33c3c9f42dc116ac9f6455d'
}.GetEnumerator() | ForEach-Object {
  $cur = (Get-FileHash -Algorithm SHA256 $_.Key).Hash.ToLower()
  if ($cur -ne $_.Value) { Write-Host "[FAIL] $($_.Key) SHA drift"; exit 1 } else { Write-Host "[OK] $($_.Key)" }
}
```

## §5 落盘报告

新建 `docs/iterations/V1_2_C_LANDING_REPORT.md` 含:

```
- date
- 6 commit hashes(从 git log 抓)
- 每 commit 的 file count + diff size
- baseline SHA 对照(§1 中 10 文件均一致 ✓)
- working tree dirty 残留(预期为空,如有列出)
- terminator: V1_2_C_LANDING_DONE
```

## §6 严禁

- ❌ `git push`
- ❌ 白名单**之外**任何 `*.go` 文件改动(包括 fmt 格式化、空白调整)。白名单见 §0 上方,即 `tools/contract_audit/main.go` + `tools/contract_audit/main_test.go` + `tools/contract_audit/testdata/**/*.go`。
- ❌ `tools/contract_audit/main.go` 内容 SHA 偏离 §1 锚 `fc86c550...`(允许 commit · 不允许内容改)
- ❌ 任何已签字治理文档(retro/SoT/ROADMAP)文本改动
- ❌ `--amend` 已有 baseline commit `207f9a1`
- ❌ 跳过 C-1~C-6 中任意一个,顺序必须严格

## §7 prompt 修订记录

- v1.0 ABORT(2026-04-26):§0 #1 字面与 C-4 commit `tools/contract_audit/**/*.go` 字面冲突 · codex 严守正确 ABORT · 见 `docs/iterations/V1_2_C_LANDING_ABORT_REPORT.md`。
- v1.1 修订(架构师补丁):§0 #1 收窄到"业务 Go 文件" · 显式列白名单 · §0 加 #8 白名单守门 · §3 #3 改为业务路径 diff 检查 · §3 加 #7 commit 内容白名单审计 · §4 改为 PowerShell 脚本带 exit code · §6 严禁条款同步收窄。

## §8 终止符

完成后输出 `V1_2_C_LANDING_DONE` 加 6 个 commit hash。架构师 verify 通过后改签 `V1_2_C_LANDING_ARCHITECT_VERIFIED`。
