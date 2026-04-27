# Skill: storage_and_assets

## 1. 适用场景

- 资产上传/下载、元数据、对象存储行为变更。
- `reference-upload` 与 `asset-center` 相关能力调整。
- `download_url`、`download_mode`、文件代理路由相关需求。

## 2. 非适用场景

- 与资产无关的通用任务流转（改用 `task_flow_and_status.md`）。
- 仅发布准备（改用 `release_prep.md`）。

## 3. 必读 Authority Docs

1. `docs/V0_9_MODEL_HANDOFF_MANIFEST.md`
2. `docs/V0_9_BACKEND_SOURCE_OF_TRUTH.md`
3. `docs/api/openapi.yaml`
4. `docs/ASSET_UPLOAD_INTEGRATION.md`
5. `docs/ASSET_ACCESS_POLICY.md`
6. `docs/ASSET_STORAGE_AND_FLOW_RULES.md`

## 4. 禁止事项

- 禁止恢复 NAS 业务上传/下载/元数据逻辑。
- 禁止把 compatibility-only 路由作为新前端接入入口。
- 禁止把 `ReferenceFileRef.url` 等兼容字段作为主契约。
- 禁止默认发布。

## 5. 最小实施原则

- 坚持 OSS-only 主链，不扩散存储后端分支。
- 保持 canonical 上传与下载路径稳定：
  - `POST /v1/tasks/reference-upload`
  - `/v1/tasks/{id}/asset-center/upload-sessions*`
  - `GET /v1/assets/files/{path}`
- 只在必需处扩展字段，避免资产模型泛化。

## 6. 必跑测试命令

- `go build ./cmd/server`
- `go test ./service/...`
- `go test ./transport/...`

若涉及存储集成分支，增加相关包级测试。

## 7. 文档更新要求

- 契约改动同步更新 `docs/api/openapi.yaml`。
- 行为改动同步更新 `docs/ASSET_UPLOAD_INTEGRATION.md` 与相关 guide。
- 明确声明是否影响前端上传流程或下载消费方式。

## 8. 标准输出格式要求

按 `templates/agent_output.md`，并额外说明：

- 上传路径变化
- 下载路径变化
- 元数据字段变化
- 兼容字段影响

## 9. 是否允许进入 release prep

- 默认不允许。
- 用户明确授权后，才可进入 `release_prep.md`。

## 10. 常见误区 / 历史包袱提醒

- “NAS 兜底看起来更安全”会重新引入双轨复杂性，必须避免。
- 兼容字段存在不代表应继续在新逻辑中依赖它。
