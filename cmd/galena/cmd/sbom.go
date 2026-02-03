package cmd

import (
	"context"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strings"
	"time"

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
	Long: `Generate a Software Bill of Materials (SBOM) for a container image using Syft.

Supported formats:
  spdx-json    - SPDX JSON format (default)
  cyclonedx    - CycloneDX JSON format
  json         - Syft JSON format

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
		ext := ".json"
		outputFile = filepath.Join(rootDir, fmt.Sprintf("sbom.%s%s", sbomFormat, ext))
	}

	resolvedRef, localImage := ensureLocalImage(ctx, imageRef)
	if resolvedRef != "" {
		imageRef = resolvedRef
	}

	var stopPodmanService func()
	if exec.CheckCommand("syft") && localImage {
		stopPodmanService = ensurePodmanService(ctx)
	}
	if stopPodmanService != nil {
		defer stopPodmanService()
	}

	if exec.CheckCommand("syft") {
		if err := generateSBOMWithSyft(ctx, imageRef, outputFile, localImage, rootDir); err != nil {
			return err
		}
	} else {
		if err := generateSBOMWithContainer(ctx, imageRef, outputFile, localImage, rootDir); err != nil {
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

func generateSBOMWithSyft(ctx context.Context, imageRef, outputFile string, localImage bool, rootDir string) error {
	syftEnv := ensureSyftEnv(rootDir)
	logger.Info("generating SBOM",
		"image", imageRef,
		"format", sbomFormat,
		"output", outputFile,
	)

	if localImage && exec.CheckCommand("podman") && os.Getenv("CONTAINER_HOST") != "" {
		podmanImageRef := "podman:" + imageRef
		logger.Info("syft scan via podman engine", "image", podmanImageRef)
		result := runSyft(ctx, syftEnv, "scan", podmanImageRef, "-o", fmt.Sprintf("%s=%s", sbomFormat, outputFile))
		if result.Err == nil {
			logger.Info("SBOM generated", "output", outputFile)
			return nil
		}
		logger.Warn("SBOM generation via podman engine failed", "stderr", exec.LastNLines(result.Stderr, 20))
	}

	if localImage && exec.CheckCommand("podman") {
		ociArchivePath := filepath.Join(rootDir, "sbom-image.oci.tar")
		save := exec.Podman(ctx, "image", "save", "--format", "oci-archive", "-o", ociArchivePath, imageRef)
		if save.Err == nil {
			defer func() {
				_ = os.Remove(ociArchivePath)
			}()
			logger.Info("syft scan via oci-archive", "path", ociArchivePath)
			result := runSyft(ctx, syftEnv, "scan", "oci-archive:"+ociArchivePath, "-o", fmt.Sprintf("%s=%s", sbomFormat, outputFile))
			if result.Err == nil {
				logger.Info("SBOM generated", "output", outputFile)
				return nil
			}
			logger.Warn("SBOM generation via oci-archive failed", "stderr", exec.LastNLines(result.Stderr, 20))
		} else {
			logger.Warn("podman image save failed", "stderr", exec.LastNLines(save.Stderr, 20))
		}
	}

	result := runSyft(ctx, syftEnv, "scan", imageRef, "-o", fmt.Sprintf("%s=%s", sbomFormat, outputFile))
	if result.Err != nil {
		logger.Error("SBOM generation failed", "stderr", exec.LastNLines(result.Stderr, 20))
		return fmt.Errorf("SBOM generation failed: %w", result.Err)
	}

	logger.Info("SBOM generated", "output", outputFile)
	return nil
}

func generateSBOMWithContainer(ctx context.Context, imageRef, outputFile string, localImage bool, rootDir string) error {
	if !exec.CheckCommand("podman") {
		msg := "syft not found and podman unavailable; cannot generate SBOM"
		logger.Error(msg)
		return fmt.Errorf(msg)
	}

	logger.Info("syft not found; using container fallback")
	target := imageRef
	ociArchivePath := ""
	if localImage {
		ociArchivePath = filepath.Join(rootDir, "sbom-image.oci.tar")
		save := exec.Podman(ctx, "image", "save", "--format", "oci-archive", "-o", ociArchivePath, imageRef)
		if save.Err != nil {
			logger.Warn("podman image save failed", "stderr", exec.LastNLines(save.Stderr, 20))
		} else {
			target = "oci-archive:" + ociArchivePath
		}
	}
	if ociArchivePath != "" {
		defer func() {
			_ = os.Remove(ociArchivePath)
		}()
	}

	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:%s:Z", rootDir, rootDir),
		"-w", rootDir,
		"ghcr.io/anchore/syft:latest",
		"scan", target,
		"-o", fmt.Sprintf("%s=%s", sbomFormat, outputFile),
	}
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

func ensurePodmanService(ctx context.Context) func() {
	if os.Getenv("CONTAINER_HOST") != "" || !exec.CheckCommand("podman") {
		return nil
	}

	socket := filepath.Join(os.TempDir(), fmt.Sprintf("podman-syft-%d.sock", time.Now().UnixNano()))
	cmd := osexec.CommandContext(ctx, "podman", "system", "service", "--time=0", "unix://"+socket)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		logger.Warn("failed to start podman service", "error", err)
		return nil
	}

	_ = os.Setenv("CONTAINER_HOST", "unix://"+socket)
	logger.Info("podman service started for syft", "socket", socket)

	return func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = os.Remove(socket)
		_ = os.Unsetenv("CONTAINER_HOST")
	}
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
