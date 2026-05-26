# ==========================================================================
# build-desktop.ps1 — One-command build for the active Go/Wails desktop app.
#
# Produces the current Wails desktop package from:
#   apps\engine-native\cmd\archscope-app
#
# Prerequisites (build machine only):
#   - Go
#   - Node 18+ with npm
#   - Wails v3 CLI (wails3)
#   - Task (go-task)
#
# Usage:
#   scripts\build-desktop.ps1             # production package
#   scripts\build-desktop.ps1 -DirOnly    # build app binary only
#
# [한글] 현재 제품 라인은 Python/Electron 이 아니라 Go/Wails 입니다.
# 이 스크립트는 예전 engines\python, apps\frontend, apps\desktop 경로를 더 이상
# 사용하지 않고 Wails Taskfile 을 호출합니다. -SkipEngine 은 예전 호환용 no-op 입니다.
# ==========================================================================

param(
    [switch]$SkipEngine,
    [switch]$DirOnly
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$appDir = Join-Path $repoRoot "apps\engine-native\cmd\archscope-app"

if ($SkipEngine) {
    Write-Host "Warning: -SkipEngine is obsolete for the Go/Wails app and will be ignored." -ForegroundColor Yellow
}

if (-not (Test-Path -LiteralPath $appDir -PathType Container)) {
    throw "Wails app directory not found: $appDir"
}

if (-not (Get-Command task -ErrorAction SilentlyContinue)) {
    throw "go-task command 'task' is required. Install Task before running this script."
}

Set-Location $appDir

if ($DirOnly) {
    Write-Host "--- Building Wails app binary only ---" -ForegroundColor Cyan
    task build
} else {
    Write-Host "--- Packaging Wails desktop app ---" -ForegroundColor Cyan
    task package
}

$exitCode = $LASTEXITCODE
if ($exitCode -ne 0) {
    exit $exitCode
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Build complete!" -ForegroundColor Green
Write-Host "  Output: $appDir\bin\" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
