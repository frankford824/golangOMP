# MODEL_SWITCH_FIRST_MESSAGE.md

When handing work to another model, send this first:

Read `docs/V0_9_MODEL_HANDOFF_MANIFEST.md` first.
Then read, in order:
1. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`
4. One focused current guide only if the task needs it

Do not use `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
`docs/iterations/*`, legacy specs, or model-memory files as the current spec.

If a document conflicts with the mounted runtime, use:
`transport/http.go` -> mounted paths
`docs/api/openapi.yaml` -> request/response contract
`docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` -> route class and compatibility policy

Stricter variant:

Do not edit code yet. First summarize:
1. the current authority set
2. the canonical route families
3. any compatibility-only or deprecated route that is relevant to the task
4. the specific focused guide, if any, that applies

Do not treat archive or historical materials as current truth unless the manifest directs you there for background only.
