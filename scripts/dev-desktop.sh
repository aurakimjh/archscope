#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../apps/frontend"
npm install
npm run dev
