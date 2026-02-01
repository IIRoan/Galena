// Package cmd provides the CLI commands for finctl
package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/build"
	"github.com/finpilot/finctl/internal/config"
	"github.com/finpilot/finctl/internal/ui"
)

var (
	// Global flags
	verbose    bool
	quiet      bool
	noColor    bool
	cfgFile    string
	projectDir string

	// Global logger
	logger *log.Logger

	// Global config
	cfg *config.Config
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "finctl",
	Short: "Build and manage OCI-native OS images",
	Long: ui.Banner() + `
finctl is a CLI tool for building, testing, and deploying
OCI-native bootable operating system images.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Setup logger
		setupLogger()

		// Load config (unless we're running init)
		if cmd.Name() != "init" && cmd.Name() != "version" && cmd.Name() != "help" {
			var err error
			if cfgFile != "" {
				cfg, err = config.Load(cfgFile)
			} else {
				cfg, err = config.LoadFromProject()
			}
			if err != nil {
				logger.Warn("could not load config, using defaults", "error", err)
				cfg = config.DefaultConfig()
			}
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return runRootTUI()
		}
		return cmd.Help()
	},
}

func runRootTUI() error {
	fmt.Println(ui.Banner())
	fmt.Println(ui.HeaderStyle.Render(" MAIN MENU "))
	fmt.Println()

	var choice string
	err := huh.NewSelect[string]().
		Title("Control Plane").
		Description("What would you like to do?").
		Options(
			huh.NewOption("Build (Interactive Wizard)", "build"),
			huh.NewOption("Fast Build (Local Image + ISO)", "fast-build"),
			huh.NewOption("Status (System Check)", "status"),
			huh.NewOption("Init (Setup Project)", "init"),
			huh.NewOption("Clean (Purge Build Files)", "clean"),
			huh.NewOption("Exit", "exit"),
		).
		Value(&choice).
		Run()

	if err != nil {
		return err
	}

	switch choice {
	case "build":
		return buildCmd.RunE(buildCmd, []string{})
	case "fast-build":
		return runFastBuild()
	case "status":
		return statusCmd.RunE(statusCmd, []string{})
	case "init":
		return initCmd.RunE(initCmd, []string{})
	case "clean":
		return cleanCmd.RunE(cleanCmd, []string{})
	case "exit":
		return nil
	}

	return nil
}

func runFastBuild() error {
	rootDir, err := getProjectRoot()
	if err != nil {
		return err
	}

	// Setup logging for this session
	logDir := filepath.Join(rootDir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, fmt.Sprintf("fast-build-%s.log", time.Now().Format("20060102-150405")))

	ctx := context.WithValue(context.Background(), "build-phase", "fast-build")
	ctx = context.WithValue(ctx, "log-file", logFile)

	fmt.Println(ui.Banner())
	fmt.Println(ui.WizardTitle.Render(" FAST BUILD "))
	fmt.Println(ui.WizardDescription.Render("Building local container image and standard ISO..."))
	fmt.Println(ui.MutedStyle.Render("Logging session to: " + logFile))
	fmt.Println()

	// 1. Build Container
	builder := build.NewBuilder(cfg, rootDir, logger)
	buildOpts := build.DefaultBuildOptions()
	buildOpts.Variant = "main"
	buildOpts.Tag = "latest"

	fmt.Println(ui.WizardStep.Render("▶ Step 1: Building OCI Container..."))
	_, err = builder.Build(ctx, buildOpts)
	if err != nil {
		return fmt.Errorf("container build failed (check %s): %w", logFile, err)
	}
	fmt.Println(ui.SuccessStyle.Render("✔ Container build complete"))

	// 2. Build ISO
	fmt.Println(ui.WizardStep.Render("▶ Step 2: Generating ISO Installer..."))
	diskBuilder := build.NewDiskBuilder(cfg, rootDir, logger)
	diskOpts := build.DefaultDiskOptions()
	diskOpts.ImageRef = cfg.ImageRef("main", "latest")
	diskOpts.OutputType = "iso"

	outputPath, err := diskBuilder.Build(ctx, diskOpts)
	if err != nil {
		return fmt.Errorf("iso build failed (check %s): %w", logFile, err)
	}

	fmt.Println(ui.SuccessStyle.Render("✔ ISO generation complete"))
	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf(
		"Fast Build Finished!\n\nISO Location: %s\nLog File: %s",
		outputPath,
		logFile,
	)))

	return nil
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: finctl.yaml)")
	rootCmd.PersistentFlags().StringVarP(&projectDir, "project", "C", "", "Project directory")

	// Add subcommands
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(diskCmd)
	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(sbomCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(validateCmd)
}

func setupLogger() {
	// Determine log level
	level := log.InfoLevel
	if verbose {
		level = log.DebugLevel
	}
	if quiet {
		level = log.WarnLevel
	}

	// Setup styles
	styles := log.DefaultStyles()
	if !noColor && os.Getenv("NO_COLOR") == "" {
		styles.Levels[log.DebugLevel] = lipgloss.NewStyle().
			SetString("DEBUG").
			Foreground(ui.Muted).
			Bold(true)
		styles.Levels[log.InfoLevel] = lipgloss.NewStyle().
			SetString("INFO").
			Foreground(ui.Primary).
			Bold(true)
		styles.Levels[log.WarnLevel] = lipgloss.NewStyle().
			SetString("WARN").
			Foreground(ui.Warning).
			Bold(true)
		styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
			SetString("ERROR").
			Foreground(ui.Error).
			Bold(true)
	}

	logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: verbose,
		TimeFormat:      time.Kitchen,
		Level:           level,
	})
	logger.SetStyles(styles)
}

// getProjectRoot returns the project root directory
func getProjectRoot() (string, error) {
	if projectDir != "" {
		return projectDir, nil
	}
	return config.FindProjectRoot()
}
