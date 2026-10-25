#!/usr/bin/env bash
set -euo pipefail
# A fresh checkout needs an embed input to compile the real Wails binding generator.
# It is replaced by Vite before any product gate/build; it is not a runtime artifact.
if [[ ! -f frontend/dist/index.html ]]; then
 mkdir -p frontend/dist
 cp frontend/index.html frontend/dist/index.html
fi
wails generate module -tags webkit2_41
