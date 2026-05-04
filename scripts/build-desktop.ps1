# ==========================================================================
# build-desktop.ps1 — One-command build for the ArchScope desktop app.
#
# Produces a distributable Electron package with the Python engine bundled
# so end-users need ZERO runtime dependencies (no Python, no Node).
#
# Prerequisites (build machine only):
#   - Python 3.9+ with pip
#   - Node 18+ with npm
#   - PyInstaller  (pip install pyinstaller)
#
# Usage:
#   scripts\build-desktop.ps1                   # full build
#   scripts\build-desktop.ps1 -SkipEngine       # reuse existing engine binary
#   scripts\build-desktop.ps1 -DirOnly          # produce unpacked dir (faster)
# ==========================================================================

param(
    [switch]$SkipEngine,
    [switch]$DirOnly
)

$ErrorActionPreference = "Stop"

$repoRoot   = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$engineDir  = Join-Path $repoRoot "engines\python"
$frontendDir = Join-Path $repoRoot "apps\frontend"
$desktopDir = Join-Path $repoRoot "apps\desktop"

# --------------------------------------------------------------------------
# Step 1: Bundle the Python engine with PyInstaller
# --------------------------------------------------------------------------
if (-not $SkipEngine) {
    Write-Host ""
    Write-Host "--- Step 1/3: Building Python engine (PyInstaller) ---" -ForegroundColor Cyan
    Write-Host ""
    Set-Location $engineDir

    pip install -e ".[dev]" --quiet 2>$null
    pip install pyinstaller --quiet

    if (Test-Path "build") { Remove-Item -Recurse -Force "build" }
    if (Test-Path "dist")  { Remove-Item -Recurse -Force "dist" }

    pyinstaller archscope-engine.spec --noconfirm --clean

    Write-Host ""
    Write-Host "  OK Engine built: $engineDir\dist\archscope-engine\" -ForegroundColor Green
} else {
    Write-Host ""
    Write-Host "--- Step 1/3: Skipping engine build (--SkipEngine) ---" -ForegroundColor Yellow
    if (-not (Test-Path (Join-Path $engineDir "dist\archscope-engine"))) {
        Write-Host "  Warning: dist\archscope-engine\ does not exist." -ForegroundColor Yellow
    }
}

# --------------------------------------------------------------------------
# Step 2: Build the React frontend
# --------------------------------------------------------------------------
Write-Host ""
Write-Host "--- Step 2/3: Building React frontend ---" -ForegroundColor Cyan
Write-Host ""
Set-Location $frontendDir
npm install --no-audit --no-fund
npm run build
Write-Host ""
Write-Host "  OK Frontend built: $frontendDir\dist\" -ForegroundColor Green

# --------------------------------------------------------------------------
# Step 3: Compile Electron + package
# --------------------------------------------------------------------------
Write-Host ""
Write-Host "--- Step 3/3: Packaging Electron app ---" -ForegroundColor Cyan
Write-Host ""
Set-Location $desktopDir
npm install --no-audit --no-fund

npx tsc -p tsconfig.electron.json

if ($DirOnly) {
    npx electron-builder --dir
} else {
    npx electron-builder
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "  Build complete!" -ForegroundColor Green
Write-Host "  Output: $desktopDir\release\" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host ""
