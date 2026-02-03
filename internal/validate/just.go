package validate

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/iiroan/galena/internal/exec"
)

// Justfiles validates Justfile syntax.
func Justfiles(ctx context.Context, rootDir string) Result {
	result := Result{}

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
		result.AddPending("No justfiles found")
		result.AddItem(StatusPending, "Justfiles", "none found")
		return result
	}

	if !exec.CheckCommand("just") {
		result.AddWarning("just not installed")
		result.AddPending("just not installed")
		result.AddItem(StatusPending, "Justfiles", "just not installed")
		return result
	}

	for _, file := range justFiles {
		relPath, _ := filepath.Rel(rootDir, file)
		checkResult := exec.RunSimple(ctx, "just", "--unstable", "--fmt", "--check", "-f", file)
		if checkResult.Err != nil {
			result.AddWarning("justfile: " + relPath)
			result.AddItem(StatusPending, relPath, "format issues")
			continue
		}
		result.AddItem(StatusSuccess, relPath, "")
	}

	return result
}
