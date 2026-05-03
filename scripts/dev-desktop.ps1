$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../apps/desktop"
npm install
npm run dev
