$ErrorActionPreference = "Stop"

$changed = $env:CONTRACT_GUARD_CHANGED_FILES
if ([string]::IsNullOrWhiteSpace($changed)) {
  $changed = git diff --cached --name-only
}

$codeChanged = $false
$contractChanged = $false
foreach ($file in ($changed -split "`n")) {
  $f = $file.Trim()
  if ($f -match '^(transport/handler/.*\.go|service/.*\.go|domain/.*\.go)$') {
    $codeChanged = $true
  }
  if ($f -eq 'docs/api/openapi.yaml') {
    $contractChanged = $true
  }
}

$msg = $env:CONTRACT_GUARD_COMMIT_MESSAGE
if ([string]::IsNullOrWhiteSpace($msg)) {
  $msg = git log -1 --pretty=%B 2>$null
}

if (-not $codeChanged -and -not $contractChanged) {
  exit 0
}

if ($codeChanged -and -not $contractChanged) {
  if ($msg -like '*[contract-skip-justified]*') {
    exit 0
  }
  Write-Error "contract-guard: code contract surface changed without docs/api/openapi.yaml. Add OpenAPI changes in the same commit or include [contract-skip-justified] for architect review."
  exit 1
}

$job = Start-Job -ScriptBlock {
  $goBin = $env:GO_BIN
  if ([string]::IsNullOrWhiteSpace($goBin)) { $goBin = "go" }
  & $goBin run ./tools/contract_audit --transport transport/http.go --handlers transport/handler --domain domain --openapi docs/api/openapi.yaml --output tmp/v1_2/contract_guard_audit.json --fail-on-drift true
}
if (-not (Wait-Job $job -Timeout 60)) {
  Stop-Job $job
  Write-Error "contract-guard: contract_audit timed out after 60s"
  exit 1
}
Receive-Job $job
if ($job.State -ne 'Completed') {
  exit 1
}
