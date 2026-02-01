package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/config"
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
	var pending []string

	ui.StartScreen("VALIDATION", "Scan configuration and build scripts")

	fmt.Println(ui.Title.Render("Configuration"))
	configPath := cfgFile
	if configPath == "" {
		configPath = filepath.Join(rootDir, "finctl.yaml")
	}
	if _, err := os.Stat(configPath); err == nil {
		loadedCfg, err := config.Load(configPath)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Config: %v", err))
			fmt.Printf("  %s %s: %v\n", ui.StatusError.String(), filepath.Base(configPath), err)
		} else if err := loadedCfg.Validate(); err != nil {
			errors = append(errors, fmt.Sprintf("Config: %v", err))
			fmt.Printf("  %s %s: %v\n", ui.StatusError.String(), filepath.Base(configPath), err)
		} else {
			fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), filepath.Base(configPath))
		}
	} else {
		warnings = append(warnings, "No finctl.yaml found")
		fmt.Printf("  %s finctl.yaml %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(not found)"))
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("Containerfile"))
	containerfile := filepath.Join(rootDir, "Containerfile")
	if _, err := os.Stat(containerfile); err == nil {
		result := exec.Podman(ctx, "build", "--no-cache", "-f", containerfile, "--target", "ctx", "-t", "validate-test", rootDir)
		if result.Err != nil {
			warnings = append(warnings, "Containerfile syntax may have issues")
			fmt.Printf("  %s Containerfile %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(could not validate)"))
		} else {
			fmt.Printf("  %s Containerfile\n", ui.StatusSuccess.String())
			exec.Podman(ctx, "rmi", "-f", "validate-test")
		}
	} else {
		errors = append(errors, "Containerfile not found")
		fmt.Printf("  %s Containerfile %s\n", ui.StatusError.String(), ui.MutedStyle.Render("(not found)"))
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("Justfiles"))
	justfile := filepath.Join(rootDir, "Justfile")
	customJustDir := filepath.Join(rootDir, "custom", "ujust")
	justFiles := []string{}
	if _, err := os.Stat(justfile); err == nil {
		justFiles = append(justFiles, justfile)
	}
	if _, err := os.Stat(customJustDir); err == nil {
		_ = filepath.Walk(customJustDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), ".just") {
				justFiles = append(justFiles, path)
			}
			return nil
		})
	}
	if len(justFiles) == 0 {
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(no justfiles found)"))
	} else if exec.CheckCommand("just") {
		for _, file := range justFiles {
			relPath, _ := filepath.Rel(rootDir, file)
			result := exec.RunSimple(ctx, "just", "--unstable", "--fmt", "--check", "-f", file)
			if result.Err != nil {
				warnings = append(warnings, fmt.Sprintf("justfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(format issues)"))
			} else {
				fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), relPath)
			}
		}
	} else {
		warnings = append(warnings, "just not installed")
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("just not installed"))
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("Brewfiles"))
	brewDir := filepath.Join(rootDir, "custom", "brew")
	brewFiles := []string{}
	if _, err := os.Stat(brewDir); err == nil {
		_ = filepath.Walk(brewDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.Contains(info.Name(), ".Brewfile") {
				brewFiles = append(brewFiles, path)
			}
			return nil
		})
	}
	if len(brewFiles) == 0 {
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(no Brewfiles found)"))
	} else if exec.CheckCommand("brew") {
		currentUser := os.Getenv("USER")
		if currentUser == "" {
			currentUser = os.Getenv("LOGNAME")
		}
		for _, brewfile := range brewFiles {
			relPath, _ := filepath.Rel(rootDir, brewfile)
			tapLines := []string{}
			file, err := os.Open(brewfile)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(read failed)"))
				continue
			}
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "tap ") {
					tapLines = append(tapLines, line)
				}
			}
			_ = file.Close()
			if err := scanner.Err(); err != nil {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(read failed)"))
				continue
			}

			tapsFile, err := os.CreateTemp("", "finctl-taps-*.Brewfile")
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(temp file failed)"))
				continue
			}
			for _, line := range tapLines {
				_, _ = tapsFile.WriteString(line + "\n")
			}
			if len(tapLines) == 0 {
				_, _ = tapsFile.WriteString("# No taps\n")
			}
			_ = tapsFile.Close()
			defer os.Remove(tapsFile.Name())

			tapsResult := exec.RunSimple(ctx, "brew", "bundle", "--file", tapsFile.Name())
			if tapsResult.Err != nil {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(tap validation failed)"))
				continue
			}

			bundleResult := exec.RunSimple(ctx, "brew", "bundle", "exec", "whoami", "--file", brewfile)
			if bundleResult.Err != nil {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(validation failed)"))
				continue
			}
			if currentUser != "" && !strings.Contains(bundleResult.Stdout, currentUser) {
				warnings = append(warnings, fmt.Sprintf("brewfile: %s", relPath))
				fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(validation failed)"))
				continue
			}

			fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), relPath)
		}
	} else {
		warnings = append(warnings, "brew not installed")
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("brew not installed"))
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("Flatpaks"))
	flatpakDirs := []string{
		filepath.Join(rootDir, "custom", "flatpaks"),
		filepath.Join(rootDir, "custom", "flatpak"),
	}
	flatpakFiles := []string{}
	for _, dir := range flatpakDirs {
		if _, err := os.Stat(dir); err == nil {
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				name := info.Name()
				if strings.HasSuffix(name, ".preinstall") || strings.HasSuffix(name, ".list") {
					flatpakFiles = append(flatpakFiles, path)
				}
				return nil
			})
		}
	}
	if len(flatpakFiles) == 0 {
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(no flatpak files found)"))
	} else if exec.CheckCommand("flatpak") {
		remoteResult := exec.RunSimple(ctx, "flatpak", "remote-add", "--user", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
		if remoteResult.Err != nil {
			warnings = append(warnings, "flatpak: could not add flathub")
			fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(could not add flathub remote)"))
		} else {
			for _, flatpakFile := range flatpakFiles {
				relPath, _ := filepath.Rel(rootDir, flatpakFile)
				ids, err := parseFlatpakIDs(flatpakFile)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("flatpak: %s", relPath))
					fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(read failed)"))
					continue
				}
				if len(ids) == 0 {
					pending = append(pending, fmt.Sprintf("flatpak: %s", relPath))
					fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(no entries)"))
					continue
				}
				failed := false
				for _, id := range ids {
					result := exec.RunSimple(ctx, "flatpak", "remote-info", "--user", "flathub", id)
					if result.Err != nil {
						failed = true
						warnings = append(warnings, fmt.Sprintf("flatpak: %s", id))
					}
				}
				if failed {
					fmt.Printf("  %s %s %s\n", ui.StatusPending.String(), relPath, ui.MutedStyle.Render("(validation failed)"))
					continue
				}
				fmt.Printf("  %s %s\n", ui.StatusSuccess.String(), relPath)
			}
		}
	} else {
		warnings = append(warnings, "flatpak not installed")
		fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("flatpak not installed"))
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("Shell Scripts"))
	buildDir := filepath.Join(rootDir, "build")
	if _, err := os.Stat(buildDir); err == nil {
		if exec.CheckCommand("shellcheck") {
			scripts := []string{}
			walkErr := filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if !info.IsDir() && filepath.Ext(path) == ".sh" {
					scripts = append(scripts, path)
				}
				return nil
			})
			if walkErr != nil {
				warnings = append(warnings, fmt.Sprintf("shellcheck: %v", walkErr))
				fmt.Printf("  %s %s\n", ui.StatusPending.String(), ui.MutedStyle.Render("(could not scan build scripts)"))
			}

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

	fmt.Println()
	if len(errors) > 0 {
		fmt.Println(ui.ErrorBox.Render(fmt.Sprintf("Validation failed with %d error(s)", len(errors))))
		return fmt.Errorf("validation failed")
	} else if len(warnings) > 0 || len(pending) > 0 {
		fmt.Println(ui.InfoBox.Render(fmt.Sprintf("Validation passed with %d warning(s)", len(warnings))))
	} else {
		fmt.Println(ui.SuccessBox.Render("Validation passed!"))
	}

	return nil
}

func parseFlatpakIDs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ids := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(path, ".list") {
			ids = append(ids, line)
			continue
		}
		if strings.HasPrefix(line, "[Flatpak Preinstall ") && strings.HasSuffix(line, "]") {
			trimmed := strings.TrimPrefix(line, "[Flatpak Preinstall ")
			trimmed = strings.TrimSuffix(trimmed, "]")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
