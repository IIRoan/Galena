package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/finpilot/finctl/internal/exec"
	"github.com/finpilot/finctl/internal/ui"
)

var (
	signKeyless  bool
	signKey      string
	signVerify   bool
)

var signCmd = &cobra.Command{
	Use:   "sign [image]",
	Short: "Sign a container image with cosign",
	Long: `Sign a container image using cosign.

By default, uses keyless signing with GitHub OIDC (recommended for CI/CD).
You can also use a local key file for signing.

Examples:
  # Keyless signing (GitHub OIDC)
  finctl sign ghcr.io/myorg/myimage:stable

  # Sign with a key file
  finctl sign ghcr.io/myorg/myimage:stable --key cosign.key

  # Verify a signature
  finctl sign --verify ghcr.io/myorg/myimage:stable`,
	Args: cobra.ExactArgs(1),
	RunE: runSign,
}

func init() {
	signCmd.Flags().BoolVar(&signKeyless, "keyless", true, "Use keyless signing with OIDC")
	signCmd.Flags().StringVarP(&signKey, "key", "k", "", "Path to cosign private key")
	signCmd.Flags().BoolVar(&signVerify, "verify", false, "Verify signature instead of signing")
}

func runSign(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	imageRef := args[0]

	if err := exec.RequireCommands("cosign"); err != nil {
		return fmt.Errorf("cosign not found: %w\nInstall with: go install github.com/sigstore/cosign/v2/cmd/cosign@latest", err)
	}

	if signVerify {
		return verifySig(ctx, imageRef)
	}

	return signImage(ctx, imageRef)
}

func signImage(ctx context.Context, imageRef string) error {
	logger.Info("signing image", "image", imageRef)

	var result *exec.Result
	if signKey != "" {
		result = exec.Cosign(ctx, "sign", "--key", signKey, "--yes", imageRef)
	} else {
		// Keyless signing
		result = exec.Cosign(ctx, "sign", "--yes", imageRef)
	}

	if result.Err != nil {
		logger.Error("signing failed", "stderr", result.Stderr)
		return fmt.Errorf("signing failed: %w", result.Err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Image signed successfully!\n\n%s", imageRef)))

	return nil
}

func verifySig(ctx context.Context, imageRef string) error {
	logger.Info("verifying signature", "image", imageRef)

	var result *exec.Result
	if signKey != "" {
		result = exec.Cosign(ctx, "verify", "--key", signKey, imageRef)
	} else {
		// Keyless verification
		result = exec.Cosign(ctx, "verify", "--certificate-identity-regexp", ".*", "--certificate-oidc-issuer-regexp", ".*", imageRef)
	}

	if result.Err != nil {
		logger.Error("verification failed", "stderr", result.Stderr)
		return fmt.Errorf("verification failed: %w", result.Err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Signature verified!\n\n%s", imageRef)))

	return nil
}
