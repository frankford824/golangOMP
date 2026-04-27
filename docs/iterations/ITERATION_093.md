# ITERATION_093

Title: Non-explicit historical normalization audit, wider live regression, and second-round cleanup boundary freeze on `v0.8`

Date: 2026-03-23
Model: GPT-5 Codex

## 1. Background and fixed boundary
- `ITERATION_091` org/permission minimum closure remained frozen.
- `ITERATION_092` already finished:
  - explicit dirty-data cleanup
  - retained historical `product_id` repair
  - historical task `500` closure
- This round therefore did **not** reopen:
  - upload-chain code
  - task detail aggregate code
  - org/permission code
  - second-round bulk delete
- The only goals of this round were:
  - widen historical-data audit from explicit markers to non-explicit suspicious residue
  - widen live regression coverage
  - freeze keep/manual/delete boundaries before any second-round mutation

## 2. Evidence source used in this round
- Live DB evidence:
  - on-host MySQL read against `jst_erp`
  - connection resolved from `/root/ecommerce_ai/shared/main.env`
- Live API evidence:
  - on-host login through `POST /v1/auth/login`
  - session-backed bearer requests against `http://127.0.0.1:8080`
- No conclusion in this iteration was based on guesswork, static docs alone, or local mock data.

## 3. DB negatives re-confirmed
- `existing_product` consistency remained closed:
  - `existing_product + product_id IS NULL = 0`
  - `tasks.product_id -> products.id` missing target = 0
  - exact `tasks.sku_code != products.sku_code` mismatch on bound existing-product tasks = 0
  - `new_product` task carrying unexpected `product_id` = 0
- Task JSON integrity remained clean:
  - invalid `product_selection_snapshot_json` = 0
  - invalid `matched_mapping_rule_json` = 0
  - invalid `reference_file_refs_json` = 0
- Core task relations remained clean:
  - missing `creator/designer/current_handler` refs = 0
  - missing `task_details` = 0
  - orphan `task_assets` = 0
  - orphan `design_assets` = 0
  - `asset_storage_refs(owner_type=task_asset)` missing owner row = 0
- Current task enums remained on the supported mainline:
  - `source_mode`: `existing_product`, `new_product`
  - `task_type`: `original_product_development`, `new_product_development`, `purchase_task`
  - `task_status`: `PendingAssign`, `InProgress`, `Completed`, `PendingAuditA`, `PendingClose`
  - `filing_status`: `pending_filing`, `filed`, `not_filed`

## 4. What new suspicious residue was found

### 4.1 Deterministic normalization candidate: `asset_storage_refs`
- Found 9 rows where:
  - `owner_type = task_asset`
  - `asset_storage_refs.asset_id = task_assets.id`
  - but the correct current design-asset foreign key is `task_assets.asset_id`
- This is not a delete signal. It is a deterministic field-level normalization candidate.
- Affected refs:
  - `9a2f3635-94eb-4f05-bb51-0397646b7ad9`
  - `a21568b5-c098-4bfb-acbc-1ed2d1968379`
  - `93f6557e-7a89-47b4-83c3-c3835384e59b`
  - `70ea67f6-9aad-4d1d-99c3-65eb381c9950`
  - `1c54dc1d-3ee2-4067-8f58-8132b0d4444d`
  - `088c9bdd-4843-4fa0-ae0d-4912ae38b541`
  - `ff1a4601-05bf-465a-b246-fdae510fc757`
  - `ffff03bf-2d48-4eca-a21f-57803dbc250f`
  - `9f841583-d77b-404c-bfaa-cd47623ebbd5`
- Proven mapping:
  - wrong `asset_id = 44~52`
  - correct `task_assets.asset_id = 35~43`
  - affected tasks = `144`, `145`

### 4.2 Non-explicit suspicious task cluster
- The audit widened beyond the previous explicit `test/demo/accept/case` boundary.
- New suspicious signals included:
  - Chinese verification words such as `验收`, `联调复测`, `黑盒`
  - verification/probe lineage such as `verify`, `probe`, `BRIDGE-REMOTE-CHECK`, `ERP Stub`, `Step87`, `Codex`
  - suspicious creator/designer/handler lineage:
    - `bb_ops3`
    - `ops_remote_0317`
    - `test_a`
    - `test_01`
    - `一流测试`
    - `bb_designer3`

## 5. Final candidate buckets frozen by this round

### 5.1 保留并归一
- `asset_storage_refs` deterministic normalization rows: 9
- Reason:
  - real task-asset linkage still exists
  - current live read path is healthy
  - field repair is deterministic
  - deleting the owning tasks would be incorrect

### 5.2 需人工确认
- Task IDs:
  - `95,96,97,106,112,113,114,115,116,117,118,119,120,122,124,125,128,130,131,132,134,135,137,138,139,140,142,144,145`
- Why these stayed in manual-confirm instead of clear-delete:
  - business-like product names or business-like change requests
  - active workflow state
  - linked `reference_file_refs`
  - linked `task_assets` / `design_assets`
  - historical test/verification actor involvement but not enough deletion certainty
- Examples:
  - `112`, `113`, `132`: linked `reference_file_refs`
  - `124`: `task_assets=1`, `design_assets=0`, still `PendingClose`
  - `137`, `142`, `144`, `145`: already linked to delivery/read-model assets
  - `95`, `96`: `联调复测` semantics, but business-like product payload and no stronger deletion proof

### 5.3 明确可删
- Task IDs:
  - `47,48,49,58,59,61,62,69,71,72,73,74,75,76,98,99,100,111,121,123,126`
- Why these are in the clear-delete bucket:
  - task content itself carries synthetic verification semantics
  - no business asset evidence strong enough to keep them
  - `design_assets = 0` across the whole bucket
  - most also have `reference_file_refs = 0` and `task_assets = 0`
- Bucket highlights:
  - `47,48,49`: `live verify`
  - `58,59,61,62`: `黑盒V04`
  - `69,71,73`: `Verify`, `BRIDGE-REMOTE-CHECK`, `Roleless Verify`
  - `72`: `ERP Stub Product A`
  - `74,75,76`: obvious placeholder/gibberish new-product tasks under test-user lineage
  - `98,99,100`: `验收defer路径` + `ERP acceptance`
  - `111`: `reference image small verify`
  - `121`: `测试新品`
  - `123,126`: `Step87`
- Important caution:
  - `58` and `61` still carry one `task_asset` row each, so later delete must cascade deliberately instead of deleting only the parent task row

## 6. Old org-field audit result
- Live `/v1/org/options` currently returns:
  - 7 departments
  - 14 teams
- Live `users` still contain 27 legacy org-field rows outside that new catalog:
  - blank department/team = 4
  - `人力行政中心 / 人力行政组` = 5
  - `设计部 / 设计组` = 5
  - `内贸运营部 / 内贸运营组` = 8
  - `采购仓储部 / 采购仓储组` = 2
  - `总经办 / 总经办组` = 3
- But task `owner_team` is a separate compatibility boundary:
  - repo code explicitly keeps legacy task teams as the valid `owner_team` source
  - see `domain.DefaultDepartmentTeams`
  - see `domain.ValidTeam`
  - see `service.validateCreateTaskEntry`
- Therefore:
  - legacy user org fields can be normalized later under the org-management lane
  - task `owner_team` must not be bulk-migrated inside this data-cleanup lane

## 7. Wider live regression result
- Full current task-set scan:
  - live task count = 66
  - `GET /v1/tasks/{id}` non-`200` = 0
  - `GET /v1/tasks/{id}/product-info` non-`200` = 0
  - `GET /v1/tasks/{id}/cost-info` non-`200` = 0
- Targeted suspicious / retained sample tasks also re-read successfully:
  - `58`, `59`, `61`, `62`, `99`, `106`, `113`, `123`, `124`, `125`, `126`, `137`, `139`, `142`, `144`
  - all detail/product-info/cost-info probes returned `200`
- Task list widened regression:
  - page 1 = `200`
  - page 2 = `200`
  - page 3 = `200`
  - page 4 = `200`
  - `task_type=original_product_development` filter = `200`
  - `task_type=purchase_task` filter = `200`
- Org / permission widened regression:
  - admin session:
    - `/v1/org/options` = `200`
    - `/v1/roles` = `200`
    - `/v1/users` = `200`
    - `/v1/permission-logs` = `200`
  - `Ops` session:
    - same routes = `403`
  - roleless session:
    - same routes = `403`

## 8. Risk assessment
- Highest risk:
  - deleting active/business-like tasks just because they were touched by test actors
- Medium risk:
  - trying to normalize task `owner_team` against account-org teams
  - that would cross into task-create/task-query compatibility semantics
- Lowest risk:
  - normalizing the 9 deterministic `asset_storage_refs` rows

## 9. Recommended next order
1. Create a fresh DB backup again before any second-round mutation.
2. Normalize the 9 deterministic `asset_storage_refs` rows first.
3. Review the 29 manual-confirm tasks with business owners / operator trace before any delete.
4. Only after that, delete the 21 clear-delete tasks with full dependent-row cleanup.
5. Re-run the same full live regression set immediately after any mutation.

## 10. What this round intentionally did not do
- No upload-chain code adjustment
- No task detail aggregate adjustment
- No org/permission model adjustment
- No delete execution for the second-round candidate set
- No deploy or release replacement
