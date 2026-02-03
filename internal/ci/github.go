// Package ci provides CI/CD integration utilities for galena
package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Environment represents the CI environment
type Environment struct {
	IsCI            bool
	IsGitHubActions bool

	// GitHub Actions specific
	Repository      string
	RepositoryOwner string
	RepositoryName  string
	Ref             string
	RefName         string
	SHA             string
	RunNumber       int
	RunID           string
	EventName       string
	DefaultBranch   string
	Actor           string
	Workflow        string

	// Computed
	IsDefaultBranch bool
	IsPullRequest   bool
}

// Detect detects the current CI environment
func Detect() *Environment {
	env := &Environment{}

	// Check for CI
	env.IsCI = os.Getenv("CI") == "true"
	env.IsGitHubActions = os.Getenv("GITHUB_ACTIONS") == "true"

	if env.IsGitHubActions {
		env.Repository = os.Getenv("GITHUB_REPOSITORY")
		env.RepositoryOwner = os.Getenv("GITHUB_REPOSITORY_OWNER")
		env.Ref = os.Getenv("GITHUB_REF")
		env.RefName = os.Getenv("GITHUB_REF_NAME")
		env.SHA = os.Getenv("GITHUB_SHA")
		env.RunID = os.Getenv("GITHUB_RUN_ID")
		env.EventName = os.Getenv("GITHUB_EVENT_NAME")
		env.DefaultBranch = os.Getenv("GITHUB_DEFAULT_BRANCH")
		env.Actor = os.Getenv("GITHUB_ACTOR")
		env.Workflow = os.Getenv("GITHUB_WORKFLOW")

		// Parse repository name
		if parts := strings.Split(env.Repository, "/"); len(parts) == 2 {
			env.RepositoryName = parts[1]
		}

		// Parse run number
		if rn := os.Getenv("GITHUB_RUN_NUMBER"); rn != "" {
			if _, err := fmt.Sscanf(rn, "%d", &env.RunNumber); err != nil {
				env.RunNumber = 0
			}
		}

		// Computed values
		env.IsDefaultBranch = env.RefName == env.DefaultBranch
		env.IsPullRequest = env.EventName == "pull_request"
	}

	return env
}

// SetOutput sets a GitHub Actions output variable
func SetOutput(name, value string) error {
	outputFile := os.Getenv("GITHUB_OUTPUT")
	if outputFile == "" {
		// Not in GitHub Actions, just print
		fmt.Printf("::set-output name=%s::%s\n", name, value)
		return nil
	}

	f, err := os.OpenFile(outputFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_OUTPUT: %w", err)
	}
	defer f.Close()

	// Handle multiline values
	if strings.Contains(value, "\n") {
		delimiter := fmt.Sprintf("EOF%d", time.Now().UnixNano())
		_, err = fmt.Fprintf(f, "%s<<%s\n%s\n%s\n", name, delimiter, value, delimiter)
	} else {
		_, err = fmt.Fprintf(f, "%s=%s\n", name, value)
	}

	return err
}

// SetEnv sets a GitHub Actions environment variable for subsequent steps
func SetEnv(name, value string) error {
	envFile := os.Getenv("GITHUB_ENV")
	if envFile == "" {
		return nil
	}

	f, err := os.OpenFile(envFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_ENV: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s=%s\n", name, value)
	return err
}

// AddPath adds a directory to the PATH for subsequent steps
func AddPath(dir string) error {
	pathFile := os.Getenv("GITHUB_PATH")
	if pathFile == "" {
		return nil
	}

	f, err := os.OpenFile(pathFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_PATH: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", dir)
	return err
}

// StartGroup starts a log group in GitHub Actions
func StartGroup(name string) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Printf("::group::%s\n", name)
	}
}

// EndGroup ends a log group in GitHub Actions
func EndGroup() {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Println("::endgroup::")
	}
}

// LogError logs an error annotation
func LogError(message string, file string, line int) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		if file != "" && line > 0 {
			fmt.Printf("::error file=%s,line=%d::%s\n", file, line, message)
		} else {
			fmt.Printf("::error::%s\n", message)
		}
	}
}

// LogWarning logs a warning annotation
func LogWarning(message string) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Printf("::warning::%s\n", message)
	}
}

// LogNotice logs a notice annotation
func LogNotice(message string) {
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		fmt.Printf("::notice::%s\n", message)
	}
}

// AddSummary adds content to the job summary
func AddSummary(markdown string) error {
	summaryFile := os.Getenv("GITHUB_STEP_SUMMARY")
	if summaryFile == "" {
		return nil
	}

	f, err := os.OpenFile(summaryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening GITHUB_STEP_SUMMARY: %w", err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s\n", markdown)
	return err
}

// GenerateTags generates image tags based on CI environment
// Matches the original workflow: stable, stable.YYYYMMDD, YYYYMMDD, and PR-specific tags
func (e *Environment) GenerateTags(defaultTag string) []string {
	if override := os.Getenv("GALENA_CI_TAGS"); override != "" {
		parts := strings.FieldsFunc(override, func(r rune) bool {
			return r == ',' || r == ' ' || r == '\t' || r == '\n'
		})
		overrideTags := []string{}
		for _, tag := range parts {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			overrideTags = append(overrideTags, sanitizeTag(tag))
		}
		if len(overrideTags) > 0 {
			return overrideTags
		}
	}

	tags := []string{}
	now := time.Now()
	dateTag := now.Format("20060102")

	if e.IsPullRequest {
		// PR tags: pr-<number> and sha-<short>
		// Extract PR number from ref (refs/pull/123/merge -> 123)
		prNum := ""
		if strings.HasPrefix(e.Ref, "refs/pull/") {
			parts := strings.Split(e.Ref, "/")
			if len(parts) >= 3 {
				prNum = parts[2]
			}
		}
		if prNum != "" {
			tags = append(tags, fmt.Sprintf("pr-%s", prNum))
		}
		if e.SHA != "" && len(e.SHA) >= 7 {
			tags = append(tags, fmt.Sprintf("sha-%s", e.SHA[:7]))
		}
	} else if e.IsDefaultBranch {
		// Main branch tags: stable, stable.YYYYMMDD, YYYYMMDD
		tags = append(tags, defaultTag)
		tags = append(tags, fmt.Sprintf("%s.%s", defaultTag, dateTag))
		tags = append(tags, dateTag)
	} else {
		// Feature branch
		tags = append(tags, sanitizeTag(e.RefName))
	}

	return tags
}

// LabelConfig holds configuration for label generation
type LabelConfig struct {
	Description string
	Keywords    string
	LogoURL     string
	License     string
}

// DefaultLabelConfig returns default label configuration
func DefaultLabelConfig() LabelConfig {
	return LabelConfig{
		Description: "Custom Universal Blue Image",
		Keywords:    "bootc,ublue,universal-blue",
		LogoURL:     "https://avatars.githubusercontent.com/u/120078124?s=200&v=4",
		License:     "Apache-2.0",
	}
}

// GenerateLabels generates OCI labels for the image including ArtifactHub labels
func (e *Environment) GenerateLabels(imageName string, cfg LabelConfig) map[string]string {
	now := time.Now()
	dateTag := now.Format("20060102")

	// Use defaults if not provided
	if cfg.Description == "" {
		cfg.Description = DefaultLabelConfig().Description
	}
	if cfg.Keywords == "" {
		cfg.Keywords = DefaultLabelConfig().Keywords
	}
	if cfg.LogoURL == "" {
		cfg.LogoURL = DefaultLabelConfig().LogoURL
	}
	if cfg.License == "" {
		cfg.License = DefaultLabelConfig().License
	}

	labels := map[string]string{
		// OCI standard labels
		"org.opencontainers.image.created":     now.Format("2006-01-02T15:04:05Z"),
		"org.opencontainers.image.title":       imageName,
		"org.opencontainers.image.description": cfg.Description,
		"org.opencontainers.image.vendor":      e.RepositoryOwner,
		"org.opencontainers.image.version":     fmt.Sprintf("stable.%s", dateTag),

		// Bootc marker
		"containers.bootc": "1",

		// ArtifactHub labels
		"io.artifacthub.package.deprecated": "false",
		"io.artifacthub.package.keywords":   cfg.Keywords,
		"io.artifacthub.package.license":    cfg.License,
		"io.artifacthub.package.logo-url":   cfg.LogoURL,
		"io.artifacthub.package.prerelease": "false",
	}

	if e.Repository != "" {
		labels["org.opencontainers.image.source"] = fmt.Sprintf("https://github.com/%s/blob/%s/Containerfile", e.Repository, e.SHA)
		labels["org.opencontainers.image.url"] = fmt.Sprintf("https://github.com/%s/tree/%s", e.Repository, e.SHA)
		labels["org.opencontainers.image.documentation"] = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/README.md", e.Repository, e.SHA)
		labels["io.artifacthub.package.readme-url"] = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/README.md", e.Repository, e.SHA)
	}

	if e.SHA != "" {
		labels["org.opencontainers.image.revision"] = e.SHA
	}

	return labels
}

// ImageRegistry returns the image registry from environment or default
func (e *Environment) ImageRegistry() string {
	if registry := os.Getenv("IMAGE_REGISTRY"); registry != "" {
		return strings.ToLower(registry)
	}
	if e.RepositoryOwner != "" {
		return fmt.Sprintf("ghcr.io/%s", strings.ToLower(e.RepositoryOwner))
	}
	return "ghcr.io"
}

// ImageName returns the image name from environment or repository name
func (e *Environment) ImageName() string {
	if name := os.Getenv("IMAGE_NAME"); name != "" {
		return strings.ToLower(name)
	}
	if e.RepositoryName != "" {
		return strings.ToLower(e.RepositoryName)
	}
	return "galena"
}

// FullImageRef returns the full image reference with registry
func (e *Environment) FullImageRef(tag string) string {
	return fmt.Sprintf("%s/%s:%s", e.ImageRegistry(), e.ImageName(), tag)
}

// ShouldPush returns whether the image should be pushed
func (e *Environment) ShouldPush() bool {
	// Don't push on PRs by default
	if e.IsPullRequest {
		return false
	}
	// Push on default branch
	return e.IsDefaultBranch
}

// sanitizeTag sanitizes a string for use as a Docker tag
func sanitizeTag(s string) string {
	// Replace invalid characters
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, " ", "-")

	// Truncate if too long
	if len(s) > 128 {
		s = s[:128]
	}

	return s
}

// CacheDir returns the cache directory for CI
func CacheDir() string {
	if dir := os.Getenv("RUNNER_TOOL_CACHE"); dir != "" {
		return filepath.Join(dir, "galena")
	}
	return filepath.Join(os.TempDir(), "galena-cache")
}
