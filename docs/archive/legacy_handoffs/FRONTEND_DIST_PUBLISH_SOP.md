# 已废弃：旧前端 dist 发布 SOP

这份文件原本用于“前端同事单独交付 `dist/front` 构建产物”的历史流程。现在仓库已经合并为前后端同仓：

- Go 后端代码在仓库根目录。
- Vue 前端代码在 `vue/` 目录。
- 前端构建、审查、发布流程不再从历史交接文档中取规则。

当前有效前端发布 SOP：

- `deploy/FRONTEND_DIST_PUBLISH_SOP.md`

后端固定发布流程仍以以下文件为准：

- `deploy/DEPLOYMENT_WORKFLOW.md`

以后 `dev/external-developer` 分支的提交应先在本机审查，再按改动范围选择发布：

- 只改 `vue/`：按当前前端 SOP 构建并发布静态站。
- 只改后端：按后端 `deploy/DEPLOYMENT_WORKFLOW.md` 发布。
- 同时改前后端：先完成前后端契约审查，再按依赖顺序发布。若前端依赖后端新接口或字段，先发布后端，再发布前端。

`dev/external-developer` 稳定后，是否合并到 `main` 由项目负责人手动判断。
