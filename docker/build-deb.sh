#!/usr/bin/env bash
# Runs inside the Ubuntu 22.04 container (glideftp-debian-builder).
# Builds a binary .deb from the pre-built binary already at /src/build/bin/linux/GlideFTP.
set -e

VERSION="${1:?Usage: build-deb.sh <version>}"

BINARY="/src/build/bin/linux/GlideFTP"
DESKTOP="/src/packaging/glideftp.desktop"
ICON="/src/build/appicon.png"
OUTPUT="/src/build/bin/linux/GlideFTP-Linux-v${VERSION}.deb"

DEB_DIR="/tmp/glideftp_${VERSION}_amd64"

mkdir -p "${DEB_DIR}/DEBIAN"
mkdir -p "${DEB_DIR}/usr/bin"
mkdir -p "${DEB_DIR}/usr/share/applications"
mkdir -p "${DEB_DIR}/usr/share/icons/hicolor/256x256/apps"

install -m755 "$BINARY"  "${DEB_DIR}/usr/bin/glideftp"
install -m644 "$DESKTOP" "${DEB_DIR}/usr/share/applications/glideftp.desktop"
install -m644 "$ICON"    "${DEB_DIR}/usr/share/icons/hicolor/256x256/apps/glideftp.png"

cat > "${DEB_DIR}/DEBIAN/control" << EOF
Package: glideftp
Version: ${VERSION}
Section: net
Priority: optional
Architecture: amd64
Depends: libwebkit2gtk-4.1-0
Maintainer: Quirky1869 <mikec18reggae@gmail.com>
Homepage: https://github.com/Quirky1869/GlideFTP
Description: Desktop FTP/SFTP client with dual-panel interface
 GlideFTP is a modern desktop FTP/SFTP client built with Go, Wails v2 and Svelte.
 It features a dual-panel interface, transfer queue, tree view, site manager with
 system keyring password storage, and support for FTP, FTPS and SFTP protocols.
EOF

dpkg-deb --build "$DEB_DIR" "$OUTPUT"

if [ -n "${HOST_UID:-}" ] && [ -n "${HOST_GID:-}" ]; then
    chown "${HOST_UID}:${HOST_GID}" "$OUTPUT"
fi

echo "   [deb] ✓ $OUTPUT"
