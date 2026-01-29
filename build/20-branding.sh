#!/usr/bin/bash
set -eoux pipefail

echo "::group:: Install Custom Branding"

# Replace Bluefin branding with Galena in OS identification
if [ -f /etc/os-release ]; then
    sed -i 's/Bluefin/Galena/g' /etc/os-release
fi
if [ -f /usr/lib/os-release ]; then
    sed -i 's/Bluefin/Galena/g' /usr/lib/os-release
fi

# Deploy custom system files (wallpapers, icons, plymouth, etc.)
if [ -d "/ctx/custom/system_files" ]; then
    echo "Deploying custom system files..."
    # Copy all custom system files to root filesystem
    # This overlays files from custom/system_files/ onto /
    cp -r /ctx/custom/system_files/* /

    # Remove placeholder files
    find /usr/share -name ".PLACEHOLDER" -delete 2>/dev/null || true
fi

# Compile gschema overrides to apply default settings
if [ -d "/usr/share/glib-2.0/schemas" ]; then
    echo "Compiling gschema overrides..."
    glib-compile-schemas /usr/share/glib-2.0/schemas/
fi

# Set Plymouth theme if available
if [ -d "/usr/share/plymouth/themes/galena" ]; then
    echo "Setting Galena Plymouth theme..."
    if command -v plymouth-set-default-theme &> /dev/null; then
        plymouth-set-default-theme galena || true
    fi
fi

echo "::endgroup::"
