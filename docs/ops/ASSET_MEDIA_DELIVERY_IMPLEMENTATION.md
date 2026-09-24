# Asset media delivery implementation

Approved scope: unified strict previews, durable media jobs, full `/p3` inventory,
Quark disabled in this application only, NAS originals plus OSS cache, native
downloads and bounded external ZIPs. AList/BFF/shared MySQL remain intact.

Baseline: `79672c8675e6c8581f2fa8dea802cc8a3454258f` on
`dev/external-developer`; clean working tree. This is an execution checklist,
not a replacement for routes/OpenAPI/V8 authority.

## Delivery gates

- [ ] Durable queue, subscriptions, version metadata and additive migration.
- [ ] Strict version-bound previews and independent ECS worker.
- [ ] Full `/p3` versioned events and completed-scan reconciliation.
- [ ] External source policy, NAS worker and scoped upload grants.
- [ ] NAS gateway: signed + online authorization, snapshots, streaming cache.
- [ ] Main-ops/workbench delivery routing, bounded previews, native downloads.
- [ ] External selection ZIP and recoverable preparation records.
- [ ] Full backend/frontend gates, real browser and integrity checks.
- [ ] Commit/push, guarded release, NAS/DNS/TLS/proxy deployment and receipts.

## Fixed operational defaults

- Originals: `/volume1/image_lib` read-only; logical root `/p3`.
- Required OSS/export root stays `/p3/仓库素材区/徐凯`.
- Gateway: `media-cache.yongbo.cloud`, NAS `192.168.0.125`, loopback port 18089.
- No new public NAS mapping; ECS/NAS retains Tailscale.
- Cache 2 TiB, evict at 85% to 75%; free-space floor 200 GiB.
- Each WAN direction: 30 Mbps 08:00–24:00, 60 Mbps 00:00–08:00 Asia/Shanghai.
- Each render pool concurrency 1, 2 GiB memory, 1 CPU, 10 minute job timeout.
- Thumb 480px/200 KiB; preview 1600px/2 MiB; no automatic original fallback.
- Preview ticket 2 minutes; original/ZIP ticket 10 minutes; reauthorize starts.
- No Codex scheduled checks. Existing product workers/reconciliation continue.

## Evidence

Update this section with measured checks, release SHA and receipts as work lands.
Do not infer deployment completion from this checklist.

### Working checkpoint

Implementation is in progress. This document records a development checkpoint,
not a production acceptance receipt. Business binaries, migration 142 and NAS
delivery have not been deployed or enabled. The certificate repair below is the
only production-serving configuration change so far.

- Added migration 142, media domain/repository, lease/queue worker and strict
  version-pinned system previews; default runtime switches remain disabled.
- Added source policy, high-resolution fingerprints, NAS scan protocol and
  watcher durable journal. Scan application runs in a separate `scan` pool.
- Added signed gateway protocol, online reauthorization coordinator, stable
  Linux snapshots and coalesced streaming cache.
- Added external preparation/ZIP coordination, independent NAS executable,
  scoped multipart grants/checkpoints and internal-ECS legacy SHA verification.
- Native workbench transfer no longer buffers originals. LAN opt-in controls,
  bounded preview states and durable request restoration are wired in part.
- Full six-step backend gate passed on 2026-09-24 before the latest deployment
  template changes. Both frontend builds and workbench architecture audit passed.
- Disposable MySQL 8 container test passed migration 142 and repository tests
  for deduplication, independent cancellation, phase progress, lease takeover
  and rejected late commits. No production database was changed; the disposable
  container and its anonymous volume were removed by the test cleanup.
- Existing service/repository/transport tests passed;
  gateway race tests passed including twenty readers/one origin, early
  streaming, integrity failure, range validation and symlink protection.
- Worker/package schemas, typed response contracts and generated frontend docs
  and types are present; contract audit passed without adding exemptions.
- Streaming cache tests now include restart partial-range reuse and online
  permission rejection status preservation; race test repeated three times.
- Added exact-shard scan validation, deletion-before-first-scan tombstones,
  persisted manifest revalidation and fsynced watcher state replacement.
- ECS live release read-only inspection: v1.396, root filesystem 197G with
  105G available. NAS volume inspection: about 13T available. These are point
  measurements, not capacity reservations.
- Browser validation encountered an expired assets.yongbo.cloud certificate.
  Existing ACME webroot requests were landing on the SPA. A scoped Nginx fix
  was backed up, applied and syntax-checked. Certbot renewal succeeded; live
  served certificate expires 2026-12-23 08:21:55 UTC. Verified HTTPS HTTP 200
  and in-app browser reached the login page without a certificate bypass.
  Renewal reload hook installed; no business binary/database changed by repair.

### Latest validation, 2026-09-24

- Final backend six-step gate passed after the worker claim-class, request
  filename and representation ETag changes. OpenAPI: zero errors/warnings.
- Frontend suite: 137 files, 686 tests passed. Main and workbench builds,
  architecture audit (142 files) and twelve-page accessibility audit passed.
- Main preparation UI, external selection ZIP/partial-error confirmation,
  two-shard completion barrier and guarded artifact cleanup are implemented.
- Authenticated local browser at `http://127.0.0.1:5178/asset-center` could browse
  actual assets. It still uses the OLD production backend. New queue/NAS endpoint
  success must NOT be inferred from this check. The preparation-panel width bug
  found in this check was fixed (measured 448px at a 2560px viewport). The old
  backend's unavailable preparation endpoint displayed a recoverable error,
  not a false ready state. No large original download was initiated.
- Isolated NAS renderer tests passed PSD, PSB, TIFF, PDF, transparent PNG,
  first-frame GIF, WebP and rejection of corrupt/unsupported input. A 133,677,304
  byte JPG produced a 46,006 byte preview and 9,644 byte thumbnail in 153.264s
  with one CPU. Original SHA-256 was unchanged. This is a sample, not a total
  traffic-saving forecast.
- NAS has no CPU quota/PIDs cgroup support. Compose uses CPU pinning and an
  nproc ulimit, with memory limits and the existing unprivileged user.
- ECS-to-NAS Tailscale direct path was verified. Local PC Tailscale was
  intermittently relayed/unreachable; LAN SSH with the existing pinned host key
  worked. No Tailscale configuration was changed.
- A trusted `media-cache.yongbo.cloud` certificate was issued on ECS, expiring
  2026-12-23. NAS installation is NOT done. Machine credentials were generated
  into a private ECS directory; no new flags or services were enabled. Private
  signing keys and whole-bucket credentials must never be copied to NAS.

### Outstanding release blockers / gaps

1. NAS administrator installation and a restricted certificate-transfer SSH
   identity need explicit user confirmation. Do not use Docker to bypass sudo,
   create unrestricted keys, or expose NAS through WAN port mappings.
2. Real NAS control-plane/worker uploads, full `/p3` initial scan and replay,
   online publication authorization, cloud ZIP and cache fault/space scenarios
   still need end-to-end acceptance. Unit/race/isolated database tests do not
   replace those checks.
3. Expired multipart-upload IDs/list-parts reconciliation, per-item manual cloud
   retry, full own-upload thumbnail/NAS adaptation, content-ID browser-cache
   coverage and audit retention cleanup need completion or an explicit reduced
   release scope. Existing group/mixed packaging is deliberately preserved.
4. Production backups/migration, SHA-bound guarded release, NAS TLS/DNS/Mihomo
   and two-subnet HTTPS byte/throughput receipts remain. No new production
   version number is reserved. No Codex recurring monitor is created.

Keep feature switches off until the applicable gates pass. Committing this
checkpoint does not authorize treating the entire plan as delivered.

Rollback invariant: once v2 source indexing is enabled, leave
`ASSET_MEDIA_VERSIONED_SOURCES_ENABLED=true` even when disabling scanner,
workers or LAN delivery. Otherwise legacy download paths could mistake an
unverified old object for the current file version. AList/BFF remain untouched.
