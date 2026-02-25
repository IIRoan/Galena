// galena is the runtime management CLI for Galena devices.
package main

import (
	"os"

	"github.com/iiroan/galena/cmd/galena/cmd"
)

func main() {
	if err := cmd.ExecuteManagement(); err != nil {
		os.Exit(1)
	}
}
