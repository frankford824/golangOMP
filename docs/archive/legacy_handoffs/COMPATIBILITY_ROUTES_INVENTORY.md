# Compatibility Routes Inventory

> Generated as part of v1.0 P0-11 convergence. This is the authoritative list of
> compatibility and deprecated routes. **Do not add new functionality on any of these paths.**

## Classification

| Status | Meaning |
|--------|---------|
| `active-compatibility` | Known to be called by current frontend/integrations. Must not be removed without migration. |
| `candidate-for-v1.0-removal` | Can be removed once v1.0 frontend migration is confirmed complete. |
| `remove-after-frontend-migration` | Blocked on frontend cutover to successor path. |
| `deprecated` | Superseded and should not be used. Removal scheduled. |

---

## Deprecated Routes

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| POST | `/v1/audit` | `/v1/tasks/{id}/audit/*` | candidate-for-v1.0-removal | deprecated |
| POST | `/v1/tasks/:id/assets/upload` | `/v1/assets/upload-sessions` | remove-after-frontend-migration | deprecated |

## Compatibility Routes — ERP / Products

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| GET | `/v1/products/search` | `/v1/erp/products` | candidate-for-v1.0-removal | **active-compatibility** |
| GET | `/v1/products/:id` | `/v1/erp/products/{id}` | candidate-for-v1.0-removal | **active-compatibility** |

> `/v1/products/search` and `/v1/products/:id` are known to be in active use by the
> current frontend. Do NOT remove until frontend has fully migrated to `/v1/erp/products*`.

## Compatibility Routes — Task-Create Asset Center

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| POST | `/v1/task-create/asset-center/upload-sessions` | `/v1/tasks/reference-upload` | remove-after-frontend-migration | compatibility |
| GET | `/v1/task-create/asset-center/upload-sessions/:session_id` | `/v1/tasks/reference-upload` | remove-after-frontend-migration | compatibility |
| POST | `/v1/task-create/asset-center/upload-sessions/:session_id/complete` | `/v1/tasks/reference-upload` | remove-after-frontend-migration | compatibility |
| POST | `/v1/task-create/asset-center/upload-sessions/:session_id/abort` | `/v1/tasks/reference-upload` | remove-after-frontend-migration | compatibility |

## Compatibility Routes — Task Assets (under `/v1/tasks/:id/assets/...`)

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| GET | `/:id/assets/timeline` | `/v1/tasks/{id}/asset-center/assets` | candidate-for-v1.0-removal | compatibility |
| GET | `/:id/assets/:asset_id/versions` | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions` | remove-after-frontend-migration | compatibility |
| GET | `/:id/assets/:asset_id/download` | `/v1/tasks/{id}/asset-center/assets/{asset_id}/download` | remove-after-frontend-migration | compatibility |
| GET | `/:id/assets/:asset_id/versions/:version_id/download` | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download` | remove-after-frontend-migration | compatibility |
| POST | `/:id/assets/upload-sessions` | `/v1/assets/upload-sessions` | remove-after-frontend-migration | compatibility |
| GET | `/:id/assets/upload-sessions/:session_id` | `/v1/assets/upload-sessions/{session_id}` | remove-after-frontend-migration | compatibility |
| POST | `/:id/assets/upload-sessions/:session_id/complete` | `/v1/assets/upload-sessions/{session_id}/complete` | remove-after-frontend-migration | compatibility |
| POST | `/:id/assets/upload-sessions/:session_id/abort` | `/v1/assets/upload-sessions/{session_id}/cancel` | remove-after-frontend-migration | compatibility |

## Compatibility Routes — Task Asset-Center (under `/v1/tasks/:id/asset-center/...`)

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| POST | `/:id/asset-center/upload-sessions` | `/v1/assets/upload-sessions` | remove-after-frontend-migration | compatibility |
| POST | `/:id/asset-center/upload-sessions/small` | `/v1/assets/upload-sessions` | remove-after-frontend-migration | compatibility |
| POST | `/:id/asset-center/upload-sessions/multipart` | `/v1/assets/upload-sessions` | remove-after-frontend-migration | compatibility |
| GET | `/:id/asset-center/upload-sessions/:session_id` | `/v1/assets/upload-sessions/{session_id}` | remove-after-frontend-migration | compatibility |
| POST | `/:id/asset-center/upload-sessions/:session_id/complete` | `/v1/assets/upload-sessions/{session_id}/complete` | remove-after-frontend-migration | compatibility |
| POST | `/:id/asset-center/upload-sessions/:session_id/cancel` | `/v1/assets/upload-sessions/{session_id}/cancel` | remove-after-frontend-migration | compatibility |
| POST | `/:id/asset-center/upload-sessions/:session_id/abort` | `/v1/assets/upload-sessions/{session_id}/cancel` | candidate-for-v1.0-removal | compatibility |

## Compatibility Routes — Outsource

| Method | Path | Successor | Removal Phase | Status |
|--------|------|-----------|---------------|--------|
| POST | `/v1/tasks/:id/outsource` | `/v1/tasks` | candidate-for-v1.0-removal | compatibility |
| GET | `/v1/outsource-orders` | `/v1/customization-jobs` | candidate-for-v1.0-removal | compatibility |

---

## Compatibility Roles

The following roles are compatibility-only and must not be used in new authorization logic:

| Role | String Value | Notes |
|------|-------------|-------|
| Admin | `Admin` | Treated as SuperAdmin equivalent for backward compatibility |
| OrgAdmin | `OrgAdmin` | Limited org-scope management, being phased out |
| RoleAdmin | `RoleAdmin` | Role assignment only |
| DesignDirector | `DesignDirector` | Design department scope |
| DesignReviewer | `DesignReviewer` | Design review scope |
| Outsource | `Outsource` | Outsource workflow |
| ERP | `ERP` | ERP integration agent |

---

## Rules

1. **No new development** may target compatibility routes or roles.
2. Frontend must complete migration to successor paths before removal.
3. Removal requires evidence that the compatibility path has zero active callers.
4. `/v1/products/*` is explicitly marked as active-compatibility — do NOT remove.
