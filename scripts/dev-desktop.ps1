$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../apps/engine-native/cmd/archscope-app/frontend"
npm install
npm run dev
