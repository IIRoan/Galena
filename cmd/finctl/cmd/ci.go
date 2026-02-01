package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/ci"
	"github.com/finpilot/finctl/internal/exec"
	"github.com/finpilot/finctl/internal/version"
)

var (
	ciDefaultTag    string
	ciPush          bool
	ciSign          bool
	ciSBOM          bool
	ciSkipLint      bool
	ciImageDesc     string
	ciImageKeywords string
	ciImageLogoURL  string
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "CI/CD pipeline commands",
	Long: `Commands optimized for CI/CD pipelines (GitHub Actions, etc.).

These commands automatically detect the CI environment and configure
themselves appropriately for automated builds.`,
}

var ciBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build image in CI environment",
	Long: `Build a container image optimized for CI/CD.

This command:
  - Detects GitHub Actions environment
  - Generates appropriate tags based on branch/PR
  - Sets GitHub Actions outputs for downstream steps
  - Handles push/sign based on branch

Environment variables:
  IMAGE_REGISTRY  - Override registry (default: ghcr.io/<owner>)
  IMAGE_NAME      - Override image name (default: repo name)
  IMAGE_DESC      - Image description for labels

Examples:
  # Run in GitHub Actions
  finctl ci build

  # Build and push (if on default branch)
  finctl ci build --push

  # Build with signing and SBOM
  finctl ci build --push --sign --sbom`,
	RunE: runCIBuild,
}

var ciSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup CI environment",
	Long: `Setup the CI environment by installing finctl and dependencies.

This is typically run as the first step in a CI pipeline to ensure
all tools are available.`,
	RunE: runCISetup,
}

var ciInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display CI environment information",
	Long:  `Display information about the detected CI environment.`,
	RunE:  runCIInfo,
}

func init() {
	ciCmd.AddCommand(ciBuildCmd)
	ciCmd.AddCommand(ciSetupCmd)
	ciCmd.AddCommand(ciInfoCmd)

	ciBuildCmd.Flags().StringVar(&ciDefaultTag, "default-tag", "stable", "Default tag for releases")
	ciBuildCmd.Flags().BoolVar(&ciPush, "push", false, "Push image to registry")
	ciBuildCmd.Flags().BoolVar(&ciSign, "sign", false, "Sign image with cosign")
	ciBuildCmd.Flags().BoolVar(&ciSBOM, "sbom", false, "Generate SBOM")
	ciBuildCmd.Flags().BoolVar(&ciSkipLint, "skip-lint", false, "Skip bootc lint")
	ciBuildCmd.Flags().StringVar(&ciImageDesc, "description", "", "Image description")
	ciBuildCmd.Flags().StringVar(&ciImageKeywords, "keywords", "", "Image keywords (default: bootc,ublue,universal-blue)")
	ciBuildCmd.Flags().StringVar(&ciImageLogoURL, "logo-url", "", "Image logo URL for ArtifactHub")

	rootCmd.AddCommand(ciCmd)
}

func runCIBuild(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	env := ci.Detect()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	ci.StartGroup("Environment Detection")
	logger.Info("CI environment detected",
		"github_actions", env.IsGitHubActions,
		"repository", env.Repository,
		"ref", env.RefName,
		"is_pr", env.IsPullRequest,
		"is_default_branch", env.IsDefaultBranch,
	)
	ci.EndGroup()

	// Generate tags
	tags := env.GenerateTags(ciDefaultTag)
	if len(tags) == 0 {
		tags = []string{ciDefaultTag}
	}
	primaryTag := tags[0]

	imageName := env.ImageName()
	registry := env.ImageRegistry()
	fullImageRef := fmt.Sprintf("%s/%s:%s", registry, imageName, primaryTag)

	logger.Info("image configuration",
		"name", imageName,
		"registry", registry,
		"tags", strings.Join(tags, ", "),
	)

	// Generate labels
	labelCfg := ci.LabelConfig{
		Description: ciImageDesc,
		Keywords:    ciImageKeywords,
		LogoURL:     ciImageLogoURL,
	}

	// Override from environment variables
	if labelCfg.Description == "" {
		labelCfg.Description = os.Getenv("IMAGE_DESC")
	}
	if labelCfg.Keywords == "" {
		labelCfg.Keywords = os.Getenv("IMAGE_KEYWORDS")
	}
	if labelCfg.LogoURL == "" {
		labelCfg.LogoURL = os.Getenv("IMAGE_LOGO_URL")
	}

	labels := env.GenerateLabels(imageName, labelCfg)

	// Add version label
	versionStr := version.Compute(cfg.Build.FedoraVersion, env.RunNumber)
	labels["org.opencontainers.image.version"] = versionStr

	// Build the image
	ci.StartGroup("Building Image")

	buildArgs := []string{}
	for k, v := range labels {
		buildArgs = append(buildArgs, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// Add all tags
	for _, tag := range tags {
		buildArgs = append(buildArgs, "-t", fmt.Sprintf("%s/%s:%s", registry, imageName, tag))
	}

	// Also tag locally without registry for lint
	buildArgs = append(buildArgs, "-t", fmt.Sprintf("%s:%s", imageName, primaryTag))

	buildArgs = append(buildArgs,
		"-f", filepath.Join(rootDir, "Containerfile"),
		rootDir,
	)

	logger.Info("running podman build")
	result := exec.PodmanBuild(ctx, rootDir, buildArgs)
	if result.Err != nil {
		ci.LogError(fmt.Sprintf("Build failed: %v", result.Err), "", 0)
		return fmt.Errorf("build failed: %w", result.Err)
	}
	ci.EndGroup()

	// Run bootc lint
	if !ciSkipLint {
		ci.StartGroup("Running bootc lint")
		lintResult := exec.Podman(ctx, "run", "--rm", fmt.Sprintf("%s:%s", imageName, primaryTag), "bootc", "container", "lint")
		if lintResult.Err != nil {
			ci.LogError("bootc lint failed", "", 0)
			return fmt.Errorf("bootc lint failed: %w", lintResult.Err)
		}
		logger.Info("bootc lint passed")
		ci.EndGroup()
	}

	// Get image digest
	digestResult := exec.Podman(ctx, "inspect", "--format", "{{.Digest}}", fmt.Sprintf("%s:%s", imageName, primaryTag))
	digest := strings.TrimSpace(digestResult.Stdout)

	// Set outputs for GitHub Actions
	setCIOutput("image", fullImageRef)
	setCIOutput("tags", strings.Join(tags, " "))
	setCIOutput("digest", digest)
	setCIOutput("version", versionStr)
	setCIOutput("registry", registry)
	setCIOutput("image_name", imageName)

	// Determine if we should push
	shouldPush := ciPush
	if !shouldPush && env.ShouldPush() {
		logger.Info("auto-enabling push for default branch")
		shouldPush = true
	}

	// Don't push PRs unless explicitly requested
	if env.IsPullRequest && !ciPush {
		shouldPush = false
		logger.Info("skipping push for pull request")
	}

	if shouldPush {
		ci.StartGroup("Pushing Image")

		for _, tag := range tags {
			imageRef := fmt.Sprintf("%s/%s:%s", registry, imageName, tag)
			logger.Info("pushing", "image", imageRef)

			pushResult := exec.PodmanPush(ctx, imageRef)
			if pushResult.Err != nil {
				ci.LogError(fmt.Sprintf("Push failed for %s: %v", imageRef, pushResult.Err), "", 0)
				return fmt.Errorf("push failed: %w", pushResult.Err)
			}
		}

		ci.EndGroup()

		// Get digest after push
		digestResult := exec.Podman(ctx, "inspect", "--format", "{{.Digest}}", fullImageRef)
		digest = strings.TrimSpace(digestResult.Stdout)
		setCIOutput("digest", digest)

		// Sign if requested
		if ciSign && exec.CheckCommand("cosign") {
			ci.StartGroup("Signing Image")

			for _, tag := range tags {
				imageRef := fmt.Sprintf("%s/%s:%s", registry, imageName, tag)
				logger.Info("signing", "image", imageRef)

				signResult := exec.Cosign(ctx, "sign", "--yes", imageRef)
				if signResult.Err != nil {
					ci.LogWarning(fmt.Sprintf("Signing failed for %s: %v", imageRef, signResult.Err))
				}
			}

			ci.EndGroup()
		}

		// Generate SBOM if requested
		if ciSBOM && exec.CheckCommand("syft") {
			ci.StartGroup("Generating SBOM")

			sbomPath := filepath.Join(rootDir, "sbom.spdx.json")
			logger.Info("generating SBOM", "output", sbomPath)

			syftResult := exec.Syft(ctx, "scan", fullImageRef, "-o", fmt.Sprintf("spdx-json=%s", sbomPath))
			if syftResult.Err != nil {
				ci.LogWarning(fmt.Sprintf("SBOM generation failed: %v", syftResult.Err))
			} else {
				setCIOutput("sbom", sbomPath)

				// Attest SBOM if signing is enabled
				if ciSign && exec.CheckCommand("cosign") {
					logger.Info("attesting SBOM")
					attestResult := exec.Cosign(ctx, "attest", "--yes", "--predicate", sbomPath, "--type", "spdxjson", fmt.Sprintf("%s/%s@%s", registry, imageName, digest))
					if attestResult.Err != nil {
						ci.LogWarning(fmt.Sprintf("SBOM attestation failed: %v", attestResult.Err))
					}
				}
			}

			ci.EndGroup()
		}
	}

	// Create build manifest
	versionInfo := version.NewInfo(cfg.Build.FedoraVersion, env.RunNumber)
	if env.SHA != "" {
		short := env.SHA
		if len(short) > 12 {
			short = short[:12]
		}
		versionInfo = versionInfo.WithGit(short, env.RefName, false)
	}
	versionInfo = versionInfo.WithImage(fullImageRef, "main", primaryTag)

	manifest := version.NewBuildManifest(imageName, versionInfo)
	manifest.AddImage(imageName, primaryTag, digest, "main", 0)

	manifestPath := filepath.Join(rootDir, "build-manifest.json")
	if err := manifest.Save(manifestPath); err != nil {
		logger.Warn("could not save manifest", "error", err)
	} else {
		setCIOutput("manifest", manifestPath)
	}

	// Add job summary
	summary := fmt.Sprintf("## Build Summary\n\n"+
		"| Property | Value |\n"+
		"|----------|-------|\n"+
		"| Image | `%s` |\n"+
		"| Tags | %s |\n"+
		"| Version | `%s` |\n"+
		"| Digest | `%s` |\n"+
		"| Pushed | %v |\n"+
		"| Signed | %v |\n\n"+
		"Built with [finctl](https://github.com/finpilot/finctl) at %s\n",
		fullImageRef,
		strings.Join(tags, ", "),
		versionStr,
		digest,
		shouldPush,
		ciSign && shouldPush,
		time.Now().Format(time.RFC3339),
	)
	addCISummary(summary)

	logger.Info("CI build completed successfully",
		"image", fullImageRef,
		"version", versionStr,
		"pushed", shouldPush,
	)

	return nil
}

func setCIOutput(name, value string) {
	if err := ci.SetOutput(name, value); err != nil {
		logger.Warn("could not set CI output", "name", name, "error", err)
	}
}

func setCIEnv(name, value string) {
	if err := ci.SetEnv(name, value); err != nil {
		logger.Warn("could not set CI env", "name", name, "error", err)
	}
}

func addCISummary(summary string) {
	if err := ci.AddSummary(summary); err != nil {
		logger.Warn("could not add CI summary", "error", err)
	}
}

func runCISetup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	env := ci.Detect()

	logger.Info("setting up CI environment")

	// Check required tools
	required := []string{"podman"}
	missing := []string{}
	for _, tool := range required {
		if !exec.CheckCommand(tool) {
			missing = append(missing, tool)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required tools: %s", strings.Join(missing, ", "))
	}

	// Log tool versions
	ci.StartGroup("Tool Versions")

	result := exec.Podman(ctx, "--version")
	if result.Err == nil {
		logger.Info("podman", "version", strings.TrimSpace(result.Stdout))
	}

	if exec.CheckCommand("cosign") {
		result = exec.Cosign(ctx, "version")
		if result.Err == nil {
			logger.Info("cosign", "version", strings.TrimSpace(strings.Split(result.Stdout, "\n")[0]))
		}
	}

	if exec.CheckCommand("syft") {
		result = exec.Syft(ctx, "--version")
		if result.Err == nil {
			logger.Info("syft", "version", strings.TrimSpace(result.Stdout))
		}
	}

	ci.EndGroup()

	// Set environment for subsequent steps
	if env.IsGitHubActions {
		setCIEnv("FINCTL_CI", "true")
	}

	logger.Info("CI setup completed")
	return nil
}

func runCIInfo(cmd *cobra.Command, args []string) error {
	env := ci.Detect()

	fmt.Println("CI Environment Information")
	fmt.Println("==========================")
	fmt.Printf("CI:                %v\n", env.IsCI)
	fmt.Printf("GitHub Actions:    %v\n", env.IsGitHubActions)
	fmt.Printf("Repository:        %s\n", env.Repository)
	fmt.Printf("Repository Owner:  %s\n", env.RepositoryOwner)
	fmt.Printf("Repository Name:   %s\n", env.RepositoryName)
	fmt.Printf("Ref:               %s\n", env.Ref)
	fmt.Printf("Ref Name:          %s\n", env.RefName)
	fmt.Printf("SHA:               %s\n", env.SHA)
	fmt.Printf("Run Number:        %d\n", env.RunNumber)
	fmt.Printf("Run ID:            %s\n", env.RunID)
	fmt.Printf("Event Name:        %s\n", env.EventName)
	fmt.Printf("Default Branch:    %s\n", env.DefaultBranch)
	fmt.Printf("Is Default Branch: %v\n", env.IsDefaultBranch)
	fmt.Printf("Is Pull Request:   %v\n", env.IsPullRequest)
	fmt.Printf("Actor:             %s\n", env.Actor)
	fmt.Println()
	fmt.Println("Computed Values")
	fmt.Println("---------------")
	fmt.Printf("Image Registry:    %s\n", env.ImageRegistry())
	fmt.Printf("Image Name:        %s\n", env.ImageName())
	fmt.Printf("Should Push:       %v\n", env.ShouldPush())
	fmt.Printf("Tags:              %s\n", strings.Join(env.GenerateTags("stable"), ", "))

	return nil
}
