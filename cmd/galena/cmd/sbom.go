package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var (
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

Examples:
  # Generate SPDX SBOM
  galena sbom ghcr.io/myorg/myimage:stable

  # Generate CycloneDX SBOM
  galena sbom ghcr.io/myorg/myimage:stable --format cyclonedx

  # Generate and attest SBOM to image
  galena sbom ghcr.io/myorg/myimage:stable --attest`,
	Args: cobra.ExactArgs(1),
	RunE: runSBOM,
}

func init() {
	sbomCmd.Flags().StringVarP(&sbomOutput, "output", "o", "", "Output file path (default: sbom.<format>)")
	sbomCmd.Flags().StringVarP(&sbomFormat, "format", "f", "spdx-json", "SBOM format (spdx-json, cyclonedx, json)")
	sbomCmd.Flags().BoolVar(&sbomAttest, "attest", false, "Attest SBOM to image using cosign")
}

func runSBOM(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	imageRef := args[0]

	if err := exec.RequireCommands("syft"); err != nil {
		return fmt.Errorf("syft not found: %w\nInstall with: curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin", err)
	}

	// Determine output file
	outputFile := sbomOutput
	if outputFile == "" {
		rootDir, _ := getProjectRoot()
		ext := ".json"
		outputFile = filepath.Join(rootDir, fmt.Sprintf("sbom.%s%s", sbomFormat, ext))
	}

	logger.Info("generating SBOM",
		"image", imageRef,
		"format", sbomFormat,
		"output", outputFile,
	)

	// Run syft
	result := exec.Syft(ctx, "scan", imageRef, "-o", fmt.Sprintf("%s=%s", sbomFormat, outputFile))
	if result.Err != nil {
		logger.Error("SBOM generation failed", "stderr", result.Stderr)
		return fmt.Errorf("SBOM generation failed: %w", result.Err)
	}

	logger.Info("SBOM generated", "output", outputFile)

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
