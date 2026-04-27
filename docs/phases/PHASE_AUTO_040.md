# PHASE_AUTO_040 - Cost-Rule Governance / Versioning / Override Hardening

## Why This Phase Now
- Step 39 completed export-center planning-boundary hardening and left cost-rule governance as one of the highest-impact remaining middle-platform gaps for the PRD mainline.
- Category center, cost preview, and task-side cost prefill already exist, but they still behave more like a usable skeleton than a governable one.
- Before any formula DSL, approval flow, ERP writeback, or finance integration is attempted, the current cost-rule layer needs stable version, effective-window, override, and history-snapshot semantics.

## Current Context
- Current state: `V7 Migration Step 39 complete`
- Current OpenAPI version before this phase: `0.35.0`
- Latest iteration: `docs/iterations/ITERATION_039.md`
- Stable cost-related skeletons already present:
  - category-center configuration
  - cost-rule CRUD + preview
  - task-side category-driven cost prefill
  - procurement-summary cost exposure
  - manual override boundary fields
- Current main gap:
  - cost-rule governance / versioning / override hardening

## Goals
- Add additive cost-rule governance fields that make one rule row express a versioned governed skeleton instead of only a pricing sample row.
- Make preview and task-side prefill explicitly trace:
  - which rule matched
  - which rule version matched
  - the rule source and governance status
  - when task-side prefill happened
- Harden manual override persistence so override reason and lightweight actor/time trace are explicit business-governance data, without claiming real auth or approval flow.
- Document and preserve the history policy:
  - new rule changes affect future preview / future prefill only
  - old task results are not silently recomputed

## Allowed Scope
- additive cost-rule governance fields on existing `cost_rules`
- additive task-detail / procurement-summary governance trace fields
- additive preview / prefill / override service hardening
- additive tests for cost-rule governance and task-side trace persistence
- additive OpenAPI/state/iteration/handover synchronization
- create `docs/phases/PHASE_AUTO_040.md`

## Forbidden Scope
- standalone `cost_rule_versions` subsystem beyond current table hardening
- complete formula DSL / expression runtime
- real finance system or financial statement layer
- real ERP cost writeback
- real approval workflow or approval routing engine
- BI / KPI / export-report deepening
- deep auth / permission scoping
- NAS / object storage / unrelated export or integration expansion

## Expected File Changes
- add one migration for cost-rule governance / task-detail trace fields
- update cost-rule domain, repo, service, preview, and handler code
- update task-detail / procurement-summary domain and service shaping
- update relevant tests
- add `docs/phases/PHASE_AUTO_040.md`
- add `docs/iterations/ITERATION_040.md`
- update `docs/api/openapi.yaml`
- update `CURRENT_STATE.md`
- update `MODEL_HANDOVER.md`
- update `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Required API / DB Changes
- DB:
  - add governance/version fields to `cost_rules`
    - `rule_version`
    - `supersedes_rule_id`
    - `governance_note`
  - add task-side governance trace fields to `task_details`
    - `matched_rule_version`
    - `prefill_source`
    - `prefill_at`
    - `override_actor`
    - `override_at`
- API:
  - no new endpoints required
  - extend `CostRule` responses additively with:
    - `rule_version`
    - `supersedes_rule_id`
    - `superseded_by_rule_id`
    - `governance_note`
    - `governance_status`
  - extend preview responses additively with:
    - `matched_rule_id`
    - `matched_rule_version`
    - `governance_status`
  - extend task-detail / procurement-summary responses additively with:
    - `matched_rule_version`
    - `prefill_source`
    - `prefill_at`
    - `override_actor`
    - `override_at`

## Success Criteria
- cost rules now expose explicit version/governance metadata without introducing a separate rule engine platform
- preview responses explicitly show matched rule/version/source/governance status while keeping current estimate/manual-review behavior stable
- task business-info persistence explicitly records prefill trace and manual override trace without claiming real approval/auth
- docs clearly state that historical task results are snapshot-based and not auto-recomputed by later rule changes
- tests pass without expanding into DSL, finance, ERP writeback, or approval flow

## Required Document Updates
- `CURRENT_STATE.md`
- `docs/iterations/ITERATION_040.md`
- `docs/api/openapi.yaml`
- `MODEL_HANDOVER.md`
- `docs/V7_MODEL_HANDOVER_APPENDIX.md`

## Risks
- additive governance fields must not be mistaken for a real approval system
- version/governance wording must stay aligned between cost-rule CRUD, preview, and task-side persistence
- history policy must stay explicit so frontend and later backend work do not assume auto-recalculation exists

## Completion Output Format
1. changed files
2. DB / migration changes
3. API changes
4. correction notes
5. risks / remaining gaps
6. next recommended single phase
