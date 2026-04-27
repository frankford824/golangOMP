# V1 · R4 Retrospective(SA-A + SA-B + SA-C + SA-D 联合回顾轮)

> 发布:2026-04-25
> 性质:**纯证据轮 · 不改一行代码 · 不动一行 OpenAPI**
> 上游签字依赖:R4-SA-A v1.0、R4-SA-B v1.0、R4-SA-C v1.0、R4-SA-D v1.0(均已签字)
> 产出:`docs/iterations/V1_R4_RETRO_REPORT.md`

---

## 0. 角色与目标

你是 R4 整轮的**回顾(Retrospective)执行者**。R4 已分四轮(SA-A/B/C/D)分别签字;
本轮唯一任务是:**用一组联合证据再次确认 R4 整体没有隐性回归 / 隐性触点 / 隐性外溢**。

**严禁触达**:

- ❌ 任何代码改动(`internal/**`、`transport/**`、`migrations/**`、`cmd/**`)
- ❌ 任何 `docs/api/openapi.yaml` 改动
- ❌ 任何生产数据库写入(probe 必须只读 `SELECT`)
- ❌ 任何 R4 报告本身的改动(SA-A/B/C/D 报告已签字 · 本轮只读)

**允许写入**:

- ✅ `docs/iterations/V1_R4_RETRO_REPORT.md`(新建本轮报告)
- ✅ `docs/iterations/r4_retro_*.log`(各类证据日志)
- ✅ `tmp/r4_retro_*.sh`(过程脚本)
- ✅ 测试库 `jst_erp_r3_test` 的临时数据(用 `t.Cleanup` 清完)

---

## 1. 输入依赖(必读 · 仅读)

1. `docs/iterations/V1_R4_SA_A_REPORT.md`(资产 7 触点)
2. `docs/iterations/V1_R4_SA_B_REPORT.md`(组织 + 用户管理 14 触点)
3. `docs/iterations/V1_R4_SA_C_REPORT.md`(草稿 + 通知 + WS 11 触点)
4. `docs/iterations/V1_R4_SA_D_REPORT.md`(全局搜索 + L1 报表 4 触点)
5. `docs/api/openapi.yaml`(只读 · 用于 path 清点)
6. `transport/http.go`(只读 · 用于 mount 清点)
7. `prompts/V1_ROADMAP.md`(只读 · 确认 R4 整体已签字)

---

## 2. 硬约束(违反任何一条 = abort 并写明)

H1. 所有代码 / OpenAPI 文件树 git diff 必须为空(可生成 `tmp/r4_retro_repo_diff.log` 自证)
H2. 整 4 个域的 integration 联跑必须 0 fail(SA-A + SA-B + SA-C + SA-D + R3 全集)
H3. `make build` / `go build -tags=integration ./...` 双 tag 全过
H4. `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml` 须 0 error 0 warning
H5. 生产 post-probe 4 域控制字段窗口内增量 = 0(非 0 必逐条解释 · 否则 abort)
H6. 联合 live smoke 36 条 R4 触点全返预期(200/2xx 或预期 4xx deny code · 不得 5xx)
H7. `t.Cleanup` 后 `jst_erp_r3_test` 库 `id >= 50000` 测试残留 = 0(独立 SSH 校验)
H8. `transport/http.go` 内对 R4 36 触点全部以**真实 handler**挂载(不得有 501)

---

## 3. 任务清单(按序)

### 3.1 Step A · 静态健全检查(预期 5 分钟)

```bash
# 文件树洁净度
git status --porcelain | tee tmp/r4_retro_repo_diff.log
# 应仅出现 docs/iterations/V1_R4_RETRO_REPORT.md / tmp/r4_retro_*.* / docs/iterations/r4_retro_*.log

# 双 tag 编译
go build ./... 2>&1                                | tee tmp/r4_retro_build_default.log
go build -tags=integration ./... 2>&1              | tee tmp/r4_retro_build_integration.log

# OpenAPI lint
go run ./cmd/tools/openapi-validate docs/api/openapi.yaml 2>&1 | tee tmp/r4_retro_openapi_validate.log

# 501 残留扫描(R4 已 4 轮全实装 · 应为 0)
grep -nR "Not Implemented" transport/ internal/ 2>&1 | tee tmp/r4_retro_501_scan.log
grep -nR "501" transport/ 2>&1 | tee tmp/r4_retro_501_scan_extra.log
```

### 3.2 Step B · R4 触点路径清点(预期 5 分钟)

从 4 个 R4 报告头的 "覆盖路径" 段中提取所有 R4 触点(预期 36 条:7+14+11+4),
存入 `tmp/r4_retro_paths.txt`,每行一条 `METHOD /v1/...` 形式。
作为后续 live smoke 的输入清单。

### 3.3 Step C · 联合 integration 批跑(预期 10-15 分钟)

```bash
# 用 R3 已建立的 jst_erp_r3_test 隧道 + DSN
# 跑 4 域 integration suite 一次性
go test -tags=integration ./... -run 'SAAI|SABI|SACI|SADI|R3' -count=1 -timeout 30m 2>&1 \
  | tee tmp/r4_retro_integration.log
```

要求 0 FAIL。任何 FAIL 抓 stack + 报告里逐条贴出 → abort。

### 3.4 Step D · 联合 live smoke(预期 15 分钟)

1. 起后端进程(端口默认 8080):
   `setsid bash -c 'go run ./cmd/server >/tmp/r4_retro_server.log 2>&1' &`
2. 等 health 200(`curl -fsS http://127.0.0.1:8080/healthz`)。
3. 用 SuperAdmin / DeptAdmin / 普通员工 三种身份各生成一个测试 JWT(走 `internal/testutil/jwt.go` 等已有工具)。
4. 对 `tmp/r4_retro_paths.txt` 中的每条触点用合理身份打 1 次:
   - 期望状态码:200/201/204(成功)、403(预期 deny)、404(资源不存在)
   - 严禁出现 5xx
   - 抓 latency · 算 P95
5. 输出 `tmp/r4_retro_live_smoke.json`(每条 path 一行 JSON:method/path/role/status/ms)
6. 关闭后端进程(`pkill -f 'cmd/server'`)。

### 3.5 Step E · 生产 post-probe(预期 5 分钟)

主对话已在 Retro 启动前跑过 `tmp/r4_retro_probe.sh` 的 baseline →
`docs/iterations/r4_retro_probe_pre.log`。

你跑完 Step D 后,用 baseline log 里的 `mysql_server_time_utc` 作为 `RETRO_START_TIME`,
通过 SSH(`ssh jst_ecs`)再跑一次 probe,落 `docs/iterations/r4_retro_probe_post.log`。

```bash
RETRO_START="$(grep mysql_server_time_utc docs/iterations/r4_retro_probe_pre.log | head -1 | awk -F': ' '{print $2}')"
echo "$RETRO_START" > tmp/r4_retro_start_ts.txt

# 复用 ssh -T 的方式
ssh jst_ecs "bash /tmp/r4_retro_probe.sh '$RETRO_START'" \
  > docs/iterations/r4_retro_probe_post.log 2>&1
```

### 3.6 Step F · 测试库残留校验(预期 2 分钟)

复用 R4-SA-D 的 `tmp/verify_sa_d_isolation.sh` 形式,但库改成 `jst_erp_r3_test`、
ID 范围 `[50000, 60000)`,确保 `users / tasks / task_modules / task_module_events /
notifications / task_drafts / permission_logs / org_move_requests / task_assets`
9 张表全为 0。落 `tmp/r4_retro_isolation.log`。

### 3.7 Step G · 写报告(预期 10 分钟)

按 §6 模板写 `docs/iterations/V1_R4_RETRO_REPORT.md`。

---

## 4. 不准做的事

- 不准修任何 R4 子轮的代码、报告、OpenAPI
- 不准在生产库做 `INSERT/UPDATE/DELETE/DDL`(包括 R3 已签字的 migration 也不动)
- 不准修 `prompts/V1_ROADMAP.md`(由架构师在签字时统一更新)
- 不准修 `docs/V1_MODULE_ARCHITECTURE.md` / `docs/V1_INFORMATION_ARCHITECTURE.md`
- 不准启动 codex 之外的子 agent 或并发 codex
- 不准用 `--yolo` / 跳过 sandbox 之外的任何额外提权

---

## 5. 失败处理

- 任何 H1-H8 不达标 → 立即停 · 在报告里写明哪条 H · 提供最小复现命令 · 不要试图掩盖
- 测试 P95 超过 SA-A/B/C/D 各自报告里的目标 1.5x → 标记 `WARN-PERF` · 不算 abort · 但报告里提
- 如果你发现某条 R4 触点的 handler 实际行为和它在 OpenAPI 里的契约不一致 → 标记 `DRIFT-CONTRACT` · 在报告里逐条列 · **不要**自己改代码或 OpenAPI

---

## 6. 报告模板(`docs/iterations/V1_R4_RETRO_REPORT.md`)

```markdown
# V1 · R4 Retrospective Report

发布:<UTC ISO8601>
范围:R4-SA-A v1.0 + R4-SA-B v1.0 + R4-SA-C v1.0 + R4-SA-D v1.0 联合回顾
执行者:codex exec autopilot
裁决:<PASS / PASS-WITH-WARN / FAIL>

## 1. 静态健全
- repo diff 净度: <git status 摘要>
- build default tag: <PASS/FAIL · 关键 line>
- build integration tag: <同上>
- openapi-validate: <error/warn 条数>
- 501 残留扫描: <transport/ 命中 N · internal/ 命中 N>

## 2. R4 触点清点
- 总条数:<N>(预期 36)
- SA-A: <列出 7 条>
- SA-B: <列出 14 条>
- SA-C: <列出 11 条>
- SA-D: <列出 4 条>

## 3. 联合 integration 批跑
- 命令:`go test -tags=integration ./... -run 'SAAI|SABI|SACI|SADI|R3'`
- 结果:<PASS count / FAIL count / SKIP count>
- 总耗时:<X 分钟>

## 4. 联合 live smoke
- 总条数:<N> · 5xx 数:<0>
- P95(ms):<X>(对照 SA-D 目标 500ms / SA-C 目标 300ms / 等)
- 详 JSON:`tmp/r4_retro_live_smoke.json`

## 5. 生产 post-probe diff
- baseline_ts: <来自 pre log>
- 4 域控制字段窗口内增量:
  - SA-A 三计数:<...>
  - SA-B 两计数:<...>
  - SA-C 三计数:<...>
  - SA-D 两计数:<...>
- 跨域 deny_code 全分布:<节选>
- 5 张冻结枚举(notification_type / module_key / event_type / priority / asset_type / source_module_key)非法行计数:<全 0?>

## 6. 测试库残留
- 9 张表 [50000, 60000) 计数:<全 0>

## 7. 已发现的 DRIFT(若有)
- DRIFT-CONTRACT-N:<path· 行为 vs 契约的差>
- DRIFT-DOC-N:<和某文档 §X.Y 不一致的点>
- (无 → 写"无")

## 8. 裁决
- 整体裁决:<PASS / PASS-WITH-WARN / FAIL>
- 关键证据三条:<...>
- 推荐下一步:<R5 起草 / 修复某条 DRIFT 后再 retro>
```

---

## 7. 验收门(给主对话架构师用)

主对话架构师将用独立脚本 `tmp/verify_r4_retro.sh` 校验:

- V1. `git status --porcelain | grep -v 'iterations\|tmp\|r4_retro' | wc -l == 0`
- V2. `tmp/r4_retro_build_default.log` 末尾无 `error`
- V3. `tmp/r4_retro_build_integration.log` 末尾无 `error`
- V4. `tmp/r4_retro_openapi_validate.log` 含 `0 error 0 warning`
- V5. `grep -c FAIL tmp/r4_retro_integration.log == 0`
- V6. `tmp/r4_retro_live_smoke.json` 内 `status >= 500` 行数 == 0
- V7. `docs/iterations/r4_retro_probe_post.log` 内 SA-A/B/C/D 控制字段窗口内增量计数全 = 0
- V8. `tmp/r4_retro_isolation.log` 内 9 张表测试 ID 残留全 = 0
- V9. `docs/iterations/V1_R4_RETRO_REPORT.md` 含完整 §1-§8 章节

任何一条 V 失败 → 整 Retro FAIL · 主对话发原因给 codex 让其补 · 不要尝试 amend 旧报告。
