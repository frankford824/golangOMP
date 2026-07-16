# V8 资源归属与发布

> 状态：V8 当前完整替换版。资源组指针是当前业务资源的唯一权威。

## 1. 三层模型

1. `task_assets` 与 `asset_storage_refs` 保存文件实体和对象存储引用。
2. `task_asset_group_revisions` 保存不可变的业务资源组合。
3. `task_asset_groups.working_revision_id/finalized_revision_id` 保存当前业务指针。

文件版本本身不代表当前有效业务资源。任何详情、搜索、下载、清理和客户发布都必须从资源组
指针开始解析。

## 2. 资源组范围

范围只有：

- `task`，`scope_ref_id=0`。
- `sku`，引用属于同一任务的 `task_sku_items`。
- `retouch_requirement`，引用属于同一任务的修图需求。

唯一性为 `task_id + scope_kind + scope_ref_id`。每组包含 `working_revision_id`、
`finalized_revision_id` 和 `lock_version`。

## 3. 不可变修订

修订状态：`draft | submitted | finalized | rejected | superseded`。

修订保存：

- `mode=single|set`
- 唯一有效 `source_task_asset_id`
- `source_stage=design|audit|retouch|migration|reopen`
- 操作者、原因和时间
- 有序 `task_asset_group_revision_items`
- 冻结的参考图引用

`revision_items.id` 是修订内稳定、不可变的子项 ID；不再建立平行 members 模型。

## 4. Staging

`task_assets.binding_state` 只有 `legacy | staged | bound | discarded`。staged 文件：

- 不更新任何旧 current pointer。
- 不进入资产中心、搜索、客户发布和普通下载。
- 只允许上传人及具备相应审核能力的业务接口短期预览。
- 绑定时校验任务、范围、角色、上传会话和操作者。
- 过期且从未绑定的文件进入 `asset_object_deletion_outbox`。

绑定成功后，文件只能通过资源组读取。

## 5. 当前资源构成

每个范围组向业务界面只提供：

- 参考图：修订创建时冻结的任务级/SKU 级引用。
- 当前有效源文件：设计类必填，修图可空。
- 当前最终成品图：single 一张；set 至少两张并保持完整顺序。

审核上传新源文件时旧源立即写 `access_revoked_at/reason`，所有受控下载拒绝访问，并在事务内
写删除 outbox。outbox 必须冻结 `storage_ref_id + storage_adapter + storage_key`；只有
`oss_upload_service` 可调用 OSS 删除，placeholder/mock/export-placeholder 直接幂等完成，未知
adapter 必须 fail-closed、告警并持续重试，禁止把 NAS 或 UploadService key 猜测为 OSS key。
对象物理删除失败不阻塞结单，404 视为幂等成功。

通用 `DELETE /v1/assets/{asset_id}` 只用于具备 `asset.manage` 且处于任务稳定组织范围内的
staged/unbound 文件。事务必须锁定主资源、派生预览、全部对应文件版本、资源组历史引用和客户
发布 pin；任一文件已绑定或曾被任何 working/finalized/historical revision 或发布引用时均拒绝。
Completed/Archived 必须先 reopen，但 reopen 不解除旧 finalized revision 的历史保护。

已经签发的对象存储直链可能在签名到期前有效；需要真正即时失效的部署必须统一走鉴权代理下载。

## 6. Finalize 与 reopen

审核通过在调用方事务内把 submitted revision 切为 finalized，并 supersede 旧工作修订。
reopen 到审核时克隆新 submitted revision；旧 finalized revision 继续为资产中心和既有客户发布
服务，直到新审核成功原子切换。

## 7. 资源搜索

`task_asset_group_search_documents` 的粒度为资源组，不是文件版本：

- `internal_text` 可索引内部任务、参考、源文件和最终文件信息。
- `final_text` 只包含可对外最终信息。

`search_reindex_outbox` 负责增量重建；cutover 必须执行一次全量 reindex。

## 8. 客户发布

沿用 `asset_workbench_client_materials`，任务资源发布保存：

- `source_type=task_resource_group`
- `resource_group_id`
- `finalized_revision_id`
- set 模式的 `cover_revision_item_id`

发布必须固定当时 finalized revision。下载解析该固定修订的全部最终成品，任何一项缺失即整体失败。
新 finalized revision 不自动改变旧发布。外部素材继续使用 `source_type=external_asset`。

## 9. 策划 SKU 图片边界

策划 SKU 产品图片复用 upload session 和 `asset_storage_refs`，创建前以
`client_create_id + client_item_id` 暂存，创建事务内绑定到 `planning_sku_revision_image`。
它不是设计资源，不进入资源组、资产搜索或客户发布。旧修订图片仅按审计保留策略受保护。

## 10. 迁移

只有任务范围、有效源文件和最终成品顺序均确定时才自动建组。无法确定的数据标记
`migration_incomplete` 并进入人工映射清单，禁止根据文件名、时间或相似度猜测。

切换完成后旧 current pointer 只允许用于迁移 parity 报告，不能参与业务读取。物理删除旧对象和
旧字段属于观察期后的独立清理动作。
