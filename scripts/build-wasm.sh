#!/usr/bin/env bash
# ブラウザ版をビルドする。
#
# wasm_exec.js は Go のバージョンと厳密に対応する起動用グルーコードなので、
# リポジトリにコミットせず必ず使用中の GOROOT からコピーする。
# ビルド済み .wasm も同様にコミットしない（.gitignore 参照）。
set -euo pipefail

cd "$(dirname "$0")/.."

cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/wasm_exec.js
GOOS=js GOARCH=wasm go build -o web/main.wasm ./cmd/tetro-wasm

echo "built: web/main.wasm ($(du -h web/main.wasm | cut -f1))"
