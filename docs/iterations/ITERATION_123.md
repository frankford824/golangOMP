# ITERATION_123

Date: 2026-04-13
Model: GPT-5 Codex

## Goal

Perform a small OSS contract cleanup round after the backend mainline review:

- lock canonical OSS asset routes for frontend rollout
- demote compatibility alias routes and compatibility-only fields in docs and OpenAPI
- obsolete the old frontend alignment entrypoint
- avoid runtime redesign, release work, and scope expansion

## Completed

- updated `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md` to restate the canonical OSS asset contract and explicitly ban compatibility aliases from new frontend work
- updated `docs/api/openapi.yaml` route descriptions and field descriptions so compatibility-only aliases and fields are marked obsolete for frontend rollout
- updated `docs/API_USAGE_GUIDE.md`, `docs/ASSET_UPLOAD_INTEGRATION.md`, `docs/V7_API_READY.md`, and `docs/V7_FRONTEND_INTEGRATION_ORDER.md` so frontend-facing docs present only canonical OSS routes for new work
- downgraded `docs/FRONTEND_ALIGNMENT_v0.5.md` into an obsolete historical marker with pointers to the current sources of truth
- refreshed `CURRENT_STATE.md`, `MODEL_HANDOVER.md`, and `docs/V0_9_MODEL_HANDOFF_MANIFEST.md` so handoff/index docs no longer leave the obsolete frontend doc usable as an onboarding source

## Not Done By Design

- no upload/download runtime redesign
- no route removal
- no behavior rewrite
- no release, deploy, migration, or live config change

## Local Validation

- `go test ./service ./transport/handler` -> passed
- `go build ./cmd/server` -> passed
- `go build ./repo/mysql ./service ./transport/handler` -> passed
- `go test ./repo/mysql` -> blocked by Windows Application Control on generated `mysql.test.exe`
