# [한글] dev-desktop.ps1 - Go/Wails 데스크톱 앱 개발 서버 실행.
#
# 예전처럼 frontend 만 npm dev 로 띄우면 Wails IPC/네이티브 파일 선택 기능을
# 확인할 수 없습니다. 현재 개발 경로는 archscope-app 의 `task dev` 입니다.
$ErrorActionPreference = "Stop"
Set-Location "$PSScriptRoot/../apps/engine-native/cmd/archscope-app"
if (-not (Test-Path "frontend/node_modules")) {
    Set-Location "frontend"
    npm install
    Set-Location ".."
}
task dev
