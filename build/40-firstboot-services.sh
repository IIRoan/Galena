#!/usr/bin/bash

set -eoux pipefail

###############################################################################
# First Boot Services
###############################################################################
# Installs first-boot systemd services for Flatpak preinstall and VS Code
# settings initialization.
###############################################################################

echo "::group:: Flatpak Auto-Install Service"

cat >/usr/libexec/galena-flatpak-preinstall.sh <<'EOF'
#!/usr/bin/bash
set -euo pipefail

STATE_DIR="/var/lib/galena"
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

chmod +x /usr/libexec/galena-flatpak-preinstall.sh

cat >/etc/systemd/system/galena-flatpak-preinstall.service <<'EOF'
[Unit]
Description=Galena Flatpak preinstall on first boot
After=network-online.target
Wants=network-online.target
ConditionPathExists=!/var/lib/galena/flatpak-preinstall.done

[Service]
Type=oneshot
ExecStart=/usr/libexec/galena-flatpak-preinstall.sh
Restart=on-failure
RestartSec=300

[Install]
WantedBy=multi-user.target
EOF

systemctl enable galena-flatpak-preinstall.service

echo "::endgroup::"

echo "::group:: VS Code Settings First-Boot Service"

cat >/usr/libexec/galena-vscode-settings.sh <<'EOF'
#!/usr/bin/bash
set -euo pipefail

STATE_DIR="/var/lib/galena"
DONE_FILE="${STATE_DIR}/vscode-settings.done"
SRC_FILE="/usr/share/galena/vscode-settings.json"
TARGET_DIR="/home/user/.config/Code/User"
TARGET_FILE="${TARGET_DIR}/settings.json"

mkdir -p "${STATE_DIR}"

if [ -f "${DONE_FILE}" ]; then
  exit 0
fi

if [ ! -f "${SRC_FILE}" ]; then
  echo "VS Code settings source file missing: ${SRC_FILE}"
  exit 1
fi

if ! getent passwd user >/dev/null 2>&1; then
  echo "User 'user' not found; will retry."
  exit 75
fi

if [ ! -d "/home/user" ]; then
  echo "/home/user not found; will retry."
  exit 75
fi

install -d -m 0755 -o user -g user "${TARGET_DIR}"
install -m 0644 -o user -g user "${SRC_FILE}" "${TARGET_FILE}"

touch "${DONE_FILE}"
EOF

chmod +x /usr/libexec/galena-vscode-settings.sh

cat >/etc/systemd/system/galena-vscode-settings.service <<'EOF'
[Unit]
Description=Galena VS Code settings on first boot
After=local-fs.target
ConditionPathExists=!/var/lib/galena/vscode-settings.done

[Service]
Type=oneshot
ExecStart=/usr/libexec/galena-vscode-settings.sh
Restart=on-failure
RestartSec=120

[Install]
WantedBy=multi-user.target
EOF

systemctl enable galena-vscode-settings.service

echo "::endgroup::"
