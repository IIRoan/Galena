#!/usr/bin/bash

set -eoux pipefail

###############################################################################
# Helium Browser
###############################################################################
# Install the native Fedora/COPR Helium package at image build time so the
# browser is available without relying on a Flatpak runtime.
###############################################################################

# Source helper functions
# shellcheck source=/dev/null
source /ctx/build/copr-helpers.sh

echo "::group:: Install Helium Browser"

copr_install_isolated "imput/helium" helium-bin

echo "::endgroup::"
