package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/platform"
	"github.com/iiroan/galena/internal/ui"
)

var (
	sbomImage  string
	sbomOutput string
	sbomFormat string
	sbomAttest bool
)

var sbomCmd = &cobra.Command{
	Use:   "sbom [image]",
	Short: "Generate SBOM for a container image",
	Long: `Generate a Software Bill of Materials (SBOM) for a container image using Trivy.

Supported formats:
  spdx-json    - SPDX JSON format (default)
  cyclonedx    - CycloneDX JSON format
  json         - Trivy JSON format

Defaults:
  - If no image is provided, defaults to galena:main
  - You can also set GALENA_SBOM_IMAGE or use --image

Examples:
  # Generate SPDX SBOM
  galena sbom ghcr.io/myorg/myimage:stable

  # Generate CycloneDX SBOM
  galena sbom ghcr.io/myorg/myimage:stable --format cyclonedx

  # Generate SBOM for default image (galena:main)
  galena sbom

  # Generate and attest SBOM to image
  galena sbom ghcr.io/myorg/myimage:stable --attest`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSBOM,
}

func init() {
	sbomCmd.Flags().StringVar(&sbomImage, "image", "", "Image reference (default: galena:main)")
	sbomCmd.Flags().StringVarP(&sbomOutput, "output", "o", "", "Output file path (default: sbom.<format>)")
	sbomCmd.Flags().StringVarP(&sbomFormat, "format", "f", "spdx-json", "SBOM format (spdx-json, cyclonedx, json)")
	sbomCmd.Flags().BoolVar(&sbomAttest, "attest", false, "Attest SBOM to image using cosign")
}

func runSBOM(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	rootDir, _ := getProjectRoot()
	imageRef := ""
	if len(args) > 0 {
		imageRef = args[0]
	} else if strings.TrimSpace(sbomImage) != "" {
		imageRef = sbomImage
	} else if envImage := strings.TrimSpace(os.Getenv("GALENA_SBOM_IMAGE")); envImage != "" {
		imageRef = envImage
	} else {
		imageRef = "galena:main"
	}
	if err := platform.RequireLinux("sbom"); err != nil {
		cmd.PrintErrln(err)
		return err
	}

	// Determine output file
	outputFile := sbomOutput
	if outputFile == "" {
		format := sbomFormat
		if format == "spdx-json" {
			format = "spdx"
		}
		outputFile = filepath.Join(rootDir, fmt.Sprintf("sbom.%s.json", format))
	}

	resolvedRef, localImage := ensureLocalImage(ctx, imageRef)
	if resolvedRef != "" {
		imageRef = resolvedRef
	}

	if exec.CheckCommand("trivy") {
		if err := generateSBOMWithTrivy(ctx, imageRef, outputFile, localImage, rootDir); err != nil {
			return err
		}
	} else {
		if err := generateSBOMWithTrivyContainer(ctx, imageRef, outputFile, localImage, rootDir); err != nil {
			return err
		}
	}

	// Attest if requested
	if sbomAttest {
		if err := attestSBOM(ctx, imageRef, outputFile); err != nil {
			return err
		}
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("SBOM generated successfully!\n\nFormat: %s\nOutput: %s", sbomFormat, outputFile)))

	return nil
}

func ensureLocalImage(ctx context.Context, imageRef string) (string, bool) {
	if !exec.CheckCommand("podman") {
		return "", false
	}
	candidates := candidateImageRefs(imageRef)
	for _, candidate := range candidates {
		exists := exec.Podman(ctx, "image", "exists", candidate)
		if exists.Err == nil && exists.ExitCode == 0 {
			return candidate, true
		}
	}

	for _, candidate := range candidates {
		logger.Info("image not found locally, attempting pull", "image", candidate)
		pull := exec.Podman(ctx, "pull", candidate)
		if pull.Err == nil {
			return candidate, true
		}
		logger.Warn("image pull failed",
			"image", candidate,
			"exit_code", pull.ExitCode,
			"stderr", exec.LastNLines(pull.Stderr, 20),
		)
	}

	return "", false
}

func generateSBOMWithTrivy(ctx context.Context, imageRef, outputFile string, localImage bool, rootDir string) error {
	trivyEnv := ensureTrivyEnv(rootDir)
	logger.Info("generating SBOM",
		"image", imageRef,
		"format", sbomFormat,
		"output", outputFile,
	)

	if localImage && exec.CheckCommand("podman") {
		archivePath := filepath.Join(rootDir, "sbom-image.tar")
		save := exec.Podman(ctx, "image", "save", "--format", "docker-archive", "-o", archivePath, imageRef)
		if save.Err == nil {
			defer func() {
				_ = os.Remove(archivePath)
			}()
			logger.Info("trivy scan via tarball", "path", archivePath)
			result := runTrivy(ctx, trivyEnv, "image", "--input", archivePath, "--format", sbomFormat, "--output", outputFile)
			if result.Err == nil {
				logger.Info("SBOM generated", "output", outputFile)
				return nil
			}
			logger.Warn("SBOM generation via tarball failed", "stderr", exec.LastNLines(result.Stderr, 20))
		} else {
			logger.Warn("podman image save failed", "stderr", exec.LastNLines(save.Stderr, 20))
		}
	}

	result := runTrivy(ctx, trivyEnv, "image", "--format", sbomFormat, "--output", outputFile, imageRef)
	if result.Err != nil {
		logger.Error("SBOM generation failed", "stderr", exec.LastNLines(result.Stderr, 20))
		return fmt.Errorf("SBOM generation failed: %w", result.Err)
	}

	logger.Info("SBOM generated", "output", outputFile)
	return nil
}

func generateSBOMWithTrivyContainer(ctx context.Context, imageRef, outputFile string, localImage bool, rootDir string) error {
	if !exec.CheckCommand("podman") {
		msg := "trivy not found and podman unavailable; cannot generate SBOM"
		logger.Error(msg)
		return fmt.Errorf(msg)
	}

	logger.Info("trivy not found; using container fallback")
	targetArgs := []string{"image"}
	archivePath := ""
	if localImage {
		archivePath = filepath.Join(rootDir, "sbom-image.tar")
		save := exec.Podman(ctx, "image", "save", "--format", "docker-archive", "-o", archivePath, imageRef)
		if save.Err != nil {
			logger.Warn("podman image save failed", "stderr", exec.LastNLines(save.Stderr, 20))
		} else {
			targetArgs = append(targetArgs, "--input", archivePath)
		}
	}
	if archivePath != "" {
		defer func() {
			_ = os.Remove(archivePath)
		}()
	}
	if len(targetArgs) == 1 {
		targetArgs = append(targetArgs, imageRef)
	}

	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:%s:Z", rootDir, rootDir),
		"-w", rootDir,
		"ghcr.io/aquasecurity/trivy:latest",
	}
	args = append(args, targetArgs...)
	args = append(args, "--format", sbomFormat, "--output", outputFile)

	result := exec.Podman(ctx, args...)
	if result.Err != nil {
		logger.Error("SBOM generation failed", "stderr", exec.LastNLines(result.Stderr, 20))
		return fmt.Errorf("SBOM generation failed: %w", result.Err)
	}

	logger.Info("SBOM generated", "output", outputFile)
	return nil
}

func candidateImageRefs(imageRef string) []string {
	candidates := []string{imageRef}
	if strings.Contains(imageRef, "/") || strings.HasPrefix(imageRef, "localhost/") {
		return candidates
	}

	name, tag := splitSimpleRef(imageRef)
	if name == "" {
		return candidates
	}

	registry := "ghcr.io"
	if cfg != nil && cfg.Registry != "" {
		registry = cfg.Registry
	}

	if cfg != nil && cfg.Repository != "" {
		candidates = append(candidates, fmt.Sprintf("%s/%s/%s:%s", registry, cfg.Repository, name, tag))
	}

	if owner, repo := detectGitHubOwnerRepo(); owner != "" {
		repoName := strings.ToLower(repo)
		if repoName == "" {
			repoName = name
		}
		candidates = append(candidates, fmt.Sprintf("%s/%s/%s:%s", registry, owner, name, tag))
		candidates = append(candidates, fmt.Sprintf("%s/%s/%s:%s", registry, owner, repoName, tag))
	}

	return uniqueStrings(candidates)
}

func splitSimpleRef(imageRef string) (string, string) {
	parts := strings.Split(imageRef, ":")
	if len(parts) == 1 {
		return parts[0], "latest"
	}
	return parts[0], parts[len(parts)-1]
}

func detectGitHubOwnerRepo() (string, string) {
	result := exec.Git(context.Background(), "", "remote", "get-url", "origin")
	if result.Err != nil {
		return "", ""
	}
	url := strings.TrimSpace(result.Stdout)
	if url == "" {
		return "", ""
	}

	// Handle git@github.com:OWNER/REPO.git
	if strings.HasPrefix(url, "git@github.com:") {
		trimmed := strings.TrimPrefix(url, "git@github.com:")
		trimmed = strings.TrimSuffix(trimmed, ".git")
		return splitOwnerRepo(trimmed)
	}

	// Handle https://github.com/OWNER/REPO(.git)
	if strings.HasPrefix(url, "https://github.com/") || strings.HasPrefix(url, "http://github.com/") {
		trimmed := strings.TrimPrefix(url, "https://github.com/")
		trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
		trimmed = strings.TrimSuffix(trimmed, ".git")
		return splitOwnerRepo(trimmed)
	}

	return "", ""
}

func splitOwnerRepo(value string) (string, string) {
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func attestSBOM(ctx context.Context, imageRef, sbomFile string) error {
	if err := exec.RequireCommands("cosign"); err != nil {
		return fmt.Errorf("cosign not found for attestation: %w", err)
	}

	logger.Info("attesting SBOM to image", "image", imageRef)

	result := exec.Cosign(ctx, "attest", "--yes", "--predicate", sbomFile, "--type", "spdxjson", imageRef)
	if result.Err != nil {
		logger.Error("SBOM attestation failed", "stderr", result.Stderr)
		return fmt.Errorf("SBOM attestation failed: %w", result.Err)
	}

	logger.Info("SBOM attested to image")
	return nil
}
