$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../engines/python"
python -m archscope_engine.cli --help
