# AGENT_EXECUTION_PROMPT.md

Execute the current task using this document order:

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `transport/http.go`
5. one focused current guide only if needed

Do not use archive or history files as current spec.

If repo documents conflict:

1. `transport/http.go` decides what is mounted.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` decides route classification and compatibility policy.

Before execution, output:

1. your understanding of the current contract
2. the planned edits
3. the expected impact area

After execution, output:

1. files changed
2. API contract changes, if any
3. validation results
4. remaining risks
