package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/build"
	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/platform"
	"github.com/iiroan/galena/internal/ui"
)

var (
	cleanImages  bool
	cleanOutput  bool
	cleanAll     bool
	cleanConfirm bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up build artifacts and images",
	Long: `Remove build artifacts, disk images, and container images.

By default, removes:
  - Local container images matching the project name
  - Generated disk images in the output directory

Examples:
  # Clean local images only
  galena-build clean --images

  # Clean output directory only
  galena-build clean --output

  # Clean everything
  galena-build clean --all

  # Skip confirmation
  galena-build clean --all -y`,
	RunE: runClean,
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanImages, "images", false, "Clean local container images")
	cleanCmd.Flags().BoolVar(&cleanOutput, "output", false, "Clean output directory")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Clean everything")
	cleanCmd.Flags().BoolVarP(&cleanConfirm, "yes", "y", false, "Skip confirmation prompt")
}

func runClean(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := platform.RequireLinux("clean"); err != nil {
		return err
	}

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Default to all if nothing specified
	if !cleanImages && !cleanOutput && !cleanAll {
		cleanAll = true
	}

	if cleanAll {
		cleanImages = true
		cleanOutput = true
	}

	// Confirm unless -y flag
	if !cleanConfirm {
		var confirm bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Clean build artifacts?").
					Description("This will remove local images and output files").
					Value(&confirm),
			),
		)
		if err := form.Run(); err != nil {
			return err
		}
		if !confirm {
			fmt.Println("Cancelled")
			return nil
		}
	}

	cleaned := []string{}

	// Clean images
	if cleanImages {
		builder := build.NewBuilder(cfg, rootDir, logger)
		images, err := builder.ListLocalImages(ctx)
		if err != nil {
			logger.Warn("could not list images", "error", err)
		} else {
			for _, img := range images {
				logger.Info("removing image", "image", img)
				result := exec.Podman(ctx, "rmi", "-f", img)
				if result.Err != nil {
					logger.Warn("could not remove image", "image", img, "error", result.Err)
				} else {
					cleaned = append(cleaned, img)
				}
			}
		}
	}

	// Clean output directory
	if cleanOutput {
		outputDir := filepath.Join(rootDir, "output")
		if _, err := os.Stat(outputDir); err == nil {
			logger.Info("removing output directory", "path", outputDir)
			if err := os.RemoveAll(outputDir); err != nil {
				logger.Warn("could not remove output directory", "error", err)
			} else {
				cleaned = append(cleaned, outputDir)
			}
		}

		// Also clean manifest
		manifestPath := filepath.Join(rootDir, "build-manifest.json")
		if _, err := os.Stat(manifestPath); err == nil {
			logger.Info("removing manifest", "path", manifestPath)
			if err := os.Remove(manifestPath); err != nil {
				logger.Warn("could not remove manifest", "error", err)
			} else {
				cleaned = append(cleaned, manifestPath)
			}
		}

		// Clean SBOM
		sbomPath := filepath.Join(rootDir, "sbom.spdx.json")
		if _, err := os.Stat(sbomPath); err == nil {
			logger.Info("removing SBOM", "path", sbomPath)
			if err := os.Remove(sbomPath); err != nil {
				logger.Warn("could not remove SBOM", "error", err)
			} else {
				cleaned = append(cleaned, sbomPath)
			}
		}
	}

	// Print summary
	if len(cleaned) > 0 {
		fmt.Println()
		fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Cleaned %d items", len(cleaned))))
	} else {
		fmt.Println()
		fmt.Println(ui.InfoBox.Render("Nothing to clean"))
	}

	return nil
}
