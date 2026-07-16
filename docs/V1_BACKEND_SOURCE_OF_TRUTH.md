# V8 Backend Source of Truth

> Status: current replacement contract for shared-backend, main-ops and asset-workbench.
> Effective after the V8 maintenance-window cutover. Historical documents and released
> migrations are evidence only and never restore a removed runtime route.

## Authority order

1. `transport/http.go` decides whether a runtime route exists.
2. `docs/api/openapi.yaml` decides request and response fields.
3. This file and the four V1-named authority documents below decide current architecture.
4. `docs/frontend/*` is generated from OpenAPI and has no independent authority.
5. `docs/archive/*`, `docs/iterations/*`, prompts and Git history are evidence only.

## Current authority set

- `docs/V1_MODULE_ARCHITECTURE.md`: workflow, explicit access, finalization and SKU planning.
- `docs/V1_INFORMATION_ARCHITECTURE.md`: menus, screens, search and frontend behavior.
- `docs/V1_ASSET_OWNERSHIP.md`: staging, resource groups, downloads and publication pinning.
- `docs/V1_CUSTOMIZATION_WORKFLOW.md`: customization as an internal design-node job.

## V8 invariants

- Design tasks follow `create -> design -> PendingAudit -> Completed`; a return decision goes to design.
- Retouch tasks finalize when every retouch requirement has a final product.
- `sku_planning` creates 1-200 SKU identities and planning revisions atomically and returns `Completed`.
- Completion is performed only by `TaskFinalizer.FinalizeInTx` inside the caller transaction.
- ERP and search projection failures never reopen a completed task.
- Current business files are resolved only through `task_asset_groups` working/finalized pointers.
- Public resource views expose reference images, one current effective source and ordered final products.
- Authorization is capability plus stable organization-ID scope. Organization names are display-only.
- `workflow_contract_version=2` and an empty `allowed_actions` array means explicitly no actions.
- Removed route families return 404 after cutover; no compatibility aliases are introduced.

## Active task states

`Draft`, `PendingAssign`, `Assigned`, `InProgress`, `PendingAudit`, `Completed`,
`Archived`, `Cancelled`, and `Blocked` are the only states new writes may produce.

## Active task types

- `original_product_development`
- `new_product_development`
- `retouch_task`
- `sku_planning`

## Public route families

- Explicit access: `/v1/access/*`
- Task workflow: `/v1/tasks/{id}/submit-design`, `/audit/decision`, `/reopen`
- Resource read model: `/v1/tasks/{id}/resource-bundle`, `/v1/resource-groups*`
- SKU planning: `/v1/tasks` with `task_type=sku_planning`, Excel, correction and ERP retry routes
- Search: `/v1/search` with task-resource-group asset results
- Client publication: existing `/v1/asset-workbench/client-materials*` pinned to a finalized revision

The complete route list and fields are defined by `transport/http.go` and OpenAPI; this file does
not create undocumented routes.

## Persistence authority

- User and task organization ownership uses existing stable department/team IDs.
- SKU identity uses `task_sku_items`.
- SKU planning business revisions use `task_planning_sku_details` and
  `task_planning_sku_revisions`.
- Code allocation uses `code_rules`, immutable revisions and revision-scoped sequences.
- File entities use `task_assets` and `asset_storage_refs`; current business-resource authority
  uses `task_asset_groups`.
- Client publication extends `asset_workbench_client_materials`; no parallel publication table exists.
- Asynchronous ERP and search work uses durable outboxes and idempotent dedupe keys.

## Cutover rule

The project uses rehearsal plus a short write freeze, not long-lived business dual-write. Formal
cutover requires snapshot, dry-run, apply, full search reindex, backend smoke tests, main-ops publish,
then asset-workbench publish. Rollback uses the captured snapshot and the migration tool's rollback
manifest. Production execution, SSH and data writes require an explicit operational instruction.

## Required gates

```bash
./scripts/agent-check.sh
python3 scripts/docs/generate_frontend_docs.py
cd vue && npm run gen:api-types && git diff --exit-code
cd vue && npm run test:unit && npm run build:prod && npm run build:asset
cd vue && npm run design:audit && npm run asset:audit
cd vue && npm run a11y:audit && npm run asset:a11y && npm run load:audit
```
