# CODEX_SESSION_BOOTSTRAP

This is the standing entry prompt for Codex / Claude Code sessions in this
monorepo: Go shared-backend at the root; main-ops and asset-workbench in `vue/`.
Append the actual task under "Task this turn".

## Start

1. Read root `AGENTS.md`, then any nested `AGENTS.md` applicable to the task.
   Root `AGENTS.md` owns repository workflow, authorization and validation;
   this prompt adds no independent gates or permission requirements.
2. Follow `AGENTS.md` sections "Session Start" and "Reading Order". Measure the
   current branch, HEAD and working tree; protect inherited changes. Historical
   snapshots are not evidence of current test results or deployment state.
3. State whether the task affects backend, frontend or both, identify the
   relevant route family/contracts when applicable, and state the matching
   validation scope before editing. For guidance/tooling work, say when no
   business route or API contract is affected.

## Execute and verify

Follow `AGENTS.md` sections "Instruction Scope And Authorization", "Before
Editing", "After Editing" and "Hard Boundaries". Continue authorized work;
resolve routine details with stated assumptions. Ask for missing information
only when it materially changes the result or authorization is absent.

A failed check requires diagnosis and an in-scope fix followed by verification.
It does not automatically require an ABORT document or ending the task. Follow
the root rules when a real blocker remains. Do not claim unexecuted checks passed.

## Close

Use `AGENTS.md`'s response format and end-of-turn Git measurements. State what
changed, the checks actually run, and any remaining limitations. Apply its
commit style when a commit is part of the task; this prompt does not request
an automatic commit, release, push or historical-baseline update.

The previous prompt and its old baseline are retained only as evidence in
`docs/archive/agent_guidance/CODEX_SESSION_BOOTSTRAP_pre_consolidation.md`.

## Task this turn

> {Replace with the actual task, scope and constraints.}
