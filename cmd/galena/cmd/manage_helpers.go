package cmd

import (
	"fmt"
	"os"
	osexec "os/exec"

	galexec "github.com/iiroan/galena/internal/exec"
)

func commandWithPrivilege(name string, args ...string) (string, []string) {
	if os.Geteuid() == 0 {
		return name, args
	}
	if galexec.CheckCommand("sudo") {
		return "sudo", append([]string{name}, args...)
	}
	if galexec.CheckCommand("pkexec") {
		return "pkexec", append([]string{name}, args...)
	}
	return name, args
}

func runAttachedCommand(name string, args []string) error {
	cmd := osexec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", name, err)
	}
	return nil
}
