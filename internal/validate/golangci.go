package validate

import (
	"context"
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

	lintResult := exec.RunInDir(ctx, rootDir, "golangci-lint", "run")
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
