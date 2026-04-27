$ErrorActionPreference = 'Stop'
if ($PSVersionTable.PSVersion.Major -ge 7) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$Root = Split-Path -Parent $ScriptDir
$EnvPath = Join-Path $Root '.vscode\deploy.local.env'

function Load-LocalDeployEnv {
    param([string]$Path)

    if (-not (Test-Path $Path)) {
        return
    }

    foreach ($line in Get-Content $Path) {
        if ([string]::IsNullOrWhiteSpace($line) -or $line.TrimStart().StartsWith('#')) {
            continue
        }
        $parts = $line -split '=', 2
        if ($parts.Count -ne 2) {
            continue
        }
        $name = $parts[0].Trim()
        $value = $parts[1]
        if (-not $name.StartsWith('DEPLOY_')) {
            continue
        }
        if ([string]::IsNullOrEmpty([Environment]::GetEnvironmentVariable($name))) {
            [Environment]::SetEnvironmentVariable($name, $value)
        }
    }
}

function Ensure-SshConfigBlock {
    param(
        [string]$ConfigPath,
        [string]$DeployHost,
        [string]$DeployUser,
        [string]$DeployPort,
        [string]$IdentityFile
    )

    $marker = '# deploy-ecommerce-ai'
    $block = @(
        ''
        $marker
        "Host $DeployHost"
        "  IdentityFile $IdentityFile"
        '  IdentitiesOnly yes'
        "  User $DeployUser"
        "  Port $DeployPort"
    ) -join "`r`n"

    if (-not (Test-Path $ConfigPath)) {
        Set-Content -Path $ConfigPath -Value $block
        return
    }

    $content = Get-Content $ConfigPath -Raw
    if ($content -notmatch [regex]::Escape($marker)) {
        Add-Content -Path $ConfigPath -Value $block
    }
}

function Test-KeyLogin {
    param(
        [string]$Target,
        [string]$Port
    )

    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        & ssh.exe -o BatchMode=yes -o ConnectTimeout=10 -o StrictHostKeyChecking=accept-new -p $Port $Target 'exit 0' *> $null
        return ($LASTEXITCODE -eq 0)
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
}

function Quote-ShSingle {
    param([string]$Value)
    return "'" + ($Value -replace "'", "'\''") + "'"
}

function Invoke-PasswordSsh {
    param(
        [string]$Target,
        [string]$Port,
        [string]$Password,
        [string]$RemoteCommand
    )

    $askpassPath = Join-Path $env:TEMP 'askpass-deploy.cmd'
    $escapedPassword = $Password -replace '([()%!^"<>&|])', '^$1'

    Set-Content -Path $askpassPath -Value "@echo off`r`necho $escapedPassword`r`n"
    $env:SSH_ASKPASS = $askpassPath
    $env:SSH_ASKPASS_REQUIRE = 'force'
    $env:DISPLAY = 'dummy'

    try {
        & ssh.exe `
            -o StrictHostKeyChecking=accept-new `
            -o PreferredAuthentications=password `
            -o PubkeyAuthentication=no `
            -p $Port `
            $Target `
            $RemoteCommand
        if ($LASTEXITCODE -ne 0) {
            throw "Password SSH command failed with exit code $LASTEXITCODE."
        }
    }
    finally {
        Remove-Item $askpassPath -Force -ErrorAction SilentlyContinue
    }
}

Load-LocalDeployEnv -Path $EnvPath

$deployHost = [Environment]::GetEnvironmentVariable('DEPLOY_HOST')
$deployUser = [Environment]::GetEnvironmentVariable('DEPLOY_USER')
$deployPort = [Environment]::GetEnvironmentVariable('DEPLOY_PORT')
$deployPassword = [Environment]::GetEnvironmentVariable('DEPLOY_PASSWORD')

if ([string]::IsNullOrWhiteSpace($deployHost)) { throw 'DEPLOY_HOST required.' }
if ([string]::IsNullOrWhiteSpace($deployUser)) { throw 'DEPLOY_USER required.' }
if ([string]::IsNullOrWhiteSpace($deployPort)) { $deployPort = '22' }

$sshDir = Join-Path $HOME '.ssh'
$keyPath = Join-Path $sshDir 'id_deploy_ecommerce'
$pubPath = "$keyPath.pub"
$configPath = Join-Path $sshDir 'config'
$target = "$deployUser@$deployHost"

New-Item -ItemType Directory -Force -Path $sshDir | Out-Null

if (-not (Test-Path $pubPath)) {
    & ssh-keygen.exe -t ed25519 -f $keyPath -N '' -C 'deploy-ecommerce-ai'
    if ($LASTEXITCODE -ne 0) {
        throw 'ssh-keygen failed.'
    }
}

$identityForConfig = $keyPath -replace '\\', '/'
Ensure-SshConfigBlock -ConfigPath $configPath -DeployHost $deployHost -DeployUser $deployUser -DeployPort $deployPort -IdentityFile $identityForConfig

if (-not (Test-KeyLogin -Target $target -Port $deployPort)) {
    if ([string]::IsNullOrWhiteSpace($deployPassword)) {
        throw 'DEPLOY_PASSWORD is required for first-time key setup when passwordless SSH is not yet available.'
    }

    $publicKey = (Get-Content $pubPath -Raw).Trim()
    $quotedKey = Quote-ShSingle -Value $publicKey
    $remoteCommand = "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; chmod 700 ~/.ssh; chmod 600 ~/.ssh/authorized_keys; grep -qxF $quotedKey ~/.ssh/authorized_keys || printf '%s\n' $quotedKey >> ~/.ssh/authorized_keys"
    Invoke-PasswordSsh -Target $target -Port $deployPort -Password $deployPassword -RemoteCommand $remoteCommand
}

if (-not (Test-KeyLogin -Target $target -Port $deployPort)) {
    throw 'SSH key verification failed after authorized_keys update.'
}

Write-Output "SSH key setup complete: $keyPath"
