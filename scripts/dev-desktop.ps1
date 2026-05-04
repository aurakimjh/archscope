$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../apps/frontend"
npm install
npm run dev
