# 设计流转自动化管理系统 - 前端

基于 `Vue 3 + TypeScript + Vite + Vue Router + Pinia + Tailwind CSS` 的业务前端项目。

> 当前项目已对接后端 API（非纯 mock），包含登录/注册、任务流转、审核、定制、仓库、组织与权限、导出与日志等核心功能。

## 快速开始

```bash
npm install
npm run dev
```

默认访问：`http://localhost:5173`

常用命令：

```bash
npm run dev
npm run build
npm run build:test
npm run build:prod
npm run preview
```

## 技术栈与目录

- 技术栈：Vue 3、TypeScript、Vite、Pinia、Vue Router、Axios、Tailwind CSS
- 主要目录：
  - `src/views`：页面层（任务中心、工作台、组织权限、日志等）
  - `src/components`：业务组件与基础组件
  - `src/services/api`：后端 API 封装
  - `src/domain`：读模型映射、业务规则与领域函数
  - `src/stores`：Pinia 状态管理
  - `docs/openapi.yaml`：当前接口契约（联调基准）

## 当前功能范围（高层）

- 认证：登录、注册、当前用户信息、密码相关流程
- 任务：列表筛选、详情、创建、设计/审核/仓库相关状态流转
- 工作台：设计工作台、审核工作台
- 协同：定制管理、仓库接收
- 组织权限：组织结构维护、用户与角色管理
- 运维侧：导出中心、日志管理、规则配置、看板/KPI/财务等视图

## 对接说明

- API 通过 `src/services/http.ts` 统一封装，使用相对路径 `baseURL: '/'`
- 开发环境依赖 Vite 代理；生产环境通常由 Nginx 反向代理
- 认证 token 存储于本地（见 `src/services/http.ts`）
- 接口字段遵循后端契约；展示层允许做中文映射，但不应改变提交字段语义

## 展示层约束（重要）

- 用户可见文案可做中文映射（状态、枚举、错误文案、占位提示等）
- 不修改后端接口定义，不擅自变更前端提交给后端的字段和值
- 映射逻辑优先放在读模型/展示层，避免污染 API DTO

## 文档索引

- OpenAPI：[`docs/openapi.yaml`](docs/openapi.yaml)
- 前端架构说明：[`docs/前端架构说明.md`](docs/前端架构说明.md)
- 对接说明：[`docs/对接后端接口说明.md`](docs/对接后端接口说明.md)
- 整改清单：[`docs/整改完成清单.md`](docs/整改完成清单.md)

## 备注

- 历史版本中曾存在大量本地 mock 描述；请以当前 README 与 `docs/openapi.yaml` 为准。
