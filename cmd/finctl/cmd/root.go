// Package cmd provides the CLI commands for finctl
package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/config"
	"github.com/finpilot/finctl/internal/ui"
)

var (
	// Global flags
	verbose   bool
	quiet     bool
	noColor   bool
	cfgFile   string
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
