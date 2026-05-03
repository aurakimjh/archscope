$ErrorActionPreference = "Stop"

$repoRoot = "$PSScriptRoot\.."
$engineDir = "$repoRoot\engines\python"
$desktopDir = "$repoRoot\apps\desktop"

Write-Host "=== Step 1: Build Python engine ===" -ForegroundColor Cyan

Set-Location $engineDir

$python = Get-Command python -ErrorAction SilentlyContinue
if (-not $python) { $python = Get-Command python3 -ErrorAction SilentlyContinue }
if (-not $python) {
    Write-Error "Python not found. Install Python 3.9+ and re-run."
    exit 1
}

Write-Host "Creating clean venv..."
& $python.Source -m venv .venv-build --clear
.\.venv-build\Scripts\python.exe -m pip install --upgrade pip -q
.\.venv-build\Scripts\pip.exe install pyinstaller -q
.\.venv-build\Scripts\pip.exe install -e "." -q

Write-Host "Running PyInstaller..."
.\.venv-build\Scripts\python.exe -m PyInstaller archscope-engine.spec `
    --distpath dist `
    --workpath build `
    --noconfirm

if (-not (Test-Path "$engineDir\dist\archscope-engine\archscope-engine.exe")) {
    Write-Error "Engine build failed: archscope-engine.exe not found."
    exit 1
}

$engineMB = [math]::Round(
    (Get-ChildItem "$engineDir\dist\archscope-engine" -Recurse | Measure-Object -Property Length -Sum).Sum / 1MB, 1
)
Write-Host "Engine built: $engineMB MB" -ForegroundColor Green

Write-Host ""
Write-Host "=== Step 2: Build Electron app ===" -ForegroundColor Cyan

Set-Location $desktopDir
npm ci
npm run dist:win

Write-Host ""
Write-Host "=== Build complete ===" -ForegroundColor Green
Write-Host "Artifacts: $desktopDir\release\" -ForegroundColor Green
Get-ChildItem "$desktopDir\release" -File | Where-Object { $_.Extension -in '.exe','.zip' } |
    ForEach-Object { Write-Host "  $($_.Name)  ($([math]::Round($_.Length/1MB, 1)) MB)" }
