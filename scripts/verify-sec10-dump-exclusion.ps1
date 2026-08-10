<#
.SYNOPSIS
    SEC-10 crash-dump exclusion evidence harness.

.DESCRIPTION
    A policy read that returns the expected value is not evidence. This script
    crashes a process that is genuinely holding plaintext in memory and then
    inspects what Windows actually wrote, which is the acceptance condition
    recorded in SYSTEM_HTTP_CAPTURE 2.1.1 and RD-135.

    It fails when:
      - the preflight does not verify on this machine (nothing to prove);
      - the probe child does not crash, or crashes before holding the canary
        (an absent artifact would then say nothing);
      - a new crash artifact attributable to the probe appears; or
      - any new artifact contains the canary.

    Artifact absence is the pass condition. The canary scan is the backstop for
    the case where an artifact appears anyway.

.NOTES
    Run from the repository root. Produces sanitized JSON evidence; it never
    writes a captured payload, a dump path outside the runner, or key material.
#>
[CmdletBinding()]
param(
    [string]$EnginePath = "apps/engine-native",
    [string]$EvidenceOut = "sec10-dump-exclusion-evidence.json"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

# Must match ForcedCrashCanary in internal/capture/dumppolicy/forcedcrash_windows_test.go.
$Canary = "ARCHSCOPE-SEC10-CANARY-9f2c41d8"

$DumpRoots = @(
    (Join-Path $env:LOCALAPPDATA "CrashDumps"),
    (Join-Path $env:LOCALAPPDATA "Microsoft\Windows\WER\ReportQueue"),
    (Join-Path $env:LOCALAPPDATA "Microsoft\Windows\WER\ReportArchive"),
    (Join-Path $env:ProgramData "Microsoft\Windows\WER\ReportQueue"),
    (Join-Path $env:ProgramData "Microsoft\Windows\WER\ReportArchive")
)

function Get-ArtifactSnapshot {
    $items = @()
    foreach ($root in $DumpRoots) {
        if (-not (Test-Path -LiteralPath $root)) { continue }
        $items += Get-ChildItem -LiteralPath $root -Recurse -File -ErrorAction SilentlyContinue |
            ForEach-Object { $_.FullName }
    }
    return , @($items)
}

Write-Host "== SEC-10 forced-crash dump-exclusion probe =="

$before = Get-ArtifactSnapshot
Write-Host ("Pre-existing crash artifacts: {0}" -f $before.Count)

$probeFailed = $false
$probeOutput = ""
Push-Location $EnginePath
try {
    $env:ARCHSCOPE_SEC10_FORCED_CRASH = "1"
    # The probe asserts the preflight verifies before it crashes anything, so a
    # non-zero exit here means either the machine is not in the protected state
    # or the crash did not happen as required. Both are failures.
    $probeOutput = & go test ./internal/capture/dumppolicy -run TestSEC10ForcedCrashProbe -v -count=1 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) { $probeFailed = $true }
} finally {
    Remove-Item Env:\ARCHSCOPE_SEC10_FORCED_CRASH -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host $probeOutput

if ($probeFailed) {
    throw "The forced-crash probe did not establish the required condition. See the output above."
}

# WER writes asynchronously; give it room before concluding nothing appeared.
Start-Sleep -Seconds 15

$after = Get-ArtifactSnapshot
$new = @($after | Where-Object { $before -notcontains $_ })
Write-Host ("New crash artifacts after the probe: {0}" -f $new.Count)

$canaryHits = @()
foreach ($path in $new) {
    try {
        $bytes = [System.IO.File]::ReadAllBytes($path)
        # Latin1 maps bytes 1:1 to chars, so a UTF-8/ASCII canary survives the
        # decode without a codepage guess corrupting the match.
        $text = [System.Text.Encoding]::Latin1.GetString($bytes)
        if ($text.Contains($Canary)) { $canaryHits += (Split-Path -Leaf $path) }
    } catch {
        Write-Warning ("Could not read {0} for canary scanning: {1}" -f (Split-Path -Leaf $path), $_.Exception.Message)
    }
}

$verdict = "pass"
if ($new.Count -gt 0) { $verdict = "fail" }
if ($canaryHits.Count -gt 0) { $verdict = "fail" }

# Evidence records counts and leaf names only. Full paths would carry the
# runner's user name, which the archived artifact must not contain.
$evidence = [ordered]@{
    schemaVersion            = 1
    check                    = "sec10_crash_dump_exclusion"
    verdict                  = $verdict
    preflightVerified        = $true
    probeCrashedWithPlaintext = $true
    preExistingArtifactCount = $before.Count
    newArtifactCount         = $new.Count
    newArtifactNames         = @($new | ForEach-Object { Split-Path -Leaf $_ })
    canaryMatchCount         = $canaryHits.Count
    canaryMatchNames         = $canaryHits
    scannedRootCount         = ($DumpRoots | Where-Object { Test-Path -LiteralPath $_ }).Count
    notDefended              = @(
        "active debugger reading process memory",
        "administrator enabling LocalDumps mid-session",
        "kernel or hypervisor memory acquisition",
        "plaintext reaching the pagefile or a hibernation image"
    )
}
$evidence | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $EvidenceOut -Encoding UTF8
Write-Host ("Evidence written to {0}" -f $EvidenceOut)

if ($verdict -ne "pass") {
    if ($canaryHits.Count -gt 0) {
        throw "SEC-10 FAILED: captured plaintext reached a crash artifact ($($canaryHits -join ', '))."
    }
    throw "SEC-10 FAILED: the crash produced $($new.Count) artifact(s) for a process that should have been excluded."
}

Write-Host "SEC-10 PASS: the forced crash produced no artifact and no canary reached disk."
