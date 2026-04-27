# ITERATION_095

Title: MAIN owner_team create compatibility closure, overwrite publish to existing `v0.8`, and live verification

Date: 2026-03-31
Model: GPT-5 Codex

## 1. Goal
- Close the live task-create blocker where frontend payloads were sending new org-tree teams such as `运营三组` into task `owner_team`.
- Keep the architecture boundary unchanged:
  - `/v1/org/options` remains the account-org source
  - task `owner_team` remains the task-side legacy compatibility field
  - no history rewrite and no task/account org-model merge
- Complete the full loop in one round:
  - root-cause audit
  - code fix
  - tests
  - overwrite publish to existing `v0.8`
  - live acceptance
  - doc sync

## 2. Root cause
- Task create still validated `owner_team` only against the legacy task enum source:
  - `domain.DefaultDepartmentTeams`
  - `domain.ValidTeam`
  - `service.validateCreateTaskEntry`
- `/v1/org/options` was already serving the newer account-org tree from:
  - `domain.DefaultOrgDepartmentTeams`
- Therefore a frontend payload such as `owner_team="运营三组"` hit the create validator before any original-product defer-local-binding logic and failed with:
  - `INVALID_REQUEST`
  - `violations[].code=invalid_owner_team`

## 3. Code change
- Added a create-time compatibility bridge in `service/task_owner_team.go`:
  - direct legacy values pass through with `mapping_source=legacy_direct`
  - supported org-team values normalize into legacy task owner teams with `mapping_source=org_team_compat`
  - unsupported values remain invalid with `mapping_source=invalid`
- Mapping is derived from existing truth sources instead of hard-coded handler branches:
  - org teams from `domain.DefaultOrgDepartmentTeams`
  - task mappings from `domain.DefaultTaskTeamMappings`
- Current compat coverage:
  - `运营一组` ~ `运营七组` -> `内贸运营组`
  - `定制美工组` / `设计审核组` -> `设计组`
  - `采购组` / `仓储组` / `烘焙仓储组` -> `采购仓储组`
- `service/task_service.go` now:
  - normalizes `owner_team` during create-param normalization
  - keeps raw owner-team input for diagnostics
  - logs `trace_id`, `task_type`, `raw_owner_team`, `normalized_owner_team`, `owner_team_mapping_applied`, `mapping_source`
- Error contract stayed unchanged:
  - still `INVALID_REQUEST`
  - still machine-readable `violations`
  - still `code=invalid_owner_team` for unsupported teams

## 4. Local verification
- Required commands passed:
  - `go test ./service ./transport/handler`
  - `go build ./cmd/server`
  - `go build ./repo/mysql ./service ./transport/handler`
- Additional targeted regressions passed:
  - `go test ./service -run "OwnerTeam|Batch|OriginalProductWithProductSelectionDeferBindingPasses"`
  - `go test ./transport/handler -run "OwnerTeam|Batch"`
- Added coverage:
  - service success cases for original/new/purchase with `owner_team="运营三组"`
  - service failure cases for empty / unknown / random invalid owner-team values
  - handler/API coverage for compat success and invalid-team rejection

## 5. Publish
- Repository deploy chain used:
  - `bash ./deploy/deploy.sh --version v0.8 --skip-tests --release-note "overwrite v0.8 owner_team create compatibility fix"`
- Result:
  - existing `v0.8` was overwritten in place
  - release directory remained `/root/ecommerce_ai/releases/v0.8`
  - `deploy/release-history.log` recorded `packaged`, `uploaded`, and `deployed`
- Packaged artifact:
  - `dist/ecommerce-ai-v0.8-linux-amd64.tar.gz`
  - SHA-256 `909e8d76e32f523b0f7f76a3fb5644946988c9fed3b5f3b57fc9cfbe44004ba7`
- Production entrypoint remained:
  - `./cmd/server`

## 6. Live verification
- Runtime health after overwrite:
  - `8080 /health` = `200`
  - `8081 /health` = `200`
  - `8082 /health` = `200`
- Active executable pointers:
  - `8080` -> `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
  - `8081` -> `/root/ecommerce_ai/releases/v0.8/erp_bridge`
  - `8082` -> `/root/ecommerce_ai/erp_bridge_sync`
- No active `/proc/<pid>/exe` pointer was in `(deleted)` state.
- Live task-create acceptance with a real bearer session:
  - `original_product_development` + defer-local-binding + `owner_team="运营三组"` -> `201`, task `150`, returned `owner_team="内贸运营组"`
  - `new_product_development` + `owner_team="运营三组"` -> `201`, task `151`, returned `owner_team="内贸运营组"`
  - `purchase_task` + `owner_team="运营三组"` -> `201`, task `152`, returned `owner_team="内贸运营组"`
  - illegal team (`不存在的组`) -> `400 INVALID_REQUEST` with `field=owner_team` and `code=invalid_owner_team`
- Live log proof from the new normalization log:
  - compat success requests logged `mapping_source=org_team_compat`
  - illegal request logged `mapping_source=invalid`

## 7. Scope boundary kept
- This round did not:
  - migrate historical `tasks.owner_team`
  - replace task validation with `/v1/org/options`
  - merge task org semantics with account org semantics
  - rewrite the org model or `/v1/org/options` contract
