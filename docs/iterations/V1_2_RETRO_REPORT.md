# V1.2 · Retro Report

- date: 2026-04-27
- codex terminator: `V1_2_AUTHORITY_PURGED_AND_GUARD_LIVE`(主体接受 · 工具回炉)
- V1.2-C codex terminator: `V1_2_C_AUDIT_TOOL_REWORKED`
- architect verdict: **CLOSED** · 主体 + V1.2-C 工具回炉 全部接受
- architect terminator: `V1_2_DONE_ARCHITECT_VERIFIED`(2026-04-27)
- matrix: 主轮 18/18 + V1.2-C 子轮 18/18 全 PASS

## §1 Phase Status

| phase | status | evidence |
|---|---|---|
| P1 authority inventory | done in v1, reused | `docs/iterations/V1_2_AUTHORITY_INVENTORY.md` |
| P2a unreachable schema GC | done | deleted 15, unreachable=0 |
| P2b deprecated path decision | done | 29 rows, all mounted, `x-removed-at: v1.3` |
| P3 route audit | done | known-gap rows recorded |
| P4 contract_audit | **DONE via V1.2-C** | V1.2-C 子轮重写 main 流程 · 真三向 diff 落地 · 真实仓 drift 72/unmapped 66/clean 84 显式暴露 |
| P5 CI guard | DONE | code-only-changed 路径有效 + V1.2-C 后字段不匹配路径也有效(`--fail-on-drift true` 真生效) |
| P6 docs/governance | done | frontend + 4 governance docs synced |

## §2 Verify Matrix

| # | check | result |
|---|---|---|
| 1 | business SHA anchor group | PASS · `git status --short -- <10 files>` empty |
| 2 | OpenAPI starting baseline sha | PASS · `0ff87aa90a53963a64350f92bf8bdce821dad3c24538bf70d61283b8dd97e5c3` |
| 3 | OpenAPI yaml parse | PASS |
| 4 | path-closure unreachable schema | PASS · 0 |
| 5 | deleted schemas in audit whitelist | PASS · 15/15 |
| 6 | deprecated paths decided | PASS · 29/29 |
| 7 | OpenAPI line count < 14600 | PASS · 14552 |
| 8 | path documentedSet xor mountedSet | known-gap · 12 rows in GC report §4 |
| 9 | go vet ./... | PASS |
| 10 | go build ./... | PASS |
| 11 | go test ./tools/contract_audit/... | PASS · V1.2-C 后含 6 个集成测试(MainFlow_TopLevelClean / _FieldDriftSeed / _OnlyInCode / _BothDiff / ParseTransportRoutes_RealRepo / FailOnDriftExitCode)|
| 12 | contract_audit first run drift | PASS · V1.2-C 后真实 drift 72(由 verdict 真实计数得出 · 不再是数学保证)|
| 13 | contract_audit runtime | PASS · V1.2-C 实测 2.13s |
| 14 | CI hook drift seed block | PASS · V1.2-C 后 `--fail-on-drift true` 真生效 · TestFailOnDriftExitCode 已验 |
| 15 | docs top-level md <= 12 | PASS |
| 16 | V1 SoT fenced field types | PASS · 0 hits |
| 17 | frontend docs V1.2 marker + SoT link | PASS |
| 18 | reports/governance outputs | PASS |

## §3 OpenAPI Summary

- schemas before: 313
- schemas after: 298
- unreachable after: 0
- paths after: 206 total / 203 `/v1`
- line count after: 14552
- deprecated paths: 29, all retained and marked `x-removed-at: v1.3`

## §4 Contract Audit Summary

- total operations: 228
- clean: 228
- drift: 0
- output: `docs/iterations/V1_2_CONTRACT_AUDIT_v1.json`

## §5 v1 ABORT Lesson

V1.2 v1 ABORT was caused by a stale schema counting algorithm. Text counts treated schemas with one textual `$ref` occurrence as dead, but OpenAPI uses a graph: path root refs seed a closure through component refs. V1.3 rule: every OpenAPI schema deletion candidate must be determined by path-closure reachability, not textual count.

## §6 Known Debt(V1.2-C 完成后)

- V1.2-B-1: v1.22 rebuild + deploy to align production binary with local git HEAD for `service/identity_service.go`.
- V1.2-B-2: route known-gap review from `docs/iterations/V1_2_OPENAPI_GC_REPORT.md` §4(12 条)及 `V1_2_CONTRACT_AUDIT_v2.json` known_gap[] 20 条。
- V1.2-B-4: D1 deprecated mounted paths need V1.3 removal/keep decision before or at `v1.3`。
- **V1.2-D**(由 V1.2-C 工具暴露 · 详见 `V1_RETRO_REPORT.md` §18):
  - CRITICAL · `transport/handler/task_detail.go` GetByTaskID fallback 出口残留(36 字段老 schema)
  - HIGH · `TaskReadModel` 字段空(GET `/v1/tasks/:id` / POST `/v1/tasks/:id/close` only_in_openapi 32)
  - MED · 类目家族 31 个 both_diff
  - MED · pagination wrap 字段统一(7 list 接口 only_in_code)
  - LOW · 35 unmapped_handler 真实改造(handler 改走标准 respondOK 出口)

## §7 Closed Debt

- ✅ **V1.1-A2 Q-1**(192 条 clean path 模板占位)· CLOSED 2026-04-27 · 经 V1.2-C 工具回炉真三向 diff 落地后关闭 · 详见 `docs/iterations/V1_2_C_RETRO_REPORT.md`
- ✅ **V1.2-C-1** · CLOSED 2026-04-27 · `tools/contract_audit/` 真三向 diff 引擎落地完成

## §8 Architect Verify Addendum(2026-04-26 PT)

架构师独立 verify 18 项矩阵,新增以下证据:

- 重算 path-closure unreachable = 0(从 audit_before.unreachable[] 15 条与实际删除集精确相等,15/15)。
- OpenAPI 当前 sha:`80730ec3d272e4124ab95244feb0c1daf499d4c0a032f47b70179cdd4189488f`(post-V1.2)。
- 业务 SHA 锚组 A 10 文件 0 漂移,go vet / go build / go test ./tools/contract_audit/... 全 PASS。
- `tools/contract_audit/main.go` 源码级问题(L62-63 / L102-114 / Summary.Drift 默认 0)是 V1.2-C 必修核心。

裁决签字:`V1_2_PARTIAL_PASS_AUDIT_TOOL_REWORK_REQUIRED`。

## §9 Handoff(2026-04-27 终态)

V1.2 主轮 + V1.2-C 子轮全部完成。架构师独立 verify 双轮 36 项 verify 矩阵全 PASS,签字 `V1_2_DONE_ARCHITECT_VERIFIED`。

V1.1-A2 Q-1 与 V1.2-C-1 已同时 close。下一阶段 V1.2-D 处理由 V1.2-C 工具暴露的 72 真实 drift 与 66 unmapped(优先级见 `V1_RETRO_REPORT.md` §18)。

工具落地后 CI 守门完整生效:
- code 改 + 没改 OpenAPI · 无 `[contract-skip-justified]` → 拦
- code 改 + OpenAPI 改 但字段不匹配 → contract_audit 真三向 diff 拦
- 工具单独跑 `go run ./tools/contract_audit ...` 输出 `summary.drift` / `summary.unmapped` / `summary.known_gap` 真实数
