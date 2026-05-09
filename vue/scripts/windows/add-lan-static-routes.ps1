#Requires -RunAsAdministrator
# Adds persistent routes: 192.168.1.0/24 and 192.168.2.0/24 via gateway (default 192.168.0.1).
# Run in elevated PowerShell:  cd D:\vue\scripts\windows
#   Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass
#   .\add-lan-static-routes.ps1
param(
  [string]$Gateway = '192.168.0.1'
)

$ErrorActionPreference = 'Stop'

$principal = [Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
  Write-Host 'Run PowerShell as Administrator.' -ForegroundColor Red
  exit 1
}

# Prefer adapter that has 192.168.0.x (your LAN)
$ipcfg = Get-NetIPConfiguration |
  Where-Object {
    $_.NetAdapter -and
    $_.NetAdapter.Status -eq 'Up' -and
    $_.IPv4Address -and
    ($_.IPv4Address | Where-Object { $_.IPAddress -like '192.168.0.*' })
  } |
  Select-Object -First 1

if ($ipcfg) {
  $ifIdx = $ipcfg.InterfaceIndex
  $nicLabel = $ipcfg.InterfaceAlias
} else {
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
    Write-Host 'No connected Ethernet/Wi-Fi adapter found.' -ForegroundColor Red
    exit 1
  }
  $ifIdx = $eth.InterfaceIndex
  $nicLabel = $eth.Name
}

if (-not $ifIdx -or $ifIdx -lt 1) {
  Write-Host 'Could not resolve InterfaceIndex.' -ForegroundColor Red
  exit 1
}

Write-Host "NIC: $nicLabel  ifIndex=$ifIdx  gateway=$Gateway" -ForegroundColor Cyan

$prevEap = $ErrorActionPreference
$ErrorActionPreference = 'SilentlyContinue'
[void](route.exe delete 192.168.1.0 2>$null)
[void](route.exe delete 192.168.2.0 2>$null)
$ErrorActionPreference = $prevEap

$add1 = Start-Process -FilePath 'route.exe' -ArgumentList @(
  '-p', 'add', '192.168.1.0', 'mask', '255.255.255.0', $Gateway, 'metric', '256', 'if', "$ifIdx"
) -Wait -PassThru -NoNewWindow

if ($add1.ExitCode -ne 0) {
  throw "route add 192.168.1.0 failed, exit=$($add1.ExitCode). Check gateway $Gateway and ifIndex $ifIdx."
}

$add2 = Start-Process -FilePath 'route.exe' -ArgumentList @(
  '-p', 'add', '192.168.2.0', 'mask', '255.255.255.0', $Gateway, 'metric', '256', 'if', "$ifIdx"
) -Wait -PassThru -NoNewWindow

if ($add2.ExitCode -ne 0) {
  throw "route add 192.168.2.0 failed, exit=$($add2.ExitCode)."
}

Write-Host 'Persistent routes added:' -ForegroundColor Green
Write-Host "  192.168.1.0/24 -> $Gateway"
Write-Host "  192.168.2.0/24 -> $Gateway"
Write-Host ''
Write-Host 'Matching routes:' -ForegroundColor Yellow
route.exe print | Select-String -Pattern '192\.168\.[12]\.'
