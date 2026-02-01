package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var lintCmd = &cobra.Command{
	Use:   "lint [image]",
	Short: "Run bootc container lint on an image",
	Long: `Run bootc container lint to validate that the image meets
bootable container requirements.

This checks:
  - Required files and directories
  - Proper systemd setup
  - ostree compatibility
  - Boot configuration

Examples:
  galena lint
  galena lint ghcr.io/myorg/myimage:stable`,
	Args: cobra.MaximumNArgs(1),
	RunE: runLint,
}

func runLint(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	imageRef := ""
	if len(args) > 0 {
		imageRef = args[0]
	} else {
		imageRef = cfg.ImageRef("main", "latest")
	}

	logger.Info("running bootc container lint", "image", imageRef)

	// Run bootc container lint inside the image
	result := exec.Podman(ctx, "run", "--rm", imageRef, "bootc", "container", "lint")
	if result.Err != nil {
		logger.Error("lint failed", "stderr", result.Stderr)
		fmt.Println()
		fmt.Println(ui.ErrorBox.Render(fmt.Sprintf("Lint failed!\n\n%s", exec.LastNLines(result.Stderr, 10))))
		return result.Err
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Lint passed!\n\n%s", imageRef)))

	return nil
}
