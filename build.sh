#!/usr/bin/env bash
# Build GlideFTP for Linux and/or Windows.
# Usage:
#   ./build.sh           → builds both platforms
#   ./build.sh linux     → Linux only
#   ./build.sh windows   → Windows only (requires mingw-w64-gcc)
#
# Install mingw on Arch Linux:
#   sudo pacman -S mingw-w64-gcc

set -e

TARGET="${1:-all}"

build_linux() {
  echo "→ Building Linux (amd64)…"
  mkdir -p build/bin/linux
  wails build -tags webkit2_41 2>&1
  mv build/bin/GlideFTP build/bin/linux/GlideFTP
  echo "   ✓ build/bin/linux/GlideFTP"
}

build_windows() {
  echo "→ Building Windows (amd64)…"
  if ! command -v x86_64-w64-mingw32-gcc &>/dev/null; then
    echo "ERROR: x86_64-w64-mingw32-gcc not found."
    echo "       Install with: sudo pacman -S mingw-w64-gcc"
    exit 1
  fi

  # Regenerate icon.ico from appicon.png if the PNG is newer or the ico is missing
  if [ build/appicon.png -nt build/windows/icon.ico ] || [ ! -f build/windows/icon.ico ]; then
    if command -v magick &>/dev/null; then
      echo "   ↻ Regenerating build/windows/icon.ico from appicon.png…"
      magick build/appicon.png \
        -define icon:auto-resize="256,128,64,48,32,16" \
        build/windows/icon.ico
    else
      echo "WARNING: magick (ImageMagick) not found — icon.ico not updated."
      echo "         Install with: sudo pacman -S imagemagick"
    fi
  fi

  mkdir -p build/bin/windows
  CC=x86_64-w64-mingw32-gcc \
  CGO_ENABLED=1 \
  GOOS=windows \
  wails build -platform windows/amd64 2>&1
  mv build/bin/GlideFTP.exe build/bin/windows/GlideFTP.exe
  echo "   ✓ build/bin/windows/GlideFTP.exe"
}

case "$TARGET" in
  linux)   build_linux ;;
  windows) build_windows ;;
  all)     build_linux; build_windows ;;
  *)       echo "Usage: $0 [linux|windows|all]"; exit 1 ;;
esac

echo ""
echo "Build done:"
ls -lh build/bin/linux/GlideFTP 2>/dev/null || true
ls -lh build/bin/windows/GlideFTP.exe 2>/dev/null || true
