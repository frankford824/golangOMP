# SKU cost reconciliation

## Scope and policy

The canonical local price is `task_sku_items.cost_price`. Migration 141 widens
cost storage/audit projections to four decimals, records every canonical cost or
manual-protection change transactionally, and bootstraps per-SKU sync states.
Existing mismatches enter `conflict`; migration never changes historical prices.
`baseline` means replica agreement only, not a fresh ERP acknowledgement.

Local human prices take priority over recalculation. A differing ERP observation
while a local edit is pending or locally protected becomes a conflict. ERP-origin
prices are protected from recalculation but subsequent ERP-only edits can still
propagate. Zero replacing a nonzero/unpriced amount and invalid prices require
explicit confirmation. This is eventual reconciliation, not a distributed atomic
transaction: ERP does not expose a verified compare-and-swap price write contract.

## Processing

- Canonical writes and revision capture are in the same MySQL transaction via
  triggers. The durable row coalesces repeated changes to a SKU.
- MAIN polls every 5 seconds, prioritizes up to 4 live changes and checks up to
  50 cold baseline SKUs in one read-only ERP batch. Explicit conflict resolution
  wakes the worker. Queue periods are not end-to-end latency promises.
- The existing 8082 collector still polls the ERP about every 10 minutes. Its
  immutable cost events are consumed by numeric ID, not mixed wall-clock dates.
- A dedicated direct OpenWeb observer has no local-cache fallback. A write is
  checked against the current local version and latest ERP price, serialized per
  SKU, and acknowledged only after another direct ERP read agrees.
- Cost-only requests contain SKU and cost fields only; they do not send name,
  style, image or dimension fields. The dedicated task cost-info endpoint queues
  cost work instead of replaying a complete product profile.
- Filing acknowledgements update filing fields only. They must not restore an
  old cost/specification snapshot after an ERP call.
- Inbound acceptance and local mirror updates are atomic. A projected revision
  prevents ERP echoes from generating another local price cycle. Manual conflict
decisions compare both local and ERP revisions and require a reason.
- Identity filing remains independent of cost confirmation. Blocked/conflicting
  cost fields are omitted from identity requests, not silently approved.

Task SKU editors can read `/v1/tasks/{id}/sku-items/{sku_item_id}/cost-sync`
within the existing task-read scope. `refresh=true` returns a fresh ERP price
and local revision as an edit baseline without writing state. A manual cost
save with this baseline checks both again, locks the canonical SKU then its
state, and saves the cost plus a `manual_saved` intent in the same transaction.
This also supports explicitly retrying the same local amount after reviewing
an old conflict. No global ERP-management permission is granted: existing own
active-task edit scope still applies. A stale baseline returns 409 without a
price change. ERP changes after save remain conflicts until reviewed again.
Cost-only edits never replay product fields or enqueue identity filing. Older
callers without a baseline retain conservative asynchronous behavior.

## Operations

Use **成本规则 → 成本同步** to view coverage, baseline checks, pending work,
retries, conflicts, and verified agreement. Resolve one SKU by retaining the
local price or accepting the ERP price. A changed price returns 409 and must be
reviewed again. Coverage does not certify the underlying business tariff.

`COST_SYNC_ENABLED=false` disables the MAIN worker and write guard for emergency
operations; use only with explicit operational review, because it also disables
the reconciliation protections. Bridge (8081) never starts this worker.

Migration and consumer rollout require a verified pre-release database backup.
Isolated MySQL validation uses `TEST_COST_SYNC_DSN` and rejects any database not
named with the `codex_cost_sync_test_` prefix. No production data is copied into
those fixtures.

## Authorized live verification

`cmd/tools/cost-sync-verify` is read-only by default. `--apply` requires a named,
eligible completed single-SKU task with zero stock, no pending filing jobs, no
manual price, initial local/ERP agreement, and a new absolute `--recovery` path.
It changes the price by 0.0001/0.0002, checks outbound, real collector/inbound,
conflict preservation and no echo, then restores the original cost and flags.
It refuses to overwrite an unrelated concurrent human edit. Recovery JSON and
audit records are retained; a failed restoration is an operational blocker.
