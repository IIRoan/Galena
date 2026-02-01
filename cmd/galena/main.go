// galena is a CLI tool for building and managing OCI-native OS images
package main

import (
	"os"

	"github.com/iiroan/galena/cmd/galena/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
