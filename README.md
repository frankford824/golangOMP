# Yongbo Workflow 开发指南

这是 Yongbo 工作流系统的主仓库，包含 Go 后端、Vue 前端、数据库迁移、接口契约、部署脚本和运维说明。本文档面向参与开发、联调、发布和远端维护的工程人员，作为 `main` 分支的入口指南。

## 1. 项目边界

系统由两条主要链路组成：

| 模块 | 路径 | 职责 |
| --- | --- | --- |
| Go 后端 | 仓库根目录 | V1 API、任务流转、资产、ERP 对接、通知、搜索、报表、身份和后台管理 |
| Vue 前端 | `vue/` | 浏览器端业务界面、任务中心、资产中心、组织权限、日志、数据看板等 |
| 静态前端发布物 | `dist/front/` | `vue/dist/` 构建后同步到此目录，再发布到线上 Web 根目录 |
| 数据库迁移 | `db/migrations/` | MySQL schema 演进，发布时由部署流程处理 |
| 接口契约 | `docs/api/openapi.yaml` | 前后端联调和契约校验的基准 |
| 部署脚本 | `deploy/` | 后端发布、前端静态发布、远端校验和环境模板 |

开发时要明确本次改动属于 `backend`、`frontend` 还是 `both`。如果前端依赖新的后端行为，应先完成并验证后端，再发布前端。

## 2. 当前权威文件

遇到文档、代码和历史说明不一致时，按下面顺序判断：

1. `transport/http.go`：运行时真实挂载的路由。
2. `docs/api/openapi.yaml`：请求和响应契约。
3. `docs/V1_BACKEND_SOURCE_OF_TRUTH.md`：V1 后端权威文档索引。
4. `docs/frontend/`：由 OpenAPI 派生的前端接口说明。
5. `AGENTS.md`：开发治理、校验和提交要求。

`docs/archive/`、`docs/iterations/`、`prompts/` 中的内容只作为历史证据，除非已经被上面的权威文件重新确认。

## 3. 技术栈

后端：

- Go `1.24`
- Gin
- MySQL
- Redis
- OpenAPI 3，校验工具为 `kin-openapi`
- Zap 结构化日志
- WebSocket 通知链路

前端：

- Vue 3
- TypeScript
- Vite
- Vue Router
- Pinia
- Axios
- Tailwind CSS
- Naive UI、lucide-vue-next、ECharts、ExcelJS、JSZip

## 4. 目录速览

```text
cmd/server/            生产 MAIN 服务入口，后端发布以它为准
cmd/api/               兼容入口，不作为新的生产发布入口
cmd/tools/             维护、预览和校验工具
config/                配置加载和 JSON 种子配置
db/migrations/         MySQL 迁移文件
domain/                领域模型、枚举、错误和共享契约类型
repo/mysql/            MySQL 仓储层
service/               业务服务和流程编排
transport/             HTTP 路由、handler、中间件、WebSocket
workers/               后台 worker
docs/api/openapi.yaml  API 契约
docs/frontend/         前端接口文档
tools/contract_audit/  路由与 OpenAPI 漂移审计工具
scripts/agent-check.sh 后端完整校验入口
deploy/                发布脚本、环境模板和线上流程文档
vue/                   Vue 前端工程
dist/front/            前端静态发布物目录
```

## 5. 本地准备

需要安装：

- Go `1.24`
- Node.js 和 npm
- MySQL
- Redis
- Bash 环境
- 可访问远端机器时需要 OpenSSH、`ssh`、`scp`、`rsync`

后端真实配置通过环境变量提供。可以从这些模板开始：

- `deploy/main.env.example`
- `deploy/bridge.env.example`
- `deploy/deploy.env.example`

不要把真实 `.env`、数据库密码、API token、SSH 私钥或本地密钥路径提交到 Git。

## 6. 后端开发

启动 MAIN 服务：

```bash
go run ./cmd/server
```

兼容入口仍在仓库中，但不用于新的生产工作：

```bash
go run ./cmd/api
```

常见环境变量：

| 变量 | 说明 |
| --- | --- |
| `SERVER_PORT` | MAIN 服务监听端口 |
| `MYSQL_DSN` | MySQL 连接串 |
| `REDIS_ADDR` | Redis 地址 |
| `REDIS_PASSWORD` | Redis 密码 |
| `REDIS_DB` | Redis DB 编号 |
| `AUTH_SETTINGS_FILE` | 身份配置文件 |
| `FRONTEND_ACCESS_SETTINGS_FILE` | 前端访问控制配置 |
| `AGENT_API_TOKEN` | `/v1/agent/*` 机器客户端 token |
| `WS_ALLOWED_ORIGINS` | WebSocket 跨域白名单 |
| `ERP_BRIDGE_BASE_URL` | ERP Bridge 地址 |
| `UPLOAD_SERVICE_*` | 上传服务相关配置 |

如果改动路由、handler、DTO 或领域模型，必须同步检查 `docs/api/openapi.yaml`。接口契约变化不能只改代码。

## 7. 前端开发

进入前端目录：

```bash
cd vue
npm ci
npm run dev
```

默认开发地址：

```text
http://localhost:5173
```

常用命令：

```bash
npm run dev
npm run test
npm run build
npm run build:test
npm run build:prod
npm run preview
```

前端 API 封装位于 `vue/src/services/api/`，HTTP 基础封装位于 `vue/src/services/http.ts`。开发环境通常通过 Vite 代理访问后端；生产环境通过 Nginx 将同源 `/v1`、`/ws`、`/upload` 等路径转发到对应服务。

## 8. 校验要求

后端完整校验：

```bash
./scripts/agent-check.sh
```

该脚本会依次执行：

1. `go vet ./...`
2. `go build ./...`
3. `go test ./... -count=1`
4. `go run ./cmd/tools/openapi-validate docs/api/openapi.yaml`
5. `go run ./tools/contract_audit ... --fail-on-drift true`

前端常规校验：

```bash
cd vue
npm run test
npm run build:prod
```

文档或 Markdown-only 改动至少应运行：

```bash
git diff --check
```

如果 `docs/api/openapi.yaml` 发生变化，需要重新生成前端接口文档：

```bash
python scripts/docs/generate_frontend_docs.py
```

## 9. 后端发布

后端发布以 `deploy/DEPLOYMENT_WORKFLOW.md` 为准。生产入口固定为 `./cmd/server`，不要把 `cmd/api` 当作生产发布入口。

安全并行验证：

```bash
bash ./deploy/deploy.sh --version <version> --parallel
```

真实切换发布：

```bash
bash ./deploy/deploy.sh --version <version>
```

远端默认布局：

```text
/root/ecommerce_ai/incoming
/root/ecommerce_ai/releases/<version>
/root/ecommerce_ai/shared
/root/ecommerce_ai/logs
/root/ecommerce_ai/run
/root/ecommerce_ai/scripts
```

运行时验证示例：

```bash
ssh jst_ecs "bash /root/ecommerce_ai/scripts/verify-runtime.sh --base-url http://127.0.0.1:8080 --bridge-url http://127.0.0.1:8081"
```

## 10. 前端静态发布

前端发布以 `deploy/FRONTEND_DIST_PUBLISH_SOP.md` 为准。前端发布不部署 Go 后端，也不修改 `/root/ecommerce_ai/releases`。

构建发布物：

```bash
cd vue
npm ci
npm run build:prod
cd ..
rm -rf dist/front
mkdir -p dist/front
cp -a vue/dist/. dist/front/
```

发布到线上静态目录：

```bash
bash ./deploy/publish-front.sh
```

Dry run：

```bash
bash ./deploy/publish-front.sh --dry-run
```

固定生产目标：

| 项目 | 值 |
| --- | --- |
| SSH host | `jst_ecs` |
| 静态 Web 根目录 | `/var/www/yongbo.cloud` |
| 备份目录 | `/var/www/backups/yongbo.cloud_<UTC_TIMESTAMP>/` |
| 临时上传目录 | `/tmp/yongbo.cloud_dist_<UTC_TIMESTAMP>/` |
| 浏览器 API 基址 | 同源 `/v1` |

## 11. 本机 SSH 连接记录说明

下面是当前本机 `~/.ssh/config` 中的连接记录，已经去掉所有 `IdentityFile` 和密钥路径。这里记录的是连接远端服务机器所需的最小信息，不是凭据。

| Host 别名 | 远端地址 | 用户 | 端口 | 说明 |
| --- | --- | --- | --- | --- |
| `jst_ecs` | `223.4.249.11` | `root` | `22` | 主要生产 ECS。后端发布、Nginx、静态站点和运行时校验默认使用此别名。 |
| `223.4.249.11` | `223.4.249.11` | `root` | `22` | 旧式直连别名，等价于 `jst_ecs`，日常应优先使用 `ssh jst_ecs`。 |
| `synology-dsm` | `100.111.214.38` | `yongbo` | `22` | Synology DSM/NAS 相关主机，属于 overlay/VPN 地址段。 |
| `spark` | `100.93.114.95` | `btjs` | `22` | 辅助 Linux 主机，属于 overlay/VPN 地址段。 |
| `openclaw` | `8.222.174.253` | `root` | `22` | 公网 Linux 主机。 |
| `finance-win` | `192.168.0.155` | `sxf` | `22` | 局域网 Windows 主机。仅在同一内网或已打通网络时可访问。 |
| `eve35` | `100.78.173.18` | `administrator` | `22` | Windows 远端主机，属于 overlay/VPN 地址段。 |
| `ybmac` | `192.168.0.29` | `teagreen` | `22` | 局域网 macOS 主机。仅在同一内网或已打通网络时可访问。 |

无密钥版配置示例：

```sshconfig
Host jst_ecs
    HostName 223.4.249.11
    User root
    Port 22
    IdentitiesOnly yes
    ServerAliveInterval 30
    ServerAliveCountMax 6
    TCPKeepAlive yes
```

实际本机连接仍需要私钥或其他认证方式，但密钥文件路径和私钥内容只应保存在使用者自己的机器上，不应写入 README、部署日志或提交记录。

## 12. 远端 SSH 服务机器要求

所有远端 SSH 主机至少需要满足：

1. 已安装并启用 SSH 服务。
2. SSH 服务监听 README 中记录的端口，当前均为 `22`。
3. 对应系统用户存在，并允许 SSH 登录。
4. 对应用户的 `~/.ssh/authorized_keys` 已写入本机公钥。
5. 远端权限建议为：`~/.ssh` 目录 `700`，`authorized_keys` 文件 `600`。
6. 防火墙、安全组、路由或 VPN ACL 允许本机访问目标端口。
7. 首次连接应确认 host key 指纹，不要盲目跳过主机校验。

生产发布主机 `jst_ecs` 还需要：

- `bash`
- `tar`
- `rsync`
- `systemctl`
- `nginx`
- 可执行 Go 后端 Linux AMD64 二进制
- 可读写 `/root/ecommerce_ai`
- 可读写 `/var/www/yongbo.cloud`
- 可访问 MySQL、Redis、ERP Bridge 和上传服务所需网络

前端发布脚本会在远端执行备份、上传、`nginx -t`、`systemctl reload nginx` 和 HTTP 探测。后端发布脚本会上传 release 包、运行迁移、启动 MAIN/Bridge 进程并写入 release history。

## 13. 开发规则

- 不提交真实凭据、token、私钥、密钥路径或本地 `.env`。
- 路由变化和 OpenAPI 契约变化必须在同一个逻辑改动里完成。
- 不要在没有迁移决策的情况下随意改 `db/migrations/**`。
- 前端展示层可以做中文映射，但不要擅自改变提交给后端的字段和值。
- 后端、前端、发布脚本的职责要分清；不要把静态前端发布写成后端发布。
- 合并 `dev/external-developer` 到 `main` 前应先完成本地评审和必要校验。
- 每个提交尽量只包含一个清晰的逻辑改动。

## 14. 推荐日常流程

开始前：

```bash
git fetch origin --prune
git status --short --branch
```

后端改动：

```bash
go test ./... -count=1
./scripts/agent-check.sh
```

前端改动：

```bash
cd vue
npm run test
npm run build:prod
```

发布前：

```bash
git diff --check
git diff --stat
git log --oneline --decorate -5
```

发布后：

- 后端：执行远端 runtime verification。
- 前端：浏览器登录真实账号，检查受影响页面。
- 同时改前后端：确认 `/v1`、`/ws`、`/upload` 代理链路都符合预期。
