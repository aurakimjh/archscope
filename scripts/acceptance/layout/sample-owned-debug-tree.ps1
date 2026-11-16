param(
    [Parameter(Mandatory)][int]$RootProcessId,
    [Parameter(Mandatory)][string]$ExpectedExecutable,
    [Parameter(Mandatory)][ValidateRange(1,65535)][int]$DebugPort,
    [string]$ExpectedStartTime = ''
)
$ErrorActionPreference = 'Stop'
# Reuse the existing executable/start-time and descendant ownership checks.
$sample = & (Join-Path $PSScriptRoot 'sample-owned-process-tree.ps1') -RootProcessId $RootProcessId -ExpectedExecutable $ExpectedExecutable -ExpectedStartTime $ExpectedStartTime | ConvertFrom-Json
$listeners = @(Get-NetTCPConnection -State Listen -LocalAddress '127.0.0.1' -LocalPort $DebugPort -ErrorAction Stop)
if ($listeners.Count -ne 1) { throw 'Expected one owned loopback debug listener' }
$entry = Get-CimInstance Win32_Process -Filter "ProcessId=$($listeners[0].OwningProcess)"
if (-not $entry) { throw 'Debug listener process disappeared' }
$identity = "$($entry.ProcessId)/$($entry.CreationDate.ToUniversalTime().ToString('o'))"
if (-not (@($sample.members | Where-Object identity -EQ $identity).Count -eq 1)) { throw 'Debug listener is not an exact owned descendant' }
$sample | Add-Member -NotePropertyName debugPortOwned -NotePropertyValue $true
$sample | Add-Member -NotePropertyName debugListenerIdentity -NotePropertyValue $identity
$sample | ConvertTo-Json -Depth 6 -Compress
