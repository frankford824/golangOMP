# 资产工作台前端维护边界

这个目录是 `assets.yongbo.cloud` 独立 App 的唯一前端边界，面向长期高频的资产交付、计件报表和结算流程。

## 目录模型

- `app/`: 应用启动、Provider、全局状态接线。
- `pages/`: 路由页，只组合 widgets/features，不沉业务细节。
- `widgets/`: 页面级复合区块，如导航、数据网格外壳、批量上传面板。
- `features/`: 用户动作闭环，如上传提交、质检、生成批次、导出。
- `entities/`: 领域实体模型、API adapter、展示组件。
- `shared/`: 工作台内部通用 UI、hooks、utils、tokens。
- `styles/`: 工作台专属 CSS 模型，必须通过 `.aw-root`、`--aw-*`、`aw-` 前缀隔离。

## 禁止依赖

- 禁止 import 主站 `AppShell`、主站 router、`src/views/*`、`src/assets/main.css`。
- 禁止直接复用主站资产预览接口契约。`AssetPreviewMedia` 只能通过 `resolvedPreviewUrl` 注入使用。
- 禁止在组件内散落硬编码颜色、`!important`、`@apply`、大段 scoped style。

## 允许复用

- `src/services/http.ts`
- 登录态、权限 store 的基础能力
- `src/components/media/AssetPreviewMedia.vue`（仅 resolvedPreviewUrl 模式）
- 导出、下载、格式化等稳定工具

新增页面或组件时，同步运行 `npm run asset:audit`，让维护规则先卡住结构漂移。
