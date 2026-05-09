#Requires -RunAsAdministrator
<#
.SYNOPSIS
  在已连接的物理网卡上额外绑定 192.168.1.x / 192.168.2.x 地址（可选）。

.DESCRIPTION
  仅在以下情况有意义：
  - 交换机/路由器对多个网段做 VLAN 终结或同一二层上多网段，且需要本机在多个网段各有一个 IP；
  - 或你明确知道自己在做的拓扑。

  普通家庭单网段 LAN 上乱加第二、第三网段 IP，可能导致异常 ARP/访问问题，请谨慎。

  默认会为 Realtek 以太网 #2 添加（若不存在则回退到第一个非虚拟已连接网卡）：
    192.168.1.37/24
    192.168.2.37/24

  若与局域网内其他设备冲突，请改参数 -Ip1 -Ip2。

  使用（管理员 PowerShell）：
    .\add-secondary-lan-ips.ps1
    .\add-secondary-lan-ips.ps1 -Ip1 192.168.1.100 -Ip2 192.168.2.100
#>
param(
  [string]$Ip1 = '192.168.1.37',
  [string]$Ip2 = '192.168.2.37',
  [string]$Mask = '255.255.255.0'
)

$ErrorActionPreference = 'Stop'

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Host '请以管理员身份运行。' -ForegroundColor Red
  exit 1
}

$eth = Get-NetAdapter | Where-Object {
  $_.Status -eq 'Up' -and $_.InterfaceDescription -match 'Realtek.*#2'
} | Select-Object -First 1

if (-not $eth) {
  $eth = Get-NetAdapter | Where-Object {
    $_.Status -eq 'Up' -and
    $_.InterfaceDescription -notmatch 'VMware|Virtual|Hyper-V|Loopback|Bluetooth|Teredo|Meta'
  } | Select-Object -First 1
}

if (-not $eth) {
  Write-Host '未找到已连接网卡。' -ForegroundColor Red
  exit 1
}

Write-Host "网卡: $($eth.Name)  将添加: $Ip1 , $Ip2" -ForegroundColor Cyan

try {
  New-NetIPAddress -InterfaceIndex $eth.InterfaceIndex -IPAddress $Ip1 -PrefixLength 24 -ErrorAction Stop
  Write-Host "已添加 $Ip1/24" -ForegroundColor Green
} catch {
  if ($_.Exception.Message -match 'already exists|已存在|object already exists') {
    Write-Host "$Ip1 可能已存在，跳过。" -ForegroundColor Yellow
  } else {
    throw
  }
}

try {
  New-NetIPAddress -InterfaceIndex $eth.InterfaceIndex -IPAddress $Ip2 -PrefixLength 24 -ErrorAction Stop
  Write-Host "已添加 $Ip2/24" -ForegroundColor Green
} catch {
  if ($_.Exception.Message -match 'already exists|已存在|object already exists') {
    Write-Host "$Ip2 可能已存在，跳过。" -ForegroundColor Yellow
  } else {
    throw
  }
}

Write-Host ''
Get-NetIPAddress -InterfaceIndex $eth.InterfaceIndex -AddressFamily IPv4 |
  Select-Object IPAddress, PrefixLength | Format-Table -AutoSize
