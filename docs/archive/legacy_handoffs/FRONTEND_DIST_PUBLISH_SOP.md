# 前端 dist 一键发布（SOP）

将前端同事交付的**正式构建产物**从本仓库固定目录同步到生产静态站，**不**发布后端、**不**修改 `/root/ecommerce_ai/releases`。

## 固定约定

| 项 | 路径或值 |
|----|-----------|
| 本地产物目录 | `<仓库根>/dist/front`（须含 `index.html` 与 `assets/`） |
| SSH Host | 默认 `jst_ecs`（来自 `~/.ssh/config`） |
| 线上静态根 | `/var/www/yongbo.cloud` |
| 备份目录 | `/var/www/backups/yongbo.cloud_<UTC时间戳>/` |
| 上传临时目录 | `/tmp/yongbo.cloud_dist_<UTC时间戳>/` |
| 生产 API | 浏览器侧须走相对路径 **`/v1`**（由 Nginx 反代到 MAIN） |
| 备案 | 登录等页须含 **苏ICP备2026007026号-1**，链接 **https://beian.miit.gov.cn/** |

## 前置条件

1. 已安装 OpenSSH 客户端：`ssh`、`scp` 在 `PATH` 中。
2. 本机可免密或使用密钥登录：`ssh jst_ecs`（或你配置的 Host）可用。
3. 远端已安装 **`rsync`**（用于 `rsync -a --delete` 覆盖正式目录并清理旧哈希资源）、**Nginx**。
4. 已将构建产物放入 **`dist/front`**。

## 使用方法

### Windows（推荐）

在仓库根目录执行：

```powershell
powershell -ExecutionPolicy Bypass -File .\deploy\publish-front.ps1
```

常用参数：

| 参数 | 说明 |
|------|------|
| `-SshHost myhost` | 改用其他 SSH Host 别名 |
| `-RepoRoot D:\path\to\repo` | 显式指定仓库根（一般不用） |
| `-SkipChecks` | 跳过本地 dist / API / 备案扫描（紧急用） |
| `-SkipVerify` | 跳过发布后 `curl` 探测 |
| `-DryRun` | 只打印约定路径，不连服务器 |

示例：

```powershell
.\deploy\publish-front.ps1 -DryRun
.\deploy\publish-front.ps1 -SshHost jst_ecs
```

### Git Bash / WSL / Linux / macOS

```bash
chmod +x deploy/publish-front.sh
./deploy/publish-front.sh
```

环境变量与参数：

| 方式 | 说明 |
|------|------|
| `SSH_HOST=jst_ecs ./deploy/publish-front.sh` | 指定 Host |
| `./deploy/publish-front.sh --host jst_ecs` | 同上 |
| `./deploy/publish-front.sh --skip-checks` | 跳过本地检查 |
| `./deploy/publish-front.sh --skip-verify` | 跳过 curl |
| `./deploy/publish-front.sh --dry-run` | 干跑 |

本机若存在 **`rsync`**，脚本会用 `rsync -av` 上传到临时目录；否则退回 **`scp -r`**。

## 脚本自动执行的步骤

1. **本地检查**（可用 `--skip-checks` 关闭）：`dist/front` 结构、`index.html` 无 localhost、主入口 JS 无典型本机 API 字面量、提示 `/v1` 与备案。
2. **远端备份**：`cp -a /var/www/yongbo.cloud/.` 到 `/var/www/backups/yongbo.cloud_<UTC>/`。
3. **上传**：到 `/tmp/yongbo.cloud_dist_<UTC>/`（不引入多一层 `front/`）。
4. **覆盖正式站**：`rsync -a --delete` 到 `/var/www/yongbo.cloud/`。
5. **权限**：`chmod -R a+rX /var/www/yongbo.cloud`。
6. **Nginx**：`nginx -t` 通过后 `systemctl reload nginx`。
7. **可选验证**：请求首页、`/login`、`/health`、`POST /v1/auth/login`（期望非 404）。

发布后请用浏览器与**真实账号**做登录、列表、详情、资源预览等抽查。

## 回滚

若线上异常，用**本次发布生成的备份路径**（脚本成功时会在输出中提示 `yongbo.cloud_<UTC>`）在服务器执行：

```bash
rsync -a --delete /var/www/backups/yongbo.cloud_<UTC>/ /var/www/yongbo.cloud/
chmod -R a+rX /var/www/yongbo.cloud
nginx -t && systemctl reload nginx
```

PowerShell 脚本在 scp 或 nginx 失败时也会打印类似的回滚命令提示。

## 常见错误

- **API 不是 `/v1`**：构建里写死 IP/端口或 localhost，本地检查会失败；修正前端环境变量后重新 build。
- **多一层目录**：必须把 `index.html` 放在 `yongbo.cloud` 根下，不要出现 `.../yongbo.cloud/front/index.html`。
- **旧 `assets` 残留**：必须用 `rsync --delete`（脚本已包含），否则易白屏或 404。
- **误传到 release 目录**：静态站只动 `/var/www/yongbo.cloud`，与 `/root/ecommerce_ai/releases` 无关。
- **history 路由 404**：属 Nginx `try_files` 配置问题，非本脚本职责；参见 `deploy/nginx/yongbo.cloud.production.conf`。

## 相关文件

- 一键脚本：`deploy/publish-front.ps1`、`deploy/publish-front.sh`
- Nginx 模板：`deploy/nginx/yongbo.cloud.conf`、`deploy/nginx/yongbo.cloud.production.conf`
