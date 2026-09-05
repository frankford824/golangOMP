# AGENTS.md

> AUTHORITY ONLY
> 1. `transport/http.go`
> 2. `docs/api/openapi.yaml`
> 3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`
>
> Historical background only: `docs/archive/legacy_specs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`.
> Do not treat `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, `docs/archive/*`,
> `docs/iterations/*`, legacy specs, model-memory files, or prompts as current spec.

This file is an assistant guidance note. It is not the backend specification.

## Instruction Scope And Authorization

- Higher-priority runtime instructions and the user's explicit task scope take precedence over repository guidance. A read-only request does not authorize edits or operational actions.
- Within repository guidance, this file owns workflow, authorization and validation rules. Nested `AGENTS.md` files add rules for their directories; entry prompts, `CLAUDE.md`, skills and editor rules must not redefine these rules or backend contracts. Apply the asset-workbench rules only within its boundary.
- Reuse authorization already given in the conversation. For routine implementation choices within that scope, state reasonable assumptions and continue. Ask only when missing information materially changes the outcome or the required action is not authorized; continue independent authorized work while waiting.
- Release preparation, tags, push, deployment, remote SSH and DB writes outside test fixtures require explicit user authorization for that action, including authorization already given for the active task. A generic request to fix, inspect or verify does not by itself authorize production operations. Reading a runbook is not authorization to execute it.
- If a rule blocks progress, identify its file and instruction, explain the missing authorization or evidence, and finish any independent work still in scope. Do not ask again for an exception the user has already explicitly authorized.

## Current Repo Baseline

- This is now a monorepo: Go backend code lives at the repository root, and Vue frontend code lives only under `vue/`.
- Keep backend and frontend ownership explicit in every diagnosis, diff, test command, and final report. Do not treat root files and `vue/` files as the same application layer.
- V1 current authority is centralized in `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`.
- Route existence is decided by `transport/http.go`.
- Request/response field contracts are decided by `docs/api/openapi.yaml`.
- New frontend or new integrations must start from the V1 SoT route families and the generated frontend docs under `docs/frontend/`.
- Compatibility and deprecated surfaces remain documented only for migration safety.
- The V1-named authority files now describe the V8 replacement contract. Read their current contents; do not infer current behavior from old release labels, handoff snapshots or archived model memory.

## Monorepo Layout

- Backend scope: repository root Go modules, `cmd/`, `config/`, `db/`, `domain/`, `service/`, `transport/`, root `docs/`, root `scripts/`, and root tests.
- Frontend scope: `vue/` and everything below it, including `vue/src/`, `vue/tests/`, `vue/docs/`, `vue/frontend/`, `vue/package.json`, and Vue/Vite config files.
- Backend authority files are the root files listed in this document. Files under `vue/docs/` and `vue/frontend/` are frontend-side references or copied frontend history, not backend authority.
- When a task crosses both layers, report ownership as `backend`, `frontend`, or `both`, and name the exact paths involved.
- Before running commands, confirm the working directory: use the repository root for Go/backend commands, and `cd vue` for Node/Vue commands.
- Do not move frontend files out of `vue/` or place new backend files under `vue/` unless the user explicitly requests a layout change.

## Project Boundary Matrix

- `main-ops`: the existing operations system. Frontend entry: `vue/src/main.ts`. Main-ops work must not import from `vue/src/asset-workbench/**`.
- `asset-workbench`: the independent asset workbench. Frontend entry: `vue/src/asset-workbench/main.ts`. Asset-workbench work must not import the main-ops shell, router, views, or `main.css`.
- `shared-backend`: the Go backend shared by both frontends. Production entrypoint: `cmd/server`. Route existence is still decided by `transport/http.go`.
- Asset workbench backend routes live under `/v1/asset-workbench/*` and must remain mounted in every production backend release unless the owner explicitly requests an asset-workbench rollback.
- Do not use a long-lived Git branch as an application boundary. Application separation is enforced by directory ownership, entrypoints, route prefixes, build commands, audit scripts, and deploy guards.

## Branch Review And Publish Rule

- `dev/external-developer` is the single integration and release branch for both `main-ops` and `asset-workbench` work.
- Short-lived feature branches are allowed, but they must merge back into `dev/external-developer` before any publish or handoff. Do not keep `feature/asset-workbench-piecework-settlement` or any other asset-workbench branch as a parallel long-lived release branch.
- Fetch `dev/external-developer` and inspect `origin/main...origin/dev/external-developer` before judging or publishing external work.
- External branch changes must be reviewed locally before any deploy or static publish.
- Classify every external-branch diff as `main-ops frontend`, `asset-workbench frontend`, `shared-backend`, or `cross-project`.
- Frontend publish authority is `deploy/FRONTEND_DIST_PUBLISH_SOP.md`; the static artifact path is `dist/front`, built from `vue/`.
- Backend publish authority is `deploy/DEPLOYMENT_WORKFLOW.md`; do not replace it with frontend SOP commands.
- If both sides changed and the frontend depends on new backend behavior, deploy or validate the backend first, then publish the frontend.
- Do not merge `dev/external-developer` into `main` automatically. The project owner decides when the branch is stable enough to merge.

## Reading Order

1. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`
2. `docs/api/openapi.yaml`
3. `transport/http.go`
4. `git log --oneline -20` for recent changes relevant to the task
5. Read historical background only when needed to explain a specific change; it does not add prerequisites to a new task.

## Non-Authoritative Materials

These directories exist for historical evidence only. Never derive current rules, contracts, paths, or commands from them. If they conflict with the three authority files in §Reading Order, the authority files win and these are silently ignored.

- `docs/archive/state_pre_v1_3/` — pre-V1.3 state/handover files (`CURRENT_STATE.md`, `CURRENT_STATE_PATCH_GUIDE.md`, `MODEL_HANDOVER.md`, `ITERATION_INDEX.md`). Moved out of repo root in 2026-04 to reduce agent context noise.
- `docs/archive/orphan_plans/` — one-off plan/scratch files written during iteration but never integrated into a prompt or report.
- `docs/archive/*` (other subfolders) — historical archives, including `legacy_specs/` and `model_memory/`.
- `docs/iterations/*` and `docs/phases/*` — retro reports and phase evidence; only what is restated in the V1 SoT counts as current.
- `prompts/archive_pre_v1_2/` — pre-V1.2 prompt experiments, including `root_legacy/` (13 early `AGENT_*` / `AUTO_*` / `COMMANDER_*` / `MODEL_SWITCH_*` / `PHASE_*` / `ITERATION_TEMPLATE` / `CLAUDE_Backend_Master_Prompt` files moved from repo root in 2026-04).
- `prompts/*` — historical execution inputs, except `prompts/CODEX_SESSION_BOOTSTRAP.md`, the standing entry prompt that references this file. Historical prompts do not authorize cleanup, SSH, migrations, approvals or baseline repair in a new task.
- `dist/*` — release build artifacts and their bundled `README.md` / `CHANGELOG.md`. Never read these for repository rules.

## Working Rule

When documents disagree:

1. `transport/http.go` decides what is mounted.
2. `docs/api/openapi.yaml` decides the current request/response contract.
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` decides route family, governance state, and milestone pointers.
4. Generated frontend docs under `docs/frontend/` are downstream artifacts from OpenAPI and must be regenerated when OpenAPI changes.
5. All other documents are evidence or history.

## Session Start

1. Confirm the working directory; run `git status --short --untracked-files=all`, `git branch --show-current`, `git rev-parse HEAD`, and `git log --oneline -20`.
2. If the working tree is dirty, identify which files are unrelated user/work-in-flight changes before editing.
3. Read the three authority files in the order above for the route family being touched.
4. Restate the route family and contract files affected before making business-code edits.
5. For guidance-only or tooling-only work with no business/API impact, state that scope and read the relevant instructions or scripts; unrelated route families need not be loaded. Measure current state instead of copying a historical PASS, SHA or release label. Do not update historical baselines as a side effect of unrelated work.

## Before Editing

- Locate the route in `transport/http.go` and the schema in `docs/api/openapi.yaml` first.
- Treat path additions/removals, query parameters, request bodies, response bodies, schema fields, pagination envelopes, and readiness/deprecation markers as API contract changes.
- If a Go struct's JSON contract changes, update OpenAPI in the same logical change; newly added OpenAPI schemas must be referenced by at least one operation or component chain.
- If OpenAPI changes, regenerate frontend docs with `python3 scripts/docs/generate_frontend_docs.py` in Linux/WSL, or the configured Python 3 interpreter on Windows.
- If `db/migrations/**` appears necessary, stop and surface the proposal unless the user explicitly authorized that migration work.

## Engineering Hygiene

- Prefer the smallest useful change and reuse existing package/service boundaries before adding new abstractions.
- Do not build platform-style future capacity unless the current task proves it is needed.
- Keep compatibility and deprecated surfaces shrinking; do not add new compatibility routes unless explicitly requested.
- Follow the explicit authorization requirements in §Instruction Scope And Authorization for operational actions.

## After Editing

Run the checks appropriate to the blast radius.

**Full gate (default for backend contract / handler / service / domain / repository changes, except the service-only case below)** — prefer the consolidated script:

```bash
./scripts/agent-check.sh        # Linux / macOS / WSL
```

```powershell
.\scripts\agent-check.ps1       # Windows PowerShell
```

The script runs, in order and stops on first failure:

1. `go vet ./...`
2. `go build ./...`
3. `go test ./... -count=1`
4. Python 3: `scripts/check_experience_migrations.py` (static inspection; does not apply migrations)
5. `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml`
6. `go run ./tools/contract_audit ... --fail-on-drift true` (output: `tmp/agent_check_audit.json`)

Both scripts run these six checks in this order. `PYTHON_BIN` selects the Python 3 executable (default `python3` in Bash, `python` in PowerShell). For this WSL checkout, run backend commands in Linux rather than Windows Go against the UNC path.

If the script fails, it stops subsequent checks, not the whole task. Diagnose the failure, fix it within the authorized scope, and rerun the required gate. If resolving it requires an unauthorized action or an unrelated change, report the blocker and continue independent work; do not claim PASS or create an ABORT file automatically. Use `AGENT_CHECK_SKIP_TESTS=1` only for fast docs/openapi iteration, never as evidence that the full gate passed.

**Narrow gate (service-only fix, no contract change)**: run the focused package test plus `go test ./... -count=1` and `go vet ./...`. The exception applies only to service implementation and its tests, with zero changes under `transport/`, `domain/`, `repo/`, `db/migrations/`, or `docs/api/openapi.yaml`; skip full-gate steps 4–6 only for this case.

**Guidance / scripts / tests only (no backend implementation or API contract change)**: review documentation references and the diff; validate changed scripts with syntax checks and relevant success/failure cases, or run the affected tests. Test files under backend directories still qualify for this exception. If backend implementation or the API contract also changes, use the full or service-only gate above. Do not run scripts that write artifacts or data during a read-only review.

**Frontend implementation**: run relevant tests and the affected app build from `vue/` (`npm run build:prod` for main-ops, `npm run build:asset` for asset-workbench). Follow applicable nested rules, including `npm run asset:audit` for workbench page/component additions. Frontend guidance-only changes use the guidance checks above.

**Frontend docs**: if OpenAPI changed, regenerate as described in §Before Editing and include `docs/frontend/*.md` in the same logical change. After required checks pass, repeat or broaden testing only for new changes, failures or unresolved concerns.

## Response format at end of any non-trivial task

Always finish with these five sections, in order:

1. **Summary** — what changed and why.
2. **Changed files** — full paths.
3. **Commands run** — exact commands and their exit status.
4. **Test result** — pass/fail counts; note any skipped.
5. **Risks / follow-ups** — anything not closed in this turn, including new governance debt.

The Changed files section MUST be derived from
`git status --short --untracked-files=all` and `git diff --stat HEAD`
measured at end-of-turn. If the measured set is larger than what you
intentionally changed, list the extras explicitly and label them
`inherited dirty from session-start (not introduced this turn)`.

## Commit Style

- One logical change per commit.
- Prefix: `fix(<area>): ...` / `feat(<area>): ...` / `docs(<area>): ...` / `refactor(<area>): ...` / `chore(<area>): ...`.
- `<area>` examples: `tasks`, `assets`, `batch`, `erp`, `openapi`, `frontend`, `audit`, `governance`.
- If the commit changes both Go and OpenAPI for the same contract, keep them in one commit.
- Never amend or force-push without an explicit user instruction.

## Hard Boundaries

- Do not edit `db/migrations/**` without explicit user instruction.
- Do not edit the current V1 SoT set without explicit user instruction:
  `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`,
  `docs/V1_MODULE_ARCHITECTURE.md`,
  `docs/V1_INFORMATION_ARCHITECTURE.md`,
  `docs/V1_ASSET_OWNERSHIP.md`, or
  `docs/V1_CUSTOMIZATION_WORKFLOW.md`.
- Do not delete `docs/iterations/**`, `prompts/**`, or archived evidence.
- Do not bulk rewrite files you have not read in this session.
- Operational actions follow §Instruction Scope And Authorization; do not infer production authorization from generic verification work.

## When Stuck

- If two authority files disagree, follow the Working Rule precedence and surface the disagreement to the user.
- If a fix crosses a hard boundary without the required explicit authorization already present, pause that action and ask for direction. Continue independent authorized work.
- If a value, command, path, or behavior cannot be confirmed from the repository itself, write `Unknown` or `To be confirmed` in any document or report you produce. Never invent rules, paths, or behaviors that the repository does not already prove.
