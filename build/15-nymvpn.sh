#!/usr/bin/bash

set -eoux pipefail

###############################################################################
# NymVPN Daemon Installation
###############################################################################
# Installs the nym-vpnd daemon at build time.
# Automatically fetches the latest release from GitHub.
# The NymVPN Flatpak frontend is installed via preinstall on first boot.
###############################################################################

echo "::group:: Install NymVPN Daemon"

# Configuration
VPND_BIN_NAME="nym-vpnd"
VPNSVC_NAME="nym-vpnd.service"
GITHUB_REPO="nymtech/nym-vpn-client"

# Installation paths (use /usr/bin for immutable image, not /usr/local which is a symlink)
SYS_BIN_DIR="/usr/bin"
SYS_UNIT_DIR="/etc/systemd/system"

VPND_BIN_TARGET="$SYS_BIN_DIR/$VPND_BIN_NAME"
VPND_UNIT_TARGET="$SYS_UNIT_DIR/$VPNSVC_NAME"

# Create temporary directory for downloads
TMPDIR="$(mktemp -d)"
cleanup() { [[ -d "$TMPDIR" ]] && rm -rf "$TMPDIR"; }
trap cleanup EXIT

# Fetch latest nym-vpn-core release from GitHub API
echo "Fetching latest nym-vpn-core release from GitHub..."
RELEASES_JSON=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases?per_page=50")

# Find the latest release with tag matching nym-vpn-core-v*
LATEST_RELEASE=$(echo "$RELEASES_JSON" | jq -r '[.[] | select(.tag_name | startswith("nym-vpn-core-v")) | select(.prerelease == false)] | first')

if [[ -z "$LATEST_RELEASE" || "$LATEST_RELEASE" == "null" ]]; then
    echo "ERROR: Could not find a valid nym-vpn-core release"
    exit 1
fi

VPND_TAG=$(echo "$LATEST_RELEASE" | jq -r '.tag_name')
# Extract version number from tag (nym-vpn-core-v1.22.0 -> 1.22.0)
VPND_VERSION="${VPND_TAG#nym-vpn-core-v}"

echo "Latest release: $VPND_TAG (version $VPND_VERSION)"

# Construct download URLs
CORE_ARCHIVE="nym-vpn-core-v${VPND_VERSION}_linux_x86_64.tar.gz"
VPND_TARBALL_URL="https://github.com/${GITHUB_REPO}/releases/download/${VPND_TAG}/${CORE_ARCHIVE}"

# Get the commit SHA for the unit file (use release target_commitish or default branch)
RELEASE_COMMIT=$(echo "$LATEST_RELEASE" | jq -r '.target_commitish // "main"')
VPND_UNIT_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/${RELEASE_COMMIT}/nym-vpn-core/crates/nym-vpnd/.pkg/aur/nym-vpnd.service"

# Download and verify daemon archive
echo "Downloading vpnd archive: $CORE_ARCHIVE"
curl -fL "$VPND_TARBALL_URL" -o "$TMPDIR/$CORE_ARCHIVE"
curl -fL "${VPND_TARBALL_URL}.sha256sum" -o "$TMPDIR/${CORE_ARCHIVE}.sha256sum"

# Verify checksum (must run from same directory as files)
(cd "$TMPDIR" && sha256sum --check --status "${CORE_ARCHIVE}.sha256sum")
echo "sha256 verification passed"

# Download systemd unit file
echo "Downloading unit file from commit: $RELEASE_COMMIT"
if ! curl -fL "$VPND_UNIT_URL" -o "$TMPDIR/$VPNSVC_NAME"; then
    echo "WARNING: Could not download unit file from release commit, trying main branch"
    VPND_UNIT_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/nym-vpn-core/crates/nym-vpnd/.pkg/aur/nym-vpnd.service"
    curl -fL "$VPND_UNIT_URL" -o "$TMPDIR/$VPNSVC_NAME"
fi

# Extract daemon binary
echo "Extracting vpnd"
tar -xzf "$TMPDIR/$CORE_ARCHIVE" -C "$TMPDIR"
EXTRACTED_DIR="${CORE_ARCHIVE%.tar.gz}"

if [[ ! -f "$TMPDIR/$EXTRACTED_DIR/$VPND_BIN_NAME" ]]; then
    echo "ERROR: Expected binary not found at: $TMPDIR/$EXTRACTED_DIR/$VPND_BIN_NAME"
    exit 1
fi

# Install daemon binary
echo "Installing daemon binary -> $VPND_BIN_TARGET"
cp "$TMPDIR/$EXTRACTED_DIR/$VPND_BIN_NAME" "$VPND_BIN_TARGET"
chmod 755 "$VPND_BIN_TARGET"

# Install systemd unit (mkdir -p needed as /etc/systemd/system may not exist in minimal images)
echo "Installing systemd unit -> $VPND_UNIT_TARGET"
mkdir -p "$SYS_UNIT_DIR"
cp "$TMPDIR/$VPNSVC_NAME" "$VPND_UNIT_TARGET"
chmod 644 "$VPND_UNIT_TARGET"

# Patch ExecStart to point to /usr/local/bin/nym-vpnd
if grep -qE '^ExecStart=' "$VPND_UNIT_TARGET"; then
    sed -i "s|^ExecStart=.*|ExecStart=$VPND_BIN_TARGET|g" "$VPND_UNIT_TARGET"
else
    echo "WARNING: unit file had no ExecStart= line; leaving as-is"
fi

# Enable the service (will start on boot)
systemctl enable "$VPNSVC_NAME"

echo "NymVPN daemon v${VPND_VERSION} installed and enabled"

echo "::endgroup::"
