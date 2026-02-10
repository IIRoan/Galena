// Package exec provides command execution utilities for galena
package exec

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// Result holds the result of a command execution
type Result struct {
	Command  string
	Args     []string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
	Err      error
}

// Options configures command execution
type Options struct {
	Dir         string
	Env         []string
	Timeout     time.Duration
	Stdin       io.Reader
	StreamStdio bool // Stream stdout/stderr to terminal in real-time
	Logger      *log.Logger
}

// DefaultOptions returns default execution options
func DefaultOptions() Options {
	return Options{
		Timeout:     30 * time.Minute,
		StreamStdio: false,
	}
}

// Run executes a command and returns the result
func Run(ctx context.Context, name string, args []string, opts Options) *Result {
	start := time.Now()

	result := &Result{
		Command: name,
		Args:    args,
	}

	// Apply timeout
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)

	// Set working directory
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}

	// Set environment
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	// Set stdin
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	}

	var stdout, stderr bytes.Buffer

	// Global session logging if enabled
	var stdoutW, stderrW io.Writer
	if opts.StreamStdio {
		stdoutW = io.MultiWriter(os.Stdout, &stdout)
		stderrW = io.MultiWriter(os.Stderr, &stderr)
	} else {
		stdoutW = &stdout
		stderrW = &stderr
	}

	// Capture to global log if phase is set
	phase, _ := ctx.Value("build-phase").(string)
	logPath, _ := ctx.Value("log-file").(string)
	if phase != "" && logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
			defer f.Close()
			stdoutW = io.MultiWriter(stdoutW, f)
			stderrW = io.MultiWriter(stderrW, f)
			fmt.Fprintf(f, "\n--- [%s] Executing: %s %s ---\n", phase, name, strings.Join(args, " "))
		}
	}

	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	if opts.Logger != nil {
		opts.Logger.Debug("executing command", "cmd", name, "args", args)
	}

	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Err = err
	}

	if opts.Logger != nil {
		if err != nil {
			opts.Logger.Debug("command failed",
				"cmd", name,
				"exit_code", result.ExitCode,
				"duration", result.Duration,
			)
		} else {
			opts.Logger.Debug("command succeeded",
				"cmd", name,
				"duration", result.Duration,
			)
		}
	}

	return result
}

// RunSimple runs a command with default options
func RunSimple(ctx context.Context, name string, args ...string) *Result {
	return Run(ctx, name, args, DefaultOptions())
}

// RunInDir runs a command in a specific directory
func RunInDir(ctx context.Context, dir string, name string, args ...string) *Result {
	opts := DefaultOptions()
	opts.Dir = dir
	return Run(ctx, name, args, opts)
}

// RunStreaming runs a command with output streaming to terminal
func RunStreaming(ctx context.Context, name string, args []string, opts Options) *Result {
	opts.StreamStdio = true
	return Run(ctx, name, args, opts)
}

// Just runs a just command
func Just(ctx context.Context, dir string, recipe string, args ...string) *Result {
	allArgs := append([]string{recipe}, args...)
	opts := DefaultOptions()
	opts.Dir = dir
	opts.StreamStdio = true
	return Run(ctx, "just", allArgs, opts)
}

// Podman runs a podman command
func Podman(ctx context.Context, args ...string) *Result {
	return RunSimple(ctx, "podman", args...)
}

// PodmanBuild runs podman build with streaming output
func PodmanBuild(ctx context.Context, dir string, args []string) *Result {
	allArgs := append([]string{"build"}, args...)
	opts := DefaultOptions()
	opts.Dir = dir
	opts.StreamStdio = true
	opts.Timeout = 60 * time.Minute
	return Run(ctx, "podman", allArgs, opts)
}

// PodmanPush pushes an image to a registry
func PodmanPush(ctx context.Context, image string) *Result {
	opts := DefaultOptions()
	opts.StreamStdio = true
	return Run(ctx, "podman", []string{"push", image}, opts)
}

// Git runs a git command
func Git(ctx context.Context, dir string, args ...string) *Result {
	opts := DefaultOptions()
	opts.Dir = dir
	return Run(ctx, "git", args, opts)
}

// Cosign runs a cosign command
func Cosign(ctx context.Context, args ...string) *Result {
	return RunSimple(ctx, "cosign", args...)
}

// Syft runs a syft command
func Syft(ctx context.Context, args ...string) *Result {
	return RunSimple(ctx, "syft", args...)
}

// Trivy runs a trivy command
func Trivy(ctx context.Context, args ...string) *Result {
	return RunSimple(ctx, "trivy", args...)
}

// BootcImageBuilder runs bootc-image-builder in a container
func BootcImageBuilder(ctx context.Context, image string, outputType string, configFile string, outputDir string) *Result {
	args := []string{
		"run",
		"--rm",
		"--privileged",
		"--pull=newer",
		"--security-opt", "label=type:unconfined_t",
		"-v", outputDir + ":/output",
	}

	if configFile != "" {
		args = append(args, "-v", configFile+":/config.toml:ro")
	}

	args = append(args,
		"quay.io/centos-bootc/bootc-image-builder:latest",
		"--type", outputType,
		"--local",
		image,
	)

	opts := DefaultOptions()
	opts.StreamStdio = true
	opts.Timeout = 60 * time.Minute

	return Run(ctx, "podman", args, opts)
}

// RunPipe executes two commands piped together: cmd1 | cmd2
func RunPipe(ctx context.Context, name1 string, args1 []string, name2 string, args2 []string, opts Options) *Result {
	start := time.Now()
	result := &Result{
		Command: fmt.Sprintf("%s | %s", name1, name2),
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	cmd1 := exec.CommandContext(ctx, name1, args1...)
	cmd2 := exec.CommandContext(ctx, name2, args2...)

	if opts.Dir != "" {
		cmd1.Dir = opts.Dir
		cmd2.Dir = opts.Dir
	}

	if len(opts.Env) > 0 {
		env := append(os.Environ(), opts.Env...)
		cmd1.Env = env
		cmd2.Env = env
	}

	// Create pipe
	pr, pw := io.Pipe()
	cmd1.Stdout = pw
	cmd2.Stdin = pr

	var stdout, stderr bytes.Buffer
	var stdoutW, stderrW io.Writer
	if opts.StreamStdio {
		stdoutW = io.MultiWriter(os.Stdout, &stdout)
		stderrW = io.MultiWriter(os.Stderr, &stderr)
	} else {
		stdoutW = &stdout
		stderrW = &stderr
	}

	cmd1.Stderr = stderrW
	cmd2.Stdout = stdoutW
	cmd2.Stderr = stderrW

	if opts.Logger != nil {
		opts.Logger.Debug("executing piped commands",
			"cmd1", name1, "args1", args1,
			"cmd2", name2, "args2", args2,
		)
	}

	if err := cmd1.Start(); err != nil {
		result.Err = fmt.Errorf("cmd1 start: %w", err)
		return result
	}
	if err := cmd2.Start(); err != nil {
		result.Err = fmt.Errorf("cmd2 start: %w", err)
		return result
	}

	// Wait for cmd1 in a goroutine
	go func() {
		_ = cmd1.Wait()
		pw.Close()
	}()

	err := cmd2.Wait()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		result.Err = err
	}

	return result
}

// CheckCommand checks if a command is available
func CheckCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RequireCommands checks if all required commands are available
func RequireCommands(commands ...string) error {
	missing := []string{}
	for _, cmd := range commands {
		if !CheckCommand(cmd) {
			missing = append(missing, cmd)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required commands: %s", strings.Join(missing, ", "))
	}
	return nil
}

// FormatCommand formats a command for display
func FormatCommand(name string, args []string) string {
	parts := append([]string{name}, args...)
	return strings.Join(parts, " ")
}

// LastNLines returns the last n lines of a string
func LastNLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
