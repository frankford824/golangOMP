# OA 验证站

`oa.yongbo.cloud` 用于验证新版。此分支保持独立，合并主站由所有者在验证后决定。访问 `/oa.html` 进入 OA；同源的 `/oa-work.html`、`/oa-assets.html` 使用隔离任务与资产服务。三个入口编译为同一份 OA 静态包。原两站的目录、服务和发布指针不参与部署。

## 运行边界

| 资源 | OA 验证配置 |
|---|---|
| 会话、密码、Redis | 私有 core；独立账号和会话表；Redis 在私网 namespace；没有导入旧站密码、会话或真实收款信息 |
| 人事、审批、费用、付款、任务、计件、成本 | 专用 MySQL 库 `yongbo_oa`，独立应用账号。付款是验证库内的业务登记 |
| 订单和提成名册 | 唯一的持续生产数据依赖为授权的 jst ClickHouse 只读视图，通过 `ops_reader` 获取；查询参数、缓存与水位见 orders.md |
| 任务/SKU 关联 | 部署时从主站有界复制组织、授权、任务主索引和 SKU 及成本字段。没有复制原文件、全部历史单据、付款明细或 ERP 队列。复制后可供沙箱改动 |
| 原应用业务操作 | 使用 `cmd/server` 的独立实例及专属 DB，在 `PrivateNetwork=true` 的系统服务中运行。API 的 `/v1/*` 只代理私有 core。公网没有代理到原主站、Bridge 或生产 ERP 的路径 |
| 上传 | 签名对象进程和费用凭证均写到 `/var/lib/yongbo-oa`，不使用生产 OSS；签名 URL 和浏览器连接留在 OA 域 |
| HTTPS | Nginx 的单独 vhost，专用证书；HTTP-01 由 Certbot webroot 续期 |
| 后台和内存 | OA 服务均有独立 systemd 单元和资源限制。主系统与验证站共用 ECS，因此基础资源仍需监测 |

## 发布

以独立发布分支的提交打包。开发机在 `vue/` 使用项目锁文件准备依赖，再从根目录执行 `bash deploy/oa/package.sh /tmp/oa-release.tgz`。上传工件及安装脚本后，用打包时记录的 SHA-256 执行 `bash install.sh /path/to/archive SHA256`。安装脚本只切换 `/srv/yongbo-oa/current`，保留前一版；验证本地双服务、三站 HTTPS 和 OA 静态版本文件。在数据结构向前兼容的前提下，回退可重新安装已核验的上一 OA 包，无需恢复或改动两个原站。

首次部署由 ECS root 执行 `deploy/oa/provision.sh`，只在专用验证库中建表、初始化目录和授权。管理员登录材料在 `/etc/yongbo-oa/`，不要将这些秘密提交到 Git 或输出到日志。OA 的 systemd EnvironmentFile 应由 systemd 读取，不是 shell 脚本。只读 ClickHouse 密码通过保密通道写入 `oa.env` 的 `OPS_READER_PASSWORD`；对象存储私钥和根目录在 `core.env` 中配置 `OA_OBJECT_STORE=1`、`OA_OBJECT_SECRET`、`OA_OBJECT_ROOT`。Core 获得 `identity.json` 的 systemd credential。私有 MySQL socket 代理运行在主机网络，core 与对象服务只可在自己的 loopback 上通信。

## 验证

先验证主站、资产站 HTTP 状态和已有服务连续运行，再以 OA 凭据验证订单、人员、费用和受签名保护的文件流。费用审批和结算测试只对验证样本操作。订单查询基准从 ECS 运行 `deploy/oa/smoke-orders.py`；需通过环境注入一次性 OA 会话，不在命令行传密码或令牌。MySQL 独立 fixture 的队列检查使用 `OA_TEST_DSN` 和空白的 `oa_office_fixture_*` schema。不要用公司真实付款资料作为验收样本。
