package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/build"
	"github.com/finpilot/finctl/internal/exec"
	"github.com/finpilot/finctl/internal/ui"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show project and build status",
	Long: `Display the current project status including:
  - Project configuration
  - Available variants
  - Local built images
  - Tool availability

Examples:
  finctl status`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	builder := build.NewBuilder(cfg, rootDir, logger)
	status, err := builder.Status(ctx)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	// Print banner
	fmt.Println(ui.Banner())

	// Project info
	fmt.Println(ui.Title.Render("Project"))
	printKV("Name", fmt.Sprintf("%v", status["project"]))
	printKV("Root", fmt.Sprintf("%v", status["root_dir"]))
	printKV("Base Image", fmt.Sprintf("%v", status["base_image"]))
	printKV("Fedora Version", fmt.Sprintf("%v", status["fedora_version"]))

	// Variants
	fmt.Println()
	fmt.Println(ui.Title.Render("Variants"))
	if variants, ok := status["variants"].([]string); ok {
		for _, v := range variants {
			fmt.Printf("  %s %s\n", ui.StatusPending.String(), v)
		}
	}

	// Local images
	fmt.Println()
	fmt.Println(ui.Title.Render("Local Images"))
	if images, ok := status["local_images"].([]string); ok && len(images) > 0 {
		for _, img := range images {
			fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), img)
		}
	} else {
		fmt.Println(ui.MutedStyle.Render("  No local images found"))
	}

	// Tools
	fmt.Println()
	fmt.Println(ui.Title.Render("Tools"))
	tools := []string{"podman", "just", "qemu-system-x86_64", "cosign", "syft", "bootc"}
	for _, tool := range tools {
		if exec.CheckCommand(tool) {
			fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), tool)
		} else {
			fmt.Printf("  %s %s %s\n", ui.StatusError.String(), tool, ui.MutedStyle.Render("(not found)"))
		}
	}

	// Git status
	fmt.Println()
	fmt.Println(ui.Title.Render("Git"))
	gitResult := exec.Git(ctx, rootDir, "rev-parse", "--short", "HEAD")
	if gitResult.Err == nil {
		printKV("Commit", strings.TrimSpace(gitResult.Stdout))
	}
	gitResult = exec.Git(ctx, rootDir, "rev-parse", "--abbrev-ref", "HEAD")
	if gitResult.Err == nil {
		printKV("Branch", strings.TrimSpace(gitResult.Stdout))
	}
	gitResult = exec.Git(ctx, rootDir, "status", "--porcelain")
	if gitResult.Err == nil {
		if strings.TrimSpace(gitResult.Stdout) != "" {
			printKV("Status", ui.WarningStyle.Render("dirty"))
		} else {
			printKV("Status", ui.SuccessStyle.Render("clean"))
		}
	}

	return nil
}

func printKV(key, value string) {
	keyStyle := lipgloss.NewStyle().Width(16).Foreground(ui.Muted)
	fmt.Printf("  %s %s\n", keyStyle.Render(key+":"), value)
}
