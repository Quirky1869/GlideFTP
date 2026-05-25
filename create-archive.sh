#!/usr/bin/env bash
set -euo pipefail

SCRIPT_NAME=$(basename "$0")

usage() {
  cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS] <version>

Create distribution archives for GlideFTP binaries.

Arguments:
  version           Version number in X.Y.Z format (e.g. 1.7.0)

Options:
  -p, --platform    Platform to archive: windows | linux | appimage | appimage-arch | appimage-debian | all  (default: all)
  -t, --type        Archive type:        gz | tar | all                    (default: all)
  -h, --help        Show this help message and exit

Archive types:
  gz   → compressed  (.tar.gz)
  tar  → uncompressed (.tar)
  all  → both gz and tar

Output files (example with version 1.7.0):
  GlideFTP-Windows-v1.7.0.tar.gz
  GlideFTP-Linux-v1.7.0.tar.gz                     ← includes README.md (dependency notice)
  GlideFTP-Linux-Arch-AppImage-v1.7.0.tar.gz        ← Arch / modern distros (GLIBC 2.38+)
  GlideFTP-Linux-Debian-AppImage-v1.7.0.tar.gz      ← Ubuntu 22.04+ / Debian (GLIBC 2.35+)
  GlideFTP-Windows-v1.7.0.tar
  GlideFTP-Linux-v1.7.0.tar                         ← includes README.md (dependency notice)
  GlideFTP-Linux-Arch-AppImage-v1.7.0.tar
  GlideFTP-Linux-Debian-AppImage-v1.7.0.tar

Examples:
  $SCRIPT_NAME 1.7.0                               # all 8 archives
  $SCRIPT_NAME -p windows 1.7.0                   # Windows archives only (gz + tar)
  $SCRIPT_NAME -p linux -t gz 1.7.0               # Linux binary .tar.gz only
  $SCRIPT_NAME -p appimage -t gz 1.7.0            # both AppImage variants .tar.gz only
  $SCRIPT_NAME -p appimage-arch -t gz 1.7.0       # Arch AppImage .tar.gz only
  $SCRIPT_NAME -p appimage-debian -t tar 1.7.0    # Debian AppImage .tar only
  $SCRIPT_NAME -p windows -t tar 1.7.0            # Windows .tar only
  $SCRIPT_NAME --platform all --type gz 2.0.0
EOF
}

# ── Defaults ──────────────────────────────────────────────────────────────────
PLATFORM="all"
TYPE="all"

# ── Parse options ─────────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    -p|--platform)
      if [[ $# -lt 2 ]]; then
        echo "Error: --platform requires a value (windows | linux | all)." >&2
        exit 1
      fi
      PLATFORM="$2"
      shift 2
      ;;
    -t|--type)
      if [[ $# -lt 2 ]]; then
        echo "Error: --type requires a value (gz | tar | all)." >&2
        exit 1
      fi
      TYPE="$2"
      shift 2
      ;;
    -*)
      echo "Error: unknown option '$1'." >&2
      echo "Run '$SCRIPT_NAME --help' for usage." >&2
      exit 1
      ;;
    *)
      break
      ;;
  esac
done

# ── Version argument ──────────────────────────────────────────────────────────
if [[ $# -eq 0 ]]; then
  echo "Error: version argument is required." >&2
  echo "  The version must be provided in X.Y.Z format (3 numbers separated by dots)." >&2
  echo "  Example: $SCRIPT_NAME 1.7.0" >&2
  echo "" >&2
  echo "Run '$SCRIPT_NAME --help' for full usage." >&2
  exit 1
fi

VERSION="$1"

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: invalid version '$VERSION'." >&2
  echo "  The version must contain exactly 3 numbers separated by dots (X.Y.Z)." >&2
  echo "  Examples: 1.7.0   2.0.1   10.3.2" >&2
  exit 1
fi

# ── Validate options ──────────────────────────────────────────────────────────
case "$PLATFORM" in
  windows|linux|appimage|appimage-arch|appimage-debian|all) ;;
  *)
    echo "Error: --platform value '$PLATFORM' is not valid." >&2
    echo "  Accepted values: windows | linux | appimage | appimage-arch | appimage-debian | all" >&2
    exit 1
    ;;
esac

case "$TYPE" in
  gz|tar|all) ;;
  *)
    echo "Error: --type value '$TYPE' is not valid." >&2
    echo "  Accepted values: gz | tar | all" >&2
    exit 1
    ;;
esac

# ── Archive functions ─────────────────────────────────────────────────────────

# Write the Linux README.md into the given directory
_write_linux_readme() {
  cat > "$1/README.md" << 'EOF'
# GlideFTP — Linux Prerequisites

Before running GlideFTP, please install the WebKit2GTK library required by the application:

- **Ubuntu / Debian** : `sudo apt install libwebkit2gtk-4.1-0`
- **Fedora** : `sudo dnf install webkit2gtk4.1`
- **Arch Linux** : `sudo pacman -S webkit2gtk-4.1`

Once installed, make the binary executable and run it:

```
chmod +x GlideFTP
./GlideFTP
```
EOF
}

make_windows_gz() {
  local out="GlideFTP-Windows-v${VERSION}.tar.gz"
  echo "→ $out"
  tar -czvf "$out" ./build/bin/windows/GlideFTP.exe
}

make_linux_gz() {
  local out="GlideFTP-Linux-v${VERSION}.tar.gz"
  local staging
  staging="$(mktemp -d)"
  cp build/bin/linux/GlideFTP "$staging/"
  _write_linux_readme "$staging"
  echo "→ $out"
  tar -czvf "$out" -C "$staging" GlideFTP README.md
  rm -rf "$staging"
}

make_windows_tar() {
  local out="GlideFTP-Windows-v${VERSION}.tar"
  echo "→ $out"
  tar -cvf "$out" ./build/bin/windows/GlideFTP.exe
}

make_linux_tar() {
  local out="GlideFTP-Linux-v${VERSION}.tar"
  local staging
  staging="$(mktemp -d)"
  cp build/bin/linux/GlideFTP "$staging/"
  _write_linux_readme "$staging"
  echo "→ $out"
  tar -cvf "$out" -C "$staging" GlideFTP README.md
  rm -rf "$staging"
}

make_appimage_arch_gz() {
  local out="GlideFTP-Linux-Arch-AppImage-v${VERSION}.tar.gz"
  echo "→ $out"
  tar -czvf "$out" -C build/bin/linux GlideFTP-Arch-x86_64.AppImage
}

make_appimage_arch_tar() {
  local out="GlideFTP-Linux-Arch-AppImage-v${VERSION}.tar"
  echo "→ $out"
  tar -cvf "$out" -C build/bin/linux GlideFTP-Arch-x86_64.AppImage
}

make_appimage_debian_gz() {
  local out="GlideFTP-Linux-Debian-AppImage-v${VERSION}.tar.gz"
  echo "→ $out"
  tar -czvf "$out" -C build/bin/linux GlideFTP-Debian-x86_64.AppImage
}

make_appimage_debian_tar() {
  local out="GlideFTP-Linux-Debian-AppImage-v${VERSION}.tar"
  echo "→ $out"
  tar -cvf "$out" -C build/bin/linux GlideFTP-Debian-x86_64.AppImage
}

# ── Run ───────────────────────────────────────────────────────────────────────
echo "GlideFTP archive builder — version v${VERSION}"
echo "Platform: $PLATFORM  |  Type: $TYPE"
echo "──────────────────────────────────────────────"

[[ "$PLATFORM" == "windows"  || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_windows_gz

[[ "$PLATFORM" == "linux"    || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_linux_gz

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-arch" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_appimage_arch_gz

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-debian" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_appimage_debian_gz

[[ "$PLATFORM" == "windows"  || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_windows_tar

[[ "$PLATFORM" == "linux"    || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_linux_tar

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-arch" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_appimage_arch_tar

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-debian" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_appimage_debian_tar

echo "──────────────────────────────────────────────"
echo "Done."
