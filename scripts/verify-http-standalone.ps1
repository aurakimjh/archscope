param(
 [Parameter(Mandatory=$true)][string]$ArchScopeExe,
 [Parameter(Mandatory=$true)][string]$OutputDirectory,
 [switch]$RequireBodies
)
$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) { throw 'Native Windows is required.' }
$taskExePath = (Resolve-Path -LiteralPath $ArchScopeExe).Path
$taskArguments = @((Join-Path $PSScriptRoot 'verify-http-standalone.mjs'), '--exe', $taskExePath, '--out', [IO.Path]::GetFullPath($OutputDirectory))
if ($RequireBodies) { $taskArguments += '--require-bodies' }
& node @taskArguments
if ($LASTEXITCODE -ne 0) { throw 'Native HTTP fixture harness failed; inspect its private local output.' }
