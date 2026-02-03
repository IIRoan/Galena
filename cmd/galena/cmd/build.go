package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/build"
	"github.com/iiroan/galena/internal/platform"
	"github.com/iiroan/galena/internal/ui"
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
	buildTimeout     string
	buildArgs        []string
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
  galena build

  # Build a specific variant and tag
  galena build --variant main --tag stable

  # Build and push to registry
  galena build --push

  # Build, sign, and generate SBOM
  galena build --push --sign --sbom

  # Use existing Justfile (Phase 1 compatibility)
  galena build --just`,
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
	buildCmd.Flags().StringVar(&buildTimeout, "timeout", "", "Build timeout (e.g. 45m, 2h)")
	buildCmd.Flags().StringArrayVar(&buildArgs, "build-arg", nil, "Additional build arg (KEY=VALUE)")
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}
	if err := platform.RequireLinux("build"); err != nil {
		return err
	}

	applyBuildDefaults(cmd)
	if err := applyBuildTimeout(cmd); err != nil {
		return err
	}

	isInteractive := buildInteractive || (len(args) == 0 && !cmd.Flags().Changed("variant") && !cmd.Flags().Changed("tag") && !cmd.Flags().Changed("just"))

	if isInteractive {
		if err := runInteractiveFlow(ctx, rootDir); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				return nil
			}
			return err
		}
		return nil
	}

	builder := build.NewBuilder(cfg, rootDir, logger)

	if buildUseJust {
		opts := build.BuildOptions{
			Variant: buildVariant,
			Tag:     buildTag,
		}
		return builder.BuildViaJust(ctx, opts)
	}

	extraArgs, err := parseKeyValuePairs(buildArgs)
	if err != nil {
		return err
	}

	opts := build.BuildOptions{
		Variant:        buildVariant,
		Tag:            buildTag,
		BuildNumber:    buildNumber,
		NoCache:        buildNoCache,
		Push:           buildPush,
		Sign:           buildSign,
		SBOM:           buildSBOM,
		Rechunk:        buildRechunk,
		DryRun:         buildDryRun,
		Timeout:        build.DefaultBuildOptions().Timeout,
		ExtraBuildArgs: extraArgs,
	}
	if buildTimeout != "" {
		parsed, err := time.ParseDuration(buildTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		opts.Timeout = parsed
	}

	manifest, err := builder.Build(ctx, opts)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(rootDir, "build-manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		logger.Warn("could not save manifest", "error", err)
	} else {
		logger.Info("manifest saved", "path", manifestPath)
	}

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

	ui.StartScreen("BUILD WIZARD", "This wizard will guide you through building your custom OS image.")

	err := huh.NewSelect[string]().
		Title("Artifact Type").
		Description("What would you like to build?").
		Options(
			huh.NewOption("OCI Container (Local/Remote)", "container"),
			huh.NewOption("Disk Image (VM/Bare Metal)", "disk"),
		).
		Value(&buildType).
		WithTheme(ui.HuhTheme()).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	if buildType == "container" {
		return interactiveContainerBuild(ctx, rootDir)
	}
	return interactiveDiskBuild(ctx, rootDir)
}

func interactiveContainerBuild(ctx context.Context, rootDir string) error {
	advancedMode := ui.CurrentPreferences.Advanced
	buildNumberInput := strconv.Itoa(buildNumber)
	extraArgsInput := ""
	if len(buildArgs) > 0 {
		extraArgsInput = strings.Join(buildArgs, ", ")
	}
	timeoutInput := buildTimeout

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

	groups := []*huh.Group{
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

			huh.NewConfirm().
				Title("Push & Distribute").
				Description("Upload to GHCR after building?").
				Value(&buildPush),
		),
	}

	if advancedMode {
		groups = append(groups,
			huh.NewGroup(
				huh.NewInput().
					Title("Build Number").
					Description("Version increment used in release scheme").
					Value(&buildNumberInput).
					Validate(func(value string) error {
						if value == "" {
							return nil
						}
						_, err := strconv.Atoi(value)
						if err != nil {
							return fmt.Errorf("enter a valid integer")
						}
						return nil
					}),
				huh.NewConfirm().
					Title("Sign & Secure").
					Description("Sign with cosign?").
					Value(&buildSign),
				huh.NewConfirm().
					Title("Audit (SBOM)").
					Description("Generate Software Bill of Materials?").
					Value(&buildSBOM),
			),
			huh.NewGroup(
				huh.NewConfirm().
					Title("No Cache").
					Description("Disable build cache").
					Value(&buildNoCache),
				huh.NewConfirm().
					Title("Rechunk").
					Description("Optimize image layer chunks").
					Value(&buildRechunk),
				huh.NewConfirm().
					Title("Dry Run").
					Description("Skip the actual build").
					Value(&buildDryRun),
				huh.NewConfirm().
					Title("Use Justfile").
					Description("Run the build via Just recipes").
					Value(&buildUseJust),
			),
			huh.NewGroup(
				huh.NewInput().
					Title("Extra Build Args").
					Description("Comma-separated KEY=VALUE pairs").
					Placeholder("FEATURE=on, CACHE=false").
					Value(&extraArgsInput),
				huh.NewInput().
					Title("Build Timeout").
					Description("Duration (e.g. 45m, 2h)").
					Placeholder("30m").
					Value(&timeoutInput).
					Validate(func(value string) error {
						if value == "" {
							return nil
						}
						_, err := time.ParseDuration(value)
						if err != nil {
							return fmt.Errorf("invalid duration")
						}
						return nil
					}),
			),
		)
	}

	form := huh.NewForm(groups...).WithTheme(ui.HuhTheme())

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	if advancedMode {
		buildNumber = 0
		if buildNumberInput != "" {
			parsed, err := strconv.Atoi(buildNumberInput)
			if err != nil {
				return fmt.Errorf("invalid build number: %w", err)
			}
			buildNumber = parsed
		}
	}

	extraArgs, err := parseKeyValueCSV(extraArgsInput)
	if err != nil {
		return err
	}

	buildTimeout = timeoutInput

	buildPlan := fmt.Sprintf(
		"Build Plan\n\nVariant: %s\nTag: %s\nPush: %t",
		buildVariant,
		buildTag,
		buildPush,
	)
	if advancedMode {
		buildPlan = fmt.Sprintf(
			"Build Plan\n\nVariant: %s\nTag: %s\nBuild Number: %d\nPush: %t\nSign: %t\nSBOM: %t\nNo Cache: %t\nRechunk: %t\nDry Run: %t\nUse Justfile: %t\nTimeout: %s\nExtra Args: %s",
			buildVariant,
			buildTag,
			buildNumber,
			buildPush,
			buildSign,
			buildSBOM,
			buildNoCache,
			buildRechunk,
			buildDryRun,
			buildUseJust,
			defaultIfEmpty(buildTimeout, "default"),
			defaultIfEmpty(formatKeyValuePairs(extraArgs), "none"),
		)
	}

	fmt.Println(ui.InfoBox.Render(buildPlan))

	fmt.Println(ui.WizardStep.Render("▶ Building OCI Container..."))

	builder := build.NewBuilder(cfg, rootDir, logger)
	if buildUseJust {
		opts := build.BuildOptions{Variant: buildVariant, Tag: buildTag}
		if err := builder.BuildViaJust(ctx, opts); err != nil {
			return err
		}
		fmt.Println(ui.SuccessStyle.Render("\n✔ Container Build Complete"))
		return nil
	}

	opts := build.BuildOptions{
		Variant:        buildVariant,
		Tag:            buildTag,
		Push:           buildPush,
		Sign:           buildSign,
		SBOM:           buildSBOM,
		BuildNumber:    buildNumber,
		NoCache:        buildNoCache,
		Rechunk:        buildRechunk,
		DryRun:         buildDryRun,
		Timeout:        build.DefaultBuildOptions().Timeout,
		ExtraBuildArgs: extraArgs,
	}
	if buildTimeout != "" {
		parsed, err := time.ParseDuration(buildTimeout)
		if err != nil {
			return fmt.Errorf("invalid timeout: %w", err)
		}
		opts.Timeout = parsed
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
	advancedMode := ui.CurrentPreferences.Advanced
	var outputType string
	rootfsType := "btrfs"
	configFile := ""
	outputDir := ""
	usePrivileged := true
	pullNewer := true
	timeoutInput := ""

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

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Output Format").
				Description("How do you want to deploy this image?").
				Options(typeOptions...).
				Value(&outputType),

			huh.NewInput().
				Title("Source Image").
				Description("Image to convert (empty for local project)").
				Placeholder("localhost/galena:latest").
				Value(&diskImage),
		),
	}

	if advancedMode {
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Root Filesystem").
				Description("Filesystem for the disk image").
				Options(
					huh.NewOption("btrfs", "btrfs"),
					huh.NewOption("ext4", "ext4"),
					huh.NewOption("xfs", "xfs"),
				).
				Value(&rootfsType),
			huh.NewInput().
				Title("Config TOML").
				Description("Optional bootc-image-builder config file").
				Placeholder("iso/disk.toml").
				Value(&configFile),
			huh.NewInput().
				Title("Output Directory").
				Description("Where to write generated artifacts").
				Placeholder("./output").
				Value(&outputDir),
			huh.NewConfirm().
				Title("Privileged Build").
				Description("Run bootc-image-builder with --privileged").
				Value(&usePrivileged),
			huh.NewConfirm().
				Title("Pull Newer").
				Description("Always pull a newer bootc-image-builder image").
				Value(&pullNewer),
			huh.NewInput().
				Title("Timeout").
				Description("Duration (e.g. 45m, 2h)").
				Placeholder("60m").
				Value(&timeoutInput).
				Validate(func(value string) error {
					if value == "" {
						return nil
					}
					_, err := time.ParseDuration(value)
					if err != nil {
						return fmt.Errorf("invalid duration")
					}
					return nil
				}),
		))
	}

	err := huh.NewForm(groups...).WithTheme(ui.HuhTheme()).Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	imageRef := diskImage
	if imageRef == "" {
		imageRef = cfg.ImageRef("main", "latest")
	}

	buildPlan := fmt.Sprintf(
		"Build Plan\n\nOutput: %s\nSource Image: %s",
		outputType,
		imageRef,
	)
	if advancedMode {
		buildPlan = fmt.Sprintf(
			"Build Plan\n\nOutput: %s\nSource Image: %s\nRootFS: %s\nConfig: %s\nOutput Dir: %s\nPrivileged: %t\nPull Newer: %t\nTimeout: %s",
			outputType,
			imageRef,
			rootfsType,
			defaultIfEmpty(configFile, "auto"),
			defaultIfEmpty(outputDir, "./output"),
			usePrivileged,
			pullNewer,
			defaultIfEmpty(timeoutInput, "default"),
		)
	}

	fmt.Println(ui.InfoBox.Render(buildPlan))

	fmt.Println(ui.WizardStep.Render("▶ Converting to " + outputType + "..."))

	diskBuilder := build.NewDiskBuilder(cfg, rootDir, logger)
	opts := build.DefaultDiskOptions()
	opts.ImageRef = imageRef
	opts.OutputType = outputType
	if advancedMode {
		opts.OutputDir = outputDir
		opts.ConfigFile = configFile
		opts.RootFSType = rootfsType
		opts.Privileged = usePrivileged
		opts.PullNewer = pullNewer
		if timeoutInput != "" {
			parsed, err := time.ParseDuration(timeoutInput)
			if err != nil {
				return fmt.Errorf("invalid timeout: %w", err)
			}
			opts.Timeout = parsed
		}
	}

	outputPath, err := diskBuilder.Build(ctx, opts)
	if err != nil {
		return err
	}

	fmt.Println(ui.SuccessStyle.Render("\n✔ Disk Build Complete"))
	fmt.Println(ui.MutedStyle.Render("Output: " + outputPath))
	return nil
}

func applyBuildDefaults(cmd *cobra.Command) {
	if cfg == nil {
		return
	}
	defaults := cfg.Build.Defaults
	if !cmd.Flags().Changed("variant") && defaults.Variant != "" {
		buildVariant = defaults.Variant
	}
	if !cmd.Flags().Changed("tag") && defaults.Tag != "" {
		buildTag = defaults.Tag
	}
	if !cmd.Flags().Changed("build-number") && defaults.BuildNumber != 0 {
		buildNumber = defaults.BuildNumber
	}
	if !cmd.Flags().Changed("no-cache") {
		buildNoCache = defaults.NoCache
	}
	if !cmd.Flags().Changed("push") {
		buildPush = defaults.Push
	}
	if !cmd.Flags().Changed("sign") {
		buildSign = defaults.Sign
	}
	if !cmd.Flags().Changed("sbom") {
		buildSBOM = defaults.SBOM
	}
	if !cmd.Flags().Changed("rechunk") {
		buildRechunk = defaults.Rechunk
	}
	if !cmd.Flags().Changed("dry-run") {
		buildDryRun = defaults.DryRun
	}
	if !cmd.Flags().Changed("just") {
		buildUseJust = defaults.UseJust
	}
}

func applyBuildTimeout(cmd *cobra.Command) error {
	if cfg == nil {
		return nil
	}
	if !cmd.Flags().Changed("timeout") && cfg.Build.Timeout != "" {
		buildTimeout = cfg.Build.Timeout
	}
	return nil
}

func defaultIfEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
