package cmd

import (
	"bufio"
	"context"
	"errors"
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
	verbose    bool
	quiet      bool
	noColor    bool
	cfgFile    string
	projectDir string
	logger     *log.Logger
	cfg        *config.Config
)

type ctxKey string

const (
	buildPhaseKey ctxKey = "build-phase"
	logFileKey    ctxKey = "log-file"
)

var rootCmd = &cobra.Command{
	Use:   "finctl",
	Short: "Build and manage OCI-native OS images",
	Long: ui.Banner() + `
finctl is a CLI tool for building, testing, and deploying
OCI-native bootable operating system images.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		setupLogger()

		if cmd.Name() != "version" && cmd.Name() != "help" {
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

		applyUISettings()
		setupLogger()

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
	menuItems := []ui.MenuItem{
		{ID: "build", TitleText: "Build", Details: "Step-by-step flow for container or disk builds with a clear plan summary"},
		{ID: "fast-build", TitleText: "Fast Build", Details: "One-shot local build for a container and standard ISO"},
		{ID: "status", TitleText: "Status", Details: "Review project config, variants, local images, and tool availability"},
		{ID: "validate", TitleText: "Validate", Details: "Run config and project checks matching CI validations"},
		{ID: "settings", TitleText: "Settings", Details: "Tune theme, layout, and default build behavior"},
		{ID: "clean", TitleText: "Clean", Details: "Delete build outputs and temporary files"},
		{ID: "exit", TitleText: "Exit", Details: "Close the control plane"},
	}

	for {
		choice, err := ui.RunMenu("CONTROL PLANE", "Choose an action to continue.", menuItems)
		if err != nil {
			return runRootFallback()
		}

		if choice == ui.MenuActionQuit || choice == "exit" || choice == "" {
			return nil
		}

		if err := runRootChoice(choice); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				continue
			}
			return err
		}

		if err := waitForEnter("Press enter to return to the Control Plane"); err != nil {
			return err
		}
	}
}

func runRootChoice(choice string) error {
	switch choice {
	case "build":
		return buildCmd.RunE(buildCmd, []string{})
	case "fast-build":
		return runFastBuild()
	case "status":
		return statusCmd.RunE(statusCmd, []string{})
	case "validate":
		return validateCmd.RunE(validateCmd, []string{})
	case "clean":
		return cleanCmd.RunE(cleanCmd, []string{})
	case "settings":
		return settingsCmd.RunE(settingsCmd, []string{})
	case "exit", ui.MenuActionQuit, ui.MenuActionBack, "":
		return nil
	default:
		return nil
	}
}

func runRootFallback() error {
	ui.StartScreen("MAIN MENU", "Choose an action to continue.")
	var fallbackChoice string
	fallbackErr := huh.NewSelect[string]().
		Title("Control Plane").
		Description("What would you like to do?").
		Options(
			huh.NewOption("Build", "build"),
			huh.NewOption("Fast Build", "fast-build"),
			huh.NewOption("Status", "status"),
			huh.NewOption("Validate", "validate"),
			huh.NewOption("Settings", "settings"),
			huh.NewOption("Clean", "clean"),
			huh.NewOption("Exit", "exit"),
		).
		Value(&fallbackChoice).
		WithTheme(ui.HuhTheme()).
		Run()
	if fallbackErr != nil {
		if errors.Is(fallbackErr, huh.ErrUserAborted) {
			return nil
		}
		return fallbackErr
	}
	return runRootChoice(fallbackChoice)
}

func waitForEnter(prompt string) error {
	if !ui.IsInteractiveTerminal() {
		return nil
	}
	fmt.Println()
	fmt.Println(ui.HintStyle.Render(prompt))
	reader := bufio.NewReader(os.Stdin)
	_, err := reader.ReadString('\n')
	return err
}

func runFastBuild() error {
	rootDir, err := getProjectRoot()
	if err != nil {
		return err
	}

	logDir := filepath.Join(rootDir, "logs")
	_ = os.MkdirAll(logDir, 0755)
	logFile := filepath.Join(logDir, fmt.Sprintf("fast-build-%s.log", time.Now().Format("20060102-150405")))

	ctx := context.WithValue(context.Background(), buildPhaseKey, "fast-build")
	ctx = context.WithValue(ctx, logFileKey, logFile)
	ui.StartScreen("FAST BUILD", "Building local container image and standard ISO...")
	fmt.Println(ui.MutedStyle.Render("Logging session to: " + logFile))
	fmt.Println()

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

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (default: finctl.yaml)")
	rootCmd.PersistentFlags().StringVarP(&projectDir, "project", "C", "", "Project directory")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(diskCmd)
	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(sbomCmd)
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(settingsCmd)
}

func applyUISettings() {
	if cfg == nil {
		ui.ApplyPreferences(ui.Preferences{
			Theme:      "aurora",
			ShowBanner: true,
			Dense:      false,
			NoColor:    noColor,
			Advanced:   false,
		})
		return
	}
	ui.ApplyPreferences(ui.Preferences{
		Theme:      cfg.UI.Theme,
		ShowBanner: cfg.UI.ShowBanner,
		Dense:      cfg.UI.Dense,
		NoColor:    cfg.UI.NoColor || noColor,
		Advanced:   cfg.UI.Advanced,
	})
}

func setupLogger() {
	level := log.InfoLevel
	if verbose {
		level = log.DebugLevel
	}
	if quiet {
		level = log.WarnLevel
	}

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

func getProjectRoot() (string, error) {
	if projectDir != "" {
		return projectDir, nil
	}
	return config.FindProjectRoot()
}
