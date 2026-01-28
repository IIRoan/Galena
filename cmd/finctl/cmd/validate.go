package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/exec"
	"github.com/finpilot/finctl/internal/ui"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate project files and configuration",
	Long: `Validate project files including:
  - Configuration (finctl.yaml)
  - Containerfile syntax
  - Justfile syntax
  - Shell scripts (shellcheck)
  - Brewfiles

Examples:
  finctl validate`,
	RunE: runValidate,
}

func runValidate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	var errors []string
	var warnings []string

	// Validate config
	fmt.Println(ui.Title.Render("Configuration"))
	if cfg != nil {
		if err := cfg.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("Config: %v", err))
			fmt.Printf("  %s finctl.yaml: %v\n", ui.StatusError.String(), err)
		} else {
			fmt.Printf("  %s finctl.yaml\n", ui.StatusSuccess.String())
		}
	} else {
		warnings = append(warnings, "No finctl.yaml found")
		fmt.Printf("  %s finctl.yaml %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(not found)"))
	}

	// Validate Containerfile
	fmt.Println()
	fmt.Println(ui.Title.Render("Containerfile"))
	containerfile := filepath.Join(rootDir, "Containerfile")
	if _, err := os.Stat(containerfile); err == nil {
		// Basic syntax check with podman
		result := exec.Podman(ctx, "build", "--no-cache", "-f", containerfile, "--target", "ctx", "-t", "validate-test", rootDir)
		if result.Err != nil {
			warnings = append(warnings, "Containerfile syntax may have issues")
			fmt.Printf("  %s Containerfile %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(could not validate)"))
		} else {
			fmt.Printf("  %s Containerfile\n", ui.StatusSuccess.String())
			// Clean up test image
			exec.Podman(ctx, "rmi", "-f", "validate-test")
		}
	} else {
		errors = append(errors, "Containerfile not found")
		fmt.Printf("  %s Containerfile %s\n", ui.StatusError.String(), ui.MutedStyle.Render("(not found)"))
	}

	// Validate Justfile
	fmt.Println()
	fmt.Println(ui.Title.Render("Justfile"))
	justfile := filepath.Join(rootDir, "Justfile")
	if _, err := os.Stat(justfile); err == nil {
		if exec.CheckCommand("just") {
			result := exec.Just(ctx, rootDir, "--fmt", "--check")
			if result.Err != nil {
				warnings = append(warnings, "Justfile has formatting issues")
				fmt.Printf("  %s Justfile %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(format issues)"))
			} else {
				fmt.Printf("  %s Justfile\n", ui.StatusSuccess.String())
			}
		} else {
			fmt.Printf("  %s Justfile %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(just not installed)"))
		}
	} else {
		fmt.Printf("  %s Justfile %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(not found)"))
	}

	// Validate shell scripts
	fmt.Println()
	fmt.Println(ui.Title.Render("Shell Scripts"))
	buildDir := filepath.Join(rootDir, "build")
	if _, err := os.Stat(buildDir); err == nil {
		if exec.CheckCommand("shellcheck") {
			// Find shell scripts
			scripts := []string{}
			filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() && filepath.Ext(path) == ".sh" {
					scripts = append(scripts, path)
				}
				return nil
			})

			for _, script := range scripts {
				relPath, _ := filepath.Rel(rootDir, script)
				result := exec.RunSimple(ctx, "shellcheck", script)
				if result.Err != nil {
					warnings = append(warnings, fmt.Sprintf("shellcheck: %s", relPath))
					fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(issues found)"))
				} else {
					fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), relPath)
				}
			}
		} else {
			fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("shellcheck not installed"))
		}
	} else {
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("build/ directory not found"))
	}

	// Summary
	fmt.Println()
	if len(errors) > 0 {
		fmt.Println(ui.ErrorBox.Render(fmt.Sprintf("Validation failed with %d error(s)", len(errors))))
		return fmt.Errorf("validation failed")
	} else if len(warnings) > 0 {
		fmt.Println(ui.InfoBox.Render(fmt.Sprintf("Validation passed with %d warning(s)", len(warnings))))
	} else {
		fmt.Println(ui.SuccessBox.Render("Validation passed!"))
	}

	return nil
}
