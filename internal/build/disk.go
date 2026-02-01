package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/finpilot/finctl/internal/config"
	"github.com/finpilot/finctl/internal/exec"
)

// DiskBuilder builds disk images (qcow2, raw, iso) using bootc-image-builder
type DiskBuilder struct {
	cfg     *config.Config
	rootDir string
	logger  *log.Logger
}

// DiskOptions configures disk image generation
type DiskOptions struct {
	ImageRef   string
	OutputType string // qcow2, raw, iso, vmdk, ami
	OutputDir  string
	ConfigFile string // Path to disk config TOML (optional)
	RootFSType string // ext4, xfs, btrfs
	Timeout    time.Duration
	Privileged bool
	PullNewer  bool
}

// DefaultDiskOptions returns default disk options
func DefaultDiskOptions() DiskOptions {
	return DiskOptions{
		OutputType: "qcow2",
		RootFSType: "ext4",
		Timeout:    60 * time.Minute,
		Privileged: true,
		PullNewer:  true,
	}
}

// NewDiskBuilder creates a new disk builder
func NewDiskBuilder(cfg *config.Config, rootDir string, logger *log.Logger) *DiskBuilder {
	return &DiskBuilder{
		cfg:     cfg,
		rootDir: rootDir,
		logger:  logger,
	}
}

// Build builds a disk image using bootc-image-builder
func (d *DiskBuilder) Build(ctx context.Context, opts DiskOptions) (string, error) {
	// Validate
	if opts.ImageRef == "" {
		return "", fmt.Errorf("image reference is required")
	}

	validTypes := map[string]bool{
		"qcow2":           true,
		"raw":             true,
		"iso":             true,
		"vmdk":            true,
		"ami":             true,
		"anaconda-iso":    true,
		"bootc-installer": true,
	}
	if !validTypes[opts.OutputType] {
		return "", fmt.Errorf("invalid output type %q, valid types: qcow2, raw, iso, vmdk, ami, anaconda-iso, bootc-installer", opts.OutputType)
	}

	if err := exec.RequireCommands("podman"); err != nil {
		return "", err
	}

	// Prepare output directory
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join(d.rootDir, "output")
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	d.logger.Info("building disk image",
		"image", opts.ImageRef,
		"type", opts.OutputType,
		"output", opts.OutputDir,
	)

	// Check if image is local (already in container storage)
	isLocal := strings.HasPrefix(opts.ImageRef, "localhost/") || !strings.Contains(opts.ImageRef, "/")

	if isLocal {
		d.logger.Info("using local container image", "image", opts.ImageRef)
		// Skip pull for local images - they're already in storage
	} else {
		d.logger.Info("pulling container image", "image", opts.ImageRef)
		pullResult := exec.Run(ctx, "podman", []string{"pull", opts.ImageRef}, exec.DefaultOptions())
		if pullResult.Err != nil {
			d.logger.Error("failed to pull image",
				"exit_code", pullResult.ExitCode,
				"stderr", exec.LastNLines(pullResult.Stderr, 10),
			)
			return "", fmt.Errorf("pulling image: %w", pullResult.Err)
		}
	}

	// Prepare config file path
	configFile := opts.ConfigFile
	if configFile == "" {
		// Select default config based on output type
		var defaultConfigs []string
		if opts.OutputType == "anaconda-iso" || opts.OutputType == "bootc-installer" {
			// Interactive installers use iso.toml
			defaultConfigs = []string{
				filepath.Join(d.rootDir, "iso", "iso.toml"),
				filepath.Join(d.rootDir, "iso", "disk.toml"),
			}
		} else {
			// Direct disk images use disk.toml
			defaultConfigs = []string{
				filepath.Join(d.rootDir, "iso", "disk.toml"),
				filepath.Join(d.rootDir, "disk_config", "image.toml"),
			}
		}
		for _, cfg := range defaultConfigs {
			if _, err := os.Stat(cfg); err == nil {
				configFile = cfg
				break
			}
		}
	}

	// Build the bootc-image-builder command
	args := d.buildBIBArgs(opts, configFile)

	d.logger.Debug("running bootc-image-builder", "args", args)

	execOpts := exec.DefaultOptions()
	execOpts.StreamStdio = true
	execOpts.Timeout = opts.Timeout

	result := exec.Run(ctx, "podman", args, execOpts)
	if result.Err != nil {
		d.logger.Error("bootc-image-builder failed",
			"exit_code", result.ExitCode,
			"stderr", exec.LastNLines(result.Stderr, 20),
		)
		return "", result.Err
	}

	// Find the output file
	outputFile := d.findOutputFile(opts.OutputDir, opts.OutputType)
	if outputFile == "" {
		d.logger.Warn("could not locate output file in directory", "dir", opts.OutputDir)
		return opts.OutputDir, nil
	}

	d.logger.Info("disk image created successfully",
		"type", opts.OutputType,
		"output", outputFile,
	)

	return outputFile, nil
}

// buildBIBArgs constructs the podman arguments for bootc-image-builder
func (d *DiskBuilder) buildBIBArgs(opts DiskOptions, configFile string) []string {
	args := []string{
		"run",
		"--rm",
	}

	if opts.Privileged {
		args = append(args, "--privileged")
	}

	if opts.PullNewer {
		args = append(args, "--pull=newer")
	}

	// Security options for SELinux
	args = append(args, "--security-opt", "label=type:unconfined_t")

	// Network host for local resolution
	args = append(args, "--net=host")

	// Mount config file if provided
	if configFile != "" {
		args = append(args, "-v", configFile+":/config.toml:ro")
	}

	// Mount container storage for local images
	args = append(args,
		"-v", "/var/lib/containers/storage:/var/lib/containers/storage",
	)

	// Mount output directory
	args = append(args, "-v", opts.OutputDir+":/output")

	// The bootc-image-builder image
	args = append(args, "quay.io/centos-bootc/bootc-image-builder:latest")

	// BIB arguments
	args = append(args, "--type", opts.OutputType)

	// Align with Justfile parameters
	args = append(args, "--use-librepo=True")
	args = append(args, "--rootfs=btrfs")

	if opts.RootFSType != "" {
		args = append(args, "--rootfs", opts.RootFSType)
	}

	// Add config if present
	if configFile != "" {
		args = append(args, "--config", "/config.toml")
	}

	// The source image - pass directly (including localhost/ for local builds)
	args = append(args, opts.ImageRef)

	return args
}

// findOutputFile finds the generated output file
func (d *DiskBuilder) findOutputFile(outputDir, outputType string) string {
	extensions := map[string][]string{
		"qcow2":           {".qcow2"},
		"raw":             {".raw", ".img"},
		"iso":             {".iso"},
		"vmdk":            {".vmdk"},
		"ami":             {".raw"},
		"anaconda-iso":    {".iso"},
		"bootc-installer": {".iso"},
	}

	exts, ok := extensions[outputType]
	if !ok {
		return ""
	}

	// Walk output directory
	var found string
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		for _, ext := range exts {
			if filepath.Ext(path) == ext {
				found = path
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil && err != filepath.SkipAll {
		d.logger.Warn("failed to scan output directory", "error", err)
	}

	return found
}

// BuildViaJust builds disk image using the existing Justfile
func (d *DiskBuilder) BuildViaJust(ctx context.Context, outputType string) error {
	if err := exec.RequireCommands("just"); err != nil {
		return err
	}

	d.logger.Info("building disk image via just", "type", outputType)

	recipe := "build-" + outputType
	if outputType == "iso" {
		recipe = "build-iso"
	}

	result := exec.Just(ctx, d.rootDir, recipe)
	if result.Err != nil {
		d.logger.Error("just "+recipe+" failed",
			"exit_code", result.ExitCode,
			"stderr", exec.LastNLines(result.Stderr, 20),
		)
		return result.Err
	}

	return nil
}

// ListOutputTypes returns available output types
func ListOutputTypes() []string {
	return []string{
		"qcow2",
		"raw",
		"iso",
		"vmdk",
		"ami",
		"anaconda-iso",
		"bootc-installer",
	}
}
