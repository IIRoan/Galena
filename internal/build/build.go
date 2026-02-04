// Package build provides build orchestration for galena
package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/iiroan/galena/internal/config"
	"github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/version"
)

// Builder orchestrates the image build process
type Builder struct {
	cfg     *config.Config
	rootDir string
	logger  *log.Logger
}

// BuildOptions configures a build
type BuildOptions struct {
	Variant        string
	Tag            string
	BuildNumber    int
	NoCache        bool
	Push           bool
	Sign           bool
	SBOM           bool
	Rechunk        bool
	DryRun         bool
	ExtraBuildArgs map[string]string
	Timeout        time.Duration
}

// DefaultBuildOptions returns default build options
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		Variant:        "main",
		Tag:            "latest",
		BuildNumber:    0,
		NoCache:        false,
		Push:           false,
		Sign:           false,
		SBOM:           false,
		Rechunk:        false,
		DryRun:         false,
		ExtraBuildArgs: nil,
		Timeout:        60 * time.Minute,
	}
}

// NewBuilder creates a new builder
func NewBuilder(cfg *config.Config, rootDir string, logger *log.Logger) *Builder {
	return &Builder{
		cfg:     cfg,
		rootDir: rootDir,
		logger:  logger,
	}
}

// Build builds an image with the given options
func (b *Builder) Build(ctx context.Context, opts BuildOptions) (*version.BuildManifest, error) {
	if opts.Timeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
			defer cancel()
		}
	}

	// Validate
	if err := b.cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Check required tools
	if err := exec.RequireCommands("podman"); err != nil {
		return nil, err
	}

	// Compute version
	versionInfo := version.NewInfo(b.cfg.Build.FedoraVersion, opts.BuildNumber)

	// Get git info
	gitCommit, gitBranch, gitDirty := b.getGitInfo(ctx)
	versionInfo = versionInfo.WithGit(gitCommit, gitBranch, gitDirty)

	// Compute image reference
	imageRef := b.cfg.ImageRef(opts.Variant, opts.Tag)
	versionInfo = versionInfo.WithImage(imageRef, opts.Variant, opts.Tag)

	b.logger.Info("starting build",
		"image", imageRef,
		"version", versionInfo.Version,
		"variant", opts.Variant,
	)

	// Create manifest
	manifest := version.NewBuildManifest(b.cfg.Name, versionInfo)

	if opts.DryRun {
		b.logger.Info("dry run - skipping actual build")
		return manifest, nil
	}

	// Build the image
	buildArgs := b.prepareBuildArgs(opts, versionInfo)
	if err := b.runPodmanBuild(ctx, opts, buildArgs); err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	// Get image digest
	digest, err := b.getImageDigest(ctx, imageRef)
	if err != nil {
		b.logger.Warn("could not get image digest", "error", err)
	}

	manifest.AddImage(b.cfg.Name, opts.Tag, digest, opts.Variant, 0)

	// Push if requested
	if opts.Push {
		if err := b.push(ctx, imageRef); err != nil {
			return nil, fmt.Errorf("push failed: %w", err)
		}
	}

	// Sign if requested
	if opts.Sign {
		if err := b.sign(ctx, imageRef); err != nil {
			return nil, fmt.Errorf("signing failed: %w", err)
		}
		manifest.AddSignature(imageRef + ".sig")
	}

	// Generate SBOM if requested
	if opts.SBOM {
		sbomPath, err := b.generateSBOM(ctx, imageRef)
		if err != nil {
			return nil, fmt.Errorf("SBOM generation failed: %w", err)
		}
		manifest.SetSBOM("spdx-json", sbomPath)
	}

	b.logger.Info("build completed successfully",
		"image", imageRef,
		"version", versionInfo.Version,
	)

	return manifest, nil
}

// prepareBuildArgs prepares build arguments for podman build
func (b *Builder) prepareBuildArgs(opts BuildOptions, ver version.Info) []string {
	args := []string{}

	// Add labels
	for k, v := range ver.Labels() {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	mergedArgs := map[string]string{}
	for k, v := range b.cfg.Build.BuildArgs {
		mergedArgs[k] = v
	}
	for k, v := range opts.ExtraBuildArgs {
		mergedArgs[k] = v
	}

	// Add build args from config and overrides
	for k, v := range mergedArgs {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", k, v))
	}

	// Standard build args
	args = append(args,
		"--build-arg", fmt.Sprintf("FEDORA_MAJOR_VERSION=%s", b.cfg.Build.FedoraVersion),
		"--build-arg", fmt.Sprintf("IMAGE_VERSION=%s", ver.Version),
	)

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	return args
}

// runPodmanBuild executes the podman build command
func (b *Builder) runPodmanBuild(ctx context.Context, opts BuildOptions, buildArgs []string) error {
	imageRef := b.cfg.ImageRef(opts.Variant, opts.Tag)

	args := append([]string{}, buildArgs...)
	args = append(args,
		"-t", imageRef,
		"-f", filepath.Join(b.rootDir, "Containerfile"),
		b.rootDir,
	)

	b.logger.Debug("running podman build", "args", args)

	result := exec.PodmanBuild(ctx, b.rootDir, args)
	if result.Err != nil {
		b.logger.Error("podman build failed",
			"exit_code", result.ExitCode,
			"stderr", exec.LastNLines(result.Stderr, 20),
		)
		return result.Err
	}

	return nil
}

// getImageDigest gets the digest of a local image
func (b *Builder) getImageDigest(ctx context.Context, imageRef string) (string, error) {
	result := exec.Podman(ctx, "inspect", "--format", "{{.Digest}}", imageRef)
	if result.Err != nil {
		return "", result.Err
	}
	return strings.TrimSpace(result.Stdout), nil
}

// push pushes an image to the registry
func (b *Builder) push(ctx context.Context, imageRef string) error {
	b.logger.Info("pushing image", "image", imageRef)

	result := exec.PodmanPush(ctx, imageRef)
	if result.Err != nil {
		return result.Err
	}

	return nil
}

// sign signs an image with cosign
func (b *Builder) sign(ctx context.Context, imageRef string) error {
	if err := exec.RequireCommands("cosign"); err != nil {
		return err
	}

	b.logger.Info("signing image", "image", imageRef)

	// Use keyless signing with GitHub OIDC
	result := exec.Cosign(ctx, "sign", "--yes", imageRef)
	if result.Err != nil {
		b.logger.Error("cosign sign failed", "stderr", result.Stderr)
		return result.Err
	}

	return nil
}

// generateSBOM generates an SBOM for the image
func (b *Builder) generateSBOM(ctx context.Context, imageRef string) (string, error) {
	if err := exec.RequireCommands("trivy"); err != nil {
		return "", err
	}

	b.logger.Info("generating SBOM", "image", imageRef)

	outputPath := filepath.Join(b.rootDir, "sbom.spdx.json")

	result := exec.Trivy(ctx,
		"image", imageRef,
		"--format", "spdx-json",
		"--output", outputPath,
	)
	if result.Err != nil {
		b.logger.Error("trivy scan failed", "stderr", result.Stderr)
		return "", result.Err
	}

	return outputPath, nil
}

// getGitInfo retrieves git information for the build
func (b *Builder) getGitInfo(ctx context.Context) (commit, branch string, dirty bool) {
	// Get commit
	result := exec.Git(ctx, b.rootDir, "rev-parse", "HEAD")
	if result.Err == nil {
		commit = strings.TrimSpace(result.Stdout)
		if len(commit) > 12 {
			commit = commit[:12]
		}
	}

	// Get branch
	result = exec.Git(ctx, b.rootDir, "rev-parse", "--abbrev-ref", "HEAD")
	if result.Err == nil {
		branch = strings.TrimSpace(result.Stdout)
	}

	// Check if dirty
	result = exec.Git(ctx, b.rootDir, "status", "--porcelain")
	if result.Err == nil {
		dirty = len(strings.TrimSpace(result.Stdout)) > 0
	}

	return
}

// BuildViaJust builds using the existing Justfile (Phase 1 approach)
func (b *Builder) BuildViaJust(ctx context.Context, opts BuildOptions) error {
	if err := exec.RequireCommands("just"); err != nil {
		return err
	}

	b.logger.Info("building via just",
		"variant", opts.Variant,
		"tag", opts.Tag,
	)

	// Determine image name (match Justfile expectations)
	image := b.cfg.Name
	if opts.Variant != "" && opts.Variant != "main" {
		image = image + "-" + opts.Variant
	}

	result := exec.Just(ctx, b.rootDir, "build", image, opts.Tag)
	if result.Err != nil {
		b.logger.Error("just build failed",
			"exit_code", result.ExitCode,
			"stderr", exec.LastNLines(result.Stderr, 20),
		)
		return result.Err
	}

	return nil
}

// Lint runs bootc container lint on the image
func (b *Builder) Lint(ctx context.Context, imageRef string) error {
	b.logger.Info("linting image", "image", imageRef)

	result := exec.Podman(ctx, "run", "--rm", imageRef, "bootc", "container", "lint")
	if result.Err != nil {
		b.logger.Error("bootc lint failed", "stderr", result.Stderr)
		return result.Err
	}

	return nil
}

// Clean removes built images
func (b *Builder) Clean(ctx context.Context, imageRef string) error {
	b.logger.Info("removing image", "image", imageRef)

	result := exec.Podman(ctx, "rmi", "-f", imageRef)
	if result.Err != nil {
		b.logger.Warn("could not remove image", "error", result.Err)
	}

	return nil
}

// ListLocalImages lists locally built images
func (b *Builder) ListLocalImages(ctx context.Context) ([]string, error) {
	filter := fmt.Sprintf("reference=*%s*", b.cfg.Name)
	result := exec.Podman(ctx, "images", "--filter", filter, "--format", "{{.Repository}}:{{.Tag}}")
	if result.Err != nil {
		return nil, result.Err
	}

	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	images := []string{}
	for _, line := range lines {
		if line != "" {
			images = append(images, line)
		}
	}

	return images, nil
}

// Status returns the current build status
func (b *Builder) Status(ctx context.Context) (map[string]interface{}, error) {
	status := map[string]interface{}{
		"project":        b.cfg.Name,
		"root_dir":       b.rootDir,
		"base_image":     b.cfg.Build.BaseImage,
		"fedora_version": b.cfg.Build.FedoraVersion,
		"variants":       b.cfg.ListVariantNames(),
	}

	// Check for Containerfile
	containerfile := filepath.Join(b.rootDir, "Containerfile")
	if _, err := os.Stat(containerfile); err == nil {
		status["containerfile"] = containerfile
	}

	// Check for Justfile
	justfile := filepath.Join(b.rootDir, "Justfile")
	if _, err := os.Stat(justfile); err == nil {
		status["justfile"] = justfile
	}

	// List local images
	images, err := b.ListLocalImages(ctx)
	if err == nil {
		status["local_images"] = images
	}

	return status, nil
}
