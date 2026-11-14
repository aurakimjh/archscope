param([string]$Run='amd64-fixed',[int]$HoldMs=1600)
$ErrorActionPreference='Stop'
$root=[IO.Path]::GetFullPath("C:\workspace\archscope\.tmp\grid-boundaries-20261114\$Run")
if (!$root.StartsWith('C:\workspace\archscope\.tmp\grid-boundaries-20261114\')) {throw 'Outside owned test root'}
$session=Get-Content -LiteralPath (Join-Path $root '.private-session.json') -Raw | ConvertFrom-Json
$target=[IO.Path]::GetFullPath((Join-Path $session.storePath 'annotations.json'))
if (!$target.StartsWith($root+'\.archscope-captures\')) {throw 'Outside owned store'}
$watcher=[IO.FileSystemWatcher]::new([IO.Path]::GetDirectoryName($target),'.annotations-*');$watcher.EnableRaisingEvents=$true
$reader=[IO.File]::Open($target,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
try { Set-Content -LiteralPath (Join-Path $root 'reader-ready.txt') -Value 'ready'; $event=$watcher.WaitForChanged([IO.WatcherChangeTypes]::Created,60000);if($event.TimedOut){throw 'No annotation staging observed'};Start-Sleep -Milliseconds $HoldMs;"Released owned annotation reader after $HoldMs ms" } finally {$reader.Dispose();$watcher.Dispose()}
