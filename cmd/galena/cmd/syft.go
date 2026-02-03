package cmd

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/iiroan/galena/internal/exec"
)

func ensureSyftEnv(rootDir string) []string {
	cacheBase := filepath.Join(rootDir, ".cache")
	syftCache := filepath.Join(cacheBase, "syft")
	stereoCache := filepath.Join(syftCache, "stereoscope")
	tmpDir := filepath.Join(cacheBase, "tmp")

	_ = os.MkdirAll(syftCache, 0o755)
	_ = os.MkdirAll(stereoCache, 0o755)
	_ = os.MkdirAll(tmpDir, 0o755)

	env := []string{}
	if os.Getenv("SYFT_CACHE_DIR") == "" {
		env = append(env, "SYFT_CACHE_DIR="+syftCache)
	}
	if os.Getenv("STEREOSCOPE_CACHE") == "" {
		env = append(env, "STEREOSCOPE_CACHE="+stereoCache)
	}
	if os.Getenv("XDG_CACHE_HOME") == "" {
		env = append(env, "XDG_CACHE_HOME="+cacheBase)
	}
	if os.Getenv("TMPDIR") == "" {
		env = append(env, "TMPDIR="+tmpDir)
	}
	if os.Getenv("SYFT_CHECK_FOR_APP_UPDATE") == "" {
		env = append(env, "SYFT_CHECK_FOR_APP_UPDATE=false")
	}
	if os.Getenv("SYFT_PARALLELISM") == "" {
		parallelism := defaultSyftParallelism()
		env = append(env, "SYFT_PARALLELISM="+strconv.Itoa(parallelism))
	}
	return env
}

func runSyft(ctx context.Context, env []string, args ...string) *exec.Result {
	opts := exec.DefaultOptions()
	opts.Env = env
	return exec.Run(ctx, "syft", args, opts)
}

func resolveSBOMScope() string {
	if scope := strings.TrimSpace(os.Getenv("GALENA_SBOM_SCOPE")); scope != "" {
		return scope
	}
	if scope := strings.TrimSpace(os.Getenv("SYFT_SCOPE")); scope != "" {
		return scope
	}
	if os.Getenv("CI") == "true" {
		return "squashed"
	}
	return "all-layers"
}

func defaultSyftParallelism() int {
	cpus := runtime.NumCPU()
	if os.Getenv("CI") == "true" {
		if cpus > 2 {
			return 2
		}
		return 1
	}
	if cpus > 0 {
		return cpus
	}
	return 1
}
