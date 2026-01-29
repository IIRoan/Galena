# Galena

My personal Linux OS built on [Bluefin DX](https://projectbluefin.io/). An immutable, container-native desktop that just works.

[![Download ISO](https://img.shields.io/badge/Download-ISO-blue?style=for-the-badge&logo=linux)](https://s3.roan.dev/install-latest.iso)

## What's Inside

Galena comes with everything I need pre-configured. Based on Bluefin DX (Fedora Silverblue + GNOME), it includes development tools out of the box and adds my preferred apps and customizations.

### Pre-installed Apps

**Browsers**
- Firefox
- Zen Browser

**Privacy & Security**
- NymVPN (daemon + app) - mixnet-based privacy

**Communication & Media**
- Vesktop (Discord client)
- Spotify
- Clapper (video player)

**Development**
- Termius (SSH client)
- Full Bluefin DX toolbox (Podman, distrobox, etc.)

**System Utilities**
- Mission Center (system monitor)
- Flatseal (Flatpak permissions)
- Warehouse (Flatpak manager)
- Extension Manager
- Impression (USB writer)
- Pinta (image editor)

**GNOME Essentials**
- Calculator, Weather, Clocks
- Text Editor, Papers (PDF viewer)
- Loupe (image viewer), File Roller
- Contacts, Connections, and more

### CLI Tools (via Homebrew)

```
bat       - cat with syntax highlighting
eza       - modern ls
fd        - fast find
rg        - ripgrep (fast grep)
gh        - GitHub CLI
starship  - cross-shell prompt
zoxide    - smarter cd
htop      - process viewer
tmux      - terminal multiplexer
```

## Installation

### From ISO (recommended)

[Download the latest ISO](https://s3.roan.dev/install-latest.iso) and boot from USB. The Anaconda installer will guide you through disk selection, encryption, and user setup.

### Switch an existing system

```bash
sudo bootc switch ghcr.io/iiroan/galena:main
sudo systemctl reboot
```

## Building

This repo uses GitHub Actions to build automatically on every push. You can also build locally.

### finctl CLI

Galena includes `finctl`, a custom Go CLI built with [Charm](https://charm.sh/) (bubbletea/lipgloss) for a beautiful terminal experience. It handles image building, signing, disk image creation, and more.

**First, build the CLI:**

```bash
go build -o finctl ./cmd/finctl/
```

**Available commands:**

```bash
./finctl disk <type>     # Build disk images (iso, anaconda-iso, qcow2, raw, vmdk)
./finctl sign <image>    # Sign images with cosign
./finctl sbom <image>    # Generate SBOM with Syft
./finctl version         # Show version info
```

### Build Examples

```bash
# Build the container image
just build

# Build an interactive ISO installer (requires sudo for bootc-image-builder)
sudo ./finctl disk anaconda-iso --image ghcr.io/iiroan/galena:main

# Build a VM disk image
sudo ./finctl disk qcow2 --image ghcr.io/iiroan/galena:main
```

## Credits

Built with:
- [Universal Blue](https://universal-blue.org/) - The immutable Linux ecosystem
- [Bluefin](https://projectbluefin.io/) - The developer-focused desktop
- [bootc](https://containers.github.io/bootc/) - Container-native OS updates
