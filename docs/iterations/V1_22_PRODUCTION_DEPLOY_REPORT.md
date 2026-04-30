# Release v1.22 Production Deploy Report

Date: 2026-04-30
Target commit: `235cea4`
Release version: `v1.22`

## Verdict

`V1_22_PRODUCTION_DEPLOYED`

The backend was deployed to production as `v1.22`.

Production users are currently low-volume, so this release used the direct
production path requested by the operator: local full gate, production read-only
precheck, backup, schema-state confirmation, direct cutover deploy, and runtime
verification.

## Precheck

Local gate:

- `./scripts/agent-check.sh`: PASS.
- OpenAPI validate: `0 error 0 warning`.
- Contract audit: PASS with no drift.

Production read-only DB precheck:

- Current release before deploy: `/root/ecommerce_ai/releases/v1.21`.
- Database: `jst_erp`.
- MySQL version: `8.0.45`.
- `utf8mb4_0900_ai_ci` support: yes.
- `tasks`: `143` rows.
- `task_sku_items`: `123` rows.
- `urgent` priority rows: `0`.
- `task_search_documents`: present.
- `task_search_documents` rows: `143`.
- `ft_task_search_text`: present.
- SKU filing projection columns: `6/6` present.
- SKU filing projection indexes: `2/2` present.

Migration decision:

- `069_v1_1_task_search_documents.sql`: already applied.
- `070_v1_1_task_sku_item_filing_projection.sql`: already applied.
- Migration execution was skipped to avoid duplicate-column failure from
  rerunning migration 070.

## Backup

Production DB backup was created before deploy:

```text
/root/ecommerce_ai/backups/20260430T082627Z_pre_v1_22_jst_erp.sql.gz
```

Backup verification:

- `gzip -t`: PASS.
- Size: `186340298` bytes.
- SHA256:
  `404cef16d305937de04e6869b388508a82896aa12fb96d79015e237763555329`.

## Deploy

Command:

```bash
bash ./deploy/deploy.sh --version v1.22 --release-note "v1.22 governance-clean HEAD deployment"
```

Release artifact:

- File: `ecommerce-ai-v1.22-linux-amd64.tar.gz`.
- SHA256:
  `1ecf764340981e07341f6a2b43d8ee3f2eef520473d0812e8405da61b2ee3602`.
- Release directory: `/root/ecommerce_ai/releases/v1.22`.

Remote runtime after deploy:

- `current`: `/root/ecommerce_ai/releases/v1.22`.
- MAIN executable: `/root/ecommerce_ai/releases/v1.22/ecommerce-api`.
- MAIN: `status=ok`, health `200`, port `8080`.
- Bridge: `status=ok`, health `200`, port `8081`.
- Sync: `status=ok`, health `200`, port `8082`.
- `OVERALL_OK=true`.

The deploy health check auto-recovered the sync service and verified it was
listening again.

## Postcheck

Remote DB readiness check:

- User/org tables: OK.
- Task flow tables: OK.
- ERP/Bridge related tables: OK.
- Logs/audit tables: OK.
- Enabled legacy departments: `0`.
- Enabled legacy teams under enabled departments: `0`.

Post-deploy schema checks:

- `tasks`: `143` rows.
- `task_search_documents`: `143` rows.
- `urgent` priority rows: `0`.
- SKU filing projection columns: `6/6`.
- SKU filing projection indexes: `2/2`.
- Search fulltext index: `1/1`.

Unauthenticated route boundary smoke:

- `/v1/auth/me`: `401`.
- `/v1/erp/iids`: `401`.
- `/v1/tasks/pool`: `401`.
- `/v1/assets/1`: `401`.

These routes are reachable through the deployed service and remain protected by
authentication.

## Follow-Up

Recommended next checks when an authenticated account is available:

- login smoke with a real user;
- `GET /v1/tasks?page=1&page_size=20`;
- `GET /v1/tasks/{id}/detail`;
- `GET /v1/erp/iids?page=1&page_size=20`;
- one task-pool read and one asset preview/download check.
