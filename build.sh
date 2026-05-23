#!/usr/bin/env bash
# Build GlideFTP for Linux, Windows, and/or as an AppImage.
# Usage:
#   ./build.sh              → Linux binary + Windows exe + AppImage
#   ./build.sh linux        → Linux binary only
#   ./build.sh windows      → Windows exe only (requires mingw-w64-gcc)
#   ./build.sh appimage     → AppImage only (builds Linux binary first if missing)
#
# Install mingw on Arch Linux:
#   sudo pacman -S mingw-w64-gcc

set -e

TARGET="${1:-all}"

# Download a file; prefers curl, falls back to wget
_download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -sSL --progress-bar -o "$dest" "$url"
  else
    wget -q --show-progress -O "$dest" "$url"
  fi
}

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

build_appimage() {
  echo "→ Building AppImage…"

  # Build Linux binary first if missing
  if [ ! -f build/bin/linux/GlideFTP ]; then
    build_linux
  fi

  mkdir -p tools

  local LD="tools/linuxdeploy-x86_64.AppImage"
  local LD_AI="tools/linuxdeploy-plugin-appimage-x86_64.AppImage"

  if [ ! -f "$LD" ]; then
    echo "   ↓ Downloading linuxdeploy…"
    _download \
      "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage" \
      "$LD"
    chmod +x "$LD"
  fi

  if [ ! -f "$LD_AI" ]; then
    echo "   ↓ Downloading linuxdeploy-plugin-appimage…"
    _download \
      "https://github.com/linuxdeploy/linuxdeploy-plugin-appimage/releases/download/continuous/linuxdeploy-plugin-appimage-x86_64.AppImage" \
      "$LD_AI"
    chmod +x "$LD_AI"
  fi

  # Create a temporary AppDir
  local APPDIR
  APPDIR="$(mktemp -d)/GlideFTP.AppDir"
  mkdir -p "$APPDIR/usr/bin"

  cp build/bin/linux/GlideFTP "$APPDIR/usr/bin/"
  # linuxdeploy requires a standard icon resolution; resize to 256x256
  magick build/appicon.png -resize 256x256 "$APPDIR/glideftp.png"

  cat > "$APPDIR/glideftp.desktop" << 'DESKTOP'
[Desktop Entry]
Type=Application
Name=GlideFTP
Exec=GlideFTP
Icon=glideftp
Categories=Network;FileTransfer;
Comment=FTP/SFTP desktop client
DESKTOP

  # linuxdeploy discovers plugins (linuxdeploy-plugin-appimage) via PATH
  export PATH="$(pwd)/tools:$PATH"
  # Destination path for the generated AppImage
  export OUTPUT="$(pwd)/build/bin/linux/GlideFTP-x86_64.AppImage"
  # Run AppImages without FUSE (extract-and-run) — avoids FUSE requirement
  export APPIMAGE_EXTRACT_AND_RUN=1
  # Disable stripping — linuxdeploy's bundled strip is too old for .relr.dyn sections
  # used by modern Arch Linux libraries (binutils >= 2.38)
  export NO_STRIP=1

  "$LD" \
    --appdir "$APPDIR" \
    --desktop-file "$APPDIR/glideftp.desktop" \
    --icon-file "$APPDIR/glideftp.png" \
    --output appimage

  echo "   ✓ build/bin/linux/GlideFTP-x86_64.AppImage"
}

case "$TARGET" in
  linux)    build_linux ;;
  windows)  build_windows ;;
  appimage) build_appimage ;;
  all)      build_linux; build_windows; build_appimage ;;
  *)        echo "Usage: $0 [linux|windows|appimage|all]"; exit 1 ;;
esac

echo ""
echo "Build done:"
ls -lh build/bin/linux/GlideFTP 2>/dev/null || true
ls -lh build/bin/linux/GlideFTP-x86_64.AppImage 2>/dev/null || true
ls -lh build/bin/windows/GlideFTP.exe 2>/dev/null || true
