package cmd

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/exec"
	"github.com/finpilot/finctl/internal/ui"
)

var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "CLI maintenance commands",
}

var cliBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Rebuild finctl and start the CLI",
	RunE:  runCLIBuild,
}

func init() {
	cliCmd.AddCommand(cliBuildCmd)
}

func runCLIBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	ui.StartScreen("CLI BUILD", "Rebuilding finctl and starting a new session")

	if err := exec.RequireCommands("make"); err != nil {
		return err
	}

	buildResult := exec.RunStreaming(ctx, "make", []string{"-C", rootDir, "build"}, exec.DefaultOptions())
	if buildResult.Err != nil {
		return fmt.Errorf("cli build failed: %w", buildResult.Err)
	}

	binaryPath := filepath.Join(rootDir, "finctl")
	if _, err := os.Stat(binaryPath); err != nil {
		return fmt.Errorf("built binary not found at %s: %w", binaryPath, err)
	}

	newCmd := osexec.Command(binaryPath)
	newCmd.Stdout = os.Stdout
	newCmd.Stderr = os.Stderr
	newCmd.Stdin = os.Stdin
	newCmd.Env = os.Environ()

	return newCmd.Run()
}
