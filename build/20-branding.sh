#!/usr/bin/bash
set -eoux pipefail

echo "::group:: Install Custom Branding"

# Replace Bluefin branding with Galena in OS identification
if [ -f /etc/os-release ]; then
    sed -i 's/Bluefin/Galena/g' /etc/os-release
    # Ensure LOGO is set correctly, replacing any existing LOGO entry
    if grep -q "^LOGO=" /etc/os-release; then
        sed -i 's/^LOGO=.*/LOGO=galena-logo/' /etc/os-release
    else
        echo "LOGO=galena-logo" >> /etc/os-release
    fi
fi
if [ -f /usr/lib/os-release ]; then
    sed -i 's/Bluefin/Galena/g' /usr/lib/os-release
    if grep -q "^LOGO=" /usr/lib/os-release; then
        sed -i 's/^LOGO=.*/LOGO=galena-logo/' /usr/lib/os-release
    else
        echo "LOGO=galena-logo" >> /usr/lib/os-release
    fi
fi

# Deploy custom system files (wallpapers, icons, plymouth, etc.)
if [ -d "/ctx/custom/system_files" ]; then
    echo "Deploying custom system files..."
    # Copy all custom system files to root filesystem
    # This overlays files from custom/system_files/ onto /
    cp -r /ctx/custom/system_files/* /

    # Ensure the logo is in pixmaps with the name expected by os-release
    if [ -f "/usr/share/pixmaps/galena-logo.png" ]; then
        # Link it without extension as some apps expect it
        ln -sf galena-logo.png /usr/share/pixmaps/galena-logo 2>/dev/null || true
    fi

    # Remove placeholder files
    find /usr/share -name ".PLACEHOLDER" -delete 2>/dev/null || true
fi

# Compile gschema overrides to apply default settings
if [ -d "/usr/share/glib-2.0/schemas" ]; then
    echo "Compiling gschema overrides..."
    glib-compile-schemas /usr/share/glib-2.0/schemas/
fi

# Create GNOME Control Center branding files for gnome-control-center
# This replaces the embedded logo in gnome-control-center binary
if [ -f "/usr/share/pixmaps/galena-logo.png" ]; then
    echo "Creating GNOME Control Center branding gresource files..."
    mkdir -p "/usr/share/gnome-control-center/branding"

    # Copy logo to gnome-control-center branding directory
    cp -f /usr/share/pixmaps/galena-logo.png "/usr/share/gnome-control-center/branding/logo.png"

    # Additional branding assets
    cp -f /usr/share/pixmaps/galena-logo.png "/usr/share/gnome-control-center/branding/distributor-logo.png"
fi

# Override bluefin branding in /usr/share/ublue-os/brand if it exists
if [ -d "/usr/share/ublue-os/branding" ]; then
    echo "Overriding Bluefin branding files..."
    # Copy galena logo to replace bluefin logo in branding directory
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/ublue-os/branding/logo.png 2>/dev/null || true
fi

# Setup GDM login screen branding using dconf (proper method for GDM)
# Reference: https://help.gnome.org/admin/system-admin-guide/stable/login-logo.html
if [ -f "/usr/share/pixmaps/galena-logo.png" ]; then
    echo "Setting up GDM login screen branding..."

    # Create dconf profile for GDM if it doesn't exist
    if [ ! -f "/etc/dconf/profile/gdm" ]; then
        mkdir -p "/etc/dconf/profile"
        cat > /etc/dconf/profile/gdm << 'EOF'
user-db:user
system-db:gdm
file-db:/usr/share/gdm/greeter-dconf-defaults
EOF
    fi

    # Create GDM database for logo
    mkdir -p "/etc/dconf/db/gdm.d"
    cat > /etc/dconf/db/gdm.d/01-logo << EOF
[org/gnome/login-screen]
logo='/usr/share/pixmaps/galena-logo.png'
EOF

    # Update dconf database to apply changes
    dconf update
fi

# Replace ALL Bluefin/Fedora branding files with Galena
# Source: projectbluefin/common system_files/bluefin/usr/share/pixmaps
if [ -f "/usr/share/pixmaps/galena-logo.png" ]; then
    echo "Replacing ALL Bluefin/Fedora logo files with Galena branding..."
    
    # Create pixmaps directory if it doesn't exist
    mkdir -p "/usr/share/pixmaps"
    
    # Replace all 8 Fedora logo files that Bluefin ships
    # These are used by various GNOME components including gnome-control-center
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora-gdm-logo.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora-logo-icon.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora-logo-small.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora-logo-sprite.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora-logo.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora_logo_med.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/fedora_whitelogo_med.png
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/pixmaps/system-logo-white.png
    
    # Replace Plymouth spinner theme watermarks (boot splash)
    # Source: projectbluefin/common system_files/bluefin/usr/share/plymouth/themes/spinner
    if [ -d "/usr/share/plymouth/themes/spinner" ]; then
        echo "Replacing Plymouth spinner theme watermarks..."
        # Use the correctly-sized watermark so it matches spinner theme expectations
        if [ -f "/usr/share/plymouth/themes/galena/watermark.png" ]; then
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/spinner/watermark.png
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/spinner/silverblue-watermark.png
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/spinner/fedora-logo.png
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/spinner/bgrt-fallback.png 2>/dev/null || true
        fi
    fi

    # Replace Plymouth BGRT theme logos (UEFI boot)
    if [ -d "/usr/share/plymouth/themes/bgrt" ]; then
        echo "Replacing Plymouth BGRT theme watermarks..."
        if [ -f "/usr/share/plymouth/themes/galena/watermark.png" ]; then
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/bgrt/watermark.png
            cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/bgrt/bgrt-fallback.png 2>/dev/null || true
            # BGRT sometimes uses the spinner files
            if [ -f "/usr/share/plymouth/themes/bgrt/fedora-logo.png" ]; then
                cp -f /usr/share/plymouth/themes/galena/watermark.png /usr/share/plymouth/themes/bgrt/fedora-logo.png
            fi
        fi
    fi

    # Replace any potential Bluefin-specific branding files
    if [ -d "/usr/share/ublue-os/branding" ]; then
        for png_file in /usr/share/ublue-os/branding/*.png; do
            [ -f "$png_file" ] && cp -f /usr/share/pixmaps/galena-logo.png "$png_file"
        done
    fi

    # Avoid blanket replacement: only update known watermark assets to reduce risk.

    # gnome-control-center uses gnome-logo from icon theme for About screen
    # This is THE critical location for GNOME Settings "About" logo
    echo "Replacing icon theme logos for GNOME Control Center..."
    mkdir -p /usr/share/icons/hicolor/128x128/apps
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/icons/hicolor/128x128/apps/gnome-logo.png
    mkdir -p /usr/share/icons/hicolor/256x256/apps
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/icons/hicolor/256x256/apps/gnome-logo.png
    mkdir -p /usr/share/icons/hicolor/scalable/apps
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/icons/hicolor/scalable/apps/gnome-logo.png

    # Also add as start-here icon for GNOME desktop
    mkdir -p /usr/share/icons/hicolor/48x48/apps
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/icons/hicolor/48x48/apps/start-here.png
    mkdir -p /usr/share/icons/hicolor/64x64/apps
    cp -f /usr/share/pixmaps/galena-logo.png /usr/share/icons/hicolor/64x64/apps/start-here.png

    # Update icon theme caches so the system recognises the new icons
    if command -v gtk-update-icon-cache &> /dev/null; then
        echo "Updating icon theme caches..."
        gtk-update-icon-cache -f -t /usr/share/icons/hicolor/ 2>/dev/null || true
    fi
    
    echo "✓ Replaced all Bluefin branding files with Galena"
fi

# Ensure Plymouth is enabled in kernel arguments for both ISO and installed system
# This is critical for the branding to show on the first boot of the installer
mkdir -p /usr/lib/bootc/kargs.d
echo "rhgb quiet" > /usr/lib/bootc/kargs.d/10-plymouth.karg

# Rebuild initramfs using Bluefin-style dracut settings
if command -v dracut &> /dev/null && [ -f "/usr/share/plymouth/themes/galena/watermark.png" ]; then
    export DRACUT_NO_XATTR=1
    KERNEL_SUFFIX=""
    QUALIFIED_KERNEL=$(
        (rpm -qa | grep -P "kernel-(|${KERNEL_SUFFIX}-)(\d+\.\d+\.\d+)" | sed -E "s/kernel-(|${KERNEL_SUFFIX}-)//" | head -n 1) || true
    )
    if [ -z "$QUALIFIED_KERNEL" ]; then
        QUALIFIED_KERNEL=$(find /lib/modules -mindepth 1 -maxdepth 1 -type d -printf '%f\n' | head -n 1)
    fi
    if [ -n "$QUALIFIED_KERNEL" ]; then
        IMG="/lib/modules/$QUALIFIED_KERNEL/initramfs.img"
        echo "Rebuilding initramfs for $QUALIFIED_KERNEL..."
        dracut --no-hostonly --kver "$QUALIFIED_KERNEL" --reproducible -v --add ostree -f "$IMG"
        chmod 0600 "$IMG"
    else
        echo "No kernel version found; skipping initramfs rebuild."
    fi
fi

echo "::endgroup::"
