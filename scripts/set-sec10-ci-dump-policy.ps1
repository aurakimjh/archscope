<#
.SYNOPSIS
    Isolates and restores Windows crash-reporting policy for the SEC-10 CI probe.

.DESCRIPTION
    GitHub-hosted Windows images may define a machine-wide WER LocalDumps key.
    That is a valid reason for the product preflight to fail closed, but it
    prevents CI from exercising the supported no-LocalDumps state.

    Prepare renames the exact policy keys that affect the probe instead of
    exporting and reconstructing them. Renaming preserves values, subkeys, and
    registry ACLs. Restore removes state created by the probe and renames the
    original keys back. The script refuses to run outside GitHub Actions.
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("Prepare", "Restore")]
    [string]$Action,

    [Parameter(Mandatory = $true)]
    [string]$StatePath
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

if ($env:GITHUB_ACTIONS -ne "true") {
    throw "This registry-isolation helper is CI-only and requires GITHUB_ACTIONS=true."
}

$targets = @(
    [ordered]@{
        id       = "machine_local_dumps"
        path     = "HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\LocalDumps"
        parent   = "HKLM:\SOFTWARE\Microsoft\Windows\Windows Error Reporting"
        leaf     = "LocalDumps"
    },
    [ordered]@{
        id       = "user_excluded_applications"
        path     = "HKCU:\SOFTWARE\Microsoft\Windows\Windows Error Reporting\ExcludedApplications"
        parent   = "HKCU:\SOFTWARE\Microsoft\Windows\Windows Error Reporting"
        leaf     = "ExcludedApplications"
    }
)

function Read-State {
    if (-not (Test-Path -LiteralPath $StatePath -PathType Leaf)) {
        throw "SEC-10 CI policy state does not exist: $StatePath"
    }
    return Get-Content -LiteralPath $StatePath -Raw | ConvertFrom-Json
}

if ($Action -eq "Prepare") {
    if (Test-Path -LiteralPath $StatePath) {
        throw "Refusing to overwrite existing SEC-10 CI policy state: $StatePath"
    }

    $stateParent = Split-Path -Parent $StatePath
    if ($stateParent) {
        New-Item -ItemType Directory -Path $stateParent -Force | Out-Null
    }

    $suffix = "ArchScopeSec10Backup-{0}" -f ([Guid]::NewGuid().ToString("N"))
    $entries = @()
    foreach ($target in $targets) {
        $existed = Test-Path -LiteralPath $target.path
        $backupLeaf = "{0}.{1}" -f $target.leaf, $suffix
        $backupPath = Join-Path $target.parent $backupLeaf
        if (Test-Path -LiteralPath $backupPath) {
            throw "SEC-10 backup registry key already exists: $backupPath"
        }
        $entries += [ordered]@{
            id         = $target.id
            path       = $target.path
            parent     = $target.parent
            leaf       = $target.leaf
            existed    = $existed
            backupLeaf = $backupLeaf
            backupPath = $backupPath
        }
    }

    # Persist the recovery map before changing the registry so the always()
    # workflow step can recover from a partial preparation failure.
    [ordered]@{
        schemaVersion = 1
        entries       = $entries
    } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $StatePath -Encoding UTF8

    foreach ($entry in $entries) {
        if ($entry.existed) {
            Rename-Item -LiteralPath $entry.path -NewName $entry.backupLeaf
        }
        if (Test-Path -LiteralPath $entry.path) {
            throw "SEC-10 policy isolation failed for $($entry.id)."
        }
    }

    Write-Host "SEC-10 CI policy prepared; original keys are preserved under temporary names."
    exit 0
}

$state = Read-State
foreach ($entry in $state.entries) {
    # The forced-crash probe can create a fresh per-user exclusion key. Remove
    # only the exact active key before putting the preserved original back.
    if (Test-Path -LiteralPath $entry.path) {
        Remove-Item -LiteralPath $entry.path -Recurse -Force
    }

    if ($entry.existed) {
        if (-not (Test-Path -LiteralPath $entry.backupPath)) {
            throw "SEC-10 backup registry key is missing for $($entry.id)."
        }
        Rename-Item -LiteralPath $entry.backupPath -NewName $entry.leaf
        if (-not (Test-Path -LiteralPath $entry.path)) {
            throw "SEC-10 policy restoration failed for $($entry.id)."
        }
    } elseif (Test-Path -LiteralPath $entry.backupPath) {
        throw "Unexpected SEC-10 backup registry key exists for $($entry.id)."
    }
}

Write-Host "SEC-10 CI policy restored."
