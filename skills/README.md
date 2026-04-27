# Skills Index

本目录提供全工程可复用的 task skill / playbook。所有任务默认先读 `AGENT_ENTRYPOINT.md`，再选一个或多个 skill 执行。

## 使用方式

最小调用模式：

1. Read `AGENT_ENTRYPOINT.md`
2. Read authority docs（manifest -> source-of-truth -> openapi）
3. Read chosen skill
4. 执行最小变更 + 本地验证 + 文档更新
5. 按 `templates/agent_output.md` 输出

## Skill 列表

- `review_first_change.md`：所有需求的默认基础流程（review-first）
- `backend_contract_change.md`：后端路由/字段/契约变更
- `runtime_feature_change.md`：后端运行时功能逻辑变更
- `frontend_handoff.md`：前端接入与对接交付
- `release_prep.md`：发布准备（仅在明确授权时）
- `doc_cleanup_and_archive.md`：文档治理与归档
- `storage_and_assets.md`：OSS/资产上传下载/元数据相关
- `task_flow_and_status.md`：任务流转、状态机、处理人语义
- `org_permissions_and_roles.md`：组织、权限、角色与访问范围
- `user_admin_and_identity.md`：用户管理、认证、身份字段

## 统一约束

- Layer 1 authority 永远优先于任何 skill 文本。
- skill 不得引导“默认发布”。
- skill 不得引导“兼容路由优先于主链”。
- skill 不得替代 OpenAPI 契约。
