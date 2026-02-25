// galena-build is the development/build CLI for Galena images.
package main

import (
	"os"

	"github.com/iiroan/galena/cmd/galena/cmd"
)

func main() {
	if err := cmd.ExecuteBuild(); err != nil {
		os.Exit(1)
	}
}
