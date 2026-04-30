# CODEX_SESSION_BOOTSTRAP

This is the standing prompt for **every new Codex / Claude Code session** on
this repository. Paste it as the first message, then append the actual task
under "Task this turn" at the bottom.

---

You are taking over a Go backend (`yongboWorkflow/go`). The repository is a
single-developer, multi-AI-session workflow. Local commits only — no remote
push, no force, no deploy unless explicitly asked.

## Step 1 — Read the contract first (mandatory, non-negotiable)

1. Read `AGENTS.md` end to end. It is the single source of truth for how to
   work in this repo. If `CLAUDE.md` and `AGENTS.md` ever disagree,
   `AGENTS.md` wins (CLAUDE.md is a stub).
2. Run `git status` and the two `git log` commands from `AGENTS.md`
   §Session Start.
3. Read `AGENTS.md` §Reading Order's three authority files for the route
   family this task touches.

Do not edit any business code, OpenAPI, or migrations until step 1 is done.

## Step 2 — Restate before editing

Before your first edit, write back to me:

- The route family / module this task touches.
- Which AGENTS.md hard boundary, if any, this task is at risk of crossing.
- Which gate you will run after editing (full vs narrow), and why.

If the task description is ambiguous, ask one round of clarifying
questions before editing. Do not guess.

## Step 3 — Edit, then validate

- Follow `AGENTS.md` §Before Editing and §Commit Style verbatim.
- After editing, run the appropriate gate from `AGENTS.md` §After Editing:
  - **Full gate** (any change to `transport/`, `domain/`, `service/`,
    `repo/`, or `docs/api/openapi.yaml`): `./scripts/agent-check.sh` on
    Linux/WSL or `.\scripts\agent-check.ps1` on Windows.
  - **Narrow gate** (docs-only or scripts-only or test-only): focused
    `go vet` + focused `go test` for the touched package, and skip
    `agent-check` only when the diff truly does not touch the surfaces
    above.
- If `agent-check` fails, **read the failing step and fix the cause**.
  Never bypass with `AGENT_CHECK_SKIP_TESTS=1` to claim done.

## Step 4 — Close the turn

End the turn with the five sections from `AGENTS.md` §Response format:

1. Summary
2. Changed files
3. Commands run (with exit status)
4. Test result (pass/fail counts; note skipped)
5. Risks / follow-ups

Commit messages follow `AGENTS.md` §Commit Style. One logical change per
commit. Use `git commit -F tmp/<msg>.txt` for multi-line messages on
Windows PowerShell.

## Hard "do not" list (full list lives in `AGENTS.md` §Hard Boundaries)

- No edits to `db/migrations/**`.
- No edits to `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`,
  `docs/V1_MODULE_ARCHITECTURE.md`, `docs/V1_INFORMATION_ARCHITECTURE.md`,
  `docs/V1_ASSET_GOVERNANCE.md`, `docs/V1_CUSTOMIZATION_WORKFLOW.md`.
- No `git tag`, no `git push`, no force, no deploy, no remote SSH, no DB
  writes outside fixtures — unless I explicitly request them.
- No self-declared PASS when `agent-check` is red. Write an ABORT note
  under `docs/iterations/` and stop.
- No bulk rewrites of files you have not read in this session.

## Repository baseline at handover

- Last release tag: `v1.21-prod` (commit `207f9a1`).
- Current HEAD at handover: `1a97e6f`
  (`chore(agent-context): unify CLAUDE.md as AGENTS.md stub + add agent-check full-gate script`).
- 53 commits exist between `v1.21-prod..HEAD` covering V1.0 / V1.1 /
  V1.2 / V1.3. The most recent 32 are V1.3 work (tasks main flow, search,
  notifications, assets, ERP, batch, retouch, category, frontend docs,
  governance).
- Working tree is clean.
- The full gate currently fails at step 5 (contract_audit) because of one
  known unresolved drift — see "Known unresolved" below. Treat this as
  the baseline; you are not required to fix it unless the task explicitly
  asks for it.

## Known unresolved (do not silently inherit; surface in your plan)

1. **`GET /v1/tasks/pool` both_diff**.
   - `only_in_code`: `completed_at, id, receipt_no, received_at, receiver_id, reject_reason, remark, source_department, status, warehouse_ready_version, workflow_lane` (11 fields).
   - `only_in_openapi`: `module_key, pool_team_code, priority, product_code, task_no, title` (6 fields).
   - Introduced by `6872fa1 fix(tasks): stabilize pool response envelope`
     during V1.3; OpenAPI was not synced. Any full-gate run will fail at
     step 5 until this is closed via a C1/C2/C3/C4 decision matrix
     (additive on OpenAPI vs removal vs known_gap vs Go-side regression).
   - Until closed, full gate's `tmp/agent_check_audit.json` will report
     `summary.drift == 1`.

2. **`prompts/V1_3_0_LOCAL_FREEZE.md`** — superseded by this bootstrap and
   by `AGENTS.md`. Treat it as deprecated. Do not execute it. If you
   touch the `prompts/` tree, prepend a "DEPRECATED — superseded by
   AGENTS.md + CODEX_SESSION_BOOTSTRAP.md" line at its top, but otherwise
   do not act on its contents.

3. **`AGENTS.md` §Reading Order #4** still says `git log --oneline -20`.
   For long unreleased windows like the current 53-commit V1.3 period,
   `git log --oneline v1.21-prod..HEAD` is more accurate. Either is
   acceptable; if you change it, change it in one small `docs(governance)`
   commit and do not bundle with business work.

4. **No new `_test.go` files were added across the 32 V1.3 commits.**
   This is V1.3-T5 governance debt (test coverage gap on V1.3 fixes).
   When you change behavior in `tasks/search/notifications/assets/erp/batch`
   in this session, prefer adding regression tests in the same commit.

## Task this turn

> **{REPLACE THIS BLOCK with the actual task}**
>
> Example:
>
> > Close the `/v1/tasks/pool` both_diff. Build the C1/C2/C3/C4 decision
> > matrix for all 17 fields, propose changes to `docs/api/openapi.yaml`
> > only (no Go edits), regenerate frontend docs, run the full gate, and
> > commit with prefix `fix(openapi):`.

Begin with Step 1.
