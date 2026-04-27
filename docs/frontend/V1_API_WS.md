# WebSocket

> Revision: V1.2-D-2 residual drift triage (2026-04-26)
> Source: docs/api/openapi.yaml (post V1.2-D-2)

> 来源: `docs/api/openapi.yaml`；业务口径参考 V1 四份权威文档。本文不覆盖 OpenAPI 契约。

实时消息连接与事件通道。

## Family 约定

- WebSocket 使用 Bearer token 建连；断线后前端按指数退避重连。
- 实时事件只是提醒，页面最终状态仍以 HTTP detail/list 结果为准。
- 本文件覆盖 `0` 个 `/v1` path；同一路径多 method 合并在同一节。

## GET /ws/v1

### 简介
当前 OpenAPI 实际挂载的 WebSocket path 是 `/ws/v1`，不是 `/v1/ws/v1`。本文按 OpenAPI 真实路径记录；`/v1` path 统计为 209。

### 鉴权与 RBAC
- 需要 Bearer token，推荐通过协议约定或查询参数传递，具体以 transport 实现和前端联调环境为准。
- 允许角色: 已登录用户。
- 字段级授权: 无。

### 请求体 schema
无 HTTP JSON 请求体；WebSocket 握手后收发消息。

### 响应体 schema
成功握手返回 101 Switching Protocols；消息体以服务端事件 JSON 为准。

### 错误码
| HTTP | code | deny_code | 说明 |
|---|---|---|---|
| 401 | UNAUTHENTICATED | - | token 缺失或过期。 |
| 403 | PERMISSION_DENIED | - | 当前账号无实时通道权限。 |

### curl 示例
```bash
curl -i -N https://api.example.com/ws/v1 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket"
```

### 前端最佳实践
- 使用指数退避重连。
- WebSocket 事件只作为刷新提示，最终页面状态回读 HTTP 接口。

