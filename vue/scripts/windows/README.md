# Windows 多网段（192.168.0 / 1 / 2）

## 你需要区分两件事

1. **本机访问其它网段**（从你的电脑 `ping` / 访问 `192.168.1.x`、`192.168.2.x`）  
   → 在 **路由器已做好网段间路由** 的前提下，给 Windows 加 **静态路由** 即可：把去往 `192.168.1.0/24`、`192.168.2.0/24` 的下一跳设为你的 **网关**（例如 `192.168.0.1`）。

2. **其它网段的设备访问你这台电脑上的服务**（例如 Vite `5174`）  
   → 通常只需它们访问你 **主网段 IP**（如 `http://192.168.0.37:5174`），由 **路由器做跨网段转发**。  
   → 若必须在 `192.168.1.x`、`192.168.2.x` 上各有一个本机 IP，才考虑 **附加 IP**（脚本 `add-secondary-lan-ips.ps1`），且需你清楚自己的拓扑，避免与普通家用单网段冲突。

## 操作步骤（推荐：先只做静态路由）

1. 以 **管理员身份** 打开 PowerShell。
2. 执行：

```powershell
cd D:\vue\scripts\windows
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
.\add-lan-static-routes.ps1
```

网关不是 `192.168.0.1` 时：

```powershell
.\add-lan-static-routes.ps1 -Gateway 你的网关IP
```

3. 重启后路由仍保留（`route -p`）。

## 可选：附加 IP

仅在有明确需求时使用：

```powershell
.\add-secondary-lan-ips.ps1
# 或自定义地址，避免与 DHCP 冲突：
.\add-secondary-lan-ips.ps1 -Ip1 192.168.1.100 -Ip2 192.168.2.100
```

## 说明

- 脚本无法代替 **路由器配置**：若网关不知道 `192.168.1.0/24`、`192.168.2.0/24` 怎么走，本机加路由也到不了对端。
- 当前环境无法在对话里替你 **提升管理员权限** 执行上述命令，必须在你本机手动运行一次。

## 若报错「路由删除失败: 找不到元素」

旧版脚本在「路由本就不存在」时执行 `route delete` 会触发 PowerShell 的终止错误。请 **拉取最新** `add-lan-static-routes.ps1`（已改为忽略不存在的删除，并用 `Get-NetIPConfiguration` 解析带 `192.168.0.x` 的网卡 `ifIndex`）。

若仍失败，在管理员 PowerShell 执行 `Get-NetIPConfiguration`，确认本机 `192.168.0.x` 所在行的 **InterfaceIndex**，并检查网关是否为 `192.168.0.1`。
