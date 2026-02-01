package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var pushCmd = &cobra.Command{
	Use:   "push [image]",
	Short: "Push an image to the container registry",
	Long: `Push a built container image to the configured registry.

If no image is specified, pushes the default image with the latest tag.

Examples:
  galena push
  galena push ghcr.io/myorg/myimage:stable
  galena push --tag stable`,
	Args: cobra.MaximumNArgs(1),
	RunE: runPush,
}

var pushTag string

func init() {
	pushCmd.Flags().StringVarP(&pushTag, "tag", "t", "latest", "Image tag to push")
}

func runPush(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	imageRef := ""
	if len(args) > 0 {
		imageRef = args[0]
	} else {
		imageRef = cfg.ImageRef("main", pushTag)
	}

	logger.Info("pushing image", "image", imageRef)

	result := exec.PodmanPush(ctx, imageRef)
	if result.Err != nil {
		return fmt.Errorf("push failed: %w", result.Err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Image pushed successfully!\n\n%s", imageRef)))

	return nil
}
