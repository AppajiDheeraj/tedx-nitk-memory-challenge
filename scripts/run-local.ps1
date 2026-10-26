# Start the memory game on http://localhost:8082
# Usage (from anywhere):
#   powershell -ExecutionPolicy Bypass -File scripts/run-local.ps1
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$envFile = Join-Path $root ".env"
if (-not (Test-Path $envFile)) {
  Write-Error "Missing .env. Copy .env.example to .env and set DATABASE_URL."
}

Get-Content $envFile | ForEach-Object {
  $line = $_.Trim()
  if ($line -eq "" -or $line.StartsWith("#") -or $line -notmatch "=") { return }
  $name, $value = $line.Split("=", 2)
  [Environment]::SetEnvironmentVariable($name.Trim(), $value.Trim().Trim('"').Trim("'"), "Process")
}

if (-not $env:DATABASE_URL) {
  Write-Error "DATABASE_URL is not set in .env"
}

# Local HTTP cannot use Secure cookies.
$env:COOKIE_SECURE = "false"
if (-not $env:PORT) { $env:PORT = "8082" }

$go = Get-Command go -ErrorAction SilentlyContinue
$localGo = Join-Path $root ".tools\go\bin\go.exe"
if ($go) {
  $goExe = $go.Source
} elseif (Test-Path $localGo) {
  $goExe = $localGo
} else {
  Write-Error "Go is not installed. Install it from https://go.dev/dl/ or re-run this script after placing go.exe in .tools\go\bin\"
}

$busy = Get-NetTCPConnection -LocalPort $env:PORT -State Listen -ErrorAction SilentlyContinue
if ($busy) {
  Write-Host "Port $($env:PORT) is in use (PID $($busy.OwningProcess)). Stop that process first, or set PORT in .env."
  exit 1
}

Write-Host "Starting TEDx memory challenge at http://localhost:$($env:PORT)"
& $goExe run .
