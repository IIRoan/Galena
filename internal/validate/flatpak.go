package validate

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/iiroan/galena/internal/exec"
)

// Flatpaks validates Flatpak preinstall files.
func Flatpaks(ctx context.Context, rootDir string) Result {
	result := Result{}

	flatpakDirs := []string{
		filepath.Join(rootDir, "custom", "flatpaks"),
		filepath.Join(rootDir, "custom", "flatpak"),
	}
	flatpakFiles := []string{}
	for _, dir := range flatpakDirs {
		if _, err := os.Stat(dir); err == nil {
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				name := info.Name()
				if strings.HasSuffix(name, ".preinstall") || strings.HasSuffix(name, ".list") {
					flatpakFiles = append(flatpakFiles, path)
				}
				return nil
			})
		}
	}

	if len(flatpakFiles) == 0 {
		result.AddPending("No flatpak files found")
		result.AddItem(StatusPending, "Flatpaks", "none found")
		return result
	}

	if !exec.CheckCommand("flatpak") {
		result.AddWarning("flatpak not installed")
		result.AddPending("flatpak not installed")
		result.AddItem(StatusPending, "Flatpaks", "flatpak not installed")
		return result
	}

	remoteResult := exec.RunSimple(ctx, "flatpak", "remote-add", "--user", "--if-not-exists", "flathub", "https://dl.flathub.org/repo/flathub.flatpakrepo")
	if remoteResult.Err != nil {
		result.AddWarning("flatpak: could not add flathub")
		result.AddItem(StatusPending, "Flatpaks", "could not add flathub remote")
		return result
	}

	for _, flatpakFile := range flatpakFiles {
		relPath, _ := filepath.Rel(rootDir, flatpakFile)
		ids, err := parseFlatpakIDs(flatpakFile)
		if err != nil {
			result.AddWarning("flatpak: " + relPath)
			result.AddItem(StatusPending, relPath, "read failed")
			continue
		}
		if len(ids) == 0 {
			result.AddPending("flatpak: " + relPath)
			result.AddItem(StatusPending, relPath, "no entries")
			continue
		}
		failed := false
		for _, id := range ids {
			infoResult := exec.RunSimple(ctx, "flatpak", "remote-info", "--user", "flathub", id)
			if infoResult.Err != nil {
				failed = true
				result.AddWarning("flatpak: " + id)
			}
		}
		if failed {
			result.AddItem(StatusPending, relPath, "validation failed")
			continue
		}
		result.AddItem(StatusSuccess, relPath, "")
	}

	return result
}

func parseFlatpakIDs(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	ids := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasSuffix(path, ".list") {
			ids = append(ids, line)
			continue
		}
		if strings.HasPrefix(line, "[Flatpak Preinstall ") && strings.HasSuffix(line, "]") {
			trimmed := strings.TrimPrefix(line, "[Flatpak Preinstall ")
			trimmed = strings.TrimSuffix(trimmed, "]")
			trimmed = strings.TrimSpace(trimmed)
			if trimmed != "" {
				ids = append(ids, trimmed)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}
