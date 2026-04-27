# ITERATION 122

Date: 2026-04-11
Model: GPT-5 Codex

## Goal

Hard-cut the MAIN business asset runtime from NAS to OSS:

- remove NAS from upload/download/storage mainline
- keep NAS SSH access for ops/developer use only
- stop at local validation and review readiness
- do not release

## Outcome

Completed locally.

- task-create reference upload is OSS-backed
- task asset-center small/multipart upload sessions are OSS-backed
- download metadata and file serving are OSS-backed
- NAS probe / allowlist / private-network business logic removed
- NAS business storage provider constants removed from active runtime
- current-use docs rewritten to OSS-only semantics
- NAS SSH access isolated into `docs/ops/NAS_SSH_ACCESS.md`

## Validation

- `go test ./service ./transport/handler` passed
- `go build ./cmd/server` passed
- `go build ./repo/mysql ./service ./transport/handler` passed
- `go test ./repo/mysql` could not execute on this host because Windows App Control blocked the generated `mysql.test.exe`

Focused regression tests passed for:

- reference upload
- reference-file-ref validation on task create
- multipart asset-center upload
- multi-SKU target SKU persistence and audit gating
- product-code preparation
- org/options backendized behavior
- canonical ownership and actor/source projections
- asset file proxy behavior

## Release Status

- not released
- no live migration
- no live config change
- ready for human review
