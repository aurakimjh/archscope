$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../apps/engine-native/cmd/archscope-profiler-app/frontend"
npm install
npm run dev
