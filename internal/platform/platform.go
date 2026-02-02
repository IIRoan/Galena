package platform

import (
	"fmt"
	"runtime"
)

// RequireLinux returns an error if the current OS is not Linux.
func RequireLinux(feature string) error {
	if runtime.GOOS != "linux" {
		if feature == "" {
			feature = "galena"
		}
		return fmt.Errorf("%s is supported on Linux only (current: %s)", feature, runtime.GOOS)
	}
	return nil
}

// IsLinux reports whether the current OS is Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}
