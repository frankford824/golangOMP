# ITERATION_088

## Title

MAIN engineering cleanup, three-endpoint control-plane fact solidification, and archival of the task-create reference 0-byte fix.

## Background

- The local MAIN workspace has already become the only practical control plane for server and NAS collaboration.
- The live server release line is still `v0.8` and the current production practice is overwrite publish onto the existing `v0.8` directory.
- The repository still contained mixed wording from older phases:
  - older live-version notes
  - older upload-mode notes
  - older task-create reference completion wording
  - pre-formal three-endpoint wording that still sounded provisional
- The task-create reference 0-byte fix was already closed in code, but it was not yet fully archived as one authoritative engineering record.

## Current Effective Entrypoints

### Runtime and build

- Production runtime/build entrypoint:
  - `cmd/server/main.go`
- Route registration:
  - `transport/http.go`
- Production packaging is locked to:
  - `./cmd/server`
- Deprecated compatibility-only entrypoint:
  - `cmd/api`

### Package and deploy

- Canonical package/deploy entrypoints:
  - `deploy/deploy.sh`
  - `deploy/remote-deploy.sh`
  - `deploy/package-local.sh`
- Current live release mode:
  - overwrite publish onto `v0.8`
- Current live MAIN binary target:
  - `/root/ecommerce_ai/releases/v0.8/ecommerce-api`
- Current live deploy scripts:
  - `/root/ecommerce_ai/releases/v0.8/deploy`
- Current live logs:
  - `/root/ecommerce_ai/logs`
- Live verification methods:
  - `/proc/<pid>/exe`
  - `sha256sum`
  - `/health`

### Three-endpoint collaboration

- Local MAIN repo = the only control plane.
- Server alias = `jst_ecs`
- NAS alias = `synology-dsm`
- Server standard tmux session = `main-live`
- NAS standard tmux session = `nas-upload`
- NAS tmux entry command:

```bash
ssh synology-dsm "source ~/.bashrc >/dev/null 2>&1; tmux new -As nas-upload"
```

## Deprecated or Conflicting Entrypoints and Wording

### Deprecated entrypoints

- `cmd/api` as a production build/deploy entry
- raw-IP daily SSH usage instead of `jst_ecs` and `synology-dsm`

### Deprecated wording

- any wording that implies the current live line is `v0.5`, `v0.6`, or a new `v0.9`
- any wording that implies `delivery` is currently a small-upload path
- any wording that implies task-create `reference` small upload must call NAS `complete`
- any wording that implies `/v1/assets/files/*` caused the reference 0-byte bug
- any wording that says the three-endpoint control plane is not yet formed

## Current Code Truth

### A. `reference-upload`

- Handler:
  - `transport/handler/task_create_reference_upload.go`
- Service:
  - `service/task_create_reference_upload_service.go`
- Upload client:
  - `service/upload_service_client.go`
- Current chain:
  1. MAIN creates a pre-task `reference` small upload session.
  2. MAIN uploads the file to NAS `/upload/files`.
  3. MAIN reads `/upload/files` response fields:
     - `file_id`
     - `storage_key`
     - `file_size`
  4. MAIN probes the stored file by `storage_key`.
  5. MAIN verifies stored `size` and `sha256`.
  6. MAIN binds the verified object into `asset_storage_refs` and returns `ReferenceFileRef`.
- Why the small path no longer calls NAS `complete`:
  - remote manual verification proved `create + /upload/files + /complete` could land a 0-byte file
  - remote manual verification proved `create + /upload/files` alone landed the file correctly
  - therefore the small reference path must stop at `/upload/files` and use probe verification before success

### B. Asset center

- Handler:
  - `transport/handler/task_asset_center.go`
- Service:
  - `service/task_asset_center_service.go`
- Current mode split:
  - `reference` = `small`
  - `delivery` = `multipart`
  - `source` = `multipart`
  - `preview` = `multipart`
- Browser multipart host:
  - `192.168.0.125:8089`
- MAIN service-to-service host:
  - `UPLOAD_SERVICE_BASE_URL`

### C. `/v1/assets/files/*`

- Handler:
  - `transport/handler/asset_files.go`
- Role:
  - read proxy only
- Current conclusion:
  - it is not the storage writer
  - it is not the root cause of the 0-byte incident
  - the incident is closed by fixing the task-create reference small path and adding stored-file verification

## Incident Record: Task-Create Reference 0-Byte Landing

### Symptom

- `POST /v1/tasks/reference-upload` returned `201`
- returned `ReferenceFileRef` structure looked normal
- newly uploaded reference `public_url` read returned:
  - `200`
  - `Content-Length=0`
  - empty body
- historical references remained normal

### Single root cause

- Not a MAIN proxy-body forwarding bug
- Not a global NAS `/files/*` failure
- Single root cause:
  - NAS small upload plus `complete` pseudo-success

### Effective fix strategy

- task-create `reference` small path no longer calls NAS `complete`
- MAIN uses `/upload/files` response as the canonical small-upload result
- MAIN probes the stored object before creating success metadata
- MAIN rejects any size/hash mismatch
- MAIN refuses to wrap a 0-byte physical object as a successful ref

### Relevant code files

- `transport/handler/task_create_reference_upload.go`
- `service/task_create_reference_upload_service.go`
- `service/upload_service_client.go`
- `transport/handler/asset_files.go`
- `service/task_asset_center_service.go`

### Acceptance

- new reference samples now land on NAS with correct size/hash
- MAIN `public_url` downloads now return correct size/hash
- creating tasks with the new refs succeeds
- task detail readback is normal
- historical references do not regress
- current routing remains:
  - `reference` still uses `small`
  - asset-center multipart browser host remains `192.168.0.125:8089`

## Current Boundaries

- This iteration does not create a new release line.
- Current live wording must stay on overwrite-published `v0.8`.
- Windows local control node must keep SSH keepalive and disable connection multiplexing.
- Linux/macOS may enable SSH multiplexing optionally.
- MAIN remains the coordinator; NAS remains the storage and transfer engine.

## Future Constraints

> `task-create reference` 的 small 上传链路以 `/upload/files` 返回结果为准，不再调用 NAS `complete`；MAIN 必须对落盘结果做 size/hash 校验，校验失败直接报错，不得生成成功 ref。

- Plain English: the task-create reference small-upload path must trust `/upload/files`, must not call NAS `complete`, and must fail immediately on stored size/hash mismatch instead of returning a successful ref.
- Future docs must preserve:
  - local MAIN repo as the only control plane
  - `jst_ecs` as the server alias
  - `synology-dsm` as the NAS alias
  - current mode split of `reference=small`, `delivery/source/preview=multipart`
  - `UPLOAD_SERVICE_BASE_URL` for service-to-service traffic
  - `192.168.0.125:8089` for browser multipart traffic

## Documentation Updated in This Iteration

- `CURRENT_STATE.md`
- `MODEL_HANDOVER.md`
- `ITERATION_INDEX.md`
- `docs/THREE_ENDPOINT_CONTROL_PLANE.md`
- `docs/ASSET_UPLOAD_INTEGRATION.md`

## Final Conclusion

- MAIN engineering/docs/entrypoint cleanup is complete for the current baseline.
- The task-create reference 0-byte landing issue is closed and fully archived.
- The local MAIN workspace is now formally documented as the only three-endpoint control plane.
