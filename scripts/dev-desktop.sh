#!/usr/bin/env bash
set -euo pipefail

# [한글] Go/Wails 데스크톱 앱 개발 서버 실행 스크립트.
# frontend 만 직접 띄우면 Wails IPC 를 확인할 수 없으므로 app root 에서 task dev 를
# 실행합니다. node_modules 가 없을 때만 npm install 을 먼저 수행합니다.
cd "$(dirname "$0")/../apps/engine-native/cmd/archscope-app"
if [ ! -d frontend/node_modules ]; then
  (cd frontend && npm install)
fi
exec task dev
