#!/bin/sh
# Compress release binaries with UPX.
# UPX doesn't support some platform/arch combos (e.g. macOS arm64) — skip silently.
upx --best --lzma "$1" 2>/dev/null || true
