$ErrorActionPreference = "Stop"

$engineArgs = @($args)
if ($engineArgs.Count -eq 0) {
    $engineArgs = @("--help")
}

if (Get-Command archscope-engine -ErrorAction SilentlyContinue) {
    & archscope-engine @engineArgs
    exit $LASTEXITCODE
}

Set-Location "$PSScriptRoot/../apps/engine-native"
go run ./cmd/archscope-engine @engineArgs
exit $LASTEXITCODE
