# agent-check.ps1 — consolidated full-gate verification for AI coding agents.
#
# PowerShell mirror of scripts/agent-check.sh.
# Runs the six checks AGENTS.md §After Editing requires; first failure exits.
#
# Environment overrides:
#   $env:GO_BIN                  — path to go binary (default: 'go')
#   $env:PYTHON_BIN              — Python 3 executable (default: 'python')
#   $env:AGENT_CHECK_SKIP_TESTS  — '1' to skip step 3
#   $env:AGENT_CHECK_AUDIT_OUT   — JSON output path (default: tmp/agent_check_audit.json)

$ErrorActionPreference = 'Stop'

$goBin = if ($env:GO_BIN) { $env:GO_BIN } else { 'go' }
$pythonBin = if ($env:PYTHON_BIN) { $env:PYTHON_BIN } else { 'python' }
if (-not (Get-Command $goBin -ErrorAction SilentlyContinue)) {
    throw "go binary not found (looked for '$goBin'). Set `$env:GO_BIN to the full path."
}

$auditOut = if ($env:AGENT_CHECK_AUDIT_OUT) { $env:AGENT_CHECK_AUDIT_OUT } else { 'tmp/agent_check_audit.json' }
$auditDir = Split-Path -Parent $auditOut
if ($auditDir -and -not (Test-Path $auditDir)) {
    New-Item -ItemType Directory -Path $auditDir -Force | Out-Null
}

function Step {
    param([string]$Label, [string]$Description)
    Write-Host ""
    Write-Host "==> [$Label] $Description"
}

function Invoke-Step {
    param([string]$Label, [scriptblock]$Action)
    & $Action
    if ($LASTEXITCODE -ne 0) {
        Write-Host ""
        Write-Error "FAIL at step [$Label] (exit $LASTEXITCODE). Stop and fix the cause. Do not bypass."
        exit 1
    }
}

Step '1/6' 'go vet ./...'
Invoke-Step '1/6' { & $goBin vet ./... }

Step '2/6' 'go build ./...'
Invoke-Step '2/6' { & $goBin build ./... }

if ($env:AGENT_CHECK_SKIP_TESTS -eq '1') {
    Step '3/6' 'go test ./... -count=1   [SKIPPED via $env:AGENT_CHECK_SKIP_TESTS=1]'
} else {
    Step '3/6' 'go test ./... -count=1'
    Invoke-Step '3/6' { & $goBin test ./... -count=1 }
}

Step '4/6' 'experience migration guard'
Invoke-Step '4/6' { & $pythonBin scripts/check_experience_migrations.py }

Step '5/6' 'openapi-validate docs/api/openapi.yaml'
Invoke-Step '5/6' { & $goBin run ./cmd/tools/openapi-validate docs/api/openapi.yaml }

Step '6/6' "contract_audit --fail-on-drift true   (output: $auditOut)"
Invoke-Step '6/6' {
    & $goBin run ./tools/contract_audit `
        --transport transport/http.go `
        --handlers transport/handler `
        --domain domain `
        --openapi docs/api/openapi.yaml `
        --output $auditOut `
        --fail-on-drift true
}

Write-Host ""
if ($env:AGENT_CHECK_SKIP_TESTS -eq '1') {
    Write-Host 'PASS - executed checks green; tests skipped (not a full gate pass).'
} else {
    Write-Host 'PASS - all 6 checks green.'
}
