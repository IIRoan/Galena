package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/iiroan/galena/internal/config"
)

// Config validates the galena configuration file.
func Config(_ context.Context, rootDir, configPath string) Result {
	result := Result{}

	path := configPath
	if path == "" {
		path = filepath.Join(rootDir, "galena.yaml")
	}

	if _, err := os.Stat(path); err == nil {
		loadedCfg, err := config.Load(path)
		if err != nil {
			result.AddError(fmt.Sprintf("Config: %v", err))
			result.AddItem(StatusError, filepath.Base(path), err.Error())
			return result
		}
		if err := loadedCfg.Validate(); err != nil {
			result.AddError(fmt.Sprintf("Config: %v", err))
			result.AddItem(StatusError, filepath.Base(path), err.Error())
			return result
		}
		result.AddItem(StatusSuccess, filepath.Base(path), "")
		return result
	}

	result.AddWarning("No galena.yaml found")
	result.AddPending("galena.yaml not found")
	result.AddItem(StatusPending, "galena.yaml", "not found")
	return result
}
