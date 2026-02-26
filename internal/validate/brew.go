package validate

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/iiroan/galena/internal/exec"
)

// Brewfiles validates Homebrew Brewfiles.
func Brewfiles(ctx context.Context, rootDir string) Result {
	result := Result{}

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
		result.AddPending("No Brewfiles found")
		result.AddItem(StatusPending, "Brewfiles", "none found")
		return result
	}

	if !exec.CheckCommand("brew") {
		result.AddWarning("brew not installed")
		result.AddPending("brew not installed")
		result.AddItem(StatusPending, "Brewfiles", "brew not installed")
		return result
	}

	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = os.Getenv("LOGNAME")
	}

	for _, brewfile := range brewFiles {
		relPath, _ := filepath.Rel(rootDir, brewfile)
		tapLines := []string{}

		file, err := os.Open(brewfile)
		if err != nil {
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "read failed")
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
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "read failed")
			continue
		}

		tapsFile, err := os.CreateTemp("", "galena-taps-*.Brewfile")
		if err != nil {
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "temp file failed")
			continue
		}
		for _, line := range tapLines {
			_, _ = tapsFile.WriteString(line + "\n")
		}
		if len(tapLines) == 0 {
			_, _ = tapsFile.WriteString("# No taps\n")
		}
		_ = tapsFile.Close()
		tapsFilePath := tapsFile.Name()
		defer func(path string) {
			_ = os.Remove(path)
		}(tapsFilePath)

		tapsResult := exec.RunSimple(ctx, "brew", "bundle", "--file", tapsFilePath)
		if tapsResult.Err != nil {
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "tap validation failed")
			continue
		}

		bundleResult := exec.RunSimple(ctx, "brew", "bundle", "exec", "--file", brewfile, "--", "whoami")
		if bundleResult.Err != nil {
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "validation failed")
			continue
		}
		if currentUser != "" && !strings.Contains(bundleResult.Stdout, currentUser) {
			result.AddWarning("brewfile: " + relPath)
			result.AddItem(StatusPending, relPath, "validation failed")
			continue
		}

		result.AddItem(StatusSuccess, relPath, "")
	}

	return result
}
