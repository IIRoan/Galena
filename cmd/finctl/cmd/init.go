package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/config"
	"github.com/finpilot/finctl/internal/ui"
)

var (
	initName       string
	initRegistry   string
	initRepo       string
	initBase       string
	initFedora     string
	initForce      bool
)

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Initialize a new finctl project",
	Long: `Initialize a new finctl project with a configuration file.

This creates a finctl.yaml configuration file with sensible defaults.
You can customize the project name, registry, and base image.

Examples:
  # Initialize in current directory
  finctl init

  # Initialize with a specific name
  finctl init --name myos

  # Initialize with custom registry
  finctl init --name myos --registry ghcr.io --repo myorg`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().StringVarP(&initName, "name", "n", "", "Project name")
	initCmd.Flags().StringVar(&initRegistry, "registry", "ghcr.io", "Container registry")
	initCmd.Flags().StringVar(&initRepo, "repo", "", "Repository namespace (e.g., myorg)")
	initCmd.Flags().StringVar(&initBase, "base", "ghcr.io/ublue-os/silverblue-main", "Base image")
	initCmd.Flags().StringVar(&initFedora, "fedora", "42", "Fedora major version")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing configuration")
}

func runInit(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	configPath := filepath.Join(dir, "finctl.yaml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil && !initForce {
		return fmt.Errorf("configuration already exists at %s (use --force to overwrite)", configPath)
	}

	// Interactive mode if name not provided
	if initName == "" {
		if err := promptInitOptions(dir); err != nil {
			return err
		}
	}

	// Create config
	cfg := config.DefaultConfig()
	cfg.Name = initName
	cfg.Registry = initRegistry
	cfg.Repository = initRepo
	cfg.Build.BaseImage = initBase
	cfg.Build.FedoraVersion = initFedora

	// Add description
	cfg.Description = fmt.Sprintf("%s - OCI-native OS appliance", initName)

	// Save config
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("saving configuration: %w", err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf(
		"Project initialized successfully!\n\nConfig: %s\n\nNext steps:\n  1. Edit finctl.yaml to customize your build\n  2. Run 'finctl build' to build your image\n  3. Run 'finctl disk qcow2' to create a VM image\n  4. Run 'finctl vm run' to test in a VM",
		configPath,
	)))

	return nil
}

func promptInitOptions(dir string) error {
	// Try to detect name from directory
	defaultName := filepath.Base(dir)
	if defaultName == "." {
		cwd, _ := os.Getwd()
		defaultName = filepath.Base(cwd)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project name").
				Description("Name for your OS image").
				Placeholder(defaultName).
				Value(&initName),

			huh.NewInput().
				Title("Registry").
				Description("Container registry (e.g., ghcr.io)").
				Placeholder("ghcr.io").
				Value(&initRegistry),

			huh.NewInput().
				Title("Repository").
				Description("Registry namespace (e.g., myorg)").
				Value(&initRepo),

			huh.NewSelect[string]().
				Title("Base image").
				Description("What base image to build from?").
				Options(
					huh.NewOption("silverblue-main (GNOME)", "ghcr.io/ublue-os/silverblue-main"),
					huh.NewOption("kinoite-main (KDE)", "ghcr.io/ublue-os/kinoite-main"),
					huh.NewOption("base-main (Minimal)", "ghcr.io/ublue-os/base-main"),
					huh.NewOption("bluefin (Developer)", "ghcr.io/ublue-os/bluefin"),
				).
				Value(&initBase),

			huh.NewSelect[string]().
				Title("Fedora version").
				Description("Which Fedora major version?").
				Options(
					huh.NewOption("42 (Latest)", "42"),
					huh.NewOption("41", "41"),
					huh.NewOption("40", "40"),
				).
				Value(&initFedora),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Use defaults if empty
	if initName == "" {
		initName = defaultName
	}
	if initRegistry == "" {
		initRegistry = "ghcr.io"
	}
	if initFedora == "" {
		initFedora = "42"
	}

	return nil
}
