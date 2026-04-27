# MAIN / Bridge Responsibility Matrix

Date: 2026-03-13

## Executive Summary

This document is the repository source of truth for MAIN 8080 versus Bridge 8081 responsibility ownership.

The convergence outcome is narrow and deliberate:

- MAIN keeps auth, users/permissions, task/workflow state, export/integration-center visibility, local ERP cache, and the current sync/background/runtime behavior needed for the smallest safe v0.4 convergence path.
- Bridge owns ERP/JST adapter query behavior and ERP mutation execution only. MAIN may expose a stable user-facing facade, but it must not absorb Bridge adapter responsibility or grow raw ERP mutation APIs.
- Compatibility surfaces stay available only where they preserve current task/mainline behavior or rollback safety. They are not the place to grow new ERP behavior.

## Terminology Guardrails

- `live MAIN` = the public business application service on `8080`
- `candidate MAIN` = a side-by-side validation instance of the same MAIN service on `18080`; it is verification-only and not a second long-term runtime role
- `Bridge` = the ERP/JST adapter runtime on `8081`; in v0.4 it does not take over MAIN business routes or generic sync/background ownership
- current runtime reality for v0.4 keeps `ERPSyncWorker` and `/v1/products/sync/*` in MAIN
- compatibility-only surfaces are rollback-safe continuity only; they are not the target architecture
- any later extraction of MAIN-owned sync/background/runtime concerns is deferred future work, not part of the current runtime model

## Decisions Already Made

- `PATCH /v1/tasks/{id}/business-info` is the only ERP filing boundary, and only for `source_mode=existing_product` when `filed_at` is set.
- `/v1/erp/*` is the query-first Bridge-facing surface for original-product picking; local `GET /v1/products/search` remains cache-based and must not become the real-time ERP query path.
- Bridge remains the ERP/JST adapter and ERP mutation executor; MAIN may call Bridge mutations only from explicit business boundaries and must not expose raw ERP mutation routes of its own.
- Default deployment remains same-host: MAIN on `8080`, Bridge on `8081`, and MAIN points to Bridge through `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`.

## v0.4 Legacy Compatibility Inventory

| Legacy-facing route / behavior | Classification | Canonical MAIN relationship | Why it remains explicit in v0.4 |
|---|---|---|---|
| Root operational probes (`GET /health`, `GET /ping`) | keep temporarily | These remain the canonical live MAIN operational probes on `8080`; they are not a second app model. | Keep deploy, rollback, and side-by-side verification continuity simple during convergence. |
| Local cached ERP reads (`GET /v1/products/search`, `GET /v1/products/{id}`) | compatibility only | Canonical Bridge-backed original-product query entry is `/v1/erp/*`; these routes stay local-cache-only on MAIN. | Preserve rollback-safe old 8080 read behavior without treating local cache as live ERP authority. |
| Legacy 8080 sync continuity (`GET /v1/products/sync/status`, `POST /v1/products/sync/run`, `ERPSyncWorker`) | keep temporarily | These remain MAIN-owned internal cache-sync controls and runtime for v0.4. | Legacy sync behavior still remains in MAIN for v0.4; do not extract or delete it in this turn. |
| Integration compatibility advance route (`POST /v1/integration/call-logs/{id}/advance`) | compatibility only | New internal execution/retry/replay surfaces are the preferred lifecycle boundary; this route is no longer the primary model. | Preserve backward compatibility for internal placeholder callers while preventing new behavior from growing here. |
| Legacy audit route (`POST /v1/audit`) | replaced by canonical MAIN | Task-centric audit routes under `/v1/tasks/{id}/audit/*` are the canonical MAIN audit surface. | Keep the old route explicit as replaced so maintainers stop treating it as an equal public audit entrypoint. |
| Same-host Bridge loopback assumption (`ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`) | compatibility only | Canonical v0.4 runtime still runs MAIN on `8080` with Bridge on `8081`; this assumption supports safe cutover and rollback. | Keep runtime continuity explicit until convergence is proven operationally. |
| Unrelated V6 public routes (`/v1/sku/*`, `/v1/agent/*`, `/v1/incidents`, `/v1/policies`) | retire after v0.4 | They are not part of the canonical MAIN versus Bridge convergence model and should not receive new production-facing work. | Leave them on disk for now to avoid risky deletion, but name them clearly as post-v0.4 retirement candidates. |
| Mixed old 8080 behavior that treated MAIN product routes as both local cache and live ERP | retired | Canonical split is now Bridge-backed `/v1/erp/*` for live ERP queries plus local-cache compatibility routes on MAIN. | Repository wording cleanup has already retired this mixed-responsibility language; do not reintroduce it. |

## Public Operational Routes

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| `GET /health` | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep as the primary MAIN liveness probe; no Bridge coupling needed. | Low. Safe for deploy and rollback checks. |
| `GET /ping` | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep as the shallow reachability probe for MAIN. | Low. Safe for side-by-side verification. |

## Auth / User / Permission Routes

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| `/v1/auth/*` (`register`, `login`, `me`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Auth/session entry remains a MAIN concern; do not move into Bridge. | Current auth is still repository-local and placeholder-oriented, not a Bridge capability. |
| `/v1/users` and `/v1/users/:id*` | MAIN 8080 | MAIN 8080 | keep in MAIN | User administration remains in MAIN because it governs MAIN route access and workflow roles. | Depends on current placeholder identity model; still not a full org/tenant system. |
| `GET /v1/roles` | MAIN 8080 | MAIN 8080 | keep in MAIN | Roles stay MAIN-owned because they protect MAIN routes and task actions. | Low. Shared role semantics still live here. |
| `GET /v1/access-rules` | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep as the admin-visible route/role contract for MAIN. | Route catalog must stay aligned with router registration. |
| `GET /v1/permission-logs` | MAIN 8080 | MAIN 8080 | keep in MAIN | Permission/audit visibility stays in MAIN; Bridge should not become the audit trail for MAIN authorization. | Depends on current permission-log persistence and placeholder auth wiring. |

## Task / Workflow Routes

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| `/v1/tasks/*` core task aggregate (`create`, `list`, `detail`, `assign`, `submit-design`, `close`, `events`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Task lifecycle, task IDs, and workflow state remain authoritative in MAIN. | High business impact; do not split task authority across Bridge. |
| `PATCH /v1/tasks/{id}/business-info` business-info persistence | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep business-info editing in MAIN. This route also remains the narrow ERP filing trigger boundary when `filed_at` is set. | Rollback-sensitive because this is where Bridge filing is triggered. |
| `/v1/task-board/*` | MAIN 8080 | MAIN 8080 | keep in MAIN | Task-board aggregation is a MAIN read model over MAIN workflow state. | Low. No direct Bridge ownership. |
| `/v1/workbench/*` | MAIN 8080 | MAIN 8080 | keep in MAIN | User workbench preferences stay with MAIN user/task experience. | Low. User-scoped state is MAIN-local. |
| Audit task actions (`/v1/tasks/{id}/audit/*`) and legacy `POST /v1/audit` | MAIN 8080 | MAIN 8080 | keep in MAIN | Audit workflow remains inside MAIN task state; do not externalize to Bridge. | High if moved; audit assignment/history is tied to MAIN task state. |
| Warehouse task actions (`/v1/tasks/{id}/warehouse/*`) and `GET /v1/warehouse/receipts` | MAIN 8080 | MAIN 8080 | keep in MAIN | Warehouse workflow and receipt visibility remain MAIN-owned. | High business impact; Bridge is not the warehouse workflow owner. |
| Outsource routes (`/v1/tasks/{id}/outsource`, `GET /v1/outsource-orders`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Outsource workflow remains task-centric inside MAIN. | Low to medium. Depends on MAIN task/event state. |
| Export routes (`/v1/export-templates`, `/v1/export-jobs/*`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Export center remains MAIN-owned even though its adapter handoff is still placeholder-only. | Current export execution remains internal/placeholder, not Bridge-owned. |
| Integration-center core routes (`/v1/integration/connectors`, `/v1/integration/call-logs*`, `/v1/integration/call-logs/*/executions*`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep call-log and execution visibility in MAIN as internal observability and filing trace infrastructure. | Internal-only today; not a real cross-system orchestration platform yet. |
| `POST /v1/integration/call-logs/{id}/advance` | MAIN 8080 | MAIN 8080 | compatibility only | Preserve only as the backward-compatible call-log lifecycle advance route. Do not build new behavior on top of it. | Explicitly documented as compatibility-only; future work should use execution routes instead. |

## ERP-Facing Routes

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| Local cached ERP product read surface (`GET /v1/products/search`, `GET /v1/products/{id}`) | MAIN 8080 | MAIN 8080 local cache layer | compatibility only | Keep for local-cache search, mapped category narrowing, and backward compatibility. Do not treat it as the real-time ERP source of truth. | Medium. Easy to misuse as if it were live ERP search. |
| Local ERP sync admin surface (`GET /v1/products/sync/status`, `POST /v1/products/sync/run`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep as the internal cache-sync control plane for MAIN-owned product cache. | Internal placeholder only; not frontend-ready. |
| MAIN Bridge facade (`GET /v1/erp/products`, `GET /v1/erp/products/{id}`, `GET /v1/erp/categories`) | MAIN 8080 facade over Bridge | Bridge 8081 for ERP/JST adapter contract, MAIN 8080 for facade stability | Bridge-owned | Keep the MAIN facade stable and query-first, but Bridge owns adapter semantics and upstream data authority. MAIN should not mirror raw Bridge mutation routes under `/v1/erp/*`. | Depends on Bridge availability and payload compatibility. |
| Bridge query routes (`GET /erp/products`, `GET /erp/products/{id}`, `GET /erp/categories`) | Bridge 8081 | Bridge 8081 | Bridge-owned | These remain the actual ERP query contract. MAIN should only normalize, guard, and bind results locally. | Bridge source is not in this repo; live contract discovery remains partially external. |
| Legacy 8080 ERP-facing responsibility that tries to be both cache and live ERP | MAIN 8080 historical tendency | None | retire | Convergence decision is to stop mixing local cache search with live Bridge search under one responsibility. | Medium. Documentation drift can recreate the old ambiguity if not enforced. |

## ERP Mutation Boundaries

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| Product filing trigger boundary at `PATCH /v1/tasks/{id}/business-info` when `filed_at` is set on `existing_product` | MAIN 8080 | MAIN 8080 | keep in MAIN | MAIN keeps the business decision of when an ERP mutation is allowed. Do not spread this trigger into task create, generic product edit, or warehouse routes. | High. This is the current rollback-sensitive Bridge mutation entrypoint. |
| Product upsert execution via Bridge `POST /erp/products/upsert` | MAIN 8080 caller through explicit business boundary, Bridge 8081 executor | Bridge 8081 | Bridge-owned | Keep mutation execution in Bridge. MAIN may prepare payloads and reject invalid filing attempts, but must not become the ERP upsert engine or expose raw ERP mutation routes. | High. Upstream Bridge availability and payload tolerance are required. |
| Local cache/binding of Bridge-selected products into `products` | MAIN 8080 | MAIN 8080 | keep in MAIN | MAIN keeps the local `products` cache row and task binding so task state stays MAIN-native even when sourced from Bridge. | Medium. Must stay consistent with Bridge product IDs. |
| Future shelve / unshelve mutation | Not implemented in repo | Bridge 8081 if introduced | Bridge-owned | If this capability lands later, it belongs on the Bridge side. Do not add shelve/unshelve logic directly into MAIN. | Open upstream contract; not implemented or verified. |
| Future virtual qty mutation | Not implemented in repo | Bridge 8081 if introduced | Bridge-owned | Virtual quantity changes are ERP-domain mutations and should not be introduced as MAIN-owned write logic. | Open upstream contract; not implemented or verified. |
| Broad ERP mutation platform beyond the filing boundary | Not implemented in repo | None | retire | Current convergence explicitly rejects widening MAIN into a broad ERP docking/writeback platform in this turn. | Avoids uncontrolled scope growth and rollback risk. |

## Background / Runtime Responsibilities

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| Incremental ERP cache sync worker (`ERPSyncWorker`) | MAIN 8080 worker runtime | MAIN 8080 | keep in MAIN | MAIN keeps the local ERP cache sync worker. Current repo default is `ERP_SYNC_INTERVAL=5m`; if operations want 10-minute cadence, that is an env/config choice, not an ownership move. | Current implementation is stub-source-based, not real ERP pull. |
| Daily full sync as a separate scheduled responsibility | None in repo | None | retire | No separate daily full-sync job exists today. Keep one configurable MAIN cache-sync responsibility unless a future real ERP integration proves the split is necessary. | If stakeholders expect a distinct daily job, that is still an unresolved product/ops question. |
| Local ERP cache / product sync (`products` upsert by `erp_product_id`, `erp_sync_runs`) | MAIN 8080 | MAIN 8080 | keep in MAIN | MAIN owns the local cached product table and sync history used by task binding and local search. | No delete/deactivate reconciliation policy beyond current upsert behavior. |
| MAIN background workers (`LeaseReaper`, `RetryScheduler`, `VerifyWorker`, `EventDispatcher`) | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep queue/event/runtime workers in MAIN because they serve MAIN persistence and lifecycle management. | Existing worker tables and runtime semantics are repository-owned. |
| Integration call logs and execution attempts, including connector `erp_bridge_product_upsert` | MAIN 8080 | MAIN 8080 | keep in MAIN | Keep Bridge filing traceability in MAIN integration-center records rather than creating a second Bridge-side audit system in this repo. | Internal/admin only; not a full external job runner. |

## Deployment / Runtime Ownership

| Responsibility / route / behavior | Current owner | Target owner | Category | Migration note | Risk / dependency |
|---|---|---|---|---|---|
| MAIN live runtime on port `8080` (`ecommerce-api`) | MAIN 8080 | MAIN 8080 | keep in MAIN | MAIN remains the user-facing operational server for this repo. | Standard rollout and rollback target. |
| Candidate MAIN validation runtime on port `18080` (`ecommerce-api`) | MAIN 18080 side-by-side validation instance | None as a separate long-term role; after cutover the same build becomes live MAIN on `8080` | compatibility only | This is the same public business app started only for pre-cutover verification. It does not introduce a second runtime owner or a Bridge replacement. | Must stay isolated and must not rewrite live symlinks before cutover. |
| Bridge live runtime on port `8081` (`erp_bridge`) | Bridge 8081 | Bridge 8081 | Bridge-owned | Bridge remains a separately owned runtime dependency even when packaged/deployed on the same host. | Bridge binary is packaged here, but Bridge source responsibility is still separate. |
| Same-host loopback dependency (`ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081`) | Shared runtime contract | Bridge 8081 dependency with MAIN 8080 consumer | compatibility only | Preserve same-host loopback as the default packaging/runtime contract. Do not assume public-IP ingress is the primary path. | Public-IP Bridge probes previously returned empty replies from this environment. |
| Compatibility surfaces for safe cutover and fallback (`/v1/products/search`, `/v1/products/{id}`, `/v1/integration/call-logs/{id}/advance`) | MAIN 8080 | MAIN 8080 | compatibility only | Keep only for backward-compatible behavior and controlled rollback. Do not expand them as the future architecture. | Medium. Easy to keep alive too long and accidentally grow scope there. |
| Rollback-sensitive Bridge-coupled items (`/v1/erp/*`, business-info filing with `filed_at`, local Bridge binding, Bridge base URL`) | MAIN 8080 entry plus Bridge 8081 dependency | Joint controlled rollout, Bridge remains upstream owner | compatibility only | Use side-by-side MAIN validation first. Prefer MAIN rollback without changing Bridge unless the Bridge contract itself changed. | High. These paths fail if Bridge ingress, payloads, or same-host routing are wrong. |

## Open Questions / Blockers

- Bridge public-IP ingress still needs on-host verification. Earlier probes from this environment reached `223.4.249.11:8081` at TCP level but received empty HTTP replies instead of stable JSON.
- Future Bridge mutation domains such as shelve/unshelve and virtual quantity are not implemented or contract-confirmed in this repo, so ownership is directional only for now.
- The repo currently implements one configurable scheduled cache sync with a default `5m` interval, not a distinct "10-minute incremental + daily full" split. If that operational split is still required, it needs an explicit follow-up decision before code changes.
