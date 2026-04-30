# AGENT_BOOTSTRAP_PROMPT.md

Do not edit code immediately.

Read and summarize these files first:

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. One focused current guide only if the task needs it

Do not start from `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
`docs/iterations/*`, legacy specs, or model-memory files.

Before implementation, output:

1. current authority set
2. relevant canonical route families
3. relevant compatibility-only or deprecated surfaces
4. the focused guide, if any, that applies
5. the files you expect to touch
