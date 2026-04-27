# V1.1 前端联调入口 · Backend Ready Handoff

> Last updated: 2026-04-25
> 前置:V1.1-A1 `/v1/tasks/{id}/detail` P99 architect-verified
> 范围:给前端工程师 / 联调模型的 backend-only 入口说明。

## §1 Backend Status

后端当前状态:

```text
V1.1-A1_DONE_ARCHITECT_VERIFIED
detail cold p99=47.525ms
detail warm p99=47.513ms
detail final warm extended n=500 p99=47.126ms
OpenAPI validate 0 error 0 warning
full integration -p 1 PASS
released_v1_21=2026-04-25T10:30:41Z
prod_detail_p99_warm_ms=32.933
prod_detail_p99_cold_ms=32.995
prod_db=jst_erp
test_db_jst_erp_r3_test=DROPPED
frontend_docs=docs/frontend/INDEX.md (16 files / 203 /v1 paths + /ws/v1 note)
```

## §2 Canonical Routes

新前端只接 canonical MAIN families:

- `/v1/auth/*`
- `/v1/users*`
- `/v1/me*`
- `/v1/erp/products*`
- `/v1/erp/products/by-code`
- `/v1/tasks*`
- `/v1/tasks/{id}/asset-center/*`
- `/v1/tasks/batch-create/*`
- `/v1/me/notifications*`
- `/v1/task-drafts*`
- `/ws/v1` (current OpenAPI WebSocket path)
- `/v1/search`
- `/v1/reports/l1/*`

不要新接:

- `withCompatibilityRoute` 标注路径
- `withDeprecatedRoute` 标注路径
- `/v1/tasks/{id}/audit_a_claim`
- `/v1/tasks/{id}/audit_b_claim`
- 旧资产 alias `/v1/assets/upload-sessions*` 作为新入口

## §3 联调优先级

1. 登录与当前用户:
   - `POST /v1/auth/login`
   - `GET /v1/me`
   - `GET /v1/me/org`

2. 任务列表与详情一屏:
   - `GET /v1/tasks`
   - `GET /v1/tasks/{id}/detail`
   - detail P99 已满足前端首屏联调门。

3. 模块动作:
   - `POST /v1/tasks/{id}/modules/{module_key}/claim`
   - `POST /v1/tasks/{id}/modules/{module_key}/actions/{action}`
   - `POST /v1/tasks/{id}/modules/{module_key}/reassign`
   - `POST /v1/tasks/{id}/modules/{module_key}/pool-reassign`

4. 资产中心:
   - `/v1/tasks/{id}/asset-center/*`
   - `/v1/assets/search`

5. 草稿 / 通知 / WS:
   - `/v1/task-drafts*`
   - `/v1/me/notifications*`
   - `/ws/v1`

6. Excel 批量:
   - `GET /v1/tasks/batch-create/template.xlsx?task_type=new_product_development`
   - `POST /v1/tasks/batch-create/parse-excel`

## §4 Backend Smoke

后端本地 smoke:

```bash
export MYSQL_DSN='root:<TEST_DB_PASSWORD>@tcp(127.0.0.1:3306)/jst_erp_r3_test?parseTime=true&multiStatements=true&loc=Local'
export R35_MODE=1
SERVER_PORT=18087 /home/wsfwk/go/bin/go run ./cmd/server
curl -sS http://127.0.0.1:18087/healthz
```

detail P99:

```bash
SUPER_ADMIN_TOKEN=$(cat tmp/v1_1_a1_super_admin_token.txt) \
BASE_URL=http://127.0.0.1:18087 \
WARMUP=100 N=500 \
/home/wsfwk/go/bin/go run tmp/v1_1_a1_p99_runner.go
```

## §5 联调硬门

- 前端不得依赖 compatibility/deprecated 路由。
- 前端请求失败必须展示后端 `error.code` / `deny_code`。
- 任务详情首屏只调 `GET /v1/tasks/{id}/detail`,不要拆成多个旧接口拼装。
- 模块按钮状态以 detail 返回的 modules/action/scope 为准,不要本地硬编码权限矩阵。
- 批量 Excel parse 只做 preview,真正创建仍走 `POST /v1/tasks`。

## §6 后端后继

联调可以开始,但后端仍建议继续 V1.1-A2:

- CI 集成测试包级并行守卫,强制 shared-DB integration 使用 `-p 1`。
- 测试稳定性统一轮可并行规划,但不要阻塞前端首轮联调。
