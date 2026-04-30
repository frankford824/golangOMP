# AGENT_COMPLETION_REVIEW.md

每轮完成后，请按以下清单自检并输出：

## Code Check
- [ ] 已执行 `go test ./...` 或 `go build ./...`
- [ ] 新增/修改的 handler/service/repo 已串联
- [ ] 无明显死代码或重复实现

## Contract Check
- [ ] OpenAPI 路径与实现一致
- [ ] request/response/schema 已同步
- [ ] 版本号已更新

## State Check
- [ ] CURRENT_STATE.md 已更新
- [ ] Latest Iteration 指向最新文件
- [ ] Ready for Frontend 列表准确
- [ ] Known Gaps 准确

## Iteration Check
- [ ] docs/iterations/ITERATION_XXX.md 已新增
- [ ] 本轮目标、改动、风险、下一轮建议已记录
- [ ] 如有纠错，已写入 Correction Notes

## Handover Check
- [ ] 其他模型仅靠仓库文档即可理解当前进度
- [ ] 下一轮边界清楚
