package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var manageBuildToolsCmd = &cobra.Command{
	Use:   "build-tools [args...]",
	Short: "Launch the galena-build developer CLI",
	Long: `Forward commands to galena-build. This keeps development and
image build operations in a dedicated binary while galena remains focused on
runtime device management.`,
	Args: cobra.ArbitraryArgs,
	RunE: runBuildTools,
}

func runBuildTools(cmd *cobra.Command, args []string) error {
	path, err := findBuildCLIPath()
	if err != nil {
		return err
	}
	return runAttachedCommand(path, args)
}

func findBuildCLIPath() (string, error) {
	if path, err := osexec.LookPath("galena-build"); err == nil {
		return path, nil
	}

	rootDir, err := getProjectRoot()
	if err == nil {
		candidate := filepath.Join(rootDir, "galena-build")
		if stat, statErr := os.Stat(candidate); statErr == nil && !stat.IsDir() {
			return candidate, nil
		}
	}

	if cwd, cwdErr := os.Getwd(); cwdErr == nil {
		candidate := filepath.Join(cwd, "galena-build")
		if stat, statErr := os.Stat(candidate); statErr == nil && !stat.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("galena-build is not installed or available in PATH")
}
