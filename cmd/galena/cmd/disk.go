package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/build"
	"github.com/iiroan/galena/internal/platform"
	"github.com/iiroan/galena/internal/ui"
)

var (
	diskImage       string
	diskOutputDir   string
	diskConfigFile  string
	diskRootFS      string
	diskUseJust     bool
	diskInteractive bool
)

var diskCmd = &cobra.Command{
	Use:   "disk <type>",
	Short: "Build bootable disk images (qcow2, raw, iso)",
	Long: `Build bootable disk images using bootc-image-builder.

Supported output types:
  qcow2           - QCOW2 disk image (for QEMU/KVM)
  raw             - Raw disk image
  iso             - Bootable ISO installer
  vmdk            - VMware disk image
  ami             - Amazon Machine Image
  anaconda-iso    - Anaconda-based installer ISO
  bootc-installer - Bootc installer ISO

Examples:
  # Build a QCOW2 image for VM testing
  galena-build disk qcow2

  # Build an ISO installer
  galena-build disk iso

  # Build with a specific image reference
  galena-build disk qcow2 --image ghcr.io/myorg/myimage:stable

  # Build with custom output directory
  galena-build disk qcow2 --output ./images

  # Use existing Justfile recipes
  galena-build disk qcow2 --just`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: build.ListOutputTypes(),
	RunE:      runDisk,
}

func init() {
	diskCmd.Flags().StringVar(&diskImage, "image", "", "Source container image (default: local build)")
	diskCmd.Flags().StringVarP(&diskOutputDir, "output", "o", "", "Output directory (default: ./output)")
	diskCmd.Flags().StringVar(&diskConfigFile, "config", "", "Disk config TOML file")
	diskCmd.Flags().StringVar(&diskRootFS, "rootfs", "ext4", "Root filesystem type (ext4, xfs, btrfs)")
	diskCmd.Flags().BoolVar(&diskUseJust, "just", false, "Use existing Justfile recipes")
	diskCmd.Flags().BoolVarP(&diskInteractive, "interactive", "i", false, "Interactive mode")
}

func runDisk(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	outputType := args[0]

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if err := platform.RequireLinux("disk builds"); err != nil {
		return err
	}

	// Interactive mode
	if diskInteractive {
		if err := promptDiskOptions(&outputType); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
	}

	diskBuilder := build.NewDiskBuilder(cfg, rootDir, logger)

	// Use just if requested
	if diskUseJust {
		return diskBuilder.BuildViaJust(ctx, outputType)
	}

	// Determine image reference
	imageRef := diskImage
	if imageRef == "" {
		imageRef = cfg.ImageRef("main", "latest")
	}

	opts := build.DefaultDiskOptions()
	opts.ImageRef = imageRef
	opts.OutputType = outputType
	opts.OutputDir = diskOutputDir
	opts.ConfigFile = diskConfigFile
	if diskRootFS != "" {
		opts.RootFSType = diskRootFS
	}

	outputPath, err := diskBuilder.Build(ctx, opts)
	if err != nil {
		return err
	}

	// Print success message
	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf(
		"Disk image created successfully!\n\nType: %s\nOutput: %s",
		outputType,
		outputPath,
	)))

	return nil
}

func promptDiskOptions(outputType *string) error {
	advancedMode := ui.CurrentPreferences.Advanced
	typeOptions := make([]huh.Option[string], 0)
	for _, t := range build.ListOutputTypes() {
		typeOptions = append(typeOptions, huh.NewOption(t, t))
	}

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select output type").
				Description("What type of disk image to create?").
				Options(typeOptions...).
				Value(outputType),

			huh.NewInput().
				Title("Image reference").
				Description("Container image to convert (leave empty for local build)").
				Value(&diskImage),
		),
	}

	if advancedMode {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Output directory").
				Description("Where to save the disk image").
				Placeholder("./output").
				Value(&diskOutputDir),
			huh.NewInput().
				Title("Config file").
				Description("Optional bootc-image-builder TOML config").
				Placeholder("iso/disk.toml").
				Value(&diskConfigFile),
			huh.NewSelect[string]().
				Title("Root filesystem").
				Description("Filesystem for the disk image").
				Options(
					huh.NewOption("ext4", "ext4"),
					huh.NewOption("xfs", "xfs"),
					huh.NewOption("btrfs", "btrfs"),
				).
				Value(&diskRootFS),
			huh.NewConfirm().
				Title("Use Justfile").
				Description("Run disk build via existing Just recipes").
				Value(&diskUseJust),
		))
	}

	form := huh.NewForm(groups...)

	return form.WithTheme(ui.HuhTheme()).Run()
}
