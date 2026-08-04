# Browser A/B ingress

This directory is a local-only, run-scoped ingress for the Dev-Plus V8 A/B
release rehearsal. It does not publish production, edit production Nginx, create
DNS, or create Cloudflare account resources.

## Boundary

The stack contains two isolated MySQL databases, two isolated Redis instances,
two backend artifacts, two clone-attestation gates, two fixture-identity gates,
and four same-origin Nginx edges:

| Edge | Frontend | Backend | Database |
| --- | --- | --- | --- |
| `edge-external-external` | frozen external production artifact | frozen external backend artifact | A |
| `edge-devplus-devplus` | frozen dev-plus artifact | frozen dev-plus backend image | B |
| `edge-external-devplus` | frozen external production artifact | frozen dev-plus backend image | B |
| `edge-devplus-external` | frozen dev-plus artifact | frozen external backend artifact | A |

Only the four edge ports are published, and every mapping is fixed to
`127.0.0.1`. A and B use separate private and ingress networks. No A service is
attached to a B network, so an A backend cannot resolve or reach B MySQL/Redis
(and vice versa), even if a DSN were accidentally changed. The
configuration has no `env_file` reference and cannot inherit
`/root/ecommerce_ai/shared/main.env` or the remote parallel-deploy environment.

The edge template mirrors the relevant production routing semantics without
editing production configuration: `/v1`, `/ws`, `/upload`, the internal
`/_protected/external-alist/p/` byte stream, static files, and SPA fallback.

## Required artifacts

Copy `ab-browser.env.example` to a run-scoped, ignored path and replace every
placeholder. The validator requires:

- MySQL, Redis, Nginx, probe, external-backend and dev-plus-backend images all
  pinned by `@sha256` digest;
- backend images pinned by digest plus matching backend fingerprint values;
- distinct database names beginning with `ab_`;
- a dedicated A backend account whose effective grants contain `SELECT` and no
  write, DDL, execute, trigger, or grant privilege; B uses the clone app account;
- byte-identical A/B auth-settings files plus their expected SHA-256;
- absolute frozen frontend directories containing `index.html`, plus the
  validator's deterministic full-tree SHA-256;
- `A_ui` and `B_ui` import receipts, receipt SHA-256 values, ready markers, and
  exact task counts;
- two local isolated upload-service origins;
- two local isolated read-only object-fixture origins;
- distinct upload/object fixture identities for A and B;
- four unique unprivileged edge ports.

Compute, do not guess, the local file/tree hashes:

```bash
sha256sum /absolute/path/to/auth-a.json /absolute/path/to/auth-b.json
python3 - <<'PY'
import pathlib
from scripts.ab.validate_browser_ab_stack import sha256_tree
for value in ("/absolute/path/to/frozen-external-front", "/absolute/path/to/frozen-devplus-front"):
    path = pathlib.Path(value)
    print(path, sha256_tree(path))
PY
sha256sum /absolute/path/to/a-ui-import-receipt.json /absolute/path/to/b-ui-import-receipt.json
sha256sum scripts/ab/ab_upload_fixture.py
```

Build the fixture image from the repository root with an immutable Python base,
then record the resulting image digest in the run manifest and env file:

```bash
FIXTURE_SHA256="$(sha256sum scripts/ab/ab_upload_fixture.py | awk '{print $1}')"
docker build -f deploy/ab-browser/fixture.Dockerfile \
  --build-arg PYTHON_BASE_IMAGE='python@sha256:<pinned-manifest-digest>' \
  --build-arg FIXTURE_SCRIPT_SHA256="$FIXTURE_SHA256" \
  -t "yongbo-ab-fixture:$FIXTURE_SHA256" .
```

The runtime Compose file never bind-mounts code from the dirty worktree. The
image label `com.yongbo.ab.fixture-script-sha256` must match
`AB_FIXTURE_SCRIPT_SHA256` in the recorded `docker image inspect` evidence.

Use `docker image inspect` to record the actual immutable image references. A
mutable tag is intentionally rejected even when that tag currently resolves to
the desired local bytes.

Use an immutable image reference and record `docker image inspect` output and
its image ID in `environment_manifest.json`. The external frontend should be copied
from the frozen production artifact. Build the dev-plus frontend from the clean
candidate commit; do not mount the dirty source tree into Nginx.

The Compose file runs four instances of the versioned
`scripts/ab/ab_upload_fixture.py` service. `fixture-upload-a` is read-only,
`fixture-upload-b` is the only writable byte store, and both object fixtures are
read-only. Every instance has a distinct identity, root and internal port; none
publishes a host port or proxies another origin. `GET /health` and exact-text
`GET /identity` are verified before either backend starts. A/B frozen seed roots
must be separate run-scoped directories made from the same approved snapshot.
The validator rejects public/host origins, reused roots, write-enabled A/object
mounts, and fixture script hash drift.

The B upload fixture implements only the current Go client contract: create/get
session, raw browser PUT or backend multipart upload, prepared multipart parts,
complete/cancel, file metadata, small multipart upload, and escaped
`GET /files/{storage_key}`. Declared size, MIME and SHA-256 are checked before a
file becomes complete. Bytes are stored only below `AB_UPLOAD_B_ROOT`; JSONL
events are deterministic and never include request headers or tokens. Encoded
path traversal and symlinked seed files fail closed.

Each import receipt is a JSON object with at least:

```json
{
  "schema": 1,
  "side": "A_ui",
  "database": "ab_<run>_a_ui",
  "status": "ready",
  "task_count": 2347,
  "source_snapshot_sha256": "<64 hex>"
}
```

Use `side=B_ui` and the B database for the other receipt. Both receipts must
attest the same `source_snapshot_sha256`. The corresponding ready-marker file
contains only that receipt file's SHA-256. `clone-ready-a/b` then verifies the
receipt hash, marker, non-empty schema, and exact live `tasks` count. This makes
an empty default MySQL volume unable to satisfy backend startup dependencies.
`clone-ready-a` also runs `SHOW GRANTS FOR CURRENT_USER` and rejects any
write/DDL/execute/trigger/grant privilege.

## Import and executable startup order

Do not use the legacy `codex-yongbo-v8-full-backend` container: its
`host.docker.internal:3308` DSN is outside this topology and is not an accepted
dependency. The attested clone endpoints (`127.0.0.1:3311` for A and
`127.0.0.1:3312` for B) may be used only as read-only dump sources. Restore
their bytes into the run-scoped `mysql-a/mysql-b` volumes, create the
SELECT-only A account, and only then write the reviewed receipts/markers.

```bash
AB_ENV=tmp/v8-ab/<run-id>/browser-ab.env
AB_COMPOSE=deploy/ab-browser/compose.yaml

# Start import destinations only; neither backend can start here.
docker compose --env-file "$AB_ENV" -f "$AB_COMPOSE" \
  up -d mysql-a mysql-b redis-a redis-b

# Run the approved read-only dump/restore workflow from 3311/3312. After exact
# count verification, B migration/replay, and A SELECT-only account creation,
# write the two receipt JSON files and their SHA-256 marker files.

python3 scripts/ab/validate_browser_ab_stack.py \
  --compose-file "$AB_COMPOSE" --env-file "$AB_ENV" \
  --output "tmp/v8-ab/<run-id>/browser-stack-preflight.json"

# Full startup blocks on clone-ready-a/b and fixture-ready-a/b.
docker compose --env-file "$AB_ENV" -f "$AB_COMPOSE" up -d
docker compose --env-file "$AB_ENV" -f "$AB_COMPOSE" ps -a
```

Expected pre-Browser evidence:

- `browser-stack-preflight.json` is `PASS` and records network, auth, frontend,
  backend, import-receipt, source-snapshot, and per-edge hashes;
- `clone-ready-a`, `clone-ready-b`, `fixture-ready-a`, and `fixture-ready-b`
  are `Exited (0)`;
- both backends and all four edges run without port `3308` or the legacy
  restarting backend;
- every loopback `GET /__ab/identity` reports the expected frontend, backend,
  and fixture identity before any Quick Tunnel is started.

## Validate and start locally

Validation is read-only:

```bash
python3 scripts/ab/validate_browser_ab_stack.py \
  --compose-file deploy/ab-browser/compose.yaml \
  --env-file tmp/v8-ab/<run-id>/browser-ab.env \
  --output tmp/v8-ab/<run-id>/browser-stack-preflight.json
```

Review `docker compose config` only after validation. Starting the stack is an
explicit local-clone action:

```bash
docker compose --env-file tmp/v8-ab/<run-id>/browser-ab.env \
  -f deploy/ab-browser/compose.yaml up -d
```

Database restore, B migrations, history mapping apply, object fixture setup and
rollback remain separate guarded workflows. Do not start Browser action tests
against the SQL/API evidence databases: restore attested `A_ui` and `B_ui`
subclones or restore the fixed state before every destructive UI wave.

## Quick Tunnel evidence

`browser_quick_tunnels.sh` is plan-only by default:

```bash
scripts/ab/browser_quick_tunnels.sh start \
  --env-file tmp/v8-ab/<run-id>/browser-ab.env \
  --run-dir tmp/v8-ab/<run-id>
```

After installing a WSL/Linux `cloudflared`, the explicit local start is:

```bash
scripts/ab/browser_quick_tunnels.sh start --execute \
  --env-file tmp/v8-ab/<run-id>/browser-ab.env \
  --run-dir tmp/v8-ab/<run-id>
```

The script first runs the stack validator and requires all four loopback
`/health` probes to return 200. After URL discovery, it requires four unique
public HTTPS URLs and rechecks public `/health`, `/__ab/identity`, and `/ping`.
The identity response carries the edge, frontend tree hash, backend image hash,
and fixture identity; `/ping` must carry the expected backend fingerprint
header. Local/public identity and API body hashes must match. It records the
`cloudflared` version, URLs, PIDs, command-line hashes, probes, logs, timestamps,
and SHA-256 evidence under `browser-tunnels/`. `status` verifies the evidence
hashes and every live PID command line. `stop` refuses any PID whose command
identity/hash does not match the recorded tunnel:

```bash
scripts/ab/browser_quick_tunnels.sh stop --execute \
  --env-file tmp/v8-ab/<run-id>/browser-ab.env \
  --run-dir tmp/v8-ab/<run-id>
```

Quick Tunnels create random public test URLs and require no DNS mutation. They
are suitable for the immediate Browser unblock, not a production endpoint.
Fixed hostnames require a separately authorized named Tunnel and DNS operation;
this repository intentionally performs neither.

## Four-combination interpretation

The two same-generation combinations are release gates:

- external frontend + external backend + A_ui proves the frozen baseline;
- dev-plus frontend + dev-plus backend + B_ui proves the candidate behavior.

The cross-generation combinations are compatibility counterexamples, not an
impossible requirement that every old-only and new-only route return identical
success responses:

- external frontend + dev-plus backend + B_ui exercises backend-first rollout;
- dev-plus frontend + external backend + A_ui exercises CDN cache/frontend
  rollback exposure.

Cross-combination differences pass only when the run's approved normalization
rules explicitly list the route, expected status/error, direction, and reason.
An unlisted difference, any 5xx, asset loss/order drift, or permission widening
is still a hard failure. Do not convert an old-only/new-only route mismatch into
a blanket ignore rule or claim 100% route parity for the cross combinations.
Start from `scripts/ab/api-normalization-rules.example.json`, replace every
review placeholder, and record that approved file's SHA-256 in the run ledger.

## Static tests

```bash
python3 scripts/ab/test_browser_ab_stack.py
bash -n scripts/ab/browser_quick_tunnels.sh
```
