param(
    [Parameter(Mandatory = $true)]
    [string]$ExePath,

    [int]$StartupSeconds = 15,

    [int]$ShutdownSeconds = 8
)

$ErrorActionPreference = "Stop"

if ($StartupSeconds -lt 1) {
    throw "StartupSeconds must be at least 1."
}

if (-not [System.IO.Path]::IsPathRooted($ExePath)) {
    $resolved = Resolve-Path -Path $ExePath -ErrorAction Stop
    $ExePath = $resolved.Path
}

if (-not (Test-Path -LiteralPath $ExePath -PathType Leaf)) {
    throw "Executable not found: $ExePath"
}

$item = Get-Item -LiteralPath $ExePath
if ($item.Length -lt 1048576) {
    throw "Executable is unexpectedly small: $($item.Length) bytes"
}

$workDir = Split-Path -Parent $ExePath
Write-Host "Starting GUI smoke target: $ExePath"
Write-Host "Working directory: $workDir"
Write-Host "Binary size: $($item.Length) bytes"

$process = Start-Process -FilePath $ExePath -WorkingDirectory $workDir -PassThru

try {
    $deadline = (Get-Date).AddSeconds($StartupSeconds)
    while ((Get-Date) -lt $deadline) {
        Start-Sleep -Seconds 1
        $process.Refresh()
        if ($process.HasExited) {
            throw "GUI process exited during startup smoke. ExitCode=$($process.ExitCode)"
        }
    }

    Write-Host "GUI process stayed alive for $StartupSeconds seconds. PID=$($process.Id)"
}
finally {
    if ($null -ne $process) {
        $process.Refresh()
        if (-not $process.HasExited) {
            Write-Host "Requesting graceful shutdown."
            [void]$process.CloseMainWindow()
            $process.WaitForExit($ShutdownSeconds * 1000) | Out-Null
            $process.Refresh()
        }
        if (-not $process.HasExited) {
            Write-Host "Forcing process shutdown."
            Stop-Process -Id $process.Id -Force
        }
    }
}

