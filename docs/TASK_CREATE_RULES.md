# Task Create Rules

Current task creation is defined only by:

1. `transport/http.go` — mounted create and upload routes.
2. `docs/api/openapi.yaml` — task-type, request-body, validation, and conflict contracts.
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` — current workflow governance.

Do not infer active task types from migrations, prompts, archived documents, or legacy handlers.
The pre-V8 rules are preserved at `docs/archive/legacy_specs/TASK_CREATE_RULES_pre_v8.md` for audit only.
