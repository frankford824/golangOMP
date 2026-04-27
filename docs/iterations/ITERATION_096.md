# ITERATION_096

Title: MAIN owner_team compatibility guardrail hardening, explicit compat freeze, overwrite publish to existing `v0.8`, and live verification

Date: 2026-03-31
Model: GPT-5 Codex

## 1. Goal
- Harden the already-live create-time `owner_team` compatibility bridge so later org / validation / batch changes cannot silently widen or break acceptance.
- Keep the architecture boundary unchanged:
  - task-side `owner_team` remains the legacy compatibility field
  - `/v1/org/options` remains the account-org department/team source
  - this round does not unify the org model and does not rewrite historical `tasks.owner_team`

## 2. Runtime hardening
- Replaced the derived compat bridge in `service/task_owner_team.go` with an explicit fixed mapping list.
- Added read-only helper `service.ListTaskOwnerTeamCompatMappings()` so tests and handover docs can reference the exact approved bridge set.
- Preserved the existing create-time log surface:
  - `raw_owner_team`
  - `normalized_owner_team`
  - `owner_team_mapping_applied`
  - `mapping_source`
- `mapping_source` guardrail remains:
  - `legacy_direct`
  - `org_team_compat`
  - `invalid`

## 3. Fixed compat mapping
- `运营一组` -> `内贸运营组`
- `运营三组` -> `内贸运营组`
- `运营七组` -> `内贸运营组`
- `定制美工组` -> `设计组`
- `设计审核组` -> `设计组`
- `采购组` -> `采购仓储组`
- `仓储组` -> `采购仓储组`
- `烘焙仓储组` -> `采购仓储组`

## 4. Test hardening
- Added service-level guardrail tests for:
  - exact compat mapping inventory
  - table-driven direct / compat / invalid normalization
  - invalid owner-team error semantics
  - create-path normalization log output
  - batch compat regression (`new_product_development`, `purchase_task`, original batch reject)
- Expanded handler/API coverage so all approved compat samples are accepted through `POST /v1/tasks` and invalid teams still return `invalid_owner_team`.
- Existing create regression remained covered for:
  - `original_product_development` + `owner_team="运营三组"`
  - `new_product_development` + `owner_team="运营三组"`
  - `purchase_task` + `owner_team="运营三组"`
  - invalid team rejection

## 5. Local verification
- Required commands passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional targeted owner-team checks passed:
  - `go test ./service -run "OwnerTeam|OriginalProductWithOrgTeamCompatOwnerTeamPasses|NewProductWithOrgTeamCompatOwnerTeamPasses|PurchaseTaskWithOrgTeamCompatOwnerTeamPasses"`
  - `go test ./transport/handler -run "OwnerTeam"`

## 6. Publish
- Used the existing deploy chain only:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team compatibility guardrail hardening"`
- Result:
  - existing `v0.8` overwritten in place
  - target release dir remained `/root/ecommerce_ai/releases/v0.8`
  - runtime verification in deploy succeeded on `8080`, `8081`, `8082`

## 7. Live verification
- Health:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
- Active executables:
  - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
- Live task create:
  - `new_product_development` + `owner_team="运营三组"` -> `201`, task `156`, returned `owner_team="内贸运营组"`
  - `new_product_development` + `owner_team="不存在的组"` -> `400 INVALID_REQUEST`, `violations[].field=owner_team`, `violations[].code=invalid_owner_team`

## 8. Guardrail reminder
- `/v1/org/options` teams must never be auto-treated as task `owner_team` truth.
- Any new org team that should be accepted by task create must add:
  - explicit compat mapping
  - regression coverage
- This round is compatibility hardening only, not org-model unification.
