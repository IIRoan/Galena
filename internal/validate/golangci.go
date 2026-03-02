package validate

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/iiroan/galena/internal/exec"
)

// Golangci validates Go code using golangci-lint.
func Golangci(ctx context.Context, rootDir string) Result {
	result := Result{}

	if !exec.CheckCommand("golangci-lint") {
		result.AddWarning("golangci-lint not installed")
		result.AddItem(StatusPending, "Go Lint", "golangci-lint not installed")
		return result
	}

	cacheBase := filepath.Join(rootDir, ".cache")
	goCache := filepath.Join(cacheBase, "go-build")
	lintCache := filepath.Join(cacheBase, "golangci-lint")
	_ = os.MkdirAll(goCache, 0o755)
	_ = os.MkdirAll(lintCache, 0o755)

	env := []string{}
	if os.Getenv("GOCACHE") == "" {
		env = append(env, "GOCACHE="+goCache)
	}
	if os.Getenv("GOLANGCI_LINT_CACHE") == "" {
		env = append(env, "GOLANGCI_LINT_CACHE="+lintCache)
	}
	if os.Getenv("CGO_ENABLED") == "" {
		env = append(env, "CGO_ENABLED=0")
	}

	opts := exec.DefaultOptions()
	opts.Dir = rootDir
	opts.Env = env
	lintResult := exec.Run(ctx, "golangci-lint", []string{"run"}, opts)
	if lintResult.Err != nil {
		msg := strings.TrimSpace(exec.LastNLines(lintResult.Stderr, 20))
		if msg == "" {
			msg = strings.TrimSpace(exec.LastNLines(lintResult.Stdout, 20))
		}
		if msg == "" {
			msg = "golangci-lint reported issues"
		}

		result.AddError("golangci-lint: issues found")
		result.AddItem(StatusError, "Go Lint", "issues found")
		result.AddWarning(msg)
		return result
	}

	result.AddItem(StatusSuccess, "Go Lint", "")
	return result
}
