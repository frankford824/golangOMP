# CURRENT_STATE_PATCH_GUIDE.md

每轮完成后，至少检查并更新以下字段：
- Current Phase
- Completed
- In Progress
- Next Step
- Latest Iteration
- API Source of Truth
- Ready for Frontend
- Known Gaps
- 最新 OpenAPI 版本号

常见错误：
1. 本轮已完成内容仍保留在 Next Step
2. Latest Iteration 未更新
3. OpenAPI 版本与文件内容不一致
4. Ready for Frontend 混入占位接口
5. 仍保留上轮已过期的 Known Gaps
