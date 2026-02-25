package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	galexec "github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var manageStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show runtime device status",
	Long: `Display runtime status for device management:
  - Operating system details
  - Setup marker files
  - Tool availability (bootc, brew, flatpak, ujust)
  - Catalog coverage for Brewfile and Flatpak manifests`,
	RunE: runManageStatus,
}

func runManageStatus(cmd *cobra.Command, args []string) error {
	ctx := context.TODO()
	if cmd != nil && cmd.Context() != nil {
		ctx = cmd.Context()
	}

	ui.StartScreen("DEVICE STATUS", "Runtime overview for this Galena installation")

	fmt.Println(ui.Title.Render("System"))
	printKV("OS", readOSReleaseValue("PRETTY_NAME", "unknown"))
	printKV("Setup Done", markerStatus("/var/lib/galena/setup.done"))
	printKV("VS Code Init", markerStatus("/var/lib/galena/vscode-settings.done"))

	fmt.Println()
	fmt.Println(ui.Title.Render("Tooling"))
	printTool("bootc")
	printTool("brew")
	printTool("flatpak")
	printTool("ujust")
	printTool("galena-build")

	fmt.Println()
	fmt.Println(ui.Title.Render("Catalog Coverage"))
	printCatalogCoverage(catalogKindBrew, "Brew")
	printCatalogCoverage(catalogKindFlatpak, "Flatpak")

	if galexec.CheckCommand("bootc") {
		fmt.Println()
		fmt.Println(ui.Title.Render("Bootc Status"))
		result := galexec.RunSimple(ctx, "bootc", "status")
		if result.Err != nil {
			fmt.Println(ui.MutedStyle.Render("  bootc status unavailable"))
		} else {
			lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
			if len(lines) > 8 {
				lines = lines[:8]
			}
			for _, line := range lines {
				if strings.TrimSpace(line) == "" {
					continue
				}
				fmt.Printf("  %s\n", line)
			}
		}
	}

	return nil
}

func printCatalogCoverage(kind catalogKind, label string) {
	items, err := loadCatalogForKinds([]catalogKind{kind})
	if err != nil {
		printKV(label, ui.MutedStyle.Render("unavailable ("+err.Error()+")"))
		return
	}
	total := len(items)
	installed := 0
	for _, item := range items {
		if item.Installed {
			installed++
		}
	}
	printKV(label, fmt.Sprintf("%d/%d installed", installed, total))
}

func printTool(name string) {
	state := ui.StatusError.String()
	details := ui.MutedStyle.Render("(not found)")
	if galexec.CheckCommand(name) {
		state = ui.StatusSuccess.String()
		details = ui.MutedStyle.Render("(available)")
	}
	fmt.Printf("  %s %-14s %s\n", state, name, details)
}

func markerStatus(path string) string {
	if _, err := os.Stat(path); err == nil {
		return ui.SuccessStyle.Render("present")
	}
	return ui.MutedStyle.Render("missing")
}

func readOSReleaseValue(key string, fallback string) string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fallback
	}
	prefix := key + "="
	for _, rawLine := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(rawLine)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		value := strings.TrimPrefix(line, prefix)
		value = strings.Trim(value, `"`)
		if value == "" {
			return fallback
		}
		return value
	}
	return fallback
}
