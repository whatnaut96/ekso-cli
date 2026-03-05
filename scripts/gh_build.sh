#!/usr/bin/env bash
set -euo pipefail

APP_NAME="ekso"
PKG="./cmd/ekso/"
OUT_DIR="dist"

GOOS="${GOOS:?GOOS not set}"
GOARCH="${GOARCH:?GOARCH not set}"
EXT="${EXT:-}"

mkdir -p "$OUT_DIR"

OUT_NAME="${APP_NAME}_${GOOS}_${GOARCH}${EXT}"

echo "Building $OUT_NAME"
GOOS="$GOOS" GOARCH="$GOARCH" \
  go build -trimpath -ldflags="-s -w" \
  -o "${OUT_DIR}/${OUT_NAME}" \
  "${PKG}"

