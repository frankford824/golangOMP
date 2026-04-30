# AUTO_PHASE_EXECUTION_PROMPT.md

When advancing an automatic phase, read in this order:

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. current phase file, if one exists

Do not use `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
`docs/iterations/*`, legacy specs, or model-memory files as the current spec.

If a phase plan is needed, create it only after summarizing:

1. the authority set
2. the canonical route families relevant to the phase
3. the compatibility-only or deprecated surfaces that must not become new mainline
4. the expected files to change

After phase execution, output:

1. phase summary
2. files changed
3. API contract changes, if any
4. validation results
5. remaining risks
