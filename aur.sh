#!/usr/bin/env bash
# Publish a GlideFTP release to the AUR (glideftp-bin).
# Automates steps B.6 to B.9 of memo/workflow-aur.md, once the GitHub release is PUBLISHED
# (not draft - a draft's download links return 404):
#   1. download the published GlideFTP-Linux-v<version>.tar.gz and compute its real sha256
#   2. update packaging/PKGBUILD (pkgver + sha256sums_x86_64, pkgrel reset to 1 on a new version)
#   3. optional local install test (--test → makepkg -si)
#   4. clone the AUR repo if missing (or pull it), copy PKGBUILD, regenerate .SRCINFO
#   5. show the diff, ask for confirmation, then commit "Release v<version>" + push to the AUR
#
# Usage:
#   ./aur.sh 1.7.9              → full run, asks before committing/pushing to the AUR
#   ./aur.sh --test 1.7.9       → same + installs the package locally with makepkg -si first
#   ./aur.sh --pkgrel 2 1.7.9   → re-publish the same version with pkgrel=2 (PKGBUILD-only fix)
#   ./aur.sh -y 1.7.9           → no confirmation prompt
#   Version can also come from a VERSION file at project root (like make.sh).
#   AUR clone location: $AUR_DIR, default ~/les-git-clones/aur/glideftp-bin
#
# Only the AUR repo is committed/pushed. packaging/PKGBUILD in this repo is updated but
# never committed - the script prints the git commands to run for it.

set -euo pipefail

SCRIPT_NAME=$(basename "$0")
REPO="Quirky1869/GlideFTP"
AUR_URL="ssh://aur@aur.archlinux.org/glideftp-bin.git"
AUR_DIR="${AUR_DIR:-$HOME/les-git-clones/aur/glideftp-bin}"
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
PKGBUILD="$PROJECT_DIR/packaging/PKGBUILD"

usage() {
  cat <<EOF
Usage: $SCRIPT_NAME [OPTIONS] <version>

Publish glideftp-bin to the AUR from the published GitHub release.

Arguments:
  version           Version number in X.Y.Z format (e.g. 1.7.9), or a VERSION file

Options:
  -t, --test        Install the package locally with makepkg -si before publishing
  -r, --pkgrel N    pkgrel to use (default: 1 for a new version, unchanged otherwise)
  -y, --yes         Do not ask for confirmation before committing/pushing to the AUR
  -h, --help        Show this help message and exit

Environment:
  AUR_DIR           AUR clone location (default: ~/les-git-clones/aur/glideftp-bin)
EOF
}

# ── Parse options ─────────────────────────────────────────────────────────────
TEST=0
YES=0
PKGREL=""
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    -t|--test) TEST=1; shift ;;
    -y|--yes)  YES=1; shift ;;
    -r|--pkgrel)
      if [[ -z "${2:-}" || ! "$2" =~ ^[0-9]+$ ]]; then
        echo "Error: --pkgrel requires a number." >&2
        exit 1
      fi
      PKGREL="$2"; shift 2 ;;
    -*)
      echo "Error: unknown option '$1'." >&2
      echo "Run '$SCRIPT_NAME --help' for usage." >&2
      exit 1 ;;
    *) VERSION="$1"; shift ;;
  esac
done

if [ -z "$VERSION" ] && [ -f "$PROJECT_DIR/VERSION" ]; then
  VERSION=$(tr -d '[:space:]' < "$PROJECT_DIR/VERSION")
fi

if [ -z "$VERSION" ]; then
  echo "Error: version argument is required." >&2
  echo "  Example: $SCRIPT_NAME 1.7.9" >&2
  exit 1
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: invalid version '$VERSION' (expected X.Y.Z)." >&2
  exit 1
fi

for cmd in curl sha256sum tar git makepkg; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Error: '$cmd' not found." >&2
    exit 1
  fi
done

if [ ! -f "$PKGBUILD" ]; then
  echo "Error: $PKGBUILD not found." >&2
  exit 1
fi

WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

ARCHIVE="GlideFTP-Linux-v${VERSION}.tar.gz"
ASSET_URL="https://github.com/${REPO}/releases/download/v${VERSION}/${ARCHIVE}"

# ── 1. Real sha256 of the published archive ───────────────────────────────────
echo "→ Downloading $ASSET_URL"
if ! curl -fsSL -o "$WORK/$ARCHIVE" "$ASSET_URL"; then
  echo "Error: could not download $ARCHIVE." >&2
  echo "  Is the release v$VERSION published (not draft) on GitHub?" >&2
  exit 1
fi

CONTENT=$(tar -tzf "$WORK/$ARCHIVE")
for f in GlideFTP glideftp.desktop glideftp.png; do
  if ! grep -qx "\(\./\)\?$f" <<< "$CONTENT"; then
    echo "Error: $ARCHIVE does not contain '$f' (needed by PKGBUILD)." >&2
    exit 1
  fi
done

HASH=$(sha256sum "$WORK/$ARCHIVE" | cut -d' ' -f1)
echo "→ sha256 = $HASH"

# ── 2. Update packaging/PKGBUILD ──────────────────────────────────────────────
OLD_VER=$(sed -n 's/^pkgver=//p' "$PKGBUILD")
if [ -z "$PKGREL" ]; then
  if [ "$OLD_VER" != "$VERSION" ]; then
    PKGREL=1
  else
    PKGREL=$(sed -n 's/^pkgrel=//p' "$PKGBUILD")
  fi
fi

sed -i \
  -e "s/^pkgver=.*/pkgver=${VERSION}/" \
  -e "s/^pkgrel=.*/pkgrel=${PKGREL}/" \
  -e "s/sha256sums_x86_64=('[^']*')/sha256sums_x86_64=('${HASH}')/" \
  "$PKGBUILD"
echo "→ packaging/PKGBUILD updated  (pkgver=${VERSION}, pkgrel=${PKGREL}, sha256=${HASH:0:16}...)"

# ── 3. Optional local install test ────────────────────────────────────────────
if [ "$TEST" -eq 1 ]; then
  echo "→ Local test: makepkg -si"
  mkdir -p "$WORK/pkgtest"
  cp "$PKGBUILD" "$WORK/pkgtest/"
  cp "$WORK/$ARCHIVE" "$WORK/pkgtest/glideftp-bin-${VERSION}.tar.gz"
  (cd "$WORK/pkgtest" && makepkg -si)
  echo "→ Installed. Check the app, then uninstall later with: sudo pacman -R glideftp-bin"
fi

# ── 4. AUR clone + PKGBUILD + .SRCINFO ────────────────────────────────────────
if [ ! -d "$AUR_DIR" ]; then
  echo "→ Cloning $AUR_URL into $AUR_DIR"
  mkdir -p "$(dirname "$AUR_DIR")"
  git clone "$AUR_URL" "$AUR_DIR"
elif [ "$(git -C "$AUR_DIR" rev-parse --show-toplevel 2>/dev/null)" != "$(cd "$AUR_DIR" && pwd -P)" ]; then
  echo "Error: $AUR_DIR exists but is not a git clone of the AUR repo." >&2
  echo "  Move it away, then rerun: the script will clone $AUR_URL there." >&2
  exit 1
else
  REMOTE=$(git -C "$AUR_DIR" remote get-url origin 2>/dev/null || true)
  if [ "$REMOTE" != "$AUR_URL" ]; then
    echo "Error: $AUR_DIR origin is '$REMOTE', expected $AUR_URL." >&2
    exit 1
  fi
  echo "→ Updating $AUR_DIR"
  git -C "$AUR_DIR" pull --ff-only
fi

cp "$PKGBUILD" "$AUR_DIR/PKGBUILD"
(cd "$AUR_DIR" && makepkg --printsrcinfo > .SRCINFO)

if [ -z "$(git -C "$AUR_DIR" status --porcelain -- PKGBUILD .SRCINFO)" ]; then
  echo "✓ The AUR already has v${VERSION}-${PKGREL} with this hash, nothing to publish."
  exit 0
fi

# ── 5. Commit + push to the AUR ───────────────────────────────────────────────
echo ""
git -C "$AUR_DIR" --no-pager diff -- PKGBUILD .SRCINFO
echo ""

if [ "$YES" -ne 1 ]; then
  read -r -p "Publish v${VERSION}-${PKGREL} to the AUR? [y/N] " answer
  if [[ ! "$answer" =~ ^[yYoO]$ ]]; then
    echo "Cancelled. PKGBUILD and .SRCINFO are left modified in $AUR_DIR."
    exit 1
  fi
fi

MSG="Release v${VERSION}"
[ "$PKGREL" != "1" ] && MSG="Release v${VERSION}-${PKGREL}"

git -C "$AUR_DIR" add PKGBUILD .SRCINFO
git -C "$AUR_DIR" commit -m "$MSG"
git -C "$AUR_DIR" push

echo ""
echo "✓ glideftp-bin v${VERSION}-${PKGREL} published: https://aur.archlinux.org/packages/glideftp-bin"

if [ -n "$(git -C "$PROJECT_DIR" status --porcelain -- packaging/PKGBUILD)" ]; then
  echo ""
  echo "packaging/PKGBUILD changed in GlideFTP, commit it yourself:"
  echo "  git add packaging/PKGBUILD"
  echo "  git commit -m \"PKGBUILD: v${VERSION}\""
  echo "  git push origin main"
fi
