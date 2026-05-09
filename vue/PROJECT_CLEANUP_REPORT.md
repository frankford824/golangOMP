# 保守型项目清洗最终总结报告

## 1. 本轮清洗背景

- 之前旧工作区曾因蓝屏导致 Git / 文件异常。
- 现在已在 `D:\vue_clean` 建立新的健康 Git 基线。
- 当前清洗是在新仓库 `main` 分支上重新执行的保守清洗。
- 目标是清理确认无用的旧组件和未引用 composable，不影响现有功能和用户体验。

## 2. 当前仓库基线

- 当前仓库地址：`git@github.com:frankford824/golangOMP.git`
- 当前仓库布局：Vue 前端位于统一仓库的 `vue/` 目录。
- 分支：`main`
- 初始恢复基线提交：
  `a2886649f248e4820c9532907619aad764e1ddb7`
  `chore: initialize recovered frontend baseline`
- 清洗提交：
  `ddf8f9df4ebb9e91201d4f94daf4bbd62da53c69`
  `chore: remove confirmed unused legacy components`

## 3. 已删除文件清单

| 文件路径 | 删除原因 | 风险等级 | 验证方式 |
|------|------|------|------|
| `src/components/task/WorkflowSidebar.vue` | 文档中已标记为废弃，主流程由 `WorkflowProgress` 承接；无实际源码引用 | 低 | 全仓 grep、import 检查、router 检查、动态组件引用检查、tests 检查、docs/data/frontend/.cursor 检查、`build` 验证 |
| `src/components/dashboard/BusinessSummaryCard.vue` | 无源码引用，无文档强制约束 | 低 | 全仓 grep、import 检查、router 检查、动态组件引用检查、tests 检查、docs/data/frontend/.cursor 检查、`build` 验证 |
| `src/components/task/AssetTimeline.vue` | 无源码引用，无文档强制约束；当前任务详情资产展示由其它组件承接 | 低 | 全仓 grep、import 检查、router 检查、动态组件引用检查、tests 检查、docs/data/frontend/.cursor 检查、`build` 验证 |
| `src/composables/useMenuVisibility.ts` | 无引用；菜单可见性已由 `AppShell`、`router`、`permissions store` 中的 `frontend_access` / `hasMenu` / `hasPermission` 等机制承接 | 低 | 全仓 grep、import 检查、router 检查、动态组件引用检查、tests 检查、docs/data/frontend/.cursor 检查、`build` 验证 |

## 4. 删除前检查依据

清洗前已完成以下检查：

- 全仓 grep
- import 检查
- router 检查
- 动态组件引用检查
- tests 检查
- docs / data / frontend / .cursor 检查
- 确认没有实际使用引用

`WorkflowSidebar` 仅在 `docs/V1_0_PRD_GAP_ANALYSIS.md` 中以“已废弃 / 可删除”的上下文出现，不构成保留依据。

## 5. 验证结果

- `npm run build`：通过
- `npm run lint:object-key`：通过
- `npm run test`：与既有基线一致，没有新增失败文件或失败用例

当前测试基线：

- `tests/task-create-fields.spec.ts`：1 failed
- `tests/v1-e2e-scenarios.spec.ts`：10 failed
- `tests/contract/p0-foundation.spec.ts`：1 failed
- 总计：3 个失败文件、12 个失败用例

这些失败属于既有测试基线，与本轮清洗无关。

## 6. 未触碰范围

本轮没有修改：

- `UserManagementView.vue`
- `TaskListView.vue`
- `DashboardView.vue`
- `TaskDetailView.vue`
- `TaskFilterBar.vue`
- `TaskCreateModal.vue`
- `src/components/task-detail/`
- API / domain / router / stores / services
- `package.json` / `package-lock.json`
- `.env`
- `docs` / `data` / `frontend`

## 7. 当前未包含的工作

当前仓库还没有重新执行：

- 用户管理页 Base 组件统一
- `TaskListView` 分页 Base 组件迁移
- Dashboard / 任务中心 / 任务详情页组件统一

这些工作将作为后续单独任务推进，不与清洗混在同一提交中。

## 8. 后续建议

1. 先重新做用户管理页 Base 组件样板。
2. 再继续 Dashboard / 任务中心 / 任务详情页组件统一。
3. 继续保持每次只改一个页面或一个区域。
4. 每次执行 `build` / `lint` / `test`。
5. 不要把清洗和 UI 统一混在同一个提交中。

## 9. PR 摘要建议

可复制到 PR / commit 描述中的摘要：

> 本次删除 4 个确认无引用的 legacy 文件，未修改核心业务逻辑。`build` / `lint` 通过，`test` 与既有基线一致，工作区干净。
