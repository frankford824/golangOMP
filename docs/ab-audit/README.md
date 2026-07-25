# A/B database audit evidence

`scripts/ab/run-ab-audit.sh` is the only entrypoint for this evidence layout.
It defaults to plan-only mode: it creates a run-scoped manifest, copies the
read-only SQL gates, and hashes its inputs. It does not contact MySQL, Docker,
HTTP, or production until an explicit execution flag is supplied.

## Preconditions

Formal A/B evidence requires one consistent source snapshot for both clones:

1. Record the source dump SHA256, export UTC, source database identity, and
   the clone import UTC in the run manifest.
2. Restore that exact dump into two isolated, locally addressed databases.
3. Record the repository commit, OpenAPI SHA256, backend image digest, and the
   migration ledger before comparing results.

Do not use the existing local ports `3307` and `3308` as formal A/B evidence.
They are historical clones with different names and no recorded shared dump
hash, import timestamp, migration-ledger baseline, or backend commit metadata.
They may be inspected for diagnosis only.

## Commands

Plan-only (safe default):

```bash
scripts/ab/run-ab-audit.sh \
  --mode clone --run-id r20260722a \
  --source-db jst_erp_snapshot --target-db ab_r20260722a_candidate
```

Run the SELECT/`information_schema` gates only after providing credentials by
MySQL defaults files. Defaults-file paths are deliberately omitted from logs.

```bash
scripts/ab/run-ab-audit.sh \
  --mode clone --run-id r20260722a \
  --source-db jst_erp_snapshot --target-db ab_r20260722a_candidate \
  --source-host 127.0.0.1 --source-port 3311 \
  --target-host 127.0.0.1 --target-port 3312 \
  --source-defaults-file /secure/source.cnf \
  --target-defaults-file /secure/target.cnf \
  --snapshot-sha256 <sha256> \
  --manifest-jsonl /evidence/reviewed-manifest.jsonl \
  --manifest-sha256 <sha256> \
  --execute-readonly
```

The runner has no database-write path. Restore and schema migration use the
deployment workflow; resource-history apply/rerun/rollback uses
`workflow-groups-migrate` with `--confirm-database`. The evidence runner only
connects to distinct, locally addressed A/B clones and wraps every SQL input in
a read-only transaction. Arbitrary restore/migration SQL is deliberately
rejected because a `USE` or qualified table name can escape a name-prefix guard.

`workflow-groups-migrate/v8.8` uses snapshot version 8.  In addition to task,
group, revision, asset, planning, organization, access, and auto-increment
state, the snapshot records every managed `task_modules` row and the
before/after task and resource-group search documents.  After task and planning state
migration, every module of a task whose final status is `Completed` is closed
with the same terminal-state contract as the runtime `CompleteModules`
operation.  The snapshot cohort explicitly includes Completed tasks that still
have non-terminal modules, even when they have no migrated resource group.
Rollback first requires the exact apply-time search projection, then restores
the exact module rows and both pre-apply search-document tables before removing
revisions.
This prevents either a later reindex or a search projection foreign key from
turning a reversible revision rehearsal into a destructive partial rollback.

Legacy `erp_product_image` migration is deterministic but optional.  A planning
item inherits an image only when exactly one uploaded, unarchived,
non-superseded, non-placeholder object-backed asset matches the same `task_id`
and exact `scope_sku_code`; its storage row must be active/recorded and owned by
that exact task asset. Dry-run revalidates the same candidate contract, and the
apply transaction repeats it under row locks before writing. No match remains
an approved empty optional image;
multiple matches or any post-review drift are `hard_blocked`. Upload time is
never used as a tie-breaker. G08 independently repeats the same exact
owner/lifecycle/uniqueness predicate for every migration-created current
planning revision; a selected image must be the one eligible storage reference,
while an approved empty image requires zero eligible candidates.

## Evidence artifacts

The default directory is `tmp/v8-ab/<run-id>/` and is ignored by Git:

```text
manifest.env                 # endpoints, flags, Git HEAD, OpenAPI hash
environment_manifest.json    # reproducibility inputs; absent values are null
decision_ledger.jsonl        # claim/evidence/reviewer audit trail
gate_report.json             # G0-G10; fail-closed BLOCKED until verified
go-no-go.md                  # initialized as NO-GO, never optimistic by default
commands.log                 # redacted command intent; never credentials
input.sha256                 # SQL and OpenAPI input hashes
sql/*.sql                    # immutable copied gate inputs
sql/source/*.tsv             # read-only source results
sql/target/*.tsv             # read-only target results
api/*.body                   # optional GET-only probe bodies
evidence.sha256              # final artifact checksum manifest
```

SQL files `00_snapshot_fingerprint.sql` through `12_legacy_timestamp_contract.sql` are
read-only business-integrity gates. `00` is the fingerprint; `01` through `11`
return only rows with `violation_code`, `entity_key`, and `detail`.
`evidence.*` rows are retained as evidence but never counted as violations.
The runner uses one MySQL session per side: it loads the reviewed manifest into
a connection-local TEMPORARY table, starts an explicit read-only transaction,
then runs all 12 queries. A is assessed as the immutable external baseline; B
is assessed against the approved entity manifest and V8 invariants. Only the
immutable event evidence from 07 is directly A/B hash-compared because the
expected workflow state differs on every other migrated surface.

The external A schema may predate the V8 resource/planning tables. A therefore
executes only common-schema gates 00, 01 and 07; the other numbered A artifacts
are typed zero-row baselines. B executes all 00-12 SQL. This is intentional:
installing V8 schema on A would cease to test the external baseline, while a
false B predicate would still make MySQL resolve missing V8 tables.

The runner validates a non-empty JSONL review manifest and its supplied SHA256.
Each row has this exact schema:

```text
run_id,gate_name,entity_key,expected_hash,expected_state,review_state,detail_json
```

`expected_hash` is lowercase SHA-256 and `review_state` is `pass`,
`proposed_review`, or `hard_blocked`. A `pass` row is not accepted merely
because it contains a plausible hash: `detail_json` must bind the canonical
components and derivation method to hashed input artifacts. For database gates,
the hash is exactly:

```text
SHA256(UTF8(component_1 + 0x1f + component_2 + ...))
```

All components are strings and must use the exact MySQL renderings listed
below. This is the same algorithm as
`SHA2(CONCAT_WS(CHAR(31), ...), 256)` in `11_manifest_state.sql`.

## Building the reviewed entity manifest

Do not hand-author the final JSONL and do not copy B hashes back into the
expected side. Build it from immutable source artifacts plus a canonical
entity input:

```bash
python scripts/ab/manifest_loader.py build \
  --run-id r20260722a \
  --entity-input tmp/v8-ab/r20260722a/canonical-entities.json \
  --mapping tmp/v8-ab/r20260722a/migration_mapping_v2.reviewed.json \
  --baseline-attestation tmp/v8-ab/r20260722a/source-attestation.json \
  --approved-decisions tmp/v8-ab/r20260722a/approved-decisions.json \
  --object-verdict tmp/v8-ab/r20260722a/object-verifier.json \
  --projection-expected tmp/v8-ab/r20260722a/g09-projection-expected.jsonl \
  --output tmp/v8-ab/r20260722a/reviewed-manifest.jsonl

sha256sum tmp/v8-ab/r20260722a/reviewed-manifest.jsonl
```

The reviewed mapping must be version 2. Every resource revision and planning
entry must be `confirmed_auto`; candidate states are rejected. The baseline
attestation must bind the frozen dump and baseline fingerprint. Approved
decisions must contain `decision=confirmed`. A G06 pass requires an object
verifier result with `status=PASS` and `violation_count=0`.

`canonical-entities.json` has this top-level contract:

```json
{
  "schema_version": 1,
  "input_sha256": {
    "mapping_sha256": "...",
    "baseline_attestation_sha256": "...",
    "approved_decisions_sha256": "...",
    "object_verdict_sha256": "...",
    "projection_expected_sha256": "..."
  },
  "entities": [
    {
      "gate_name": "G01",
      "entity_key": "task:123",
      "expected_state": "approved",
      "review_state": "pass",
      "derivation_method": "reviewed_mapping_a_truth",
      "components": ["123", "design_task", "Completed", "", "9"],
      "detail": {"source": "frozen A task plus reviewed status mapping"}
    }
  ]
}
```

The loader recomputes every supplied file hash and refuses a mismatch. Required
derivation methods are fixed: G01-G05/G08 use
`reviewed_mapping_a_truth`, G07 uses `immutable_a_truth`, and G09 uses
`independent_projection`. G06 uses `object_verifier`; G10 uses
`human_decision`. If a component cannot be derived independently, emit one
or more `hard_blocked` rows for that gate. Do not use a post-apply B
observation as the expected value; B observations are actuals only.

Canonical database components, in exact order:

| Gate/entity | Components |
| --- | --- |
| G01 `task:<id>` | task id, type, status, current handler or empty, workflow revision |
| G02 `group:<task>:<kind>:<scope>` | task id, scope kind/ref, working revision no/status or empty, finalized revision no/status or empty, migration incomplete, issue |
| G03 `revision:<task>:<kind>:<scope>:<no>` | task/scope/revision no, status, mode, source task asset id or empty, source stage, actor, persisted reason, submitted/finalized UTC timestamp or empty |
| G04 `revision-source:*` | source asset id/type/hash/binding state/role/SKU scope/retouch scope |
| G04 `revision-final:*:<order>` | final asset id/order/item name/type/hash/binding state/role/SKU scope/retouch scope |
| G05 `revision-reference:*:<order>` | reference id, formal asset storage ref or empty, order, frozen ref id/file name/scope |
| G07 task/module event | the immutable event fields in SQL 07/11, including canonical JSON text and microsecond timestamp |
| G08 planning revision | SKU item id/version, approved planning fields, actor, image storage ref |
| G08 retouch requirement | task/requirement ids, text fields, order, deleted flag |
| G09 task/group search | natural task or task/scope keys, finalized revision no, approved projection fields and SHA256 of generated search text |

G02 deliberately uses revision numbers and statuses, not apply-generated
revision IDs. G05 uses the formal asset storage ref, not its surrogate ID.
Legacy task/SKU/reference/asset IDs already frozen in A may remain in natural
entity keys. Deterministic bundle task-asset IDs must already be declared in
the reviewed mapping. Search text must come from an independent deterministic
projection builder; if that builder is unavailable, G09 stays hard-blocked.

### Independent G09 projection expectations

Freeze canonical source inputs from clone A in a read-only consistent snapshot,
then build G09 expectations without connecting to clone B or reading either
search-document table:

```bash
python scripts/ab/projection_expected.py freeze-a \
  --host 127.0.0.1 --port 3311 --user audit_reader \
  --defaults-extra-file /secure/a.cnf --database ab_r20260722_a \
  --snapshot-sha256 <attested-dump-sha256> \
  --output tmp/v8-ab/<run-id>/frozen-a-projection-input.jsonl

python scripts/ab/projection_expected.py build \
  --mapping tmp/v8-ab/<run-id>/migration_mapping_v2.reviewed.json \
  --frozen-a tmp/v8-ab/<run-id>/frozen-a-projection-input.jsonl \
  --snapshot-sha256 <attested-dump-sha256> \
  --output tmp/v8-ab/<run-id>/g09-projection-expected.jsonl
```

The output is canonical, sorted JSONL accepted by `manifest_loader.py build`
through `--projection-expected`. Its SHA-256 is added to every manifest row's
input provenance. Task and resource-group entity keys use natural IDs; no
post-apply revision, group, or task-asset surrogate ID is copied from B.

Task projection ordering is part of the canonical rebuild contract:
task assets use `task_assets.id`, while planning text uses
`task_sku_items.id, task_planning_sku_revisions.id`. The full reindex command
calls the same repository upsert used by incremental writes; it does not carry
a second copy of task projection SQL. Resource-group references and finals use
their explicit revision-item sort order. A pre-materialized source bundle must
already have a frozen filename; otherwise its group is blocked.

After clone-B migration/apply, rebuild all exact-search projections before G09:

```bash
go run ./cmd/tools/search-reindex \
  --dsn '<clone-B-dsn>' --tasks=true --assets=true --products=true
```

Use `--dry-run` first to capture source/before counts. The task rebuild deletes
and recreates `task_search_documents` atomically, verifies its after-count
equals the task source count, and rolls back on any drift or row failure.

G09 also manifests every customer publication pin using its natural
task/group/revision/item coordinates. Enabled task-resource-group pins may
point to an immutable `finalized` or historical `superseded` revision in the
same reviewed group; draft/submitted/rejected or missing revisions are blocked.
SQL gate 09 independently
rejects malformed pins, retry/alert-threshold outbox failures, wrong ERP SKU
scope, and duplicate ERP/search dedupe keys. Because the search outbox schema
has no terminal/dead status, `retry` at attempt 5 or later is treated as the
verifiable permanent-failure threshold rather than silently ignored.

To turn a blocker into a reproducible pass: resolve the candidate in the
reviewed mapping/decision ledger, regenerate the affected canonical components
from frozen A plus the approved rule, rerun the object/projection verifier when
applicable, update all bound input hashes, rebuild the JSONL, and rerun
the entire audit. Editing `expected_hash` directly is rejected by the loader.

## Frozen-clone migration candidate generator

`scripts/ab/generate_migration_manifests.py` reads a loopback MySQL clone in
one `REPEATABLE READ`, `READ ONLY`, consistent-snapshot transaction. It emits
`migration_mapping_v2.candidate.json`, `manual_review.csv`,
`object_manifest.jsonl`, and `manifest_hashes.json`. It rejects remote hosts,
system databases, an empty expected-scope result, and output-directory reuse.

The result is deliberately not apply-ready. Event-time reconstruction is
`proposed_review`; missing actors/files, multiple source candidates and
purchase-task planning are `hard_blocked`. Reviewers must confirm revision
boundaries, ordered asset membership and scope; materialize any deterministic
ZIP; fill the review fields; then recompute the migration tool's canonical row
hash. The generator never turns an inference into `confirmed_auto`.

```bash
python scripts/ab/generate_migration_manifests.py \
  --host 127.0.0.1 --port 3306 --user audit_reader \
  --defaults-extra-file /path/to/local-clone.cnf \
  --database yongbo_v8_frozen_b \
  --output-dir tmp/v8-ab/<run-id>/migration-candidates
```

`db/migrations/` remains the production schema migration source. The A/B SQL
numbering is audit-phase numbering only and must never be added to or confused
with the deployment migration ledger.

## Coverage matrix and current limits

The SQL gates are database-only checks; they are not an object-store verifier.
The manifest is consumed in the same session, and independent object/projection
evidence is admitted only through hash-bound pass rows. Missing evidence stays
blocked rather than being silently treated as PASS.

| Gate | Database coverage now | Manifest/object coverage | Status |
| --- | --- | --- | --- |
| 01 task state | structural state assertions | exact expected task rows | pass only with reviewed mapping/A truth |
| 02 group coverage | scope ownership and completeness | exact expected groups/pointers | pass only with reviewed mapping/A truth |
| 03 revision chain | pointers, sequence and timestamps | exact history/reason/evidence metadata | pass only with reviewed mapping/A truth |
| 04 asset role/scope | binding/type/order | exact approved asset membership | ZIP requires verified G06 |
| 05 references | task/scope/snapshot identity | exact frozen references | pass only with reviewed mapping/A truth |
| 06 storage | relational storage checks | object existence/MIME/size/SHA256 | requires verified G06 artifact |
| 07 events | immutable parity and trace integrity | exact event hashes and revision reason linkage | requires immutable A truth and G03 |
| 08 planning/retouch | planning/retouch completeness | exact approved business fields | pass only with reviewed mapping/A truth |
| 09 search/publish/outbox | projection and outbox integrity | exact deterministic search documents | blocked without independent projection |
| 10 negatives | unresolved/orphan/unavailable checks | confirmed decision state | any non-pass manifest row blocks |

`ab_manifest_entities` is always a connection TEMPORARY table and is never
created on production or either persistent clone schema. An object adapter that
cannot HEAD/read any manifest object must emit a failing verdict; the builder
then forces G06 to `hard_blocked`.

### G06 read-only object verification

`object_manifest_verifier.py` is the byte-level G06 verifier. The
upload-service adapter performs only `GET /files/<escaped-object-key>`; the
optional OSS/read-gateway adapter performs `HEAD` followed by `GET` (and falls
back to `GET` when `HEAD` returns 405). Every successful read compares object
existence, actual byte count, normalized `Content-Type`, and a streamed
SHA-256. `ETag` is never accepted as a SHA-256 substitute.

```bash
AB_UPLOAD_READ_BASE_URL=http://127.0.0.1:8092/files \
AB_UPLOAD_READ_HEADERS_FILE=/secure/ab-upload-read-headers.json \
AB_UPLOAD_READ_BEARER_TOKEN="$READ_ONLY_TOKEN" \
python scripts/ab/object_manifest_verifier.py \
  tmp/v8-ab/<run-id>/object_manifest.jsonl \
  tmp/v8-ab/<run-id>/object_verdict.json
```

Use `AB_OSS_READ_BASE_URL`, `AB_OSS_READ_HEADERS_FILE`, and
`AB_OSS_READ_BEARER_TOKEN` for an explicitly authorized OSS-compatible HTTP
read endpoint. Base URLs may also be supplied with `--upload-base-url` or
`--oss-base-url`; credentials themselves are accepted only from environment
variables or JSON header files. Header files must contain a JSON object of
string header names and values. Do not place credentials in command arguments.

The verifier does not follow redirects, never records URLs, response bodies,
request headers, or tokens, and issues no write request. Each unreadable or
unconfigured object is retained as an individual `hard_blocked` violation.
Manifest entities that identify the same key and the same expected
size/MIME/SHA-256 reuse one completed byte read; every entity still receives
its own checked result or violation. Failed and incomplete reads are not
cached.
The deterministic result contains `status`, `violation_count`,
`checked_count`, `manifest_sha256`, `evidence_hash`, and the sorted violations.
Only `status=PASS` together with `violation_count=0` satisfies the G06 manifest
loader contract.

`verify_delivery_source_alias_object_coverage.py` closes the relational half
of G06 after Clone B apply. It enumerates every binary-exact
`source + migration` asset without filtering remarks, verifies every approved
delivery-source alias and revision use, and treats the seven source bundles as
a separate exact contract. A PASS artifact binds the historical-unavailable
attestation, the administrator decision template, the confirmed bundle
manifest, all frozen bundle asset/storage/member allocations, the seven bundle
object-manifest rows, and canonical hashes of the observed Clone B asset,
group, storage, and revision-usage rows. Bundle rows are excluded from legacy
alias counts only after this full validation succeeds.

#### Frozen task-2807 recovery domain

The approved task-2807 deleted-asset recovery is not a remote-object
exception. `prepare_g06_object_manifest.py` removes exactly the three original
HTTP-404 rows (`task_asset:23989`, `task_asset:23990`, and
`task_asset:23991`) from remote hydration, in addition to the independent
`task_asset:12323` historical-unavailable tombstone. It accepts only the
final-reviewed mapping
`725074175bb2ef15a149f87ed4c1da6fc3dc8aac143b49c178a43847d8733ce9`
and the hold-open-004 recovery plan
`1e3fbaabb2cad62d69473d1456a9b4e079bb3c383eff97175b92c745316d84ff`.
No old OSS row is assigned a digest.

`rebase_g06_recovery_checkpoint.py` preserves every non-target completed and
failed checkpoint record byte-for-byte, removes only the three exact
`object_manifest.missing/http_status=404` records, and binds the checkpoint to
the new hydration-input hash. Any other row delta, failure result, or
checkpoint result is rejected.

At finalization, the three entities are reconstructed as
`storage_adapter=clone_b_recovery` rows from the plan's deterministic target
UUID, object key, size, MIME, and SHA-256. The independent
`verify_g06_clone_b_recoveries.py` verifier performs contained, non-symlink,
regular-file reads of those three local Clone B targets and also validates:

- initial DB apply `changed=3/already=0`, committed;
- idempotent DB apply `changed=0/already=3`, committed;
- database `ab_r20260723_01_b`, loopback host, run ID, and plan hash;
- component `APPLIED` receipt, artifact hashes, retained rollback guard, and
  `production_writes_executed=false`.

Finalization has two independent mapping boundaries and neither may substitute
for the other. `--recovery-mapping` must hash to the frozen final-reviewed
mapping above and remains bound to the recovery plan. `--bundle-mapping` must
hash to the source mapping recorded by the confirmed source-bundle manifest;
the confirmed manifest and materialized registry are then validated against
that separate hash. Both expected SHA-256 values are required by the CLI and
are emitted separately as `recovery_mapping_sha256` and
`bundle_mapping_sha256`. Hash separation alone is insufficient: the finalizer
also requires the final-reviewed mapping to contain exactly the seven frozen
bundle scopes and compares each normalized `source_bundle` object, including
ordered members, with the validated registry. The summary records
`reviewed_bundle_scope_count=7` and
`reviewed_bundle_semantics_sha256` over the canonical ordered scope projection.

The final G06 adjudicator separately revalidates the same plan, receipts,
recovery rows, local verifier verdict, rebased checkpoint, and hydration
evidence. The frozen composition is exactly 29,046 remote rows, three recovery
rows, seven bundle rows, one historical-unavailable row, and 29,057 rows total;
the hydration boundary is exactly 9,867 missing fingerprints and 8,829 unique
remote targets.
