# 核心页面 Base 组件统一迁移总结报告

## 1. 本轮迁移目标

- 目标是将核心页面中低风险的原生 input / select / button / date input 逐步统一为项目已有 Base 组件。
- 迁移遵循小步提交、单文件或单区域变更、build / lint / test 验证、人工冒烟确认。
- 本轮不追求全站一次性替换，不碰业务动作按钮，不碰上传、审核、终止、结单、仓库动作等高风险区域。

## 2. 当前已完成范围总览

| 页面 / 区域 | 文件 | 迁移内容 | 提交 hash | 状态 |
|---|---|---|---|---|
| 用户管理页工具栏 | `src/views/org-permission/UserManagementView.vue` | 搜索 input -> BaseInput；状态 / 角色 / 部门 / 小组 select -> BaseSelect；查询 button -> BaseButton | `93d05d5b77d72344378f36006e0ad6cf368d196b` | 已完成 |
| 用户管理页页头与分页 | `src/views/org-permission/UserManagementView.vue` | 页头新增用户 -> BaseButton；分页上一页 / 下一页 -> BaseButton；每页条数 select -> BaseSelect | `9f7d5fed20e078b277cc4b65b236849de103f823` | 已完成 |
| 任务中心分页区 | `src/views/TaskListView.vue` | 每页条数 select -> BaseSelect；上一页 / 下一页 / 跳转 button -> BaseButton；跳页 input[type=number] -> BaseInput | `97927e861d8c13dafbfd17b39478bc88cabac61b` | 已完成 |
| 任务中心日期筛选 | `src/components/task/TaskFilterBar.vue` | `dateFrom` / `dateTo` input[type=date] -> BaseDatePicker | `7aacd9492fa20eb0d9e3c0b15369ee231f80aaef` | 已完成 |
| Dashboard 风险列表 | `src/components/dashboard/RiskListCard.vue` | 查看更多 / 收起 button -> BaseButton | `e49911e18eae225f73b1e29b9b7b3c681ce8ef69` | 已完成 |
| 任务详情页顶部非业务按钮 | `src/views/TaskDetailView.vue` | 返回 / 刷新 / 事件日志 / 任务资产页 -> BaseButton | `7c590ffe7aca2cb26a633e6268c794d0e336cc31` | 已完成 |

## 3. 各页面迁移说明

### 3.1 用户管理页

- 作为 Base 组件统一样板页。
- 外层列表区域已统一。
- 弹层内部表单、角色管理弹层、表格操作按钮未纳入本轮。
- 搜索 trim 逻辑保持在查询 / `loadUsers` 参数组装阶段。
- `BaseSelect` clearable 保持空字符串 `''` 表示全部。
- `pageSize` 保持 `number`。
- `100` 显示截断通过局部 class 修复，不改 `BaseSelect` 全局组件。

### 3.2 任务中心

- 分页区已完成 `BaseSelect / BaseButton / BaseInput` 迁移。
- `TaskFilterBar` 中普通 `BaseSelect` 已存在，本轮只补 `dateFrom / dateTo` 到 `BaseDatePicker`。
- 日期字段保持 `string`。
- 日期格式保持 `yyyy-MM-dd`。
- 清空语义保持空字符串 `''`。
- `TaskStatusMultiSelect` 未迁移。
- `overdueOnly` checkbox 未迁移。
- `TaskCreateModal`、`DesignerSelectDialog`、批量操作、接单按钮未迁移。

### 3.3 Dashboard

- Dashboard 原生表单控件很少，不适合大面积迁移。
- 本轮只迁移 `RiskListCard` 的查看更多 / 收起按钮。
- 不动图表、KPI、空状态整体、错误态整体、loading 骨架、数据映射、store / API。
- 图表和空状态更多是展示结构问题，不属于简单 Base 控件替换。

### 3.4 任务详情页

- 任务详情页风险最高。
- 本轮只迁移顶部非业务按钮：
  - 返回
  - 刷新
  - 事件日志
  - 任务资产页
- 不迁移：
  - 终止任务
  - 结单
  - 审核通过 / 驳回
  - 上传
  - 指派 / 重指派
  - 仓库动作
  - 保存类弹层
  - module action
  - `input[type=file]`
- 这些高风险动作涉及 API mutation、权限判断、任务状态机和上传链路，后续必须单独设计和回归。

## 4. 验证结果

每个批次都执行过：

- `npm run build`：通过
- `npm run lint:object-key`：通过
- `npm run test`：与既有测试基线一致，无新增失败文件或失败用例
- 人工冒烟：通过
- 每个批次均已推送 `origin/main`

当前既有测试基线：

- `tests/task-create-fields.spec.ts`：1 failed
- `tests/v1-e2e-scenarios.spec.ts`：10 failed
- `tests/contract/p0-foundation.spec.ts`：1 failed
- 总计：3 个失败文件，12 个失败用例

说明：

这些失败属于既有测试基线，与本轮 Base 组件迁移无关。

## 5. 人工冒烟覆盖点

### 用户管理页

- 搜索回车
- 查询按钮
- 四个筛选下拉
- 清空后恢复全部
- 部门 / 小组联动
- 新增用户按钮打开弹层
- 每页条数 20 / 50 / 100
- 上一页 / 下一页

### 任务中心

- 分页区正常显示
- 每页条数 20 / 50 / 100
- 上一页 / 下一页
- 跳页输入和跳转
- 日期筛选
- 清空全部筛选
- 日期 + 状态多选
- 日期 + 仅逾期
- 搜索、筛选、批量选择、创建任务入口未受影响

### Dashboard

- 风险列表展示
- 查看更多
- 收起
- 风险项跳转
- loading / error / empty 没有明显异常

### 任务详情页

- 页面打开
- 返回
- 刷新
- 事件日志
- 任务资产页
- 终止 / 结单显示、禁用态、点击行为未受影响

## 6. 明确保留未迁移的内容

- `TaskStatusMultiSelect`
- `overdueOnly` checkbox
- `TaskCreateModal`
- `DesignerSelectDialog`
- 任务中心批量操作
- 接单按钮
- Dashboard 图表组件
- Dashboard 空状态 / 错误态整体容器
- `TaskDetailView` 业务动作按钮
- `task-detail` 下上传、审核、仓库、保存、终止、结单相关控件
- `input[type=file]`
- 所有 API mutation 按钮

## 7. 保留原因

- 多选组件不是普通 `BaseSelect`。
- checkbox / switch 需要统一交互规范后再处理。
- 上传涉及文件选择、accept、多文件、上传状态和失败处理。
- 终止、结单、审核、仓库动作涉及任务状态机和 API mutation。
- Dashboard 图表和空状态更偏展示结构，不是简单控件替换。
- 弹层保存类按钮涉及校验、回显、接口保存，不适合自动迁移。

## 8. 迁移原则总结

1. 一次只改一个页面或一个区域。
2. 优先迁移低风险 UI 控件。
3. 不借 UI 统一修改业务逻辑。
4. 不修改 API / domain / router / stores / services。
5. 不修改 Base 全局组件，除非单独评审。
6. 不碰上传、审核、终止、结单等高风险动作。
7. `pageSize` 等数值类型必须保持 `number`。
8. 日期字段必须保持原格式和清空语义。
9. 样式问题优先页面局部 class 修复。
10. 每次都要 `build / lint / test / 人工冒烟`。

## 9. 后续建议

1. 本轮核心页面低风险迁移可以阶段性收尾。
2. 暂不继续迁移任务详情页业务动作按钮。
3. 如果继续推进，建议先制定：
   - `BaseCheckbox / BaseSwitch` 使用规范
   - 上传组件设计规范
   - 任务详情业务动作按钮视觉规范
4. 可以单独盘点：
   - `overdueOnly` checkbox 是否适合 `BaseSwitch`
   - `TaskStatusMultiSelect` 是否需要抽象为 `BaseMultiSelect`
   - Dashboard 空状态 / 错误态是否需要统一组件
5. 任何高风险迁移必须单独 PR、单独回归。

## 10. PR / 汇报摘要

- 完成用户管理页、任务中心、Dashboard、任务详情页的低风险 Base 组件统一。
- 覆盖 `BaseInput`、`BaseSelect`、`BaseDatePicker`、`BaseButton`。
- 未修改业务逻辑、API、store、domain、router。
- 未触碰上传、审核、终止、结单、仓库动作等高风险链路。
- `build / lint` 通过，`test` 与既有基线一致。
- 所有迁移均已人工冒烟通过并推送远程。
