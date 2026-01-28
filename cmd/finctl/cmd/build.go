package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/build"
	"github.com/finpilot/finctl/internal/ui"
)

var (
	buildVariant     string
	buildTag         string
	buildNumber      int
	buildNoCache     bool
	buildPush        bool
	buildSign        bool
	buildSBOM        bool
	buildRechunk     bool
	buildDryRun      bool
	buildUseJust     bool
	buildInteractive bool
)

var buildCmd = &cobra.Command{
	Use:   "build [image]",
	Short: "Build an OCI container image",
	Long: `Build an OCI container image from the Containerfile.

This command builds a bootable container image that can be deployed
with bootc. It supports multiple variants (main, nvidia, dx) and
tags (stable, latest, beta).

Examples:
  # Build with defaults (main variant, latest tag)
  finctl build

  # Build a specific variant and tag
  finctl build --variant main --tag stable

  # Build and push to registry
  finctl build --push

  # Build, sign, and generate SBOM
  finctl build --push --sign --sbom

  # Use existing Justfile (Phase 1 compatibility)
  finctl build --just`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBuild,
}

func init() {
	buildCmd.Flags().StringVarP(&buildVariant, "variant", "V", "main", "Image variant (main, nvidia, dx)")
	buildCmd.Flags().StringVarP(&buildTag, "tag", "t", "latest", "Image tag (stable, latest, beta)")
	buildCmd.Flags().IntVarP(&buildNumber, "build-number", "n", 0, "Build number for versioning")
	buildCmd.Flags().BoolVar(&buildNoCache, "no-cache", false, "Build without cache")
	buildCmd.Flags().BoolVar(&buildPush, "push", false, "Push image to registry after build")
	buildCmd.Flags().BoolVar(&buildSign, "sign", false, "Sign image with cosign after push")
	buildCmd.Flags().BoolVar(&buildSBOM, "sbom", false, "Generate SBOM with syft")
	buildCmd.Flags().BoolVar(&buildRechunk, "rechunk", false, "Rechunk image for optimization")
	buildCmd.Flags().BoolVar(&buildDryRun, "dry-run", false, "Show what would be done without executing")
	buildCmd.Flags().BoolVar(&buildUseJust, "just", false, "Use existing Justfile recipes")
	buildCmd.Flags().BoolVarP(&buildInteractive, "interactive", "i", false, "Interactive mode with prompts")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	// Interactive mode
	if buildInteractive {
		if err := promptBuildOptions(); err != nil {
			return err
		}
	}

	builder := build.NewBuilder(cfg, rootDir, logger)

	// Use just if requested
	if buildUseJust {
		opts := build.BuildOptions{
			Variant: buildVariant,
			Tag:     buildTag,
		}
		return builder.BuildViaJust(ctx, opts)
	}

	// Native build
	opts := build.BuildOptions{
		Variant:     buildVariant,
		Tag:         buildTag,
		BuildNumber: buildNumber,
		NoCache:     buildNoCache,
		Push:        buildPush,
		Sign:        buildSign,
		SBOM:        buildSBOM,
		Rechunk:     buildRechunk,
		DryRun:      buildDryRun,
	}

	manifest, err := builder.Build(ctx, opts)
	if err != nil {
		return err
	}

	// Save manifest
	manifestPath := filepath.Join(rootDir, "build-manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		logger.Warn("could not save manifest", "error", err)
	} else {
		logger.Info("manifest saved", "path", manifestPath)
	}

	// Print success message
	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf(
		"Build completed successfully!\n\nImage: %s\nVersion: %s",
		manifest.Version.ImageRef,
		manifest.Version.Version,
	)))

	return nil
}

func promptBuildOptions() error {
	variants := cfg.ListVariantNames()
	if len(variants) == 0 {
		variants = []string{"main"}
	}

	variantOptions := make([]huh.Option[string], len(variants))
	for i, v := range variants {
		variantOptions[i] = huh.NewOption(v, v)
	}

	tagOptions := []huh.Option[string]{
		huh.NewOption("latest", "latest"),
		huh.NewOption("stable", "stable"),
		huh.NewOption("beta", "beta"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select variant").
				Description("Which image variant to build?").
				Options(variantOptions...).
				Value(&buildVariant),

			huh.NewSelect[string]().
				Title("Select tag").
				Description("Which tag to use?").
				Options(tagOptions...).
				Value(&buildTag),

			huh.NewConfirm().
				Title("Push to registry?").
				Description("Push the built image to the container registry").
				Value(&buildPush),
		),
	)

	return form.Run()
}
