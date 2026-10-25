#!/usr/bin/env bash
set -euo pipefail
[[ "$(go env GOVERSION)" == go1.27.3 ]]
[[ "$GOTOOLCHAIN" == local ]]
[[ "$(node --version)" == v24.16.0 ]]
[[ "$(npm --version)" == 11.17.0 ]]
wails version | grep -Fx v2.17.0
staticcheck -version | grep -Fx 'staticcheck 2026.2.2 (0.8.2)'
[[ "$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v2)" == v2.17.0 ]]
[[ "$(go list -m -f '{{.Version}}' modernc.org/sqlite)" == v1.61.0 ]]
[[ "$(go list -m -f '{{.Version}}' golang.org/x/crypto)" == v0.53.0 ]]
[[ "$(node -p 'JSON.parse(require("node:fs").readFileSync("frontend/node_modules/@playwright/test/package.json","utf8")).version')" == 1.64.1 ]]
pkg-config --modversion gtk+-3.0 webkit2gtk-4.1 libsoup-3.0
printf 'Pinned tools and headers: PASS\n'

# BF04: existing native decoder, explicitly authorized; never auto-install.
pkg-config --modversion libwebp
