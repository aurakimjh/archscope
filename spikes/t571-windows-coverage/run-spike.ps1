<#
.SYNOPSIS
  T-571 Windows proof-of-capability spike orchestrator.
  docs/ko/SYSTEM_HTTP_CAPTURE.md §10.4.

.DESCRIPTION
  Runs the whole spike end to end on a Windows host:
    1. verifies it is running elevated (kernel ETW + WFP capture require it),
    2. builds the six spike binaries with the repo Go toolchain,
    3. measures a baseline CPU sample (for CAP-5 overhead),
    4. for each candidate probe: starts the probe, drives controlled traffic
       with loadgen, and collects one obs_*.json,
    5. runs judge to produce results\report.md and results\report.json,
       including the two appendix-A ledger rows.

  Nothing is installed on the box beyond Go: the probes wrap logman, tracerpt,
  netsh, expand, typeperf, and Get-NetTCPConnection, all of which ship with
  Windows.

.PARAMETER Window
  Capture window per candidate (seconds). Default 30.

.PARAMETER Tps
  Target total transactions/sec for loadgen. Default 500 (the §10.4.2 load).

.PARAMETER Workers
  Number of control worker processes. Default 5 (the last one is the CAP-4
  proxy-bypass control).

.PARAMETER Target
  Optional remote listener host:port. If omitted, loadgen runs its own
  loopback listener. NOTE: loopback attribution differs from real-NIC
  attribution for some candidates — see README "Loopback caveat".

.EXAMPLE
  # From an elevated PowerShell:
  .\run-spike.ps1 -Window 30 -Tps 500 -Workers 5
#>
[CmdletBinding()]
param(
  [int]$Window = 30,
  [int]$Tps = 500,
  [int]$Workers = 5,
  [string]$Target = "",
  [string]$ResultsDir = "results",
  # Which candidate probes to run this pass. Accepts either an array
  # (-Candidates wfp,tcpowner) or a quoted string (-Candidates "wfp,tcpowner").
  # e.g. -Candidates wfp,tcpowner  skips the heavy ETW re-capture.
  [string[]]$Candidates = @("etw", "wfp", "tcpowner")
)

$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $here

function Assert-Elevated {
  $id = [Security.Principal.WindowsIdentity]::GetCurrent()
  $p = New-Object Security.Principal.WindowsPrincipal($id)
  if (-not $p.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "이 스파이크는 관리자 권한 PowerShell에서 실행해야 합니다 (커널 ETW/WFP 캡처 필요)."
  }
}

Write-Host "== T-571 Windows proof-of-capability spike ==" -ForegroundColor Cyan
Assert-Elevated

# --- build ---
# This spike is a standalone Go module (its own go.mod, no external deps) and
# is intentionally NOT listed in the repo root go.work. Disable workspace mode
# so `go build` uses this module directly instead of failing with
# "not one of the workspace modules". The repo go.work is left untouched.
$env:GOWORK = "off"

$bin = Join-Path $here "bin"
New-Item -ItemType Directory -Force -Path $bin | Out-Null
New-Item -ItemType Directory -Force -Path $ResultsDir | Out-Null

foreach ($c in @("loadgen","etwprobe","wfpprobe","tcpownerprobe","bypassclient","judge")) {
  Write-Host "building $c..." -ForegroundColor DarkGray
  & go build -o (Join-Path $bin "$c.exe") "./cmd/$c"
  if ($LASTEXITCODE -ne 0) { Write-Error "build failed: $c" }
}

# --- baseline CPU (idle) for CAP-5 ---
Write-Host "baseline CPU 샘플 측정 중 (약 5초)..." -ForegroundColor DarkGray
$baselineRaw = & typeperf "\Processor(_Total)\% Processor Time" -si 1 -sc 5 2>$null
$vals = @()
foreach ($line in $baselineRaw) {
  if ($line -match '","([\d\.]+)"\s*$') { $vals += [double]$Matches[1] }
}
$baseline = if ($vals.Count -gt 0) { ($vals | Measure-Object -Average).Average } else { 0 }
Write-Host ("baseline _Total % Processor Time = {0:N1}%" -f $baseline)

$loadgen = Join-Path $bin "loadgen.exe"

# From here on, a single misbehaving native command (netsh noise, a probe that
# times out) must NOT abort the whole run — we still want judge to produce a
# report from whatever observations completed. Build errors already aborted
# above under Stop; switch to Continue for the capture/judge phase.
$ErrorActionPreference = "Continue"

# Clean up any capture session/collection left running by an interrupted prior
# run so it does not conflict with this pass.
Write-Host "이전 실행 잔여 캡처 세션 정리..." -ForegroundColor DarkGray
& logman stop archscope-t571-etw -ets 2>$null | Out-Null
& netsh wfp capture stop 2>$null | Out-Null
& netsh wfp set options netevents=off 2>$null | Out-Null

# Runs one probe while loadgen drives traffic. The probe owns the capture
# window; loadgen runs slightly shorter so its connections exist for the whole
# probe window. The wait is generous and NON-fatal: some probes (notably WFP,
# whose `netsh wfp capture stop` collects a diagnostic cab) take well over a
# minute to finalize. A timeout warns and moves on rather than aborting.
function Invoke-Candidate {
  param([string]$ProbeExe, [string]$ObsPath, [string]$GtName, [int]$MaxWaitSec = 120, [string[]]$ExtraArgs = @())

  if (Test-Path $ObsPath) { Remove-Item $ObsPath -Force }
  $probeWorkArgs = @("-window", "$($Window)s", "-out", $ObsPath)
  if ($ExtraArgs) { $probeWorkArgs += $ExtraArgs }
  Write-Host "starting probe: $ProbeExe $($probeWorkArgs -join ' ')" -ForegroundColor Yellow
  $probe = Start-Process -FilePath $ProbeExe -ArgumentList $probeWorkArgs -PassThru -NoNewWindow

  Start-Sleep -Milliseconds 800  # let the capture session come up first

  # Each pass uses fresh ephemeral ports, so it writes its OWN ground truth
  # (ground_truth_<GtName>.json) that judge pairs with this obs. A copy is also
  # written to ground_truth.json for the report summary / fallback.
  $gtPath = Join-Path $ResultsDir "ground_truth_$GtName.json"
  $lgArgs = @("-role","parent","-tps","$Tps","-workers","$Workers",
              "-duration","$([math]::Max(1,$Window-2))s",
              "-out", $gtPath)
  if ($Target -ne "") { $lgArgs += @("-listen", $Target) }
  Write-Host "driving load: $loadgen $($lgArgs -join ' ')" -ForegroundColor Yellow
  & $loadgen @lgArgs
  if (Test-Path $gtPath) { Copy-Item $gtPath (Join-Path $ResultsDir "ground_truth.json") -Force }

  $waitMs = [math]::Max(30, $Window + $MaxWaitSec) * 1000
  Write-Host "probe 종료 대기 (최대 $([math]::Round($waitMs/1000))s; WFP는 cab 생성에 시간이 걸립니다)..." -ForegroundColor DarkGray
  if (-not $probe.WaitForExit($waitMs)) {
    Write-Warning "probe가 제한 시간 내 종료되지 않았습니다: $ProbeExe. obs 파일이 없으면 judge가 '미측정'으로 기록합니다."
  }
}

# Each candidate is a separate capture session run in its own pass, so their
# CPU overhead is measured independently. loadgen re-runs each pass and
# overwrites ground_truth.json — every pass drives identical load, so the
# last-written ground truth is consistent for judge.
$want = @($Candidates) | ForEach-Object { $_ -split ',' } | ForEach-Object { $_.Trim().ToLower() } | Where-Object { $_ }
if ($want -contains "etw") {
  Invoke-Candidate -ProbeExe (Join-Path $bin "etwprobe.exe")      -ObsPath (Join-Path $ResultsDir "obs_etw.json")      -GtName "etw"      -MaxWaitSec 120
}
if ($want -contains "wfp") {
  Invoke-Candidate -ProbeExe (Join-Path $bin "wfpprobe.exe")      -ObsPath (Join-Path $ResultsDir "obs_wfp.json")      -GtName "wfp"      -MaxWaitSec 300
}
if ($want -contains "tcpowner") {
  Invoke-Candidate -ProbeExe (Join-Path $bin "tcpownerprobe.exe") -ObsPath (Join-Path $ResultsDir "obs_tcpowner.json") -GtName "tcpowner" -MaxWaitSec 60
}

# --- judge ---
Write-Host "판정(judge) 실행..." -ForegroundColor Cyan
& (Join-Path $bin "judge.exe") -dir $ResultsDir -baseline-cpu $baseline `
    -out (Join-Path $ResultsDir "report.json") -md (Join-Path $ResultsDir "report.md")

Write-Host ""
Write-Host "완료. 결과:" -ForegroundColor Green
Write-Host "  $ResultsDir\report.md   (사람이 읽는 판정표 + 부록 A 행)"
Write-Host "  $ResultsDir\report.json (기계 판독용)"
Write-Host ""
Write-Host "다음: report.md 의 부록 A 행을 docs/ko/SYSTEM_HTTP_CAPTURE.md 에 반영하고 T-571 게이트를 갱신하세요."
