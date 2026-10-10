#!/bin/sh
set -eu
MACOCR_PROJECT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$MACOCR_PROJECT_DIR"
swift build -c release --arch arm64
MACOCR_BIN_DIR="$(swift build -c release --arch arm64 --show-bin-path)"
MACOCR_APP_DIR="$MACOCR_PROJECT_DIR/dist/MacOCR.app"
mkdir -p "$MACOCR_APP_DIR/Contents/MacOS"
cp "$MACOCR_BIN_DIR/macocr" "$MACOCR_APP_DIR/Contents/MacOS/macocr"
cp Support/Info.plist "$MACOCR_APP_DIR/Contents/Info.plist"
codesign --force --sign - --identifier com.macocr.app "$MACOCR_APP_DIR"
printf 'Built %s\n' "$MACOCR_APP_DIR"
