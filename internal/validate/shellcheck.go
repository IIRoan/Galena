package validate

import (
	"context"
	"os"
	"path/filepath"

	"github.com/iiroan/galena/internal/exec"
)

// ShellScripts validates build shell scripts with shellcheck.
func ShellScripts(ctx context.Context, rootDir string) Result {
	result := Result{}

	buildDir := filepath.Join(rootDir, "build")
	if _, err := os.Stat(buildDir); err != nil {
		result.AddPending("build/ directory not found")
		result.AddItem(StatusPending, "Shell Scripts", "build/ directory not found")
		return result
	}

	if !exec.CheckCommand("shellcheck") {
		result.AddWarning("shellcheck not installed")
		result.AddItem(StatusPending, "Shell Scripts", "shellcheck not installed")
		return result
	}

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
		result.AddWarning("shellcheck: " + walkErr.Error())
		result.AddItem(StatusPending, "Shell Scripts", "could not scan build scripts")
		return result
	}

	for _, script := range scripts {
		relPath, _ := filepath.Rel(rootDir, script)
		scResult := exec.RunSimple(ctx, "shellcheck", script)
		if scResult.Err != nil {
			result.AddWarning("shellcheck: " + relPath)
			result.AddItem(StatusPending, relPath, "issues found")
			continue
		}
		result.AddItem(StatusSuccess, relPath, "")
	}

	return result
}
