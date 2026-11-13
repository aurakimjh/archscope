param([string]$Manifest, [string]$Ready)
$lockStream = [System.IO.File]::Open($Manifest, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
try {
  [System.IO.File]::WriteAllText($Ready, 'ready')
  Start-Sleep -Seconds 3
} finally {
  $lockStream.Dispose()
}
