package validate

import (
	"context"
	"os"
	"path/filepath"

	"github.com/iiroan/galena/internal/exec"
)

// Containerfile validates Containerfile syntax by building the ctx stage.
func Containerfile(ctx context.Context, rootDir string) Result {
	result := Result{}

	containerfile := filepath.Join(rootDir, "Containerfile")
	if _, err := os.Stat(containerfile); err != nil {
		result.AddError("Containerfile not found")
		result.AddItem(StatusError, "Containerfile", "not found")
		return result
	}

	if !exec.CheckCommand("podman") {
		result.AddWarning("podman not installed")
		result.AddPending("podman not installed")
		result.AddItem(StatusPending, "Containerfile", "podman not installed")
		return result
	}

	buildResult := exec.Podman(ctx, "build", "--no-cache", "-f", containerfile, "--target", "ctx", "-t", "validate-test", rootDir)
	if buildResult.Err != nil {
		result.AddWarning("Containerfile syntax may have issues")
		result.AddPending("Containerfile validation failed")
		result.AddItem(StatusPending, "Containerfile", "could not validate")
		return result
	}

	exec.Podman(ctx, "rmi", "-f", "validate-test")
	result.AddItem(StatusSuccess, "Containerfile", "")
	return result
}
