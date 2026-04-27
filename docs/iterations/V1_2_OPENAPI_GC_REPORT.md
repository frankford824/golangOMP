# V1.2 · OpenAPI GC Report

## §1 unreachable-schemas-deleted

- before: schemas=313, reachable=298, unreachable=15
- after: schemas=298, reachable=298, unreachable=0
| schema | grep evidence | bucket | action |
|---|---|---|---|
| AuditRecord | 4 hits: transport/handler/audit_log.go;transport/handler/audit_log_test.go;transport/handler/audit_v7.go;transport/handler/task_action_authorization_test.go | B/C evidence-cleaned-or-non-contract | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| AvailableAction | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| BatchCreatePreviewItem | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| BatchCreateViolation | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| DerivedStatus | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| ExportJobAdapterMode | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| ExportJobStorageMode | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| RouteAccessPlaceholder | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskCostInfo | 1 hits: transport/handler/task.go | B/C evidence-cleaned-or-non-contract | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskCostQuotePreviewResponse | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskModule | 1 hits: docs/frontend/V1_API_TASKS.md | B/C evidence-cleaned-or-non-contract | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskModuleProjection | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskModuleScope | 0 hits: - | A | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskPriority | 1 hits: transport/handler/task.go | B/C evidence-cleaned-or-non-contract | deleted from OpenAPI; frontend reference cleaned in P6 where needed |
| TaskProductInfo | 1 hits: transport/handler/task.go | B/C evidence-cleaned-or-non-contract | deleted from OpenAPI; frontend reference cleaned in P6 where needed |

## §2 deprecated-paths-decision

| method | path | bucket | mount evidence | alternative path | action |
|---|---|---|---|---|---|
| GET | `/v1/products/search` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/products/{id}` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/task-create/asset-center/upload-sessions` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/task-create/asset-center/upload-sessions/{session_id}` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/complete` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/task-create/asset-center/upload-sessions/{session_id}/abort` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/assets/timeline` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/assets/{asset_id}/download` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/assets/{asset_id}/versions/{version_id}/download` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/assets/upload-sessions` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/assets/upload-sessions/{session_id}` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/complete` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/assets/upload-sessions/{session_id}/abort` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/assets/upload` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/asset-center/assets` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/download` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/asset-center/assets/{asset_id}/versions/{version_id}/download` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/small` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/multipart` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/complete` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/cancel` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/asset-center/upload-sessions/{session_id}/abort` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/tasks/{id}/outsource` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| GET | `/v1/outsource-orders` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |
| POST | `/v1/audit` | D1 | mounted: True | successor/compatibility headers in transport where applicable | kept; `x-removed-at: v1.3` |

## §3 cascade-deleted-schemas

No cascade deletion after deprecated path decision; all 29 deprecated paths remain mounted and were not deleted.

## §4 known-gap

| class | method/path | evidence | disposition |
|---|---|---|---|
| mounted-not-documented | `GET /health` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `GET /healthz` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `GET /ping` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `GET /v1/rule-templates` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `GET /v1/rule-templates/{type}` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `POST /v1/incidents/{id}/assign` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `POST /v1/incidents/{id}/resolve` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `POST /v1/sku/preview_code` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `PUT /v1/policies/{id}` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| mounted-not-documented | `PUT /v1/rule-templates/{type}` | transport AST route audit | V1.2-B OpenAPI/path policy review |
| documented-not-mounted | `GET /internal/jst/ping` | OpenAPI parser vs transport AST route audit | V1.2-B integration route ownership review |
| documented-not-mounted | `POST /jst/sync/inc` | OpenAPI parser vs transport AST route audit | V1.2-B integration route ownership review |
