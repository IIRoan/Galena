package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/finpilot/finctl/internal/config"
	"github.com/finpilot/finctl/internal/exec"
)

// VMRunner runs virtual machines for testing
type VMRunner struct {
	cfg     *config.Config
	rootDir string
	logger  *log.Logger
}

// VMOptions configures VM execution
type VMOptions struct {
	ImagePath string
	Memory    string // e.g., "4G"
	CPUs      int
	Display   string // gtk, sdl, vnc, none
	SSH       bool
	SSHPort   int
	KVM       bool
	UEFI      bool
}

// DefaultVMOptions returns default VM options
func DefaultVMOptions() VMOptions {
	return VMOptions{
		Memory:  "4G",
		CPUs:    2,
		Display: "gtk",
		SSH:     true,
		SSHPort: 2222,
		KVM:     true,
		UEFI:    true,
	}
}

// NewVMRunner creates a new VM runner
func NewVMRunner(cfg *config.Config, rootDir string, logger *log.Logger) *VMRunner {
	return &VMRunner{
		cfg:     cfg,
		rootDir: rootDir,
		logger:  logger,
	}
}

// Run starts a VM with the given disk image
func (v *VMRunner) Run(ctx context.Context, opts VMOptions) error {
	// Validate
	if opts.ImagePath == "" {
		return fmt.Errorf("image path is required")
	}

	if _, err := os.Stat(opts.ImagePath); err != nil {
		return fmt.Errorf("image not found: %s", opts.ImagePath)
	}

	// Determine which QEMU to use
	qemuBinary := "qemu-system-x86_64"
	if err := exec.RequireCommands(qemuBinary); err != nil {
		return err
	}

	v.logger.Info("starting VM",
		"image", opts.ImagePath,
		"memory", opts.Memory,
		"cpus", opts.CPUs,
	)

	args := v.buildQEMUArgs(opts)

	v.logger.Debug("running qemu", "args", args)

	execOpts := exec.DefaultOptions()
	execOpts.StreamStdio = true

	result := exec.Run(ctx, qemuBinary, args, execOpts)
	if result.Err != nil {
		v.logger.Error("qemu failed", "error", result.Err)
		return result.Err
	}

	return nil
}

// buildQEMUArgs constructs QEMU arguments
func (v *VMRunner) buildQEMUArgs(opts VMOptions) []string {
	args := []string{
		"-m", opts.Memory,
		"-smp", fmt.Sprintf("%d", opts.CPUs),
	}

	// KVM acceleration
	if opts.KVM {
		args = append(args, "-enable-kvm", "-cpu", "host")
	}

	// UEFI firmware
	if opts.UEFI {
		// Try common OVMF paths
		ovmfPaths := []string{
			"/usr/share/edk2/ovmf/OVMF_CODE.fd",
			"/usr/share/OVMF/OVMF_CODE.fd",
			"/usr/share/qemu/OVMF.fd",
		}
		for _, path := range ovmfPaths {
			if _, err := os.Stat(path); err == nil {
				args = append(args, "-bios", path)
				break
			}
		}
	}

	// Display
	switch opts.Display {
	case "none":
		args = append(args, "-nographic")
	case "vnc":
		args = append(args, "-vnc", ":0")
	default:
		args = append(args, "-display", opts.Display)
	}

	// Disk image
	ext := filepath.Ext(opts.ImagePath)
	format := "raw"
	if ext == ".qcow2" {
		format = "qcow2"
	}
	args = append(args, "-drive", fmt.Sprintf("file=%s,format=%s,if=virtio", opts.ImagePath, format))

	// Network with SSH forwarding
	if opts.SSH {
		args = append(args, "-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp::%d-:22", opts.SSHPort))
		args = append(args, "-device", "virtio-net-pci,netdev=net0")
	} else {
		args = append(args, "-net", "none")
	}

	return args
}

// RunViaJust runs VM using the existing Justfile
func (v *VMRunner) RunViaJust(ctx context.Context, image string) error {
	if err := exec.RequireCommands("just"); err != nil {
		return err
	}

	v.logger.Info("running VM via just", "image", image)

	result := exec.Just(ctx, v.rootDir, "run-vm", image)
	if result.Err != nil {
		v.logger.Error("just run-vm failed",
			"exit_code", result.ExitCode,
			"stderr", exec.LastNLines(result.Stderr, 20),
		)
		return result.Err
	}

	return nil
}

// SSH connects to a running VM via SSH
func (v *VMRunner) SSH(ctx context.Context, port int, user string) error {
	if err := exec.RequireCommands("ssh"); err != nil {
		return err
	}

	if user == "" {
		user = "finpilot"
	}
	if port == 0 {
		port = 2222
	}

	v.logger.Info("connecting to VM", "port", port, "user", user)

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-p", fmt.Sprintf("%d", port),
		fmt.Sprintf("%s@localhost", user),
	}

	execOpts := exec.DefaultOptions()
	execOpts.StreamStdio = true

	result := exec.Run(ctx, "ssh", args, execOpts)
	if result.Err != nil {
		return result.Err
	}

	return nil
}

// FindDiskImage finds the most recent disk image in the output directory
func (v *VMRunner) FindDiskImage(outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = filepath.Join(v.rootDir, "output")
	}

	// Look for common disk image extensions
	extensions := []string{".qcow2", ".raw", ".img"}

	var newest string
	var newestTime int64

	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		for _, e := range extensions {
			if ext == e {
				if info.ModTime().Unix() > newestTime {
					newest = path
					newestTime = info.ModTime().Unix()
				}
				break
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walking output directory: %w", err)
	}

	if newest == "" {
		return "", fmt.Errorf("no disk image found in %s", outputDir)
	}

	return newest, nil
}
