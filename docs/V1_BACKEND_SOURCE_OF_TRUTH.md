# V1 Backend · Source of Truth(当前依据 · 替代 V0.9 SoT 的 V1 主线)

> Tier-2 真相文档 · 不重复字段定义 · 字段查 OpenAPI
> Last verified: 2026-04-27T00:00:00Z
> Authority head: V1.2 · git commit 207f9a1

## §0 SoT 优先级

1. `transport/http.go` — Tier-0 · 路由是否 mount = API 是否存在
2. `docs/api/openapi.yaml` — Tier-1 · 字段契约
3. 本文档 — Tier-2 · 路由家族 / 治理状态 / 里程碑指针
4. 其他 — Tier-3 · 事件流 / 归档 / prompts(指针,不描述契约)

## §1 当前 release 与部署

- production: v1.21
- artifact sha256: `977da0e4561a6baf841f89fca1c2cd0cb1c14b93bb97d981ee72632488a513bc`
- detail P99: warm 32.933ms / cold 32.995ms
- contract state: V1.2-D-2 CLOSED(2026-04-26 PT);post-D-2 OpenAPI sha `94f1ff0d4a2bdf766506153f263c00c7fa7a5fcab02bd2110ca310fd726e25ba`(14936 行 · 209 `/v1` paths)
- contract audit: V1.2-D-2 `tools/contract_audit/` 真三向 diff 工具已上线并收口残留漂移 · final audit `clean=179 / drift=0 / unmapped=0 / known_gap=54`(`docs/iterations/V1_2_D_2_FINAL_AUDIT.json`)· CI 守门 `[contract-skip-justified]` 与 `--fail-on-drift` 双轨生效

## §2 路由家族总览(只列 family · 字段查 OpenAPI)

| family | path prefix | mount 行号 | OpenAPI tag | frontend doc |
|---|---|---:|---|---|
| Auth | `/v1/auth` | `transport/http.go:100` | Auth | `docs/frontend/V1_API_AUTH.md` |
| Me | `/v1/me` | `transport/http.go:112` | Me | `docs/frontend/V1_API_ME.md` |
| Users / Org | `/v1/users`, `/v1/org`, `/v1/departments` | `transport/http.go:153` | Users, Org | `docs/frontend/V1_API_USERS.md`, `docs/frontend/V1_API_ORG.md` |
| ERP | `/v1/erp` | `transport/http.go:228` | ERP | `docs/frontend/V1_API_ERP.md` |
| Tasks | `/v1/tasks` | `transport/http.go:287` | Tasks | `docs/frontend/V1_API_TASKS.md` |
| Customization | `/v1/customization-jobs` | `transport/http.go:369` | Tasks | `docs/frontend/V1_API_TASKS.md` |
| Assets | `/v1/assets` | `transport/http.go:378` | Assets | `docs/frontend/V1_API_ASSETS.md` |
| Drafts | `/v1/task-drafts`, `/v1/me/task-drafts` | `transport/http.go:120` | Drafts | `docs/frontend/V1_API_DRAFTS.md` |
| Notifications | `/v1/me/notifications` | `transport/http.go:141` | Notifications | `docs/frontend/V1_API_NOTIFICATIONS.md` |
| Reports | `/v1/reports/l1` | `transport/http.go:133` | Reports | `docs/frontend/V1_API_REPORTS.md` |
| WebSocket | `/ws/v1` | `transport/http.go:147` | WS | `docs/frontend/V1_API_WS.md` |

## §3 已交付里程碑(指针)

- R6.A.4 · `docs/iterations/V1_RETRO_REPORT.md`
- V1.1-A1 · `docs/iterations/V1_1_A1_DETAIL_P99_REPORT.md`
- v1.21 release · `docs/iterations/V1_RELEASE_v1_21_REPORT.md`
- V1.1-A2 · `docs/iterations/V1_1_A2_RETRO_REPORT.md`
- V1.2 · `docs/iterations/V1_2_RETRO_REPORT.md`

## §4 已知遗留(指针)

- V1.2-B-1 · v1.22 rebuild for local git HEAD / production binary alignment;详见 `docs/iterations/V1_2_RETRO_REPORT.md`。
- V1.2-B-2 · 54 条 classified known-gap(`docs/iterations/V1_2_D_2_FINAL_AUDIT.json` known_gap[])review。
- V1.2-B-4 · 29 D1 deprecated mounted paths 在或之前于 v1.3 做去留决断。
- **V1.2-D** · CLOSED by V1.2-D-1 + V1.2-D-2. See `docs/iterations/V1_2_D_1_REPORT.md` and `docs/iterations/V1_2_D_2_RETRO_REPORT.md`.

## §4.1 已关闭遗留

- ✅ V1.1-A2 Q-1 · CLOSED 2026-04-27(经 V1.2-C 工具回炉关闭)
- ✅ V1.2-C-1 · CLOSED 2026-04-27(`tools/contract_audit/` 真三向 diff 引擎落地)
- ✅ V1.2-D · CLOSED 2026-04-26 PT(`drift=0`, `unmapped=0`, final audit landed)

## §5 反向规则(必读)

- 任何“V1 当前契约字段”问题 → 看 `docs/api/openapi.yaml` · 不要看 prompts/iterations/archive。
- 任何“V1 当前 mount 路由是否存在”问题 → 看 `transport/http.go` · 不要看 OpenAPI。
- 本文档与 `transport/http.go` 或 OpenAPI 冲突 → 以本文档为错 · 立即修。
