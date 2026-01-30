// Package config handles configuration loading and validation for finctl
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the main configuration for finctl
type Config struct {
	// Project metadata
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Registry    string `yaml:"registry"`
	Repository  string `yaml:"repository"`

	// Build configuration
	Build BuildConfig `yaml:"build"`

	// Version configuration
	Version VersionConfig `yaml:"version"`

	// Image variants
	Variants []Variant `yaml:"variants"`

	// Dependencies (digest-pinned images)
	Dependencies map[string]Dependency `yaml:"dependencies"`
}

// BuildConfig holds build-related settings
type BuildConfig struct {
	BaseImage     string            `yaml:"base_image"`
	FedoraVersion string            `yaml:"fedora_version"`
	BuildArgs     map[string]string `yaml:"build_args"`
	CacheMounts   []string          `yaml:"cache_mounts"`
	Timeout       string            `yaml:"timeout"`
}

// VersionConfig holds versioning settings
type VersionConfig struct {
	Scheme  string `yaml:"scheme"` // e.g., "fedora.date.build"
	Current string `yaml:"current"`
}

// Variant represents an image variant (e.g., main, nvidia, dx)
type Variant struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Flavor      string   `yaml:"flavor"`
	Scripts     []string `yaml:"scripts"`
	Packages    []string `yaml:"packages"`
}

// Dependency represents a pinned external dependency
type Dependency struct {
	Image  string `yaml:"image"`
	Digest string `yaml:"digest"`
	Tag    string `yaml:"tag"`
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		Name:        "finpilot",
		Description: "OCI-native OS appliance",
		Registry:    "ghcr.io",
		Repository:  "",
		Build: BuildConfig{
			BaseImage:     "ghcr.io/ublue-os/silverblue-main",
			FedoraVersion: "42",
			BuildArgs:     make(map[string]string),
			CacheMounts: []string{
				"/var/cache/rpm-ostree",
				"/var/cache/libdnf5",
			},
			Timeout: "30m",
		},
		Version: VersionConfig{
			Scheme: "fedora.date.build",
		},
		Variants: []Variant{
			{
				Name:        "main",
				Description: "Standard desktop variant",
				Flavor:      "main",
				Scripts:     []string{"10-build.sh"},
			},
		},
		Dependencies: make(map[string]Dependency),
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}

// Save saves configuration to a file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("name is required")
	}
	if c.Build.BaseImage == "" {
		return fmt.Errorf("build.base_image is required")
	}
	if c.Build.FedoraVersion == "" {
		return fmt.Errorf("build.fedora_version is required")
	}
	return nil
}

// ImageRef returns the full image reference for a variant and tag
func (c *Config) ImageRef(variant, tag string) string {
	name := c.Name
	if variant != "" && variant != "main" {
		name = name + "-" + variant
	}

	// For pushes and remote references, registry and repository must be set
	if c.Registry != "" && c.Repository != "" {
		return fmt.Sprintf("%s/%s/%s:%s", c.Registry, c.Repository, name, tag)
	}

	// For local-only builds without registry/repository set, default to localhost/
	return fmt.Sprintf("localhost/%s:%s", name, tag)
}

// ComputeVersion computes the version string based on the scheme
func (c *Config) ComputeVersion(buildNum int) string {
	now := time.Now()

	switch c.Version.Scheme {
	case "fedora.date.build":
		return fmt.Sprintf("%s.%s.%d", c.Build.FedoraVersion, now.Format("20060102"), buildNum)
	case "date.build":
		return fmt.Sprintf("%s.%d", now.Format("20060102"), buildNum)
	case "semver":
		if c.Version.Current != "" {
			return c.Version.Current
		}
		return "0.1.0"
	default:
		return fmt.Sprintf("%s.%s.%d", c.Build.FedoraVersion, now.Format("20060102"), buildNum)
	}
}

// FindProjectRoot finds the project root by looking for Containerfile
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "Containerfile")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Fall back to current directory
	cwd, _ := os.Getwd()
	return cwd, nil
}

// GetConfigPath returns the path to finctl.yaml in the project root
func GetConfigPath() (string, error) {
	root, err := FindProjectRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "finctl.yaml"), nil
}

// LoadFromProject loads configuration from the project root
func LoadFromProject() (*Config, error) {
	path, err := GetConfigPath()
	if err != nil {
		return nil, err
	}
	return Load(path)
}

// GetVariant returns a variant by name
func (c *Config) GetVariant(name string) (*Variant, error) {
	for i := range c.Variants {
		if c.Variants[i].Name == name {
			return &c.Variants[i], nil
		}
	}
	return nil, fmt.Errorf("variant %q not found", name)
}

// ListVariantNames returns a list of variant names
func (c *Config) ListVariantNames() []string {
	names := make([]string, len(c.Variants))
	for i, v := range c.Variants {
		names[i] = v.Name
	}
	return names
}

// GetDependencyRef returns the full reference for a dependency (with digest if available)
func (c *Config) GetDependencyRef(name string) (string, error) {
	dep, ok := c.Dependencies[name]
	if !ok {
		return "", fmt.Errorf("dependency %q not found", name)
	}

	if dep.Digest != "" {
		return fmt.Sprintf("%s@%s", dep.Image, dep.Digest), nil
	}
	if dep.Tag != "" {
		return fmt.Sprintf("%s:%s", dep.Image, dep.Tag), nil
	}
	return dep.Image, nil
}

// NormalizeTag normalizes a tag name
func NormalizeTag(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.ToLower(tag)
	return tag
}
