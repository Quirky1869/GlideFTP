#!/usr/bin/env bash
# Runs inside the Fedora 40 container (glideftp-rpm-builder).
# Builds a binary .rpm from the pre-built binary at /src/build/bin/linux/GlideFTP-v<version>.
set -e

VERSION="${1:?Usage: build-rpm.sh <version>}"

BINARY="/src/build/bin/linux/GlideFTP-v${VERSION}"
DESKTOP="/src/packaging/glideftp.desktop"
ICON="/src/build/appicon.png"
SPEC_TPL="/src/packaging/glideftp.spec"
OUTPUT_DIR="/src/build/bin/linux"

rpmdev-setuptree
SOURCES=~/rpmbuild/SOURCES
SPECS=~/rpmbuild/SPECS

install -m755 "$BINARY"  "${SOURCES}/GlideFTP"
install -m644 "$DESKTOP" "${SOURCES}/glideftp.desktop"
install -m644 "$ICON"    "${SOURCES}/glideftp.png"

CHANGELOG_DATE=$(LC_TIME=C date "+%a %b %d %Y")
sed \
    -e "s/@VERSION@/${VERSION}/g" \
    -e "s/@CHANGELOG_DATE@/${CHANGELOG_DATE}/g" \
    "$SPEC_TPL" > "${SPECS}/glideftp.spec"

# -bb = binary RPM only (no source package, no compilation step)
rpmbuild -bb "${SPECS}/glideftp.spec"

RPM_FILE=$(find ~/rpmbuild/RPMS/x86_64/ -name "*.rpm" | head -1)
OUTPUT="${OUTPUT_DIR}/GlideFTP-Linux-v${VERSION}.rpm"
cp "$RPM_FILE" "$OUTPUT"

if [ -n "${HOST_UID:-}" ] && [ -n "${HOST_GID:-}" ]; then
    chown "${HOST_UID}:${HOST_GID}" "$OUTPUT"
fi

echo "   [rpm] ✓ $OUTPUT"
