package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/ci"
	"github.com/iiroan/galena/internal/platform"
	"github.com/iiroan/galena/internal/ui"
	"github.com/iiroan/galena/internal/validate"
)

var (
	validateSkipContainerfile bool
	validateSkipBrew          bool
	validateSkipFlatpak       bool
	validateOnly              []string
	validateSkip              []string
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate project files and configuration",
	Long: `Validate project files including:
  - Configuration (galena.yaml)
  - Containerfile syntax
  - Justfile syntax
  - Shell scripts (shellcheck)
  - Go linting (golangci-lint)
  - Brewfiles
  - Flatpak files

In CI environments (GitHub Actions), output is formatted with
log groups and annotations for better integration.

Examples:
  galena-build validate
  galena-build validate --skip-containerfile  # Skip Containerfile validation
  galena-build validate --skip-brew           # Skip Brewfile validation
  galena-build validate --skip-flatpak        # Skip Flatpak validation
  galena-build validate --only shellcheck     # Run only shellcheck
  galena-build validate --only golangci       # Run only golangci-lint
  galena-build validate --skip brew,flatpak   # Skip specific checks`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().BoolVar(&validateSkipContainerfile, "skip-containerfile", false, "Skip Containerfile validation (useful in CI without podman)")
	validateCmd.Flags().BoolVar(&validateSkipBrew, "skip-brew", false, "Skip Brewfile validation")
	validateCmd.Flags().BoolVar(&validateSkipFlatpak, "skip-flatpak", false, "Skip Flatpak validation")
	validateCmd.Flags().StringArrayVar(&validateOnly, "only", nil, "Run only specific checks (config, containerfile, just, brew, flatpak, shellcheck, golangci)")
	validateCmd.Flags().StringArrayVar(&validateSkip, "skip", nil, "Skip specific checks (config, containerfile, just, brew, flatpak, shellcheck, golangci)")
}

func runValidate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	ciEnv := ci.Detect()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	if err := platform.RequireLinux("validation"); err != nil {
		return err
	}

	checks, err := resolveValidationChecks()
	if err != nil {
		return err
	}

	var errors []string
	var warnings []string
	var pending []string

	ui.StartScreen("VALIDATION", "Scan configuration and build scripts")

	sections := []validationSection{
		{
			ID:    "config",
			Title: "Configuration",
			Run: func(ctx context.Context) validate.Result {
				return validate.Config(ctx, rootDir, cfgFile)
			},
		},
		{
			ID:    "containerfile",
			Title: "Containerfile",
			Run: func(ctx context.Context) validate.Result {
				return validate.Containerfile(ctx, rootDir)
			},
		},
		{
			ID:    "just",
			Title: "Justfiles",
			Run: func(ctx context.Context) validate.Result {
				return validate.Justfiles(ctx, rootDir)
			},
		},
		{
			ID:    "brew",
			Title: "Brewfiles",
			Run: func(ctx context.Context) validate.Result {
				return validate.Brewfiles(ctx, rootDir)
			},
		},
		{
			ID:    "flatpak",
			Title: "Flatpaks",
			Run: func(ctx context.Context) validate.Result {
				return validate.Flatpaks(ctx, rootDir)
			},
		},
		{
			ID:    "shellcheck",
			Title: "Shell Scripts",
			Run: func(ctx context.Context) validate.Result {
				return validate.ShellScripts(ctx, rootDir)
			},
		},
		{
			ID:    "golangci",
			Title: "Go Lint",
			Run: func(ctx context.Context) validate.Result {
				return validate.Golangci(ctx, rootDir)
			},
		},
	}

	first := true
	for _, section := range sections {
		if !checks[section.ID] {
			continue
		}

		if !first {
			fmt.Println()
		}
		first = false

		ci.StartGroup(section.Title)
		fmt.Println(ui.Title.Render(section.Title))

		result := section.Run(ctx)
		printValidationResult(ciEnv, section.Title, result)

		errors = append(errors, result.Errors...)
		warnings = append(warnings, result.Warnings...)
		pending = append(pending, result.Pending...)
		ci.EndGroup()
	}

	fmt.Println()
	if len(errors) > 0 {
		fmt.Println(ui.ErrorBox.Render(fmt.Sprintf("Validation failed with %d error(s)", len(errors))))
		if ciEnv.IsCI {
			_ = ci.AddSummary(fmt.Sprintf("## Validation Failed\n\n%d error(s) found", len(errors)))
		}
		return fmt.Errorf("validation failed")
	}
	if len(warnings) > 0 || len(pending) > 0 {
		fmt.Println(ui.InfoBox.Render(fmt.Sprintf("Validation passed with %d warning(s)", len(warnings))))
		if ciEnv.IsCI {
			_ = ci.AddSummary(fmt.Sprintf("## Validation Passed\n\n%d warning(s)", len(warnings)))
		}
		return nil
	}

	fmt.Println(ui.SuccessBox.Render("Validation passed!"))
	if ciEnv.IsCI {
		_ = ci.AddSummary("## Validation Passed\n\nAll checks passed successfully!")
	}
	return nil
}

type validationSection struct {
	ID    string
	Title string
	Run   func(context.Context) validate.Result
}

func printValidationResult(ciEnv *ci.Environment, title string, result validate.Result) {
	for _, item := range result.Items {
		status := statusIcon(item.Status)
		if item.Details != "" {
			fmt.Printf("  %s %s %s\n", status, item.Name, ui.MutedStyle.Render("("+item.Details+")"))
		} else {
			fmt.Printf("  %s %s\n", status, item.Name)
		}

		if ciEnv.IsCI {
			switch item.Status {
			case validate.StatusError:
				ci.LogError(fmt.Sprintf("%s: %s", title, item.Name), "", 0)
			case validate.StatusWarning:
				ci.LogWarning(fmt.Sprintf("%s: %s", title, item.Name))
			}
		}
	}

	for _, msg := range result.Errors {
		if ciEnv.IsCI {
			ci.LogError(msg, "", 0)
		}
	}
	for _, msg := range result.Warnings {
		if ciEnv.IsCI {
			ci.LogWarning(msg)
		}
	}
}

func statusIcon(status validate.Status) string {
	switch status {
	case validate.StatusSuccess:
		return ui.StatusSuccess.String()
	case validate.StatusError:
		return ui.StatusError.String()
	case validate.StatusWarning:
		return ui.StatusPending.String()
	case validate.StatusPending:
		return ui.StatusPending.String()
	default:
		return ui.StatusPending.String()
	}
}

func resolveValidationChecks() (map[string]bool, error) {
	all := map[string]bool{
		"config":        true,
		"containerfile": true,
		"just":          true,
		"brew":          true,
		"flatpak":       true,
		"shellcheck":    true,
		"golangci":      true,
	}

	selected := map[string]bool{}
	if len(validateOnly) == 0 {
		for key := range all {
			selected[key] = true
		}
	} else {
		for _, entry := range validateOnly {
			for _, token := range splitList(entry) {
				name, ok := normalizeCheck(token)
				if !ok {
					return nil, fmt.Errorf("unknown check: %s", token)
				}
				selected[name] = true
			}
		}
	}

	for _, entry := range validateSkip {
		for _, token := range splitList(entry) {
			name, ok := normalizeCheck(token)
			if !ok {
				return nil, fmt.Errorf("unknown check: %s", token)
			}
			selected[name] = false
		}
	}

	if validateSkipContainerfile {
		selected["containerfile"] = false
	}
	if validateSkipBrew {
		selected["brew"] = false
	}
	if validateSkipFlatpak {
		selected["flatpak"] = false
	}

	any := false
	for _, enabled := range selected {
		if enabled {
			any = true
			break
		}
	}
	if !any {
		return nil, fmt.Errorf("no validation checks selected")
	}

	return selected, nil
}

func normalizeCheck(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "config", "cfg":
		return "config", true
	case "container", "containerfile", "container-file":
		return "containerfile", true
	case "just", "justfile", "justfiles":
		return "just", true
	case "brew", "brewfile", "brewfiles":
		return "brew", true
	case "flatpak", "flatpaks", "flatpakfile", "flatpakfiles":
		return "flatpak", true
	case "shell", "shellcheck", "shellchecks", "shell-script", "shell-scripts":
		return "shellcheck", true
	case "golangci", "golangci-lint", "golint", "go-lint":
		return "golangci", true
	default:
		return "", false
	}
}

func splitList(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
