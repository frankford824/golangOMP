# CLAUDE.md

> **Single source of truth**: this repository's canonical agent contract is `AGENTS.md`.
> Read `AGENTS.md` in full at the start of every Claude Code session.
> If anything in this file disagrees with `AGENTS.md`, `AGENTS.md` wins.

## Why this stub exists

Anthropic's Claude Code reads `CLAUDE.md` automatically. To avoid two drifting agent contracts, all repository rules live in `AGENTS.md`. This file only adds Claude-Code-specific behavior on top.

## At session start (Claude Code)

1. Read `AGENTS.md` end to end.
2. Run `git status` and the two `git log` commands from `AGENTS.md` §Session Start.
3. Read the three authority files in `AGENTS.md` §Reading Order for the route family being touched.
4. Do not assume any previous chat context is preserved.

## Validation gate (Claude Code)

Prefer the consolidated script over running commands by hand:

```bash
./scripts/agent-check.sh        # Linux / macOS / WSL
```

```powershell
.\scripts\agent-check.ps1       # Windows PowerShell
```

If the script fails, do not work around it — read its output, fix the cause, rerun.
For narrow service-only changes, the narrow gate in `AGENTS.md` §After Editing still applies.

## Response format at end of any non-trivial task

Always finish with these five sections, in order:

1. **Summary** — what changed and why.
2. **Changed files** — full paths.
3. **Commands run** — exact commands and their exit status.
4. **Test result** — pass/fail counts; note any skipped.
5. **Risks / follow-ups** — anything not closed in this turn, including new governance debt.

## When something is genuinely unknown

Write `Unknown` or `To be confirmed` in any document you produce. Do not invent rules, paths, or behaviors that the repository does not already prove.
