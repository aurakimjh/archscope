param([string]$Run = 'amd64-baseline')
$ErrorActionPreference = 'Stop'
$root = [IO.Path]::GetFullPath("C:\workspace\archscope\.tmp\live-grid-20261114\$Run")
if (-not $root.StartsWith('C:\workspace\archscope\.tmp\live-grid-20261114\')) { throw 'Invalid owned run' }
$session = Get-Content -LiteralPath (Join-Path $root '.private-session.json') -Raw | ConvertFrom-Json
$target = [IO.Path]::GetFullPath((Join-Path $session.storePath 'annotations.json'))
if (-not $target.StartsWith($root + '\.archscope-captures\')) { throw 'Outside owned capture store' }
$watcher = [IO.FileSystemWatcher]::new([IO.Path]::GetDirectoryName($target), '.annotations-*')
$watcher.EnableRaisingEvents = $true
$reader = [IO.File]::Open($target, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::ReadWrite)
try {
 Set-Content -LiteralPath (Join-Path $root 'reader-ready.txt') -Value 'ready'
 $event = $watcher.WaitForChanged([IO.WatcherChangeTypes]::Created, 60000)
 if ($event.TimedOut) { throw 'No annotation write observed' }
 Start-Sleep -Milliseconds 250
 'Observed staging creation; released transient reader after 250ms'
} finally { $reader.Dispose(); $watcher.Dispose() }
