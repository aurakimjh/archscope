<#
.SYNOPSIS
Temporarily isolates LocalDumps for an explicitly authorized local body test.
.DESCRIPTION
Run elevated as the same Windows user as the app. Hold preserves the machine
key by renaming it and snapshots only the named test executables' user WER
exclusions. It restores in finally on release.json or after MaximumMinutes.
Restore can recover the persisted state after an interrupted worker. State
contains local configuration and must not be committed or published.
#>
[CmdletBinding()]
param(
    [ValidateSet('Hold', 'Restore')][string]$Action = 'Hold',
    [Parameter(Mandatory)][string]$StateDirectory,
    [Parameter(Mandatory)][string]$OwnerSid,
    [switch]$AllowLocalPolicyChange,
    [ValidateRange(1, 60)][int]$MaximumMinutes = 30
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'windows-policy-registry.ps1')
if (-not $AllowLocalPolicyChange) { throw 'Explicit -AllowLocalPolicyChange is required.' }
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if ($identity.User.Value -ne $OwnerSid) { throw 'Run as the same Windows user as the unelevated app.' }
if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'An elevated administrator token is required.'
}
$machinePath = 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps'
$userPath = 'HKCU:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\ExcludedApplications'
$statePath = Join-Path $StateDirectory 'policy-state.json'
$readyPath = Join-Path $StateDirectory 'ready.json'
$releasePath = Join-Path $StateDirectory 'release.json'
$resultPath = Join-Path $StateDirectory 'restored.json'

function Write-Json([string]$Path, $Value) {
    $Value | ConvertTo-Json -Depth 40 | Set-Content -LiteralPath $Path -Encoding utf8
}
function Read-KeyTree([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    $key = Get-Item -LiteralPath $Path
    try {
        $values = @($key.GetValueNames() | Sort-Object | ForEach-Object {
            [ordered]@{ name = $_; kind = $key.GetValueKind($_).ToString();
                value = $key.GetValue($_, $null, [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames) }
        })
        $children = @($key.GetSubKeyNames() | Sort-Object | ForEach-Object {
            [ordered]@{ name = $_; tree = Read-KeyTree (Join-Path $Path $_) }
        })
        return [ordered]@{ acl = (Get-Acl -LiteralPath $Path).Sddl; values = $values; children = $children }
    } finally { $key.Close() }
}
function Get-TreeHash($Tree) {
    $json = ConvertTo-Json -InputObject $Tree -Depth 40 -Compress
    return [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData([Text.Encoding]::UTF8.GetBytes($json)))
}
function Restore-Policy($State) {
    $expectedParent = 'HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\'
    if (-not $State.backupPath.StartsWith($expectedParent, [StringComparison]::OrdinalIgnoreCase) -or
        (Split-Path $State.backupPath -Leaf) -notmatch '^LocalDumps\.ArchScopeBodyTest-[0-9a-f]{32}$') {
        throw 'Recovery backup path is outside the expected policy namespace.'
    }
    $machineError = $null
    try {
        if ($State.machineExisted -and (Test-Path -LiteralPath $State.backupPath)) {
            if (Test-Path -LiteralPath $machinePath) {
                throw 'An unexpected active LocalDumps key appeared; preserved backup remains available for recovery.'
            }
            Rename-WindowsPolicyKey -Path $State.backupPath -NewName 'LocalDumps'
        }
        if ((Get-TreeHash (Read-KeyTree $machinePath)) -ne $State.machineHash) {
            throw 'Original LocalDumps values/subkeys/ACL do not match the saved state.'
        }
    } catch { $machineError = $_ }
    # Always undo this test's per-executable user changes, even if a machine
    # policy conflict needs separate recovery. Never overwrite other values.
    if (Test-Path -LiteralPath $userPath) {
        $key = [Microsoft.Win32.Registry]::CurrentUser.OpenSubKey('SOFTWARE\Microsoft\Windows\Windows Error Reporting\ExcludedApplications', $true)
        try {
            foreach ($entry in $State.userValues) {
                if ($entry.existed) {
                    $kind = [Microsoft.Win32.RegistryValueKind]::$($entry.kind)
                    $value = $entry.value
                    if ($entry.kind -eq 'Binary') { $value = [byte[]]$value }
                    if ($entry.kind -eq 'MultiString') { $value = [string[]]$value }
                    if ($entry.kind -eq 'DWord') { $value = [int]$value }
                    if ($entry.kind -eq 'QWord') { $value = [long]$value }
                    $key.SetValue($entry.name, $value, $kind)
                } else { $key.DeleteValue($entry.name, $false) }
            }
            $empty = $key.ValueCount -eq 0 -and $key.SubKeyCount -eq 0
        } finally { $key.Close() }
        if (-not $State.userKeyExisted -and $empty) { Remove-Item -LiteralPath $userPath }
    }
    $userMatches = (Get-TreeHash (Read-KeyTree $userPath)) -eq $State.userHash
    if (-not $userMatches) { throw 'User WER exclusion tree differs from the saved values/subkeys/ACL.' }
    if ($null -ne $machineError) { throw $machineError }
    Write-Json $resultPath ([ordered]@{ policyRestored = $true; machineValuesSubkeysAclMatch = $true;
        userValuesSubkeysAclMatch = $true; backupKeyRemoved = -not (Test-Path -LiteralPath $State.backupPath) })
}

if ($Action -eq 'Restore') {
    $state = Get-Content -LiteralPath $statePath -Raw | ConvertFrom-Json
    if ($state.ownerSid -ne $OwnerSid) { throw 'Recovery state belongs to a different Windows user.' }
    Restore-Policy $state
    exit 0
}
New-Item -ItemType Directory -Path $StateDirectory -Force | Out-Null
if (Test-Path -LiteralPath $statePath) { throw 'Refusing to overwrite an existing recovery state.' }
$backupLeaf = 'LocalDumps.ArchScopeBodyTest-' + [guid]::NewGuid().ToString('N')
$userTree = Read-KeyTree $userPath
$userValues = @()
foreach ($name in @('archscope.exe', 'archscope-app.test.exe', 'proxy.test.exe')) {
    $found = @()
    if ($null -ne $userTree) { $found = @($userTree.values | Where-Object { $_.name -ieq $name }) }
    if ($found.Count) {
        $userValues += [ordered]@{ name = $found[0].name; existed = $true; kind = $found[0].kind; value = $found[0].value }
    } else { $userValues += [ordered]@{ name = $name; existed = $false; kind = ''; value = $null } }
}
$machineTree = Read-KeyTree $machinePath
$state = [ordered]@{ ownerSid = $OwnerSid; machineExisted = (Test-Path -LiteralPath $machinePath);
    backupPath = Join-Path (Split-Path $machinePath) $backupLeaf;
    machineHash = Get-TreeHash $machineTree; machineTree = $machineTree;
    userKeyExisted = (Test-Path -LiteralPath $userPath); userHash = Get-TreeHash $userTree; userTree = $userTree; userValues = $userValues }
Write-Json $statePath $state
try {
    if ($state.machineExisted) { Rename-WindowsPolicyKey -Path $machinePath -NewName $backupLeaf }
    if (Test-Path -LiteralPath $machinePath) { throw 'LocalDumps isolation did not complete.' }
    Write-Json $readyPath ([ordered]@{ prepared = $true; automaticRestoreMinutes = $MaximumMinutes })
    $deadline = (Get-Date).AddMinutes($MaximumMinutes)
    while (-not (Test-Path -LiteralPath $releasePath) -and (Get-Date) -lt $deadline) { Start-Sleep -Seconds 1 }
} catch {
    Write-Json (Join-Path $StateDirectory 'error.json') @{ message = $_.Exception.Message }
    throw
} finally {
    try { Restore-Policy $state }
    catch {
        Write-Json (Join-Path $StateDirectory 'restore-error.json') @{ message = $_.Exception.Message }
        throw
    }
}
