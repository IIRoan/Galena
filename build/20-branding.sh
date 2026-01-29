#!/usr/bin/bash
set -eoux pipefail

echo "::group:: Install Custom Branding"

# Replace Bluefin branding with Galena in OS identification
sed -i 's/Bluefin/Galena/g' /etc/os-release /usr/lib/os-release

# Deploy custom system files (wallpapers, icons, plymouth, etc.)
if [ -d "/ctx/custom/system_files" ]; then
    # Copy all custom system files to root filesystem
    # This overlays files from custom/system_files/ onto /
    cp -r /ctx/custom/system_files/* /

    # Remove placeholder files
    find / -name ".PLACEHOLDER" -delete 2>/dev/null || true
fi

# Set Galena as default GNOME wallpaper (optional - uncomment to enable)
# if command -v gsettings &> /dev/null; then
#     gsettings set org.gnome.desktop.background picture-uri "file:///usr/share/backgrounds/galena/wallpaper.png"
#     gsettings set org.gnome.desktop.background picture-uri-dark "file:///usr/share/backgrounds/galena/wallpaper-dark.png"
# fi

# Set Plymouth theme (optional - uncomment to enable)
# if [ -d "/usr/share/plymouth/themes/galena" ]; then
#     plymouth-set-default-theme galena
# fi

echo "::endgroup::"
