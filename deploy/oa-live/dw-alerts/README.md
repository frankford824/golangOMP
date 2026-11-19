# 订单开放层告警接收端

jst_ecs 的接收地址为 `POST http://100.125.196.22:8090/internal/dw-alerts`。签名、原始 JSON 和事件时间遵循《订单数据开放：接口说明》第 7 节。时间窗检查针对请求头中的发送时间，补发事件可以保留更早的北京时间。持久化成功或同一事件已存在时返回 204。签名失败或请求时间戳偏差超过 300 秒返回 401；持久化失败返回 503 供发送方重试。

接收服务独立于公网 OA 站点。以 root 在 jst_ecs 执行 `bash install.sh --key-stdin`，标准输入为数据层交付的原始密钥字节。部署脚本将密钥按 Base64 无损编码存入权限为 600 的 `/etc/yongbo-oa-live/dw-alerts.env`。后续更新执行 `bash install.sh` 复用已有配置。部署不变更发送方 URL、Tailscale 或 ClickHouse。

`oa-dw-alerts.service` 只在上述 tailnet 地址开放 POST。事件存入 `/var/lib/oa-dw-alerts/events.sqlite3`，按来源和告警 ID 关联。触发和恢复分别去重，持续提醒按本次事件时间去重。info 只留历史记录。warning 和 critical 计入未恢复数，恢复后移出计数。OA 管理员可在通知中心「数据告警」查看最近 100 条事件。OA 扩展经本地 Unix 套接字读取，读接口按现有管理员身份鉴权。
