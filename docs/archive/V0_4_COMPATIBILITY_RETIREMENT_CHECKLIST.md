# v0.4 Compatibility Surface Retirement Checklist

Date: 2026-03-13
Source references:
- `docs/V0_4_MAIN_BRIDGE_CONVERGENCE_PLAN.md`
- `docs/MAIN_BRIDGE_RESPONSIBILITY_MATRIX.md`

## Purpose

Turn the current v0.4 compatibility classification into an explicit post-v0.4 retirement checklist without removing the routes, entrypoints, or runtime behaviors in this step.

## Current Priority Note

- this checklist remains the governance baseline for later compatibility cleanup
- it does not currently outrank mainline feature delivery, integration, verification, release, or deployment work
- retirement items should continue only in engineering review windows, version close-out windows, or post-release governance windows unless priority changes again

## Bucket Summary

| Bucket | Meaning | Current action |
|---|---|---|
| Retired | Compatibility item already removed from active repository guidance without changing runtime behavior | Record only; no further action |
| Keep through v0.4 | Required for the smallest safe convergence path or rollback-safe validation during v0.4 | Keep unchanged in this turn |
| Retire immediately after v0.4 | Remove once v0.4 stabilization confirms that canonical MAIN and Bridge surfaces have replaced the compatibility path | Plan now, retire in the first post-v0.4 cleanup step |
| Defer to a later phase | Retirement depends on broader caller inventory, topology decisions, or future runtime extraction work | Keep documented as deferred; do not remove right after v0.4 |

## Retired

| Item | What it was | Why it was safe to retire first | What changed in this repository |
|---|---|---|---|
| Mixed old `8080` wording that treated MAIN product routes as both cache and live ERP | A wording-only compatibility remnant in planning/state language | It was repository-only guidance, had no runtime effect, and removing it reduces future ownership confusion without touching live routes or rollback behavior | Active convergence docs now treat that mixed-responsibility wording as retired, while `/v1/erp/*` remains the live Bridge-backed query facade and local product routes remain compatibility-only |

## Keep Through v0.4

| Item | What it is | Why it is still kept now | What must be true before retirement review | Risk if removed too early | Timing |
|---|---|---|---|---|---|
| `GET /health`, `GET /ping` | Root operational probes on live MAIN `8080` | They are still the canonical liveness and shallow reachability checks during convergence | Replacement operational probes and all deploy/verify scripts must be explicitly switched first | Deploy verification and rollback checks lose the simplest stable probes | Keep through v0.4; review later only if ops contract changes |
| `GET /v1/products/sync/status`, `POST /v1/products/sync/run`, `ERPSyncWorker` | MAIN-owned sync continuity surface and worker runtime | v0.4 explicitly keeps sync/runtime ownership in MAIN for the smallest safe convergence path | A separate approved extraction plan, replacement runtime ownership, and matching cutover/rollback procedures must exist | MAIN loses current product-cache sync continuity before a new owner exists | Keep through v0.4; defer beyond the immediate post-v0.4 cleanup |
| Candidate MAIN validation on `18080` | Side-by-side validation mode and isolated runtime files for pre-cutover checks | It is the current safe verification path for Bridge-coupled cutover work | Repeated releases must prove an equally safe verification path or an explicit decision must retire side-by-side validation | Cutover becomes riskier because Bridge-coupled checks lose the isolated warm-up path | Keep through v0.4; later-phase review |

## Retire Immediately After v0.4

| Candidate | What it is | Why it is still kept now | What must be true before retirement | Risk if removed too early | Timing |
|---|---|---|---|---|---|
| `GET /v1/products/search`, `GET /v1/products/{id}` | Legacy `8080` local-cache ERP-facing compatibility reads on MAIN | They preserve rollback-safe old read behavior while `/v1/erp/*` becomes the canonical live ERP query facade | Known callers must either move to `/v1/erp/*` for live ERP query or be confirmed to need only local-cache behavior; docs, OpenAPI, and smoke/runbook references must no longer require these routes as part of the convergence path | Original-product selection or rollback investigations may fail if callers still depend on local-cache semantics or old response shape | v0.4 post-step |
| `POST /v1/integration/call-logs/{id}/advance` | Backward-compatible integration lifecycle advance route | It preserves compatibility for internal/manual placeholder callers while execution routes become the preferred lifecycle boundary | Internal/admin callers, runbooks, and tests must be switched to execution-oriented routes; no supported workflow may rely on the legacy advance action | Internal placeholder troubleshooting or replay flows can break abruptly, leaving call-log operations stranded on undocumented paths | v0.4 post-step |
| `POST /v1/audit` | Replaced legacy audit route on MAIN | It remains on disk only to avoid risky broad retirement during convergence | Caller inventory must show that task-centric `/v1/tasks/{id}/audit/*` routes fully replace it; docs and tests must no longer mention it as usable behavior | Hidden callers can lose audit submission unexpectedly and create false regression noise during cleanup | v0.4 post-step |
| `cmd/api` on-disk compatibility entrypoint remnant | Deprecated non-production entrypoint kept only after production packaging moved to `./cmd/server` | It gives a narrow compatibility buffer while the new production-only entrypoint settles | At least one stable managed/package cycle after v0.4 must show `entrypoint=./cmd/server`, and no deploy/package/runbook path may still reference `cmd/api` | Removing it before the new entrypoint path is operationally boring can reduce rollback confidence and obscure entrypoint regressions | v0.4 post-step |

## Defer To A Later Phase

| Candidate | What it is | Why it is still kept now | What must be true before retirement | Risk if removed too early | Timing |
|---|---|---|---|---|---|
| Same-host loopback `ERP_BRIDGE_BASE_URL=http://127.0.0.1:8081` assumption | Current deployment/runtime continuity assumption between MAIN and Bridge | It keeps deploy, rollback, and side-by-side validation aligned to the current same-host production model | Bridge ingress/topology ownership must be explicitly settled, and deploy/verify docs must support the intended long-term runtime contract without ambiguity | Runtime cutover can fail if the repo drops the only proven Bridge routing assumption before another one is validated | Later phase |
| Unrelated V6 public routes `/v1/sku/*`, `/v1/agent/*`, `/v1/incidents`, `/v1/policies` | Legacy public surface left on disk outside the MAIN/Bridge convergence target | They were intentionally excluded from risky broad deletion during v0.4 | Caller inventory, deletion tests, and a focused legacy-removal step must exist per route family | Unknown external or internal consumers can break, turning convergence cleanup into uncontrolled regression work | Later phase |
| Historical release-history records showing `entrypoint=./cmd/api` | Pre-convergence operational history in `deploy/release-history.log` | They are audit history, not active runtime behavior | Only remove if history retention policy explicitly allows it; otherwise keep permanently | Deleting them too early destroys the evidence trail that explains the pre-convergence packaging drift | Later phase, and likely never |

## Sequenced Retirement Order

1. Retire wording-only remnants first.
Checklist:
- Remove any remaining mixed cache/live ERP wording from active docs and handover notes.
- Reconfirm that no current contract description promotes compatibility routes as canonical.

2. Retire the replaced or duplicate compatibility routes next.
Checklist:
- Confirm caller inventory for `POST /v1/audit`.
- Confirm caller inventory for `GET /v1/products/search`, `GET /v1/products/{id}`, and `POST /v1/integration/call-logs/{id}/advance`.
- Update tests, smoke docs, and runbooks before route removal.

3. Retire the entrypoint-era remnant after one boring release cycle.
Checklist:
- Keep `cmd/api` out of production packaging and deploy paths.
- Verify at least one post-v0.4 package/deploy record uses only `./cmd/server`.
- Remove `cmd/api` only after rollback guidance no longer depends on it.

4. Review runtime compatibility assumptions only after topology decisions are explicit.
Checklist:
- Do not retire loopback Bridge assumptions or side-by-side validation mode as part of the immediate post-v0.4 cleanup.
- Revisit them only in a later runtime/deploy phase with explicit ops ownership.

## Explicit Non-Goals For This Step

- No compatibility routes are removed here.
- No deploy/runtime behavior changes are introduced here.
- No business behavior or Bridge contract is widened here.
