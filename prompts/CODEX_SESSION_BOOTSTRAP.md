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

> Refresh this section in one `docs(governance)` commit whenever the
> facts below drift more than ±5 commits or the audit summary changes.

- Last release tag: `v1.21-prod` (commit `207f9a1`).
- Current HEAD at handover: `b1e4551`
  (`docs(governance): add V1.4 governance prompt - drift purge + sprawl cleanup roadmap`).
- 59 commits exist between `v1.21-prod..HEAD` covering V1.0 / V1.1 /
  V1.2 / V1.3 / V1.4-governance. The 32-commit V1.3 batch landed task
  main flow, search, notifications, assets, ERP, batch, retouch,
  category, and frontend-doc work. The most recent 6 commits are
  V1.4 governance (CLAUDE/AGENTS unification, root-md archival,
  route-split refactor, contract drift closure, V1.4 governance
  prompt).
- Working tree is clean.
- Full audit gate is **green**: `drift=0 unmapped=0 known_gap=65 clean=169 total=234`.
  No baseline drift to inherit. Any new drift introduced in this session
  is yours to close before commit.

## Known unresolved (do not silently inherit; surface in your plan)

1. **`known_gap=65` are all `<no-reason>`.** The audit's grey-zone
   mechanism is unaudited — some of those 65 may be silenced drift.
   The standing prompt to fix this is `prompts/V1_4_GOVERNANCE.md §A1`.
   If your task is unrelated, leave them alone; if your task is
   governance, start with §A1.

2. **`AGENTS.md` §Reading Order #4** still says `git log --oneline -20`.
   For long unreleased windows like the current 59-commit window,
   `git log --oneline v1.21-prod..HEAD` is more accurate. Either is
   acceptable; if you change it, change it in one small
   `docs(governance)` commit and do not bundle with business work.

3. **No new `_test.go` files were added across the 32 V1.3 commits.**
   This is V1.3-T5 governance debt (test coverage gap on V1.3 fixes).
   When you change behavior in `tasks/search/notifications/assets/erp/batch`
   in this session, prefer adding regression tests in the same commit.
   Tracked in `prompts/V1_4_GOVERNANCE.md §C`.

4. **Four oversized service/repo files awaiting refactor split**:
   `service/identity_service.go` (110 KB), `service/task_service.go`
   (103 KB), `service/export_center_service.go` (82 KB),
   `repo/mysql/task.go` (68 KB). Tracked in
   `prompts/V1_4_GOVERNANCE.md §B`. Do not split them as a side effect
   of any other task.

## Task this turn

> **{REPLACE THIS BLOCK with the actual task}**
>
> Example:
>
> > Execute `prompts/V1_4_GOVERNANCE.md §A` (governance phase 1).
> > Run A1 → A8 in order, each subsection its own commit, full
> > agent-check gate green after every commit. Stop on ABORT-A1 if
> > K3 count > 10.

Begin with Step 1.
