# V1.4 GOVERNANCE — drift purge + sprawl cleanup

Goal: eliminate every currently known drift source and sprawl source so
that future iterations cannot silently re-introduce them. After §A,
the audit gate, the SoT, the agent contract, and the doc layout are all
mutually consistent and self-documenting.

This document has three sections:

- **§A — P0 must-do, single session, ~1–2 hours.** Closes every known
  governance debt that does not require splitting code.
- **§B — P1 sprawl, multi-session.** Split the four oversized files.
  Each file is its own session and its own commit.
- **§C — P2 ongoing.** Test-coverage regression debt and CI gate
  upgrade. Tracked, not executed in this prompt.

Prerequisites: working tree clean, on commit `d401ce2` or later,
`./scripts/agent-check.sh` (or `.ps1`) currently green
(`drift=0 unmapped=0 known_gap=65 clean=169 total=234`).

---

## §A · P0 must-do (single session)

Each subsection ends with one git commit. Do not bundle subsections.
Run the full agent-check gate after every commit; if it goes red,
stop and ABORT — do not continue to the next subsection.

### A1 — Annotate every `known_gap` with a `reason`

Problem: `tmp/agent_check_audit.json` shows `known_gap=65` and **all 65
have `reason=<no-reason>`**. This means the audit's grey-zone mechanism
has no audit trail. Some of these 65 may actually be drift that was
silenced, not legitimate gaps.

Action:

1. Read `tools/contract_audit/main.go` to find where `known_gap`
   verdicts are produced and how a `reason` field can be attached.
2. For each of the 65 `known_gap` paths, classify into one of:
   - **K1 · legitimate gap** — the field genuinely cannot be expressed
     in OpenAPI (e.g. dynamic `oneOf` keyed by another field).
     Reason: `legitimate-<short-text>`.
   - **K2 · OpenAPI uses `$ref` / `oneOf` / `allOf` and audit cannot
     follow** — tool limitation. Reason: `tool-deref-limit-<short>`.
   - **K3 · this is real drift, was previously silenced** — promote
     to `drift` and fix in the same subsection (extend the OpenAPI
     schema or remove the silently-returned field).
3. Persist the reason inside the audit tool itself (a static map
   keyed by `method+path[+field]`, or a small JSON sidecar that the
   tool reads). Do not annotate paths in OpenAPI comments.
4. Re-run the full gate. End state: every `known_gap` row has a
   non-empty reason; K3 promotions cause `drift>0` then drop to 0
   after the OpenAPI fixes ship.

ABORT-A1: if K3 count exceeds 10, stop and write findings to
`tmp/v1_4_a1_k3_findings.md` and return for architect direction
before any K3 fix is committed.

Commit: `fix(audit): annotate every known_gap with a reason and promote K3 to drift`

### A2 — Real-size `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`

Problem: the file is 48 lines yet AGENTS.md declares it the first
authority. Real authority content lives in `V1_MODULE_ARCHITECTURE.md`,
`V1_INFORMATION_ARCHITECTURE.md`, `V1_ASSET_OWNERSHIP.md`,
`V1_CUSTOMIZATION_WORKFLOW.md`.

Action: rewrite `docs/V1_BACKEND_SOURCE_OF_TRUTH.md` so it is an
explicit index pointing at the four files above, with one paragraph
per pointer summarizing what each owns. No new business content;
the four target files remain the substance.

Commit: `docs(governance): make V1_BACKEND_SOURCE_OF_TRUTH.md a real index of authority files`

### A3 — Complete the AGENTS.md `Hard Boundaries #2` SoT list

Problem: AGENTS.md `Hard Boundaries #2` lists 5 V1 SoT files; the repo
has 6+ (`V1_ASSET_OWNERSHIP.md` is missing; review whether
`V1_TO_V2_MODEL_HANDOFF_MANIFEST.md` belongs).

Action: list all `docs/V1_*.md` files, decide which are SoT (do not
edit without explicit instruction) vs evidence (free to update), and
make `Hard Boundaries #2` enumerate the SoT set exactly. Cross-link
each from the new SoT index built in A2.

Commit: `docs(governance): complete Hard Boundaries SoT list (V1_ASSET_OWNERSHIP, etc.)`

### A4 — Archive `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`

Problem: 437-line legacy SoT still sits in `docs/` root and is
referenced by `AGENTS.md §Reading Order #5` as "historical background".
It is real noise next to the V1 SoT files.

Action: `git mv` it to `docs/archive/legacy_specs/`, update the one
AGENTS.md line that references it, leave a `<!-- archived to ... -->`
breadcrumb is not necessary since AGENTS.md will point to the new path.

Commit: `chore(governance): archive V0_9 SoT to docs/archive/legacy_specs/`

### A5 — Resolve three ambiguous root/`docs/` files

Files (3 total — earlier draft of this prompt mistakenly said "four";
the count is 3):
`API_DOC_SYNC_RULES.md` (root, 22 lines),
`docs/RELEASE_NOTES.md` (2 lines, empty shell),
`docs/ENGINEERING_RULES.md` (43 lines).

Action per file:

- **`API_DOC_SYNC_RULES.md`**: read content. If it is a subset of
  AGENTS.md `Before Editing`/`After Editing`, delete it. If it carries
  any rule not in AGENTS.md, fold the missing rules into AGENTS.md and
  delete the file.
- **`docs/RELEASE_NOTES.md`**: delete. Release reports live in
  `docs/iterations/`.
- **`docs/ENGINEERING_RULES.md`**: same merge-or-delete decision as
  `API_DOC_SYNC_RULES.md`.

Commit: `chore(governance): consolidate root/docs ambiguous files into AGENTS.md`

### A6 — Add the "Changed files must be measured, not recalled" rule

Problem: a previous codex session reported `Changed files = 4` while
the working tree had 31 dirty files. Future drift risk.

Action: in both `AGENTS.md §Response format` and
`prompts/CODEX_SESSION_BOOTSTRAP.md` Step 4, add: *"The Changed files
section MUST be derived from `git status --short --untracked-files=all`
and `git diff --stat HEAD` measured at end-of-turn. If the measured
set is larger than what you intentionally changed, list the extras
explicitly and label them `inherited dirty from session-start (not
introduced this turn)`."*

Commit: `docs(governance): require measured Changed files in agent response format`

### A7 — Fix `tools/contract_audit` side effect on docs/iterations

Problem: codex session 2026-04-29 reported the audit tool, when run
with default flags, writes/overwrites
`docs/iterations/V1_2_CONTRACT_AUDIT_v2.md`. An audit tool must never
mutate non-output paths.

Action: route any incidental markdown emission to `tmp/`. Output
locations are controlled solely by `--markdown` and `--output` flags;
no defaults that touch `docs/`. Add a `_test.go` that invokes the tool
with no `--markdown` and asserts `docs/iterations/` is untouched.

Commit: `fix(audit): contract_audit must never mutate docs/iterations/ by default`

### A8 — Phase 1 retro report

Action: write `docs/iterations/V1_4_GOVERNANCE_PHASE1_REPORT.md`
covering A1–A7, with:

- one paragraph per subsection: what was found, what was changed.
- final audit summary (drift / unmapped / known_gap with reasons /
  clean / total).
- the four known oversized files as P1 follow-up (link to §B below).
- explicit "no business code, no migration, no SoT V1_*.md substance
  was changed" attestation.

Commit: `docs(governance): V1.4 governance phase 1 retro`

### §A end-state assertions

When §A is complete, **all of these must be true**, verified by the
commands listed:

```bash
git status --short                # empty
./scripts/agent-check.sh          # green; drift=0 unmapped=0
python -c "import json; \
  d=json.load(open('tmp/agent_check_audit.json')); \
  kg=[p for p in d['paths'] if p['verdict']=='known_gap']; \
  bad=[p for p in kg if not p.get('reason') or p['reason']=='<no-reason>']; \
  print('known_gap_no_reason=', len(bad)); \
  assert len(bad)==0, bad[:3]"
test ! -f docs/RELEASE_NOTES.md
test ! -f docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md   # moved
test -f docs/archive/legacy_specs/V0_9_BACKEND_SOURCE_OF_TRUTH.md
```

---

## §B · P1 sprawl, multi-session, scheduled per file

Do not run §B during §A. Each oversized file gets its own session
with its own bootstrap, its own audit, and its own commit. Splits
must be pure refactor: zero behavior change, zero contract change.

| File | Bytes | Tentative split axis |
|---|---|---|
| `service/identity_service.go` | 110 KB | by responsibility: auth / role / permission / membership |
| `service/task_service.go` | 103 KB | by lifecycle stage: create / assign / progress / close |
| `service/export_center_service.go` | 82 KB | by export domain: tasks / assets / users / reports |
| `repo/mysql/task.go` | 68 KB | by aggregate: task / module / event / asset |
| `transport/http.go` | 70 KB on disk → already 600 lines after §V1.4 split | further split if any sub-area exceeds 200 lines |

Each §B session uses bootstrap + this rule: *"Split a single file by
responsibility. Do not change exported function signatures, do not
change package paths, do not touch OpenAPI, do not touch tests beyond
moving them next to their subject. Audit gate must remain green
before and after."*

Per file, expected commit message:
`refactor(<area>): split <filename> by <axis>`

---

## §C · P2 ongoing — test coverage and CI gate

Tracked here, not executed in this prompt:

1. **Test coverage regression:** the 32-commit V1.3 batch added
   zero new `_test.go` files. Backfill regression tests for the
   five highest-risk surfaces touched in V1.3
   (`service/task_*`, `service/notification/*`, `service/permission/*`,
   `repo/mysql/task_*`, `transport/handler/task*`). Target: keep
   overall test:source ratio ≥ 44.4 % indefinitely.

2. **CI gate upgrade:** `.cursor/hooks/contract-guard.json` runs
   `scripts/contract-guard.ps1` on `git commit`, which only enforces
   OpenAPI-when-Go-touched. It does not run `go vet` / `go build` /
   `go test`. Decide between:
   - **C-a (cheap):** keep contract-guard at commit-time; run full
     `agent-check` at session-end via prompt enforcement only.
   - **C-b (strict):** add a second hook that runs `agent-check` on
     `git commit` for any commit touching `transport/`, `domain/`,
     `service/`, `repo/`, or `docs/api/openapi.yaml`. Slower but
     forecloses the most common drift entry path.

   Default recommendation: **C-b**, with `AGENT_CHECK_SKIP_TESTS=1`
   allowed only on `docs/`-only commits.

3. **Coverage measurement:** add `scripts/agent-coverage.sh` that runs
   `go test -coverprofile=tmp/cover.out ./... && go tool cover -func=tmp/cover.out | tail -1`
   so coverage is observable per session, not estimated.

---

## Hard boundaries for the entire V1.4 governance prompt

- No edits to `db/migrations/**`.
- No edits to the V1 SoT V1_*.md files (substance) — A2/A3 only edit
  the SoT *index* (`docs/V1_BACKEND_SOURCE_OF_TRUTH.md`) and
  AGENTS.md cross-links.
- No `git push`, no `git tag`, no force, no deploy.
- No business code change. The only `_test.go` written is A7's
  audit-tool side-effect regression test.
- Each subsection commits independently; no bundling, no amend.
- After every commit, `./scripts/agent-check.sh` (or .ps1) must be
  green. If it goes red, stop and write
  `docs/iterations/V1_4_ABORT_<subsection>.md` with the failing step
  output, then return for direction.

---

## Begin

Start with §A · A1.
