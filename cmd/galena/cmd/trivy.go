package cmd

import (
	"context"
	"os"
	"path/filepath"

	"github.com/iiroan/galena/internal/exec"
)

func ensureTrivyEnv(rootDir string) []string {
	cacheBase := filepath.Join(rootDir, ".cache")
	trivyCache := filepath.Join(cacheBase, "trivy")

	_ = os.MkdirAll(trivyCache, 0o755)

	env := []string{}
	if os.Getenv("TRIVY_CACHE_DIR") == "" {
		env = append(env, "TRIVY_CACHE_DIR="+trivyCache)
	}
	if os.Getenv("TRIVY_SKIP_DB_UPDATE") == "" {
		env = append(env, "TRIVY_SKIP_DB_UPDATE=false")
	}
	if os.Getenv("TRIVY_SKIP_JAVA_DB_UPDATE") == "" {
		env = append(env, "TRIVY_SKIP_JAVA_DB_UPDATE=false")
	}
	return env
}

func runTrivy(ctx context.Context, env []string, args ...string) *exec.Result {
	opts := exec.DefaultOptions()
	opts.Env = env
	return exec.Run(ctx, "trivy", args, opts)
}
