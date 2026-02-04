# Galena

My personal Linux OS built on [Bluefin DX](https://projectbluefin.io/). An immutable, container-native desktop that just works.

[![Download ISO](https://img.shields.io/badge/Download-ISO-blue?style=for-the-badge&logo=linux)](https://s3.roan.dev/install-latest.iso)

[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/galena)](https://artifacthub.io/packages/search?repo=galena)
[![Build container image](https://github.com/iiroan/galena/actions/workflows/build.yml/badge.svg)](https://github.com/iiroan/galena/actions/workflows/build.yml)

## What Makes Galena Different?

Galena is based on [Bluefin DX](https://projectbluefin.io/) and includes these customizations:

### Build-time Additions

- **NymVPN Daemon** - Privacy-focused VPN with mixnet technology (auto-updated from latest GitHub releases)
- **Custom Branding** - Galena logos, wallpapers, and Plymouth boot themes throughout the system
- **First-boot Services** - Optimized systemd services for a smooth initial experience

### Runtime Applications (Flatpak)

Over 60 applications pre-configured for first-boot installation including:

- **Browsers**: Firefox, Zen Browser, Brave, Chrome, Chromium
- **Privacy & Security**: NymVPN, Proton Pass, YubiKey Authenticator
- **Communication**: Vesktop (Discord), Spotify, Termius
- **Development**: Podman Desktop, Beekeeper Studio, DevToolbox
- **System Tools**: Mission Center, Flatseal, Warehouse, Extension Manager
- **GNOME Suite**: Full collection of GNOME applications
- **3D Printing**: Bambu Studio

### CLI Tools (Homebrew)

Modern command-line replacements and development tools:

```
bat       - cat with syntax highlighting
eza       - modern ls replacement
fd        - simple, fast find alternative
rg        - ripgrep (faster grep)
gh        - GitHub CLI
starship  - cross-shell prompt
zoxide    - smarter cd command
htop      - interactive process viewer
tmux      - terminal multiplexer
```

## Installation

### From ISO (Recommended)

[Download the latest ISO](https://s3.roan.dev/install-latest.iso) and boot from USB. The Anaconda installer will guide you through disk selection, encryption, and user setup.

### Rebase an Existing System

If you're already running a bootc-compatible system (Fedora Silverblue, Bluefin, etc.):

```bash
sudo bootc switch ghcr.io/iiroan/galena:stable
sudo systemctl reboot
```

## Building Locally

Galena uses a custom Go CLI tool for building, testing, and managing the OS image. The CLI provides an interactive TUI and command-line interface.

### Prerequisites

**Recommended: Use a Universal Blue image** (Bluefin, Bazzite, Aurora, etc.)

Universal Blue images come with all the necessary tools pre-installed:

- Podman (container engine)
- bootc-image-builder (for disk images)
- Development tools

**If not on Universal Blue, you'll need:**

- **Linux system** (required for bootc-image-builder)
- **Go 1.23+** (for building the CLI)
- **Podman** (for container builds)
- **sudo access** (for disk image builds)

### Installing Go

If Go is not installed on your system:

**On Universal Blue (Bluefin/Bazzite/Aurora):**

```bash
# Install Go via Homebrew (recommended)
brew install go
```

**Or download directly from golang.org:**

```bash
# Download and install the latest version
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

Verify Go installation:

```bash
go version  # Should show go1.23 or higher
```

### Building the galena CLI

**Step 1: Clone the repository**

```bash
git clone https://github.com/iiroan/galena.git
cd galena
```

**Step 2: Build the CLI**

```bash
# Using Make (recommended)
make build

# Or using Go directly
go build -o galena ./cmd/galena/

# Verify it works
./galena version
```

**Step 3: (Optional) Install system-wide**

```bash
# Install to /usr/local/bin
sudo make install

# Now you can run from anywhere
galena version
```

### Quick Start

Once the CLI is built:

```bash
# Run interactive mode (TUI)
./galena

# Or use specific commands
./galena build              # Interactive build wizard
./galena build --push       # Build and push to registry
./galena disk iso           # Build ISO installer
./galena status             # Show project status
./galena validate           # Run all validation checks
```

### The galena CLI

The `galena` CLI is built with [Charm](https://charm.sh/) libraries (Bubble Tea, Lipgloss, Huh) for a beautiful terminal experience.

**Main Commands:**

```bash
galena                      # Interactive TUI control plane
galena build                # Build container image (interactive or flags)
galena disk <type>          # Build bootable disk images
galena vm run               # Run VM for testing
galena sign <image>         # Sign images with cosign
galena sbom <image>         # Generate SBOM with Trivy
galena validate             # Run validation checks
galena status               # Show project and system status
galena clean                # Clean build artifacts
galena version              # Show version information
```

**Build Flags:**

```bash
galena build \
  --variant main \          # Image variant (main, nvidia, dx)
  --tag stable \            # Image tag (stable, latest, beta)
  --push \                  # Push to registry after build
  --sign \                  # Sign with cosign
  --sbom \                  # Generate Software Bill of Materials
  --no-cache                # Build without cache
```

**Disk Image Types:**

```bash
galena disk iso             # Standard ISO installer
galena disk anaconda-iso    # Anaconda-based ISO installer
galena disk qcow2           # QEMU/KVM virtual machine image
galena disk raw             # Raw disk image
galena disk vmdk            # VMware disk image
```

### Build Workflows

**Local Development:**

```bash
# Fast build: container + ISO
./galena                    # Choose "Fast Build" from menu

# Full workflow with testing
./galena build              # Build container
./galena disk qcow2         # Create VM image
./galena vm run             # Test in VM
```

**CI/CD (GitHub Actions):**

```bash
# The CI command is optimized for GitHub Actions
./galena ci build --push --sign --sbom
```

**Using Just (Legacy):**

```bash
# The Justfile is still available for compatibility
just build                  # Build container with podman
just build-iso              # Build ISO (calls galena CLI internally)
just run-vm                 # Run VM for testing
```

### Configuration

Project configuration is in `galena.yaml`:

```yaml
name: galena
description: OCI-native OS appliance built on Universal Blue
registry: ghcr.io
repository: iiroan

build:
  base_image: ghcr.io/ublue-os/bluefin-dx:stable
  fedora_version: "42"

variants:
  - name: main
    description: Standard desktop variant with GNOME
    scripts:
      - 10-build.sh
      - 15-nymvpn.sh
      - 20-branding.sh
      - 40-firstboot-services.sh
```

## Development

### Getting Started

**Best Practice: Develop on a Universal Blue image**

For the smoothest development experience, use a Universal Blue-based system (Bluefin, Bazzite, Aurora). These images include:

- All required build tools (Podman, bootc-image-builder)
- Go toolchain (via Homebrew or system packages)
- Proper container runtime configuration
- Development utilities pre-configured

**If you're not on Universal Blue:**

1. Install Go 1.23+ (see "Installing Go" section above)
2. Install Podman: `sudo dnf install podman` (Fedora/RHEL) or `sudo apt install podman` (Ubuntu)
3. Ensure your user is in the `docker` or `podman` group for rootless containers

**Build the CLI first:**

```bash
# Clone and build
git clone https://github.com/iiroan/galena.git
cd galena
make build

# Verify
./galena version
```

### Project Structure

```
├── cmd/galena/          # Go CLI source code
│   ├── cmd/             # Cobra commands (build, disk, vm, etc.)
│   └── main.go          # CLI entry point
├── internal/            # Internal Go packages
│   ├── build/           # Build logic (container, disk, VM)
│   ├── config/          # Configuration management
│   ├── ui/              # TUI components (Charm libraries)
│   └── validate/        # Validation logic
├── build/               # Build-time scripts (run during image build)
│   ├── 10-build.sh      # Main build script
│   ├── 15-nymvpn.sh     # NymVPN daemon installation
│   ├── 20-branding.sh   # Custom branding
│   └── 40-firstboot-services.sh
├── custom/              # Runtime customizations
│   ├── brew/            # Homebrew Brewfiles
│   ├── flatpaks/        # Flatpak preinstall configs
│   ├── ujust/           # User commands (ujust recipes)
│   └── system_files/    # System files (wallpapers, icons, etc.)
├── Containerfile        # Multi-stage container build
├── galena.yaml          # Project configuration
├── Justfile             # Just recipes (legacy compatibility)
└── Makefile             # Make targets for Go CLI
```

### Adding Customizations

**Build-time packages** (baked into image):

```bash
# Edit build/10-build.sh
dnf5 install -y package-name
```

**Runtime CLI tools** (installed by user):

```bash
# Edit custom/brew/default.Brewfile
brew "package-name"
```

**GUI applications** (installed on first boot):

```bash
# Edit custom/flatpaks/default.preinstall
[Flatpak Preinstall org.app.Name]
Branch=stable
```

**User commands** (ujust shortcuts):

```bash
# Edit custom/ujust/custom-apps.just
[group('Apps')]
my-command:
    echo "Hello from ujust!"
```

### Validation

The project includes comprehensive validation that runs on every PR:

```bash
./galena validate              # Run all checks
./galena validate --only shellcheck
./galena validate --only brew
./galena validate --only flatpak
./galena validate --only just
```

Validation checks:

- ✓ Shell script syntax (shellcheck)
- ✓ Brewfile syntax (brew bundle check)
- ✓ Flatpak app IDs (flathub verification)
- ✓ Just file syntax
- ✓ Containerfile lint (bootc container lint)
- ✓ Configuration schema (galena.yaml)

## CI/CD

GitHub Actions automatically:

- Builds on every push to `main`
- Creates `:stable` and datestamped tags
- Validates PRs before merge
- Signs images with cosign (optional)
- Generates SBOMs with Trivy
- Cleans old images (>90 days)
- Updates dependencies via Renovate

**Image Tags:**

- `ghcr.io/iiroan/galena:stable` - Latest stable release
- `ghcr.io/iiroan/galena:stable.YYYYMMDD` - Datestamped release
- `ghcr.io/iiroan/galena:testing` - PR builds (same-repo PRs only)

## Troubleshooting

### "go: command not found"

Install Go (see "Installing Go" section above). On Universal Blue:

```bash
brew install go
```

### "bootc-image-builder: permission denied"

Disk image builds require sudo:

```bash
sudo ./galena disk iso
```

### "podman: command not found"

Podman should be pre-installed on Universal Blue images. If missing, you can layer it:

```bash
# On Universal Blue (immutable)
rpm-ostree install podman
sudo systemctl reboot
```

### Build fails on non-Linux systems

The `galena` CLI requires Linux for disk image builds (bootc-image-builder limitation). Container builds work on any platform with Podman.

**Workaround for macOS/Windows:**

- Use GitHub Actions for builds (automatic on push)
- Use a Linux VM or container
- Use WSL2 on Windows

### "Cannot connect to Podman socket"

Ensure Podman is running:

```bash
systemctl --user start podman.socket
systemctl --user enable podman.socket
```

## Credits

Built with:

- [Universal Blue](https://universal-blue.org/) - The immutable Linux ecosystem
- [Bluefin](https://projectbluefin.io/) - The developer-focused desktop
- [bootc](https://containers.github.io/bootc/) - Container-native OS updates
- [Charm](https://charm.sh/) - Beautiful CLI tools (Bubble Tea, Lipgloss, Huh)
- [Cobra](https://cobra.dev/) - CLI framework
- [Podman](https://podman.io/) - Container engine

## License

MIT License - See [LICENSE](LICENSE) for details
