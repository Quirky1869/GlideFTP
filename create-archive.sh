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
                    (Linux / AppImage archives only - Windows is always .7z + .exe)
  -h, --help        Show this help message and exit

Archive types (Linux / AppImage):
  gz   → compressed  (.tar.gz)
  tar  → uncompressed (.tar)
  all  → both gz and tar

Windows (whatever --type is):
  GlideFTP-Windows-vX.Y.Z.7z   → 7z archive containing GlideFTP.exe (requires 7z:
                                 Arch: sudo pacman -S 7zip / Ubuntu: sudo apt install p7zip-full)
  GlideFTP-Windows-vX.Y.Z.exe  → the executable itself, uploaded as-is to the release

Output files (example with version 1.7.0):
  GlideFTP-Windows-v1.7.0.7z
  GlideFTP-Windows-v1.7.0.exe
  GlideFTP-Linux-v1.7.0.tar.gz                  ← GlideFTP + glideftp.desktop + glideftp.png + README.md (EN+FR)
  GlideFTP-Linux-Arch-AppImage-v1.7.0.tar.gz    ← AppImage + README.md (EN+FR) - Arch / modern distros (GLIBC 2.38+)
  GlideFTP-Linux-Debian-AppImage-v1.7.0.tar.gz  ← AppImage + README.md (EN+FR) - Ubuntu 22.04+ / Debian 12+ (GLIBC 2.35+)
  GlideFTP-Linux-v1.7.0.tar                     ← same content as the .tar.gz, uncompressed
  GlideFTP-Linux-Arch-AppImage-v1.7.0.tar
  GlideFTP-Linux-Debian-AppImage-v1.7.0.tar
  (+ GlideFTP-Linux-v1.7.0.deb / .rpm copied from build/bin/linux/ with -p all, if built)

Examples:
  $SCRIPT_NAME 1.7.0                              # Windows .7z + .exe and all 6 Linux archives
  $SCRIPT_NAME -p windows 1.7.0                   # Windows .7z + .exe only
  $SCRIPT_NAME -p linux -t gz 1.7.0               # Linux binary .tar.gz only (also updates packaging/PKGBUILD)
  $SCRIPT_NAME -p appimage -t gz 1.7.0            # both AppImage variants .tar.gz only
  $SCRIPT_NAME -p appimage-arch -t gz 1.7.0       # Arch AppImage .tar.gz only
  $SCRIPT_NAME -p appimage-debian -t tar 1.7.0    # Debian AppImage .tar only
  $SCRIPT_NAME --platform all --type gz 1.7.0     # Windows .7z + .exe and the 3 Linux .tar.gz
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

# Write the Linux binary README.md into the given directory (EN + FR)
_write_linux_readme() {
  cat > "$1/README.md" << 'EOF'
# ENGLISH

# GlideFTP - Linux Binary

## GlideFTP - Linux Prerequisites

Before running GlideFTP, install the WebKit2GTK library required by the application:

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Then make the binary executable and run it:

  chmod +x GlideFTP
  ./GlideFTP

## Running the binary executable

  chmod +x GlideFTP
  ./GlideFTP

## Password Storage - Keyring

GlideFTP stores saved site passwords in the system keyring (gnome-keyring or kwallet).
Most desktop environments include one by default, but some minimal setups may not.

Check if a keyring daemon is currently running:

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "no keyring daemon found"
  or
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

If no daemon is running, install one:

  # Ubuntu / Debian - GNOME Keyring (recommended for non-KDE environments)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommended for non-KDE environments)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (for KDE/Plasma)
  sudo pacman -S kwallet

After installing, log out and log back in so the daemon starts automatically with your session.

If no keyring is available, GlideFTP will show a warning banner in the Site Manager.
In that case, use the "Ask password" authentication type so passwords are prompted at
connect time rather than stored.

==============================================================================
# FRANCAIS

# GlideFTP - Binaire Linux

## GlideFTP - Prerequis Linux

Avant de lancer GlideFTP, installez la bibliotheque WebKit2GTK requise :

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Rendez ensuite le binaire executable et lancez-le :

  chmod +x GlideFTP
  ./GlideFTP

## Lancer le binaire executable

  chmod +x GlideFTP
  ./GlideFTP

## Stockage des mots de passe - Keyring

GlideFTP stocke les mots de passe de vos sites dans le keyring systeme (gnome-keyring ou kwallet).
La plupart des environnements de bureau en incluent un par defaut, mais certaines installations
minimalistes peuvent ne pas en avoir.

Verifiez si un daemon keyring est actif :

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "aucun daemon keyring trouve"
  ou
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

Si aucun daemon n'est actif, installez-en un :

  # Ubuntu / Debian - GNOME Keyring (recommande hors KDE)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommande hors KDE)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (pour KDE/Plasma)
  sudo pacman -S kwallet

Apres l'installation, deconnectez-vous puis reconnectez-vous pour que le daemon
demarre automatiquement avec votre session.

Si aucun keyring n'est disponible, GlideFTP affiche une banniere d'avertissement dans
le Gestionnaire de sites. Dans ce cas, utilisez le type d'authentification
"Demander le mot de passe" afin que le mot de passe soit saisi a la connexion.
EOF
}

# Write the Arch AppImage README.md into the given directory (EN + FR)
_write_arch_appimage_readme() {
  cat > "$1/README.md" << 'EOF'
# ENGLISH

# GlideFTP - AppImage Arch

## GlideFTP - Linux Prerequisites

Before running GlideFTP, install the WebKit2GTK library required by the application:

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Then make the binary executable and run it:

  chmod +x GlideFTP-Arch-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Arch-x86_64-v@VERSION@.AppImage

## Running the AppImage

  chmod +x GlideFTP-Arch-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Arch-x86_64-v@VERSION@.AppImage

## Password Storage - Keyring

GlideFTP stores saved site passwords in the system keyring (gnome-keyring or kwallet).
Most desktop environments include one by default, but some minimal setups may not.

Check if a keyring daemon is currently running:

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "no keyring daemon found"
  or
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

If no daemon is running, install one:

  # Ubuntu / Debian - GNOME Keyring (recommended for non-KDE environments)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommended for non-KDE environments)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (for KDE/Plasma)
  sudo pacman -S kwallet

After installing, log out and log back in so the daemon starts automatically with your session.

If no keyring is available, GlideFTP will show a warning banner in the Site Manager.
In that case, use the "Ask password" authentication type so passwords are prompted at
connect time rather than stored.

==============================================================================
# FRANCAIS

# GlideFTP - AppImage Arch

## GlideFTP - Prerequis Linux

Avant de lancer GlideFTP, installez la bibliotheque WebKit2GTK requise :

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Rendez ensuite le binaire executable et lancez-le :

  chmod +x GlideFTP-Arch-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Arch-x86_64-v@VERSION@.AppImage

## Lancer l'AppImage

  chmod +x GlideFTP-Arch-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Arch-x86_64-v@VERSION@.AppImage

## Stockage des mots de passe - Keyring

GlideFTP stocke les mots de passe de vos sites dans le keyring systeme (gnome-keyring ou kwallet).
La plupart des environnements de bureau en incluent un par defaut, mais certaines installations
minimalistes peuvent ne pas en avoir.

Verifiez si un daemon keyring est actif :

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "aucun daemon keyring trouve"
  ou
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

Si aucun daemon n'est actif, installez-en un :

  # Ubuntu / Debian - GNOME Keyring (recommande hors KDE)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommande hors KDE)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (pour KDE/Plasma)
  sudo pacman -S kwallet

Apres l'installation, deconnectez-vous puis reconnectez-vous pour que le daemon
demarre automatiquement avec votre session.

Si aucun keyring n'est disponible, GlideFTP affiche une banniere d'avertissement dans
le Gestionnaire de sites. Dans ce cas, utilisez le type d'authentification
"Demander le mot de passe" afin que le mot de passe soit saisi a la connexion.
EOF
  # The AppImage keeps its versioned name inside the archive
  sed -i "s/@VERSION@/${VERSION}/g" "$1/README.md"
}

# Write the Debian AppImage README.md into the given directory (EN + FR)
_write_debian_appimage_readme() {
  cat > "$1/README.md" << 'EOF'
# ENGLISH

# GlideFTP - AppImage Debian

## GlideFTP - Linux Prerequisites

Before running GlideFTP, install the WebKit2GTK library required by the application:

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Then make the binary executable and run it:

  chmod +x GlideFTP-Debian-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Debian-x86_64-v@VERSION@.AppImage

## Running the AppImage

  chmod +x GlideFTP-Debian-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Debian-x86_64-v@VERSION@.AppImage

## Password Storage - Keyring

GlideFTP stores saved site passwords in the system keyring (gnome-keyring or kwallet).
Most desktop environments include one by default, but some minimal setups may not.

Check if a keyring daemon is currently running:

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "no keyring daemon found"
  or
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

If no daemon is running, install one:

  # Ubuntu / Debian - GNOME Keyring (recommended for non-KDE environments)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommended for non-KDE environments)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (for KDE/Plasma)
  sudo pacman -S kwallet

After installing, log out and log back in so the daemon starts automatically with your session.

If no keyring is available, GlideFTP will show a warning banner in the Site Manager.
In that case, use the "Ask password" authentication type so passwords are prompted at
connect time rather than stored.

==============================================================================
# FRANCAIS

# GlideFTP - AppImage Debian

## GlideFTP - Prerequis Linux

Avant de lancer GlideFTP, installez la bibliotheque WebKit2GTK requise :

  Ubuntu / Debian : sudo apt install libwebkit2gtk-4.1-0
  Fedora          : sudo dnf install webkit2gtk4.1
  Arch Linux      : sudo pacman -S webkit2gtk-4.1

Rendez ensuite le binaire executable et lancez-le :

  chmod +x GlideFTP-Debian-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Debian-x86_64-v@VERSION@.AppImage

## Lancer l'AppImage

  chmod +x GlideFTP-Debian-x86_64-v@VERSION@.AppImage
  ./GlideFTP-Debian-x86_64-v@VERSION@.AppImage

## Stockage des mots de passe - Keyring

GlideFTP stocke les mots de passe de vos sites dans le keyring systeme (gnome-keyring ou kwallet).
La plupart des environnements de bureau en incluent un par defaut, mais certaines installations
minimalistes peuvent ne pas en avoir.

Verifiez si un daemon keyring est actif :

  pgrep -f gnome-keyring-daemon || pgrep -f kwalletd6 || echo "aucun daemon keyring trouve"
  ou
  pacman -Qs gnome-keyring ; pacman -Qs kwallet

Si aucun daemon n'est actif, installez-en un :

  # Ubuntu / Debian - GNOME Keyring (recommande hors KDE)
  sudo apt install gnome-keyring

  # Fedora
  sudo dnf install gnome-keyring

  # Arch Linux - GNOME Keyring (recommande hors KDE)
  sudo pacman -S gnome-keyring

  # Arch Linux - KWallet (pour KDE/Plasma)
  sudo pacman -S kwallet

Apres l'installation, deconnectez-vous puis reconnectez-vous pour que le daemon
demarre automatiquement avec votre session.

Si aucun keyring n'est disponible, GlideFTP affiche une banniere d'avertissement dans
le Gestionnaire de sites. Dans ce cas, utilisez le type d'authentification
"Demander le mot de passe" afin que le mot de passe soit saisi a la connexion.
EOF
  # The AppImage keeps its versioned name inside the archive
  sed -i "s/@VERSION@/${VERSION}/g" "$1/README.md"
}

# 7z binary: 7z (p7zip / 7zip), 7za or 7zz (official 7-Zip for Linux)
_find_7z() {
  local bin
  for bin in 7z 7za 7zz; do
    if command -v "$bin" >/dev/null 2>&1; then
      echo "$bin"
      return 0
    fi
  done
  echo "Error: 7z not found - needed for the Windows archive." >&2
  echo "  Arch: sudo pacman -S 7zip   |   Ubuntu/Debian: sudo apt install p7zip-full" >&2
  return 1
}

# Windows: .7z with GlideFTP.exe inside (no tar/tar.gz - native on Windows 11's
# Explorer and 7-Zip) + the standalone .exe published as-is
make_windows_7z() {
  local src="build/bin/windows/GlideFTP-v${VERSION}.exe"
  local out="GlideFTP-Windows-v${VERSION}.7z"
  local sevenz staging root="$PWD"
  sevenz="$(_find_7z)"
  staging="$(mktemp -d)"
  cp "$src" "$staging/GlideFTP.exe"
  rm -f "$out"   # 7z "a" adds to an existing archive instead of replacing it
  echo "→ $out"
  (cd "$staging" && "$sevenz" a -t7z -mx=9 "$root/$out" GlideFTP.exe)
  rm -rf "$staging"
}

copy_windows_exe() {
  local out="GlideFTP-Windows-v${VERSION}.exe"
  cp "build/bin/windows/GlideFTP-v${VERSION}.exe" "$out"
  echo "→ $out"
}

make_linux_gz() {
  local out="GlideFTP-Linux-v${VERSION}.tar.gz"
  local staging
  staging="$(mktemp -d)"
  cp "build/bin/linux/GlideFTP-v${VERSION}" "$staging/GlideFTP"
  cp packaging/glideftp.desktop "$staging/"
  cp build/appicon.png "$staging/glideftp.png"
  _write_linux_readme "$staging"
  echo "→ $out"
  tar -czvf "$out" -C "$staging" GlideFTP glideftp.desktop glideftp.png README.md
  rm -rf "$staging"
}

make_linux_tar() {
  local out="GlideFTP-Linux-v${VERSION}.tar"
  local staging
  staging="$(mktemp -d)"
  cp "build/bin/linux/GlideFTP-v${VERSION}" "$staging/GlideFTP"
  cp packaging/glideftp.desktop "$staging/"
  cp build/appicon.png "$staging/glideftp.png"
  _write_linux_readme "$staging"
  echo "→ $out"
  tar -cvf "$out" -C "$staging" GlideFTP glideftp.desktop glideftp.png README.md
  rm -rf "$staging"
}

make_appimage_arch_gz() {
  local src="build/bin/linux/GlideFTP-Arch-x86_64-v${VERSION}.AppImage"
  local out="GlideFTP-Linux-Arch-AppImage-v${VERSION}.tar.gz"
  if [ ! -f "$src" ]; then
    echo "⚠  $src not found - skipping Arch AppImage archive (not built in CI; run: ./make.sh appimage-arch ${VERSION})"
    return
  fi
  local staging
  staging="$(mktemp -d)"
  cp "$src" "$staging/"
  _write_arch_appimage_readme "$staging"
  echo "→ $out"
  tar -czvf "$out" -C "$staging" "GlideFTP-Arch-x86_64-v${VERSION}.AppImage" README.md
  rm -rf "$staging"
}

make_appimage_arch_tar() {
  local src="build/bin/linux/GlideFTP-Arch-x86_64-v${VERSION}.AppImage"
  local out="GlideFTP-Linux-Arch-AppImage-v${VERSION}.tar"
  if [ ! -f "$src" ]; then
    echo "⚠  $src not found - skipping Arch AppImage archive (not built in CI; run: ./make.sh appimage-arch ${VERSION})"
    return
  fi
  local staging
  staging="$(mktemp -d)"
  cp "$src" "$staging/"
  _write_arch_appimage_readme "$staging"
  echo "→ $out"
  tar -cvf "$out" -C "$staging" "GlideFTP-Arch-x86_64-v${VERSION}.AppImage" README.md
  rm -rf "$staging"
}

make_appimage_debian_gz() {
  local out="GlideFTP-Linux-Debian-AppImage-v${VERSION}.tar.gz"
  local staging
  staging="$(mktemp -d)"
  cp "build/bin/linux/GlideFTP-Debian-x86_64-v${VERSION}.AppImage" "$staging/"
  _write_debian_appimage_readme "$staging"
  echo "→ $out"
  tar -czvf "$out" -C "$staging" "GlideFTP-Debian-x86_64-v${VERSION}.AppImage" README.md
  rm -rf "$staging"
}

make_appimage_debian_tar() {
  local out="GlideFTP-Linux-Debian-AppImage-v${VERSION}.tar"
  local staging
  staging="$(mktemp -d)"
  cp "build/bin/linux/GlideFTP-Debian-x86_64-v${VERSION}.AppImage" "$staging/"
  _write_debian_appimage_readme "$staging"
  echo "→ $out"
  tar -cvf "$out" -C "$staging" "GlideFTP-Debian-x86_64-v${VERSION}.AppImage" README.md
  rm -rf "$staging"
}

# ── PKGBUILD auto-update ─────────────────────────────────────────────────────
# Called after the Linux .tar.gz is built; updates pkgver and sha256sums in packaging/PKGBUILD.
update_pkgbuild() {
  local archive="GlideFTP-Linux-v${VERSION}.tar.gz"

  if [ ! -f "packaging/PKGBUILD" ]; then
    echo "⚠  packaging/PKGBUILD not found - skipping AUR update"
    return
  fi

  if [ ! -f "$archive" ]; then
    echo "⚠  $archive not found - skipping AUR update"
    return
  fi

  local hash
  hash=$(sha256sum "$archive" | cut -d' ' -f1)

  sed -i \
    -e "s/^pkgver=.*/pkgver=${VERSION}/" \
    -e "s/sha256sums_x86_64=('[^']*')/sha256sums_x86_64=('${hash}')/" \
    packaging/PKGBUILD

  echo "→ packaging/PKGBUILD updated  (pkgver=${VERSION}, sha256=${hash:0:16}...)"
}

copy_packages() {
  local deb="build/bin/linux/GlideFTP-Linux-v${VERSION}.deb"
  local rpm="build/bin/linux/GlideFTP-Linux-v${VERSION}.rpm"
  local copied=0

  if [ -f "$deb" ]; then
    cp "$deb" "GlideFTP-Linux-v${VERSION}.deb"
    echo "→ GlideFTP-Linux-v${VERSION}.deb"
    copied=1
  else
    echo "⚠  .deb not found (run: ./make.sh deb ${VERSION})"
  fi

  if [ -f "$rpm" ]; then
    cp "$rpm" "GlideFTP-Linux-v${VERSION}.rpm"
    echo "→ GlideFTP-Linux-v${VERSION}.rpm"
    copied=1
  else
    echo "⚠  .rpm not found (run: ./make.sh rpm ${VERSION})"
  fi
}

# ── Run ───────────────────────────────────────────────────────────────────────
echo "GlideFTP archive builder - version v${VERSION}"
echo "Platform: $PLATFORM  |  Type: $TYPE"
echo "----------------------------------------------"

[[ "$PLATFORM" == "windows"  || "$PLATFORM" == "all" ]] && \
  make_windows_7z && copy_windows_exe

[[ "$PLATFORM" == "linux"    || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_linux_gz && update_pkgbuild

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-arch" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_appimage_arch_gz

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-debian" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "gz"  || "$TYPE" == "all" ]] && make_appimage_debian_gz

[[ "$PLATFORM" == "linux"    || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_linux_tar

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-arch" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_appimage_arch_tar

[[ "$PLATFORM" == "appimage" || "$PLATFORM" == "appimage-debian" || "$PLATFORM" == "all" ]] && \
  [[ "$TYPE" == "tar" || "$TYPE" == "all" ]] && make_appimage_debian_tar

[[ "$PLATFORM" == "all" ]] && copy_packages

echo "──────────────────────────────────────────────"
echo "Done."
