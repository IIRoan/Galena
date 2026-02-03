// Package version handles version computation and management for galena
package version

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Info holds version information for a build
type Info struct {
	Version       string    `json:"version"`
	FedoraVersion string    `json:"fedora_version"`
	BuildDate     time.Time `json:"build_date"`
	BuildNumber   int       `json:"build_number"`
	GitCommit     string    `json:"git_commit,omitempty"`
	GitBranch     string    `json:"git_branch,omitempty"`
	GitDirty      bool      `json:"git_dirty,omitempty"`
	ImageRef      string    `json:"image_ref,omitempty"`
	Variant       string    `json:"variant,omitempty"`
	Tag           string    `json:"tag,omitempty"`
}

// BuildManifest holds the complete build manifest
type BuildManifest struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	Project       string    `json:"project"`
	Version       Info      `json:"version"`
	Images        []Image   `json:"images"`
	Artifacts     []string  `json:"artifacts,omitempty"`
	SBOM          *SBOM     `json:"sbom,omitempty"`
	Signatures    []string  `json:"signatures,omitempty"`
}

// Image represents a built image
type Image struct {
	Name    string `json:"name"`
	Tag     string `json:"tag"`
	Digest  string `json:"digest,omitempty"`
	Size    int64  `json:"size,omitempty"`
	Variant string `json:"variant"`
}

// SBOM holds SBOM metadata
type SBOM struct {
	Format    string `json:"format"` // e.g., "spdx-json", "cyclonedx"
	Location  string `json:"location"`
	Algorithm string `json:"algorithm,omitempty"`
	Hash      string `json:"hash,omitempty"`
}

// Compute computes a version string based on the Bluefin pattern
// Format: FedoraMajor.YYYYMMDD.BuildNumber
func Compute(fedoraVersion string, buildNumber int) string {
	now := time.Now()
	return fmt.Sprintf("%s.%s.%d", fedoraVersion, now.Format("20060102"), buildNumber)
}

// ComputeWithDate computes a version string with a specific date
func ComputeWithDate(fedoraVersion string, date time.Time, buildNumber int) string {
	return fmt.Sprintf("%s.%s.%d", fedoraVersion, date.Format("20060102"), buildNumber)
}

// NewInfo creates a new version info
func NewInfo(fedoraVersion string, buildNumber int) Info {
	now := time.Now()
	return Info{
		Version:       Compute(fedoraVersion, buildNumber),
		FedoraVersion: fedoraVersion,
		BuildDate:     now,
		BuildNumber:   buildNumber,
	}
}

// WithGit adds git information to the version info
func (v Info) WithGit(commit, branch string, dirty bool) Info {
	v.GitCommit = commit
	v.GitBranch = branch
	v.GitDirty = dirty
	return v
}

// WithImage adds image information to the version info
func (v Info) WithImage(imageRef, variant, tag string) Info {
	v.ImageRef = imageRef
	v.Variant = variant
	v.Tag = tag
	return v
}

// NewBuildManifest creates a new build manifest
func NewBuildManifest(project string, version Info) *BuildManifest {
	return &BuildManifest{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Project:       project,
		Version:       version,
		Images:        []Image{},
		Artifacts:     []string{},
	}
}

// AddImage adds an image to the manifest
func (m *BuildManifest) AddImage(name, tag, digest, variant string, size int64) {
	m.Images = append(m.Images, Image{
		Name:    name,
		Tag:     tag,
		Digest:  digest,
		Size:    size,
		Variant: variant,
	})
}

// AddArtifact adds an artifact to the manifest
func (m *BuildManifest) AddArtifact(path string) {
	m.Artifacts = append(m.Artifacts, path)
}

// SetSBOM sets the SBOM information
func (m *BuildManifest) SetSBOM(format, location string) {
	m.SBOM = &SBOM{
		Format:   format,
		Location: location,
	}
}

// AddSignature adds a signature reference
func (m *BuildManifest) AddSignature(ref string) {
	m.Signatures = append(m.Signatures, ref)
}

// Save saves the manifest to a file
func (m *BuildManifest) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating manifest directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	return nil
}

// LoadManifest loads a manifest from a file
func LoadManifest(path string) (*BuildManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}

	var m BuildManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	return &m, nil
}

// OSReleaseVars returns os-release compatible variables
func (v Info) OSReleaseVars() map[string]string {
	return map[string]string{
		"IMAGE_VERSION":     v.Version,
		"IMAGE_DATE":        v.BuildDate.Format("2006-01-02"),
		"IMAGE_BUILD_DATE":  v.BuildDate.Format(time.RFC3339),
		"IMAGE_VARIANT":     v.Variant,
		"IMAGE_TAG":         v.Tag,
		"FEDORA_VERSION":    v.FedoraVersion,
		"BUILD_NUMBER":      fmt.Sprintf("%d", v.BuildNumber),
		"GIT_COMMIT":        v.GitCommit,
		"GIT_BRANCH":        v.GitBranch,
	}
}

// Labels returns OCI labels for the image
func (v Info) Labels() map[string]string {
	labels := map[string]string{
		"org.opencontainers.image.version":  v.Version,
		"org.opencontainers.image.created":  v.BuildDate.Format(time.RFC3339),
		"org.opencontainers.image.revision": v.GitCommit,
	}

	if v.Variant != "" {
		labels["io.galena.variant"] = v.Variant
	}
	if v.Tag != "" {
		labels["io.galena.tag"] = v.Tag
	}

	return labels
}
