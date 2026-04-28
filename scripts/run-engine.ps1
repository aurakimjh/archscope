$ErrorActionPreference = "Stop"

$engineArgs = @($args)
if ($engineArgs.Count -eq 0) {
    $engineArgs = @("--help")
}

if (Get-Command archscope-engine -ErrorAction SilentlyContinue) {
    & archscope-engine @engineArgs
    exit $LASTEXITCODE
}

Set-Location "$PSScriptRoot/../engines/python"
$pythonCommand = Get-Command python -ErrorAction SilentlyContinue
if (-not $pythonCommand) {
    $pythonCommand = Get-Command python3 -ErrorAction SilentlyContinue
}

if ($pythonCommand) {
    & $pythonCommand.Source -c "import typer, rich" 2>$null
    if ($LASTEXITCODE -eq 0) {
        & $pythonCommand.Source -m archscope_engine.cli @engineArgs
        exit $LASTEXITCODE
    }
}

Write-Error "archscope-engine is not installed. Run 'cd engines/python && pip install -e .' first."
exit 127
