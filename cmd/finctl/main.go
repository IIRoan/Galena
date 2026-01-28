// finctl is a CLI tool for building and managing OCI-native OS images
package main

import (
	"os"

	"github.com/finpilot/finctl/cmd/finctl/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
