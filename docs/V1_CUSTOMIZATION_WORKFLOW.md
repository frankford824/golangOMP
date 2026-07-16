# V8 定制任务规则

> 状态：V8 当前完整替换版。定制是设计节点内部能力，不是独立主流程。

## 1. 主状态机

定制任务和普通设计任务使用完全相同的主状态机：

```text
创建 -> 设计(InProgress) -> 统一审核(PendingAudit) -> 已结单(Completed)
                            -> 打回设计(InProgress)
```

`business_lane=customization` 只决定设计节点内部表单、job 和资源准备方式，不得产生额外任务
主状态。效果沟通和制作迭代都发生在 `InProgress` 内。

## 2. 内部 job

定制设计可维护一个或多个内部 job，用于：

- 需求澄清与沟通稿记录。
- 效果确认记录。
- 源文件和最终成品准备。
- 处理人及内部事件追溯。

内部 job 不能绕过 `/v1/tasks/{id}/submit-design`，也不能自行结单。任务提交必须覆盖全部
SKU/范围组并满足 single/set 完整性。

## 3. 统一审核

定制与普通任务共用 `/v1/tasks/{id}/audit/decision`：

- `approve`：继承或完整替换资源，随后事务内结单。
- `return_to_design`：原因必填，任务回到 `InProgress`，保留处理人并建立新 working draft。

审核上传新源文件时新源成为唯一有效源；不上传时继承设计源。套装必须完整有序替换。

## 4. Reopen

已结单定制任务可由具备 `task.reopen` 的管理员 reopen 到 `design|audit`，请求包含原因、
workflow CAS 和幂等键。旧 finalized revision 在再次审核完成前继续对资产中心和既有固定发布有效。

## 5. 权限与组织

定制相关操作只使用显式能力和稳定组织 ID 范围。设计团队或部门的显示名称不得触发默认角色、
默认可见性或默认可操作性。前端只消费 `allowed_actions`。

## 6. 资源

定制资源和普通设计资源使用相同的 `task_asset_groups`：

- 每个 SKU 一份当前有效源文件。
- single 一张最终成品；set 至少两张有序最终成品。
- 参考图在修订上冻结。
- 审核换源后旧源立即撤销受控访问并进入删除 outbox。

客户发布只能固定 finalized revision，不得发布 working draft 或策划 SKU 产品图片。

## 7. 迁移

迁移把可确定的历史定制制作和效果记录转为 `InProgress + internal job`，把可确定的审核等待记录
转为 `PendingAudit`。资源关系或内部状态冲突的数据进入人工清单，不自动结单，也不猜测文件顺序。
