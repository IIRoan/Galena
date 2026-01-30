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

	// Interactive mode: if no arguments/flags or specifically requested
	isInteractive := buildInteractive || (len(args) == 0 && !cmd.Flags().Changed("variant") && !cmd.Flags().Changed("tag") && !cmd.Flags().Changed("just"))

	if isInteractive {
		if err := runInteractiveFlow(ctx, rootDir); err != nil {
			return err
		}
		return nil
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

func runInteractiveFlow(ctx context.Context, rootDir string) error {
	var buildType string
	
	fmt.Println(ui.Banner())
	fmt.Println(ui.WizardTitle.Render(" BUILD WIZARD "))
	fmt.Println(ui.WizardDescription.Render("This wizard will guide you through building your custom OS image."))

	// 1. Choose Build Type
	err := huh.NewSelect[string]().
		Title("Artifact Type").
		Description("What would you like to build?").
		Options(
			huh.NewOption("OCI Container (Local/Remote)", "container"),
			huh.NewOption("Disk Image (VM/Bare Metal)", "disk"),
		).
		Value(&buildType).
		Run()
	if err != nil {
		return err
	}

	if buildType == "container" {
		return interactiveContainerBuild(ctx, rootDir)
	}
	return interactiveDiskBuild(ctx, rootDir)
}

func interactiveContainerBuild(ctx context.Context, rootDir string) error {
	variants := cfg.ListVariantNames()
	if len(variants) == 0 {
		variants = []string{"main"}
	}

	variantOptions := make([]huh.Option[string], len(variants))
	for i, v := range variants {
		variantOptions[i] = huh.NewOption(v, v)
	}

	tagOptions := []huh.Option[string]{
		huh.NewOption("latest (stable development)", "latest"),
		huh.NewOption("stable (production ready)", "stable"),
		huh.NewOption("beta (testing)", "beta"),
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Variant").
				Description("Target hardware/feature set").
				Options(variantOptions...).
				Value(&buildVariant),

			huh.NewSelect[string]().
				Title("Tag").
				Description("Release channel tag").
				Options(tagOptions...).
				Value(&buildTag),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Push & Distribute").
				Description("Upload to GHCR after building?").
				Value(&buildPush),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Sign & Secure").
				Description("Sign with cosign?").
				Value(&buildSign),
		),
		huh.NewGroup(
			huh.NewConfirm().
				Title("Audit (SBOM)").
				Description("Generate Software Bill of Materials?").
				Value(&buildSBOM),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}

	fmt.Println(ui.WizardStep.Render("▶ Building OCI Container..."))
	
	builder := build.NewBuilder(cfg, rootDir, logger)
	opts := build.BuildOptions{
		Variant:     buildVariant,
		Tag:         buildTag,
		Push:        buildPush,
		Sign:        buildSign,
		SBOM:        buildSBOM,
		BuildNumber: buildNumber,
	}

	manifest, err := builder.Build(ctx, opts)
	if err != nil {
		return err
	}

	fmt.Println(ui.SuccessStyle.Render("\n✔ Container Build Complete"))
	fmt.Println(ui.MutedStyle.Render("Reference: " + manifest.Version.ImageRef))
	return nil
}

func interactiveDiskBuild(ctx context.Context, rootDir string) error {
	var outputType string
	typeOptions := make([]huh.Option[string], 0)
	for _, t := range build.ListOutputTypes() {
		label := t
		switch t {
		case "qcow2":
			label = "Virtual Machine (QCOW2)"
		case "iso":
			label = "ISO Installer (Standard)"
		case "anaconda-iso":
			label = "ISO Installer (Anaconda)"
		case "raw":
			label = "Raw Disk Image"
		}
		typeOptions = append(typeOptions, huh.NewOption(label, t))
	}

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Output Format").
				Description("How do you want to deploy this image?").
				Options(typeOptions...).
				Value(&outputType),

			huh.NewInput().
				Title("Source Image").
				Description("Image to convert (empty for local project)").
				Placeholder("localhost/finpilot:latest").
				Value(&diskImage),
		),
	).WithTheme(huh.ThemeCharm()).Run()
	if err != nil {
		return err
	}

	fmt.Println(ui.WizardStep.Render("▶ Converting to " + outputType + "..."))

	diskBuilder := build.NewDiskBuilder(cfg, rootDir, logger)
	imageRef := diskImage
	if imageRef == "" {
		imageRef = cfg.ImageRef("main", "latest")
	}

	opts := build.DefaultDiskOptions()
	opts.ImageRef = imageRef
	opts.OutputType = outputType
	
	outputPath, err := diskBuilder.Build(ctx, opts)
	if err != nil {
		return err
	}

	fmt.Println(ui.SuccessStyle.Render("\n✔ Disk Build Complete"))
	fmt.Println(ui.MutedStyle.Render("Output: " + outputPath))
	return nil
}
