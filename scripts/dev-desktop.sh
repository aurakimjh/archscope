#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../apps/engine-native/cmd/archscope-app/frontend"
npm install
npm run dev
