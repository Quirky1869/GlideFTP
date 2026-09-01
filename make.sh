#!/usr/bin/env bash
# Build GlideFTP for Linux, Windows, AppImage, and native packages.
# Usage:
#   ./make.sh                       → Linux binary + Windows exe + Arch AppImage + Debian AppImage + .deb + .rpm
#   ./make.sh linux          1.7.5  → Linux binary only          → build/bin/linux/GlideFTP-v1.7.5
#   ./make.sh windows        1.7.5  → Windows exe only (requires mingw-w64-gcc) → build/bin/windows/GlideFTP-v1.7.5.exe
#   ./make.sh appimage       1.7.5  → Arch AppImage + Debian AppImage
#   ./make.sh appimage-arch  1.7.5  → Arch AppImage only    → build/bin/linux/GlideFTP-Arch-x86_64-v1.7.5.AppImage
#   ./make.sh appimage-debian 1.7.5 → Debian/Ubuntu AppImage (Docker) → build/bin/linux/GlideFTP-Debian-x86_64-v1.7.5.AppImage
#   ./make.sh deb    1.7.5          → .deb binary package (Docker - Ubuntu 22.04)
#   ./make.sh rpm    1.7.5          → .rpm binary package (Docker - Fedora 40)
#   ./make.sh packages 1.7.5        → .deb + .rpm
#
# Version argument (required for every target, including linux/windows/appimage*):
#   Pass as 2nd argument: ./make.sh deb 1.7.5
#   Or create a VERSION file at project root: echo '1.7.5' > VERSION
#   All final artefact filenames embed the version, e.g. GlideFTP-v1.7.5,
#   GlideFTP-v1.7.5.exe, GlideFTP-Arch-x86_64-v1.7.5.AppImage - same convention
#   already used by the .deb/.rpm output.
#
# Install mingw on Arch Linux:
#   sudo pacman -S mingw-w64-gcc
# Install Docker or Podman for Debian AppImage / .deb / .rpm:
#   sudo pacman -S docker  (then: sudo systemctl enable --now docker)
#   or: sudo pacman -S podman

set -e

TARGET="${1:-all}"
VERSION_ARG="${2:-}"

get_version() {
  if [ -n "$VERSION_ARG" ]; then
    echo "$VERSION_ARG"
    return
  fi
  if [ -f "VERSION" ]; then
    cat VERSION
    return
  fi
  echo "ERROR: Version required for this target." >&2
  echo "       Pass it as second argument: ./make.sh $TARGET 1.7.5" >&2
  echo "       Or create a VERSION file:   echo '1.7.5' > VERSION" >&2
  exit 1
}

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
  local version="$1"
  echo "→ Building Linux (amd64)…"
  mkdir -p build/bin/linux
  wails build -tags webkit2_41 2>&1
  mv build/bin/GlideFTP "build/bin/linux/GlideFTP-v${version}"
  echo "   ✓ build/bin/linux/GlideFTP-v${version}"
}

build_windows() {
  local version="$1"
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
      echo "WARNING: magick (ImageMagick) not found - icon.ico not updated."
      echo "         Install with: sudo pacman -S imagemagick"
    fi
  fi

  mkdir -p build/bin/windows
  CC=x86_64-w64-mingw32-gcc \
  CGO_ENABLED=1 \
  GOOS=windows \
  wails build -platform windows/amd64 2>&1
  mv build/bin/GlideFTP.exe "build/bin/windows/GlideFTP-v${version}.exe"
  echo "   ✓ build/bin/windows/GlideFTP-v${version}.exe"
}

build_appimage() {
  local version="$1"
  echo "→ Building AppImage…"

  # Build Linux binary first if missing
  if [ ! -f "build/bin/linux/GlideFTP-v${version}" ]; then
    build_linux "$version"
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

  cp "build/bin/linux/GlideFTP-v${version}" "$APPDIR/usr/bin/GlideFTP"
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
  export OUTPUT="$(pwd)/build/bin/linux/GlideFTP-Arch-x86_64-v${version}.AppImage"
  # Run AppImages without FUSE (extract-and-run) - avoids FUSE requirement
  export APPIMAGE_EXTRACT_AND_RUN=1
  # Disable stripping - linuxdeploy's bundled strip is too old for .relr.dyn sections
  # used by modern Arch Linux libraries (binutils >= 2.38)
  export NO_STRIP=1

  "$LD" \
    --appdir "$APPDIR" \
    --desktop-file "$APPDIR/glideftp.desktop" \
    --icon-file "$APPDIR/glideftp.png" \
    --output appimage

  echo "   ✓ build/bin/linux/GlideFTP-Arch-x86_64-v${version}.AppImage"
}

build_appimage_debian() {
  local version="$1"
  echo "→ Building Debian/Ubuntu AppImage (via Docker - Ubuntu 22.04)…"

  local DOCKER_CMD
  if command -v docker &>/dev/null; then
    DOCKER_CMD="docker"
  elif command -v podman &>/dev/null; then
    DOCKER_CMD="podman"
  else
    echo "ERROR: docker or podman not found."
    echo "       Install with: sudo pacman -S docker  (then: sudo systemctl enable --now docker)"
    echo "       or:           sudo pacman -S podman"
    exit 1
  fi

  echo "   ↑ Building container image (first run may take several minutes)…"
  "$DOCKER_CMD" build -t glideftp-debian-builder -f docker/Dockerfile.appimage .

  mkdir -p build/bin/linux

  # Mount host Go module cache to speed up repeated builds
  local HOST_GOPATH
  HOST_GOPATH="$(go env GOPATH 2>/dev/null || echo "$HOME/go")"

  "$DOCKER_CMD" run --rm \
    -v "$(pwd):/src" \
    -v "${HOST_GOPATH}/pkg/mod:/root/go/pkg/mod" \
    -v "$HOME/.npm:/root/.npm" \
    -e HOST_UID="$(id -u)" \
    -e HOST_GID="$(id -g)" \
    glideftp-debian-builder \
    bash /src/docker/build-appimage.sh "$version"

  echo "   ✓ build/bin/linux/GlideFTP-Debian-x86_64-v${version}.AppImage"
}

build_deb() {
  local version="$1"
  echo "→ Building .deb package (via Docker - Ubuntu 22.04)…"

  if [ ! -f "build/bin/linux/GlideFTP-v${version}" ]; then
    echo "ERROR: build/bin/linux/GlideFTP-v${version} not found. Run ./make.sh linux ${version} first."
    exit 1
  fi

  local DOCKER_CMD
  if command -v docker &>/dev/null; then
    DOCKER_CMD="docker"
  elif command -v podman &>/dev/null; then
    DOCKER_CMD="podman"
  else
    echo "ERROR: docker or podman not found."
    echo "       Install: sudo pacman -S docker && sudo systemctl enable --now docker"
    exit 1
  fi

  "$DOCKER_CMD" build -t glideftp-debian-builder -f docker/Dockerfile.appimage .

  "$DOCKER_CMD" run --rm \
    -v "$(pwd):/src" \
    -e HOST_UID="$(id -u)" \
    -e HOST_GID="$(id -g)" \
    glideftp-debian-builder \
    bash /src/docker/build-deb.sh "$version"

  echo "   ✓ build/bin/linux/GlideFTP-Linux-v${version}.deb"
}

build_rpm() {
  local version="$1"
  echo "→ Building .rpm package (via Docker - Fedora 40)…"

  if [ ! -f "build/bin/linux/GlideFTP-v${version}" ]; then
    echo "ERROR: build/bin/linux/GlideFTP-v${version} not found. Run ./make.sh linux ${version} first."
    exit 1
  fi

  local DOCKER_CMD
  if command -v docker &>/dev/null; then
    DOCKER_CMD="docker"
  elif command -v podman &>/dev/null; then
    DOCKER_CMD="podman"
  else
    echo "ERROR: docker or podman not found."
    exit 1
  fi

  "$DOCKER_CMD" build -t glideftp-rpm-builder -f docker/Dockerfile.rpm .

  "$DOCKER_CMD" run --rm \
    -v "$(pwd):/src" \
    -e HOST_UID="$(id -u)" \
    -e HOST_GID="$(id -g)" \
    glideftp-rpm-builder \
    bash /src/docker/build-rpm.sh "$version"

  echo "   ✓ build/bin/linux/GlideFTP-Linux-v${version}.rpm"
}

show_help() {
  cat <<EOF
Usage: $0 <target> [version]

All targets require a version (e.g. 1.7.6) - every final artefact filename
embeds it, e.g. GlideFTP-v1.7.6, GlideFTP-v1.7.6.exe, GlideFTP-Arch-x86_64-v1.7.6.AppImage,
GlideFTP-Linux-v1.7.6.deb:

  linux            <version>   Build Linux binary              → build/bin/linux/GlideFTP-v<version>
  windows          <version>   Build Windows exe               → build/bin/windows/GlideFTP-v<version>.exe
  appimage         <version>   Arch AppImage + Debian AppImage → build/bin/linux/GlideFTP-{Arch,Debian}-x86_64-v<version>.AppImage
  appimage-arch    <version>   Arch AppImage only
  appimage-debian  <version>   Debian/Ubuntu AppImage (Docker)
  deb              <version>   .deb binary package (Docker - Ubuntu 22.04)  → build/bin/linux/GlideFTP-Linux-v<version>.deb
  rpm              <version>   .rpm binary package (Docker - Fedora 40)     → build/bin/linux/GlideFTP-Linux-v<version>.rpm
  packages         <version>   .deb + .rpm
  all              <version>   Everything above

Version can also be read from a VERSION file at the project root:
  echo '1.7.6' > VERSION
  ./make.sh all          (no version argument needed)
  ./make.sh linux         (no version argument needed)

Examples:
  ./make.sh linux 1.7.6
  ./make.sh deb 1.7.6
  ./make.sh all 1.7.6
  echo '1.7.6' > VERSION && ./make.sh all

Requirements:
  mingw-w64-gcc   for Windows cross-compile  (sudo pacman -S mingw-w64-gcc)
  docker/podman   for appimage-debian, deb, rpm
EOF
  exit 0
}

case "$TARGET" in
  -h|--help|help)   show_help ;;
  linux)            VER=$(get_version) || exit 1; build_linux "$VER" ;;
  windows)          VER=$(get_version) || exit 1; build_windows "$VER" ;;
  appimage)         VER=$(get_version) || exit 1; build_appimage "$VER"; build_appimage_debian "$VER" ;;
  appimage-arch)    VER=$(get_version) || exit 1; build_appimage "$VER" ;;
  appimage-debian)  VER=$(get_version) || exit 1; build_appimage_debian "$VER" ;;
  deb)              VER=$(get_version) || exit 1; build_deb "$VER" ;;
  rpm)              VER=$(get_version) || exit 1; build_rpm "$VER" ;;
  packages)         VER=$(get_version) || exit 1; build_deb "$VER"; build_rpm "$VER" ;;
  all)              VER=$(get_version) || exit 1; build_linux "$VER"; build_windows "$VER"; build_appimage "$VER"; build_appimage_debian "$VER"; build_deb "$VER"; build_rpm "$VER" ;;
  *)                echo "Unknown target '$TARGET'. Run ./make.sh -h for help."; exit 1 ;;
esac

echo ""
echo "Build done:"
ls -lh "build/bin/linux/GlideFTP-v${VER}" 2>/dev/null || true
ls -lh "build/bin/linux/GlideFTP-Arch-x86_64-v${VER}.AppImage" 2>/dev/null || true
ls -lh "build/bin/linux/GlideFTP-Debian-x86_64-v${VER}.AppImage" 2>/dev/null || true
ls -lh "build/bin/windows/GlideFTP-v${VER}.exe" 2>/dev/null || true
ls -lh "build/bin/linux/GlideFTP-Linux-v${VER}.deb" 2>/dev/null || true
ls -lh "build/bin/linux/GlideFTP-Linux-v${VER}.rpm" 2>/dev/null || true
