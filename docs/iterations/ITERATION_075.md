# ITERATION_075

## Goal

Strictly converge this round to two lines:

1. push `8081` Bridge toward real external-ERP remote writeback acceptance
2. complete multi-role runtime verification for `/v1/erp/*`, and apply minimal permission fix if current policy is over-open

## Scope

- No frontend change
- No unrelated business logic refactor
- No version bump (still `v0.4`)
- Keep `8080/8081/8082` live stability

## Confirmed Facts

- Live deploy host remains `223.4.249.11`, with `cmd/server/main.go` as shared source entry.
- Post-deploy process state is healthy:
  - `main` pid `3507849`, exe `/root/ecommerce_ai/releases/v0.4/ecommerce-api`
  - `bridge` pid `3507876`, exe `/root/ecommerce_ai/releases/v0.4/erp_bridge`
  - `sync` pid `3508034`, exe `/root/ecommerce_ai/erp_bridge_sync`
  - all `exe_deleted=false`
- Live bridge remote config after this round is still:
  - `ERP_REMOTE_MODE=local`
  - `ERP_REMOTE_BASE_URL=` (empty)
  - `ERP_REMOTE_AUTH_MODE=none`
  - `ERP_REMOTE_AUTH_HEADER_TOKEN/APP_KEY/APP_SECRET/ACCESS_TOKEN` all empty
  - timeout/retry/fallback keys exist (`15s`, `2`, `600ms`, `true`)
- Therefore external ERP formal remote prerequisites are still incomplete; no safe condition to switch live to `hybrid/remote`.
- Pre-fix runtime verification proved current permission was too open:
  - a roleless valid session could call `GET /v1/erp/products` and `POST /v1/erp/products/upsert` with `200`.
- Minimal fix was applied only on ERP route role guards in `transport/http.go`:
  - read routes now require one of: `Ops/Designer/Audit_A/Audit_B/Warehouse/Outsource/ERP/Admin`
  - write routes now require one of: `Ops/Warehouse/ERP/Admin`
  - sync-log routes now require one of: `Ops/Warehouse/ERP/Admin`
- Post-fix dual-role verification (real accounts/tokens) on bridge localhost:
  - `Admin` and `Ops` both returned `200` for:
    - `GET /v1/erp/products`
    - `GET /v1/erp/products/{id}`
    - `GET /v1/erp/categories`
    - `POST /v1/erp/products/upsert`
    - `GET /v1/erp/sync-logs`
    - `GET /v1/erp/sync-logs/{id}`
    - `POST /v1/erp/products/shelve/batch`
    - `POST /v1/erp/products/unshelve/batch`
    - `POST /v1/erp/inventory/virtual-qty`
- Post-fix roleless verification:
  - roleless valid session now gets `403` on:
    - `GET /v1/erp/products`
    - `GET /v1/erp/categories`
    - `POST /v1/erp/products/upsert`
- Minimal regression after deploy is green:
  - `8081`: `/health`, read ERP, write ERP all `200` (authorized roles)
  - `8080`: `/health` and `/v1/tasks` `200`
  - `8082`: `/health` and `/internal/jst/ping` `200`
- Bridge log scan on latest file (`/root/ecommerce_ai/logs/erp_bridge-20260317T055252Z.log`) shows no `remote` or `fallback` markers, consistent with live `local` mode.

## Inferences

- The bridge side is code-ready for remote/hybrid mode, but external formal writeback acceptance is blocked by missing external connection contract and credentials, not by current repo code path.
- Current policy after fix is now role-graded and no longer equivalent to “any valid session can write ERP”.

## Verification Steps

1. Read live `bridge.env` and bridge process env for all `ERP_REMOTE_*` keys.
2. Verified remote prerequisites completeness status (base URL/auth/sign credentials/paths).
3. Ran pre-fix roleless runtime probe and captured over-open `200` behavior.
4. Applied minimal ERP route role guard update in `transport/http.go`.
5. Ran `go test ./transport/...`.
6. Redeployed in-place with `bash ./deploy/deploy.sh --version v0.4 ...` (no version bump).
7. Re-verified post-deploy process/exe state and three-service health.
8. Executed post-deploy dual-role (`Admin`, `Ops`) runtime verification for all `/v1/erp/*` routes.
9. Executed post-deploy roleless runtime verification and confirmed `403` gating.
10. Re-scanned bridge logs for `remote/fallback` markers and confirmed live remains local-path only.

## External Remote Acceptance Status

- Attempted: yes (runtime/env/log evidence audit completed)
- Completed: no
- Blocking dependencies:
  1. missing external ERP `base_url`
  2. missing remote auth credentials (`token/app_key/app_secret/access_token`)
  3. missing finalized signing/timestamp/nonce external acceptance contract evidence
  4. whitelist/network-side confirmation not available in this round
- Live final mode after this round: `local` (kept safe)

## Files Changed

- `transport/http.go`
- `CURRENT_STATE.md`
- `MODEL_v0.4_memory.md`
- `MODEL_HANDOVER.md`
- `docs/iterations/ITERATION_075.md`
- `ITERATION_INDEX.md`

