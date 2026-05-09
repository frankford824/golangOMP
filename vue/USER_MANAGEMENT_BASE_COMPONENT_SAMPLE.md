# 用户管理页 Base 组件样板总结

## 1. 样板目标

- 将用户管理页外层列表区域的输入框、下拉框、按钮统一迁移到 Base 组件。
- 为后续 Dashboard、任务中心、任务详情页等核心页面迁移提供参考。
- 不改变业务逻辑、接口参数、权限判断、分页行为和用户体验。
- 采用小步迁移、每步验证、人工冒烟的方式完成。

## 2. 改造范围

本次实际改造范围：

- `src/views/org-permission/UserManagementView.vue`
- 用户列表工具栏
- 页头新增用户按钮
- 分页上一页 / 下一页
- 每页条数下拉

本次没有改造：

- 角色管理弹层
- 新增用户弹层内部表单
- 编辑用户弹层
- 表格内操作按钮
- 其它页面
- `DashboardView.vue`
- `TaskListView.vue`
- `TaskDetailView.vue`
- API / domain / router / stores / services

## 3. 控件替换清单

| 区域 | 原控件 | 新控件 | 行为是否保持 | 说明 |
|------|--------|--------|--------------|------|
| 搜索输入框 | 原生 `input` | `BaseInput` | 是 | 保留回车触发 `onSearch`，输入过程中不强制 `trim` |
| 状态筛选 | 原生 `select` | `BaseSelect` | 是 | 使用 `clearable`，清空后回到空字符串 `''` 的“全部状态”语义 |
| 角色筛选 | 原生 `select` | `BaseSelect` | 是 | 选项仍由 `roleOptions` 映射 |
| 部门筛选 | 原生 `select` | `BaseSelect` | 是 | 仍沿用原 `departmentOptions` 语义 |
| 小组筛选 | 原生 `select` | `BaseSelect` | 是 | 仍沿用原 `teamOptionsFiltered` 语义与联动逻辑 |
| 查询按钮 | 原生 `button` | `BaseButton` | 是 | 继续触发 `onSearch` |
| 页头新增用户 | 原生 `button` | `BaseButton` | 是 | 继续受 `canCreateUser` 控制，点击仍打开新增用户弹层 |
| 分页上一页 / 下一页 | 原生 `button` | `BaseButton` | 是 | 保持 `ghost` + `sm` 风格，保留 `disabled` 与 `goPage` 行为 |
| 每页条数 | 原生 `select` | `BaseSelect` | 是 | 选项保持 `20 / 50 / 100`，`value` 保持 `number` |

## 4. 关键行为保持点

### 搜索

- `placeholder` 保持“搜索用户名 / 姓名”。
- 回车仍触发 `onSearch`。
- 查询按钮仍触发 `onSearch`。
- 输入过程中不强制 `trim`。
- 查询 / `loadUsers` 组装请求参数前处理 `trim`。
- `usersApi.list` 参数结构未改变。

### 筛选

- 状态 / 角色 / 部门 / 小组筛选仍使用原有状态变量。
- `BaseSelect clearable` 后仍回到空字符串 `''`。
- 空字符串仍表示全部。
- 不额外保留空 `option` 行。
- 部门与小组联动逻辑未改变。
- `watch` / `loadUsers` 链路未改变。

### 分页

- 上一页 / 下一页 `disabled` 逻辑未改变。
- `goPage` 行为未改变。
- `page`、`totalPages`、`total`、`pageSize` 逻辑未改变。
- 每页条数仍使用 `number` 类型，不变成字符串。
- `20 / 50 / 100` 选项保持不变。

### 权限

- 新增用户按钮仍受 `canCreateUser` 控制。
- 不改变权限判断与显示条件。
- 点击后仍打开新增用户弹层。

## 5. Base 组件使用经验

### BaseInput

- 适合替换普通搜索框。
- 如果旧代码使用 `v-model.trim`，不建议输入中强制 `trim`。
- 推荐在 `onSearch` 或 `loadUsers` 组装请求参数前 `trim`。

### BaseSelect

- 适合替换筛选 `select` 和每页条数 `select`。
- `clearable` 时应明确清空语义。
- 全部语义推荐使用空字符串 `''`。
- `options` 的 `value` 类型必须保持不变。
- `pageSize` 这类值必须保持 `number`。
- 下拉宽度需要结合具体场景设置，不要盲目使用过窄宽度。

### BaseButton

- 主操作使用 `variant="primary"`。
- 分页辅助按钮可使用 `variant="ghost" + size="sm"`。
- 替换按钮时保持点击函数、`disabled` 条件和文案不变。

## 6. 样式处理经验

- 不要为了局部显示问题修改 Base 全局组件。
- 可以通过页面局部 `class` 控制宽度、间距和对齐。
- 每页条数 `BaseSelect` 需要保证 `20 / 50 / 100` 都完整显示，避免 `100` 被截断成 `1...`。
- 不应删除仍被弹层或表单使用的 `.input`、`.input.small`、`.um-btn`、`.pager-btn` 等旧样式。
- 只删除或调整已确认不影响其它区域的局部样式。
- 页面布局以保持原视觉结构为优先，不借迁移大改页面信息架构。

## 7. 验证结果

相关提交：

- `93d05d5b77d72344378f36006e0ad6cf368d196b`
  `feat(org): use Base components in user list toolbar`

- `9f7d5fed20e078b277cc4b65b236849de103f823`
  `feat(org): use Base components in user header and pagination`

验证结果：

- `npm run build`：通过
- `npm run lint:object-key`：通过
- `npm run test`：与既有基线一致，没有新增失败文件或失败用例
- 人工冒烟：通过

当前测试基线：

- `tests/task-create-fields.spec.ts`：1 failed
- `tests/v1-e2e-scenarios.spec.ts`：10 failed
- `tests/contract/p0-foundation.spec.ts`：1 failed
- 总计：3 个失败文件，12 个失败用例

这些失败属于既有测试基线，与本次 Base 组件迁移无关。

## 8. 人工冒烟检查项

本次已检查通过的内容：

- 用户管理页正常打开
- 搜索框输入和回车查询正常
- 查询按钮正常
- 状态 / 角色 / 部门 / 小组下拉正常
- 下拉清空后恢复全部语义
- 部门变化后小组选项正确联动
- 新增用户按钮正常打开弹层
- 每页条数 `20 / 50 / 100` 切换正常
- `100` 完整显示，不截断
- 上一页 / 下一页 `disabled` 和跳转正常
- 弹层和表格操作未受影响

## 9. 后续页面迁移建议

- 不建议全站一次性替换。
- 继续采用一页 / 一块区域的方式。
- 优先处理列表页的工具栏、筛选区、分页区。
- 弹层、复杂表单、任务详情动作按钮应单独批次处理。
- 每次迁移后都需要 `build` / `lint` / `test`，并做人工冒烟。
- 如果出现局部样式问题，优先局部 `class` 修复，不改 Base 全局组件。

## 10. 下一批候选建议

1. Dashboard：
   - 低风险展示区
   - 普通入口按钮
   - 空状态 / 错误态 / 加载态统一

2. 任务中心：
   - 分页区
   - 工具栏 / 筛选区中仍未 Base 化的原生控件
   - 注意不要碰批量操作、任务创建弹层、任务动作逻辑

3. 任务详情页：
   - 只读盘点优先
   - 仅考虑非业务动作按钮或展示态
   - 上传、终止、结单、审核、模块动作按钮暂不建议自动迁移

## 11. 后续迁移提示词模板

可复用提示词模板：

```text
你现在在项目目录中。

本轮任务：只对一个页面的列表工具栏 / 筛选区 / 分页区做 Base 组件迁移。

严格限制：
1. 只允许修改一个指定页面。
2. 只处理工具栏 / 筛选区 / 分页区。
3. 不处理弹层。
4. 不处理业务逻辑。
5. 不处理 API / domain / router / stores / services。
6. 不修改 Base 全局组件。
7. 不大面积格式化。

迁移要求：
1. 普通搜索框改为 BaseInput。
2. 普通筛选下拉改为 BaseSelect。
3. 普通按钮改为 BaseButton。
4. 保持原有点击行为、disabled 条件、权限判断、接口参数结构不变。
5. 如果旧代码使用 v-model.trim，不要在输入过程中强制 trim；改为在 onSearch 或 loadUsers 组装请求参数前 trim。
6. BaseSelect 的 clearable、clearValue、options value 类型必须与原逻辑语义一致。
7. pageSize 等数值型下拉必须保持 number，不要变成 string。

验证要求：
1. 执行 npm run build。
2. 执行 npm run lint:object-key。
3. 执行 npm run test。
4. 不允许新增失败文件、失败用例或新的失败原因。

人工冒烟检查点：
1. 页面能正常打开。
2. 搜索输入、回车查询、查询按钮正常。
3. 筛选切换与清空语义正常。
4. 分页切换、上一页 / 下一页 disabled 状态正常。
5. 相关弹层、表格操作、业务动作不受影响。

如果验证通过，再提交；不要顺带迁移其它页面或其它区域。
```
