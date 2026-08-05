#Requires -Version 5.1
# UTF-8 BOM: required for Windows PowerShell 5.1 to parse non-ASCII in this file.
<#
.SYNOPSIS
  Publish local dist/front to jst_ecs:/var/www/yongbo.cloud (frontend static SOP).

.DESCRIPTION
  Requires OpenSSH (ssh, scp) and SSH config Host (default: jst_ecs).
  Does not deploy backend or touch /root/ecommerce_ai/releases.

.PARAMETER SshHost
  SSH config Host name. Default: jst_ecs.

.PARAMETER RepoRoot
  Repository root. Default: parent of deploy/.

.PARAMETER SkipChecks
  Skip local dist/API/beian checks (emergency only).

.PARAMETER SkipVerify
  Skip post-deploy curl checks.

.PARAMETER DryRun
  Print paths only; no SSH upload.
#>
param(
    [string]$SshHost = "jst_ecs",
    [string]$RepoRoot = "",
    [switch]$SkipChecks,
    [switch]$SkipVerify,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"
if ($PSVersionTable.PSVersion.Major -ge 7) {
    $PSNativeCommandUseErrorActionPreference = $false
}

function Test-CommandExists {
    param([string]$Name)
    return [bool](Get-Command $Name -ErrorAction SilentlyContinue)
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if ([string]::IsNullOrWhiteSpace($RepoRoot)) {
    $RepoRoot = (Resolve-Path (Join-Path $ScriptDir "..")).ProviderPath
} else {
    $RepoRoot = (Resolve-Path $RepoRoot).ProviderPath
}
$FrontDir = Join-Path $RepoRoot "dist\front"
$RemoteWebRoot = "/var/www/yongbo.cloud"
$RemoteBackupParent = "/var/www/backups"

function Write-Step { param([string]$Msg) Write-Host "[publish-front] $Msg" -ForegroundColor Cyan }

function Invoke-ArtifactIdentityGuard {
    Write-Step "Artifact identity guard: main-ops frontend ..."
    if (-not (Test-Path $FrontDir -PathType Container)) {
        throw "Missing directory: $FrontDir"
    }
    $indexPath = Join-Path $FrontDir "index.html"
    $assetsDir = Join-Path $FrontDir "assets"
    $assetEntryPath = Join-Path $FrontDir "asset.html"
    $manifestPath = Join-Path $FrontDir "static-artifact-manifest.json"
    if (-not (Test-Path $indexPath -PathType Leaf)) {
        throw "Missing file: $indexPath"
    }
    if (-not (Test-Path $assetsDir -PathType Container)) {
        throw "Missing directory: $assetsDir"
    }
    if (Test-Path $assetEntryPath -PathType Leaf) {
        throw "dist/front contains asset.html; refusing to publish an asset-workbench bundle to yongbo.cloud"
    }
    if (-not (Test-Path $manifestPath -PathType Leaf)) {
        throw "Missing file: $manifestPath"
    }

    $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($manifest.schema -ne 1) { throw "Manifest schema must be 1" }
    if ($manifest.app -ne "main-ops") { throw "Manifest app must be main-ops" }
    if ($manifest.entry -ne "index.html") { throw "Manifest entry must be index.html" }
    if ($manifest.targetHost -ne "yongbo.cloud") { throw "Manifest targetHost must be yongbo.cloud" }
    if ($manifest.targetWebRoot -ne "/var/www/yongbo.cloud") { throw "Manifest targetWebRoot must be /var/www/yongbo.cloud" }
    if ($manifest.buildCommand -ne "npm run build:prod") { throw "Manifest buildCommand must be npm run build:prod" }
    if ($manifest.gitCommit -notmatch '^[0-9a-f]{40}$') { throw "Manifest gitCommit must be a 40-character commit hash" }
    if ([string]::IsNullOrWhiteSpace($manifest.builtAt)) { throw "Manifest builtAt is required" }

    $indexRaw = Get-Content -LiteralPath $indexPath -Raw -Encoding UTF8
    if ($indexRaw -notmatch 'id="app"') {
        throw "dist/front/index.html does not contain the main-ops mount node"
    }
    if ($indexRaw -match 'asset-workbench-app|src="/assets/asset-[^"]+\.js"') {
        throw "dist/front/index.html looks like an asset-workbench entry; rebuild the main frontend before publishing yongbo.cloud"
    }
    $assetRefs = [regex]::Matches($indexRaw, '(?:src|href)="\/(assets\/[^"]+)"')
    foreach ($match in $assetRefs) {
        $assetRel = $match.Groups[1].Value -replace '/', [IO.Path]::DirectorySeparatorChar
        $assetPath = Join-Path $FrontDir $assetRel
        if (-not (Test-Path $assetPath -PathType Leaf)) {
            throw "Referenced asset is missing: $assetPath"
        }
    }
}

function Invoke-LocalChecks {
    Write-Step "Local check: dist/front ..."
    if (-not (Test-Path $FrontDir -PathType Container)) {
        throw "Missing directory: $FrontDir (place production build under dist/front)"
    }
    $indexPath = Join-Path $FrontDir "index.html"
    if (-not (Test-Path $indexPath -PathType Leaf)) {
        throw "Missing file: $indexPath"
    }
    $assetsDir = Join-Path $FrontDir "assets"
    if (-not (Test-Path $assetsDir -PathType Container)) {
        throw "Missing directory: $assetsDir"
    }

    $indexRaw = Get-Content -LiteralPath $indexPath -Raw -Encoding UTF8
    if ($indexRaw -match 'localhost|127\.0\.0\.1') {
        throw "index.html contains localhost or 127.0.0.1; fix production build before publish"
    }

    if ($indexRaw -match 'src="(/assets/[^"]+\.js)"') {
        $rel = $Matches[1].TrimStart("/")
        $mainJs = Join-Path $FrontDir ($rel -replace "/", [IO.Path]::DirectorySeparatorChar)
        if (Test-Path -LiteralPath $mainJs) {
            $js = Get-Content -LiteralPath $mainJs -Raw -Encoding UTF8
            # Libraries may contain the substring "http://localhost" in docs/errors; treat as warning only.
            if ($js -match 'https?://localhost|https?://127\.0\.0\.1|127\.0\.0\.1:\d+') {
                Write-Warning 'Main bundle contains localhost/127.0.0.1 literals; confirm API base is still relative /v1.'
            }
            if ($js -notmatch '"/v1') {
                Write-Warning 'Main bundle: literal "/v1" not found; verify API base manually.'
            }
        }
    } else {
        Write-Warning "Could not parse main JS path from index.html; skipped main-bundle API scan"
    }

    $beianHits = Get-ChildItem -LiteralPath $FrontDir -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Extension -match '^\.(html|js|css)$' } |
        Select-String -Pattern "beian\.miit\.gov\.cn|2026007026" -SimpleMatch:$false -List -ErrorAction SilentlyContinue
    if (-not $beianHits) {
        Write-Warning "No beian.miit.gov.cn / 2026007026 found under dist/front; confirm ICP footer on login/home"
    }

    Write-Step "Local checks passed"
}

Invoke-ArtifactIdentityGuard

if (-not $SkipChecks) {
    Invoke-LocalChecks
}

if ($DryRun) {
    Write-Step "DryRun: Host=$SshHost source=$FrontDir target=$RemoteWebRoot"
    Write-Step "DryRun: remote backup yongbo.cloud_<UTC> and staging /tmp/yongbo.cloud_dist_<UTC>"
    exit 0
}

if (-not (Test-CommandExists "ssh") -or -not (Test-CommandExists "scp")) {
    throw "OpenSSH required: ssh and scp must be on PATH"
}

Write-Step "Fetch remote UTC timestamp ..."
$remoteTs = (ssh $SshHost "date -u +%Y%m%dT%H%M%SZ").Trim()
if ([string]::IsNullOrWhiteSpace($remoteTs)) {
    throw "Could not read remote timestamp; check: ssh $SshHost"
}

$backupPath = "${RemoteBackupParent}/yongbo.cloud_${remoteTs}"
$stagingPath = "/tmp/yongbo.cloud_dist_${remoteTs}"

Write-Step "Remote backup: $backupPath"
$backupCmd = "mkdir -p `"$backupPath`" && cp -a `"$RemoteWebRoot`"/. `"$backupPath`"/ && mkdir -p `"$stagingPath`""
ssh $SshHost $backupCmd
if ($LASTEXITCODE -ne 0) {
    throw "Remote backup failed"
}

Write-Step "Upload to staging (scp): $stagingPath"
$scpTarget = "${SshHost}:${stagingPath}/"
& scp -r "$FrontDir\*" $scpTarget
if ($LASTEXITCODE -ne 0) {
    throw "scp failed. Rollback: ssh $SshHost 'rsync -a --delete $backupPath/ $RemoteWebRoot/ && chmod -R a+rX $RemoteWebRoot && nginx -t && systemctl reload nginx'"
}

Write-Step "Remote artifact guard: main-ops staging ..."
$remoteGuardScript = @'
set -euo pipefail
stage=__STAGING_PATH__
expected_commit=__EXPECTED_COMMIT__
test -f "$stage/index.html"
test -d "$stage/assets"
test -f "$stage/static-artifact-manifest.json"
test ! -f "$stage/asset.html"
grep -Fq 'id="app"' "$stage/index.html"
! grep -Fq 'asset-workbench-app' "$stage/index.html"
python3 - "$stage/static-artifact-manifest.json" "$expected_commit" <<'PY'
import json
import sys

path, expected_commit = sys.argv[1:]
with open(path, encoding="utf-8") as handle:
    manifest = json.load(handle)
expected = {
    "schema": 1,
    "app": "main-ops",
    "entry": "index.html",
    "targetHost": "yongbo.cloud",
    "targetWebRoot": "/var/www/yongbo.cloud",
    "gitCommit": expected_commit,
}
actual = {key: manifest.get(key) for key in expected}
if actual != expected:
    raise SystemExit(f"staged artifact identity mismatch: {actual!r}")
PY
'@
$expectedCommit = (Get-Content -LiteralPath (Join-Path $FrontDir "static-artifact-manifest.json") -Raw -Encoding UTF8 | ConvertFrom-Json).gitCommit
$remoteGuardScript = $remoteGuardScript.Replace("__STAGING_PATH__", $stagingPath).Replace("__EXPECTED_COMMIT__", $expectedCommit)
$remoteGuardScript = $remoteGuardScript -replace "`r", ""
$remoteGuardScript | ssh $SshHost "tr -d '\r' | bash -s"
if ($LASTEXITCODE -ne 0) {
    throw "staged artifact failed the main-ops identity guard"
}

Write-Step "Rsync to webroot, chmod, nginx reload ..."
$deployCmd = "rsync -a --delete `"$stagingPath`"/ `"$RemoteWebRoot`"/ && chmod -R a+rX `"$RemoteWebRoot`" && nginx -t && systemctl reload nginx"
ssh $SshHost $deployCmd
if ($LASTEXITCODE -ne 0) {
    throw "Deploy failed (nginx -t may have failed). Rollback: ssh $SshHost 'rsync -a --delete $backupPath/ $RemoteWebRoot/ && chmod -R a+rX $RemoteWebRoot && nginx -t && systemctl reload nginx'"
}

Write-Step "Done. Backup kept at: $backupPath"

if (-not $SkipVerify) {
    if (-not (Test-CommandExists "curl.exe")) {
        Write-Warning "curl.exe not found; skip HTTP verify"
        exit 0
    }
    Write-Step "Post verify https://yongbo.cloud/ ..."
    $code = [int](curl.exe -sS -o NUL -w "%{http_code}" "https://yongbo.cloud/")
    if ($code -ne 200) { Write-Warning "Home HTTP $code" }
    $codeLogin = [int](curl.exe -sS -o NUL -w "%{http_code}" "https://yongbo.cloud/login")
    if ($codeLogin -ne 200) { Write-Warning "/login HTTP $codeLogin" }
    $codeHealth = [int](curl.exe -sS -o NUL -w "%{http_code}" "https://yongbo.cloud/health")
    if ($codeHealth -ne 200) { Write-Warning "/health HTTP $codeHealth" }
    $codeV1 = [int](curl.exe -sS -o NUL -w "%{http_code}" -X POST "https://yongbo.cloud/v1/auth/login" -H "Content-Type: application/json" -d "{}")
    if ($codeV1 -eq 404) { Write-Warning "POST /v1/auth/login returned 404; check Nginx /v1 proxy" }
    Write-Step "HTTP probe done; use a browser + real account for functional smoke test"
}
