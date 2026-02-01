#!/usr/bin/bash

set -eoux pipefail

###############################################################################
# Main Build Script
###############################################################################
# This script follows the @ublue-os/bluefin pattern for build scripts.
# It uses set -eoux pipefail for strict error handling and debugging.
###############################################################################

# Source helper functions
# shellcheck source=/dev/null
source /ctx/build/copr-helpers.sh

# Enable nullglob for all glob operations to prevent failures on empty matches
shopt -s nullglob

echo "::group:: Copy Bluefin Config from Common"

# Copy just files from @projectbluefin/common (includes 00-entry.just which imports 60-custom.just)
mkdir -p /usr/share/ublue-os/just/
shopt -s nullglob
cp -r /ctx/oci/common/bluefin/usr/share/ublue-os/just/* /usr/share/ublue-os/just/
shopt -u nullglob

echo "::endgroup::"

echo "::group:: Copy Custom Files"

# Copy Brewfiles to standard location
mkdir -p /usr/share/ublue-os/homebrew/
cp /ctx/custom/brew/*.Brewfile /usr/share/ublue-os/homebrew/

# Consolidate Just Files
find /ctx/custom/ujust -iname '*.just' -exec printf "\n\n" \; -exec cat {} \; >> /usr/share/ublue-os/just/60-custom.just

# Copy Flatpak preinstall files
mkdir -p /etc/flatpak/preinstall.d/
cp /ctx/custom/flatpaks/*.preinstall /etc/flatpak/preinstall.d/

echo "::endgroup::"

echo "::group:: Install Packages"

# Install packages using dnf5
# Example: dnf5 install -y tmux
# Example using COPR with isolated pattern:
# copr_install_isolated "ublue-os/staging" package-name

echo "::endgroup::"

echo "::group:: System Configuration"

# Enable/disable systemd services
systemctl enable podman.socket
# Example: systemctl mask unwanted-service

echo "::endgroup::"

echo "::group:: Flatpak Auto-Install Service"

cat >/usr/libexec/finpilot-flatpak-preinstall.sh <<'EOF'
#!/usr/bin/bash
set -euo pipefail

STATE_DIR="/var/lib/finpilot"
DONE_FILE="${STATE_DIR}/flatpak-preinstall.done"

mkdir -p "${STATE_DIR}"

if [ -f "${DONE_FILE}" ]; then
  exit 0
fi

if ! command -v flatpak >/dev/null 2>&1; then
  echo "flatpak not available; skipping flatpak preinstall."
  exit 0
fi

# Ensure flathub remote exists
if ! flatpak remotes --system --columns=name | grep -q '^flathub$'; then
  flatpak remote-add --if-not-exists --system flathub https://flathub.org/repo/flathub.flatpakrepo
fi

# Basic internet connectivity check: quick HTTPS request to Flathub
# If this fails, exit non-zero so systemd can retry the service later.
if ! curl -sSfI --max-time 5 https://flathub.org >/dev/null 2>&1; then
  echo "No internet connectivity detected; will retry flatpak preinstall when service is restarted."
  exit 75
fi

shopt -s nullglob

for preinstall in /etc/flatpak/preinstall.d/*.preinstall; do
  current_app_id=""
  current_branch=""
  current_is_runtime="false"

  while IFS= read -r line || [ -n "${line}" ]; do
    case "${line}" in
      \[Flatpak\ Preinstall\ *\])
        # Install previous entry if any
        if [ -n "${current_app_id}" ]; then
          ref="${current_app_id}"
          if [ -n "${current_branch}" ] && [ "${current_branch}" != "stable" ]; then
            ref="${ref}//${current_branch}"
          fi

          if [ "${current_is_runtime}" = "true" ]; then
            flatpak install -y --system --or-update --runtime flathub "${ref}" || true
          else
            flatpak install -y --system --or-update flathub "${ref}" || true
          fi
        fi

        current_app_id="${line#\[Flatpak Preinstall }"
        current_app_id="${current_app_id%]}"
        current_branch=""
        current_is_runtime="false"
        ;;
      Branch=*)
        current_branch="${line#Branch=}"
        ;;
      IsRuntime=*)
        current_is_runtime="${line#IsRuntime=}"
        ;;
      "")
        # Blank line terminates current section
        if [ -n "${current_app_id}" ]; then
          ref="${current_app_id}"
          if [ -n "${current_branch}" ] && [ "${current_branch}" != "stable" ]; then
            ref="${ref}//${current_branch}"
          fi

          if [ "${current_is_runtime}" = "true" ]; then
            flatpak install -y --system --or-update --runtime flathub "${ref}" || true
          else
            flatpak install -y --system --or-update flathub "${ref}" || true
          fi
          current_app_id=""
        fi
        ;;
    esac
  done <"${preinstall}"

  # Handle last entry if file doesn't end with a blank line
  if [ -n "${current_app_id}" ]; then
    ref="${current_app_id}"
    if [ -n "${current_branch}" ] && [ "${current_branch}" != "stable" ]; then
      ref="${ref}//${current_branch}"
    fi

    if [ "${current_is_runtime}" = "true" ]; then
      flatpak install -y --system --or-update --runtime flathub "${ref}" || true
    else
      flatpak install -y --system --or-update flathub "${ref}" || true
    fi
  fi
done

shopt -u nullglob

touch "${DONE_FILE}"
EOF

chmod +x /usr/libexec/finpilot-flatpak-preinstall.sh

cat >/etc/systemd/system/finpilot-flatpak-preinstall.service <<'EOF'
[Unit]
Description=Finpilot Flatpak preinstall on first boot
After=network-online.target
Wants=network-online.target
ConditionPathExists=!/var/lib/finpilot/flatpak-preinstall.done

[Service]
Type=oneshot
ExecStart=/usr/libexec/finpilot-flatpak-preinstall.sh
Restart=on-failure
RestartSec=300

[Install]
WantedBy=multi-user.target
EOF

systemctl enable finpilot-flatpak-preinstall.service

echo "::endgroup::"

# Restore default glob behavior
shopt -u nullglob

echo "Custom build complete!"
