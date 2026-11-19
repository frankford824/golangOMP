# 主站正式迭代的 OA 发布入口

OA 使用现有 main-ops 的 `index.html`、AppShell、路由、登录组件及权限 store。`bash scripts/build-oa.sh` 为该入口打开新版功能并输出独立静态产物。`npm run build:prod` 继续生成当前主站版本。两者调用同一套正式 `/v1/*` 服务；资产工作台继续使用自己的地址、业务线和原发布目录。

## 数据与权限

订单服务通过 ops_reader 读取开放视图；任务、人员、组织、资产、SKU 与角色始终归现有系统。原生业务写操作使用登录人的正式凭证与现有后端规则。OA 域上的人工修改就是正式业务修改。独立 OS 进程负责新增的 `/oa-api/*` 扩展，不提供账号注册、登录或模拟身份。扩展进程的只读连接仅获正式库必要字段的 SELECT，写连接按表授权，人员目录联查只读必要身份与组织列，已授权的归档流程沿用原任务审计权限。通过主站原生人员接口确认 Excel 资料修改时继续执行原授权。文件目录 `/var/lib/yongbo-oa-live/files` 仅用于新增真实费用附件。

迁移 152 仅新增费用模板、模板版本、委托、财务权限、单据、附件、任职元数据、通知、导入记录、商品反馈和任务来源表。旧演练数据、密码与单据不导入。未指定的人员不取得出纳权限；正式模板、审批人与财务经办人由获授权人员在实际组织中配置。

迁移 154 保存费用审计事件，运行账号仅获 SELECT 和 INSERT。迁移 155 保存 OA 人员档案及组织负责人，运行账号获 SELECT、INSERT、UPDATE。已有 OA 服务在应用这两项迁移后，加载原部署数据库管理环境，运行 `python3 deploy/oa-live/provision.py --office-grants` 增量授权，原服务凭据继续生效。

## 发布和切换

1. 在主站最新集成分支审查变更，执行 Go 门禁、前端测试、`build:prod` 及 `build:oa`。从 WSL 运行 `bash deploy/oa-live/package.sh /tmp/oa-main-ops.tgz`。该包只有主站新版前端和 OA 扩展服务，不包含另一份 cmd/server、Redis 或核心业务库。
2. 确认正式库迁移台账及同名表，按现有 `deploy/run-pending-migrations.sh` 执行获准的 OA 迁移 152、154、155 的正向部分。不要执行回滚段。按范围为新服务配置正式库账号：加载主站 DB 管理环境，传入订单只读凭据到 `deploy/oa-live/provision.py` 的 stdin；秘密不进入命令参数、Git 或终端输出。
3. OA 域证书由 Certbot HTTP-01 续期。把审核后的包传到 ECS，执行 `bash install.sh <包> <SHA256>`。只切换 `/srv/yongbo-oa-live/current`、OA 虚拟主机和 `yongbo-oa-live.service`；原 8080 服务、主站静态目录和资产站均不切换。OA 虚拟主机沿用在线主站的 API、限流、上传、文件和 WebSocket 路由，另将 `/oa-api/` 送往 127.0.0.1:18095。
4. 通过 `/oa-release.json` 核实版本，检查三站 HTTPS、正式 API 的同源公开数据、无凭证拒绝以及 ClickHouse 水位。生产验收不进行演示入职、离职、改价、报销、付款或结算。需要端到端写验证时使用本地 fixture。所有者认可后另行决定是否把主站默认前端切换到新版；OA 发布不会自动切换主站。

接口见 `docs/api/oa-openapi.yaml`、`docs/oa/orders.md` 和 `docs/oa/office.md`。

## 费用与人事 P1 发布

通过正式迁移台账只应用 156、157、158 的正向段，然后使用原数据库部署环境执行 `python3 deploy/oa-live/provision.py --office-p1`。该模式向现有 OA 账号增量授权；在首次启用时生成 `/etc/yongbo-oa-live/office-key.env`（0600），之后发布继续沿用原密钥。恢复含草稿或账户的数据库时必须同时恢复该密钥，不能替换为随机新密钥。预览审批人和人事详情需要正式只读账号可读取头像与邮箱。OA 内部健康检查使用扩展进程监听的回环端口；公网 OA 虚拟主机对此路径返回 404。原主站和资产域名继续使用各自的发布流程。
