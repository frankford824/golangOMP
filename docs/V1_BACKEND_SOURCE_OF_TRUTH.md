# V1 Backend Source of Truth Index

> Tier-2 authority index. This file does not repeat route fields,
> response fields, SQL shape, workflow rules, or business substance.
> Field contracts live in OpenAPI; mounted routes live in code; V1
> architecture substance lives in the linked V1 authority files below.

## Authority Order

1. `transport/http.go` decides whether a runtime route exists.
2. `docs/api/openapi.yaml` decides request and response field contracts.
3. This index decides where current V1 backend authority is located.
4. Generated frontend docs under `docs/frontend/` are downstream artifacts.
5. `docs/iterations/**`, `docs/archive/**`, `prompts/**`, and historical
   handoff files are evidence only unless their content is restated in the
   V1 authority files linked below.

If this index conflicts with `transport/http.go` or
`docs/api/openapi.yaml`, treat this index as stale and fix it.

## V1 Authority Files

All current `docs/V1_*.md` files are classified here so agents do not infer
authority from filename alone.

`docs/V1_BACKEND_SOURCE_OF_TRUTH.md` is this index. It owns the authority map,
the reading rule, and the classification of V1 documents. It does not own
business behavior or field substance.

`docs/V1_MODULE_ARCHITECTURE.md` owns the V1 task/module architecture:
Task as container, Module as work unit, workflow blueprints, module states,
task type to module composition, pool/team routing, permission layers,
status derivation, event semantics, and rollout/migration strategy. Use it
when deciding backend module boundaries, task workflow behavior, task pool
semantics, or role/scope rules.

`docs/V1_INFORMATION_ARCHITECTURE.md` owns the V1 product information
architecture: menu visibility, personal center, organization management,
global search, reports, notification surfaces, task center tabs, task draft
entry points, and frontend-facing navigation semantics. Use it when deciding
which V1 surface a user should see or which route family supports a screen.

`docs/V1_ASSET_OWNERSHIP.md` owns V1 asset governance: task asset ownership,
source module attribution, reference-file flattening, version anchoring,
asset lifecycle state, global asset-center behavior, archive/delete policy,
and download/read-model semantics. Use it when deciding how files, versions,
references, OSS lifecycle, or cross-task assets are represented.

`docs/V1_CUSTOMIZATION_WORKFLOW.md` owns the customization task workflow:
customer customization and regular customization module flow, customization
state machine, customization pool/team mapping, creation metadata, ERP product
code lookup, design source lookup, audit rejection return path, and legacy
customization migration mapping. Use it when deciding any customization-only
workflow rule.

`docs/V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` is handoff evidence for future V2
planning. It is not current V1 contract authority; if it disagrees with this
index, `transport/http.go`, `docs/api/openapi.yaml`, or the four V1 substance
authority files above, ignore the manifest and update it only as evidence.

## Contract Files

- Route existence: `transport/http.go`
- Request/response fields: `docs/api/openapi.yaml`
- Frontend generated docs: `docs/frontend/*.md`
- Contract drift gate: `tools/contract_audit/`

## Reading Rule

For current V1 behavior, start from this index and then read the specific
authority file above for the affected module. Do not derive current contracts
from `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`, `docs/archive/**`,
`docs/iterations/**`, or `prompts/**`.
