# API_DOC_SYNC_RULES.md

当以下任一情况发生时，必须更新 docs/api/openapi.yaml：
- 新增路径
- 删除路径
- query 参数变化
- request body 变化
- response body 变化
- schema 字段变化
- 分页结构变化
- 版本号变化
- ready / placeholder 状态变化

更新后必须检查：
1. info.version 是否递增
2. 新增 schema 是否被引用
3. 旧路径是否仍兼容
4. CURRENT_STATE.md 是否同步写明当前 API Source of Truth
5. 前端是否能只依赖 OpenAPI 完成对接

推荐输出附带：
- 本轮新增/修改路径列表
- 版本号变更说明
- 哪些接口 ready for frontend
- 哪些接口仍是 placeholder
