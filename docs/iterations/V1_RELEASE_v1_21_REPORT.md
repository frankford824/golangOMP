# Release v1.21 Report · 2026-04-25

## §0 Verdict

`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`

V1.0(R1~R6.A.4) + V1.1-A1 backend has been deployed as v1.21 to production `jst_erp`. The old v1.20 V0.9 backend was replaced. Production detail P99 is GREEN. Test DB `jst_erp_r3_test` was backed up and dropped. Frontend integration docs are ready under `docs/frontend/`.

## §1 Baseline (P1)

- 11 sha anchors checked in `tmp/v1_21_baseline_sha.log`; OpenAPI and the 7 business anchors matched the release prompt, and V1.1-A1 fast-path files were recorded.
- OpenAPI `/v1` path count: `203`; `paths:` count: `1`; `transport/http.go` `"/v1/` grep count: `66`.
- Dangling `501` count: `0`.
- `go build ./...`: PASS (`tmp/v1_21_build.log`).
- `go vet ./...`: PASS (`tmp/v1_21_vet.log`).
- `go test -count=1 ./...`: PASS (`tmp/v1_21_unit.log`).
- `openapi-validate`: `0 error 0 warning` (`tmp/v1_21_openapi.log`).
- Full integration: `go test -tags=integration -p 1 -count=1 -timeout 60m ./...` PASS (`tmp/v1_21_integration.log`).
- `[80000,90000)` isolation audit: BEFORE/AFTER all zero (`tmp/v1_21_isolation.log`).

## §2 Release v1.21 (P2)

- Production pre-release backup: `/root/ecommerce_ai/backups/20260425T102852Z_pre_v1_21_jst_erp.sql.gz` (175M).
- Deploy command used `deploy/deploy.sh --version v1.21 --release-note ...`.
- Controlled deviation: local `deploy/main.env` and `deploy/bridge.env` were absent; deploy proceeded without those flags, preserving the remote shared env files already used by the release scripts.
- Release history deployed line timestamp: `2026-04-25T10:30:41Z`.
- Deploy directory: `jst_ecs:/root/ecommerce_ai/releases/v1.21`.
- Artifact sha256: `977da0e4561a6baf841f89fca1c2cd0cb1c14b93bb97d981ee72632488a513bc`.
- Runtime verify: `MAIN_STATUS=ok`, `BRIDGE_STATUS=ok`, `SYNC_STATUS=ok`, `OVERALL_OK=true`.

## §3 Production P99 (P3)

- Production task ID query returned 107 eligible non-Archived tasks under `id < 20000`; the P99 runner reused them cyclically for `N=200`.
- Token obtained from production auth identity via `/v1/auth/login`; token was written to `tmp/v1_21_prod_token.txt` without printing it.
- Warm run: `n=200 p50=28.697382ms p95=30.539294ms p99=32.933295ms max=32.945958ms`.
- Cold run: `n=200 p50=28.731575ms p95=30.238283ms p99=32.994852ms max=35.66832ms`.
- Gates: warm `< 80ms` PASS; cold `< 150ms` PASS; non-200 count `0`.

## §4 Test DB Drop (P4)

- Test DB backup: `/root/ecommerce_ai/backups/20260425T103533Z_pre_drop_jst_erp_r3_test.sql.gz` (175M).
- `DROP DATABASE IF EXISTS jst_erp_r3_test` executed.
- Post-drop verify: `SHOW DATABASES LIKE "jst_erp_r3_test"` returned empty (`tmp/v1_21_test_db_drop_verify.log`).
- Remote test ports 18086/18087: no listeners.
- Rebuild scripts retained: `scripts/r35/setup_test_db.sh`, `scripts/r35/build_test_dsn.sh`, and related R35 support code.

## §5 Frontend Docs (P5)

- Generated docs under `docs/frontend/`.
- File count: `16` Markdown files. This intentionally follows the release prompt table (INDEX + 14 family docs + CHEATSHEET), despite the text saying "15".
- `/v1` path sections: `203`.
- CHEATSHEET rows: `203`; one row per `/v1` path, with multiple methods combined in the `Methods` column.
- INDEX sections: `§0` through `§7` present.
- CHEATSHEET line count: `211`.
- WebSocket note: current OpenAPI exposes `/ws/v1`, not `/v1/ws/v1`; `docs/frontend/V1_API_WS.md` records the real OpenAPI path without changing the `/v1` path count.

## §6 Sync (P6)

- `prompts/V1_ROADMAP.md`: status updated; Release-v1.21 overview row added; changelog v30/v31/v32 appended.
- `docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md`: Release v1.21 evidence added; production version and dropped test DB noted.
- `prompts/V1_NEXT_MODEL_ONBOARDING.md`: current state, test DB drop note, completed release/docs note, and next changelog version updated.
- `prompts/V1_1_FRONTEND_INTEGRATION_HANDOFF.md`: production release status, P99 values, DB state, and frontend docs path added.

## §7 Verification Matrix

| # | Item | Result |
|---|---|---|
| V1 | sha anchors 11 files | PASS |
| V2 | OpenAPI `/v1` path count = 203 | PASS |
| V3 | go build / go vet | PASS |
| V4 | go test -count=1 | PASS |
| V5 | openapi-validate 0/0 | PASS |
| V6 | full integration `-p 1 -timeout 60m` | PASS |
| V7 | `[80000,90000)` isolation | PASS |
| V8 | dangling 501 = 0 | PASS |
| V9 | release-history v1.21 deployed | PASS |
| V10 | production detail P99 gates | PASS |
| V11 | `jst_erp_r3_test` dropped | PASS |
| V12 | frontend docs coverage | PASS |
| V13 | roadmap/handoff/onboarding/frontend-handoff sync | PASS |

## §8 终止符

`V1_RELEASE_v1_21_DONE_FRONTEND_DOCS_READY`
