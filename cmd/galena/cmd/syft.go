package cmd

import (
	"context"
	"os"
	"path/filepath"

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
	return env
}

func runSyft(ctx context.Context, env []string, args ...string) *exec.Result {
	opts := exec.DefaultOptions()
	opts.Env = env
	return exec.Run(ctx, "syft", args, opts)
}
