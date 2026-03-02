package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	galexec "github.com/iiroan/galena/internal/exec"
)

type catalogKind string

const (
	catalogKindBrew    catalogKind = "brew"
	catalogKindFlatpak catalogKind = "flatpak"
)

type catalogItem struct {
	Name      string
	Kind      catalogKind
	Sources   []string
	Installed bool
}

func loadCatalogForKinds(kinds []catalogKind) ([]catalogItem, error) {
	items := make([]catalogItem, 0)
	for _, kind := range kinds {
		var (
			loaded []catalogItem
			err    error
		)
		switch kind {
		case catalogKindBrew:
			loaded, err = loadBrewCatalog()
		case catalogKindFlatpak:
			loaded, err = loadFlatpakCatalog()
		default:
			err = fmt.Errorf("unknown catalog kind %q", kind)
		}
		if err != nil {
			return nil, err
		}
		items = append(items, loaded...)
	}
	return items, nil
}

func loadBrewCatalog() ([]catalogItem, error) {
	files := discoverCatalogFiles(
		[]string{"/usr/share/ublue-os/homebrew", "custom/brew"},
		[]string{".Brewfile"},
	)
	if len(files) == 0 {
		return nil, fmt.Errorf("no Brewfiles found in /usr/share/ublue-os/homebrew or custom/brew")
	}

	ctx := context.Background()
	installed := listInstalledBrewPackages(ctx)
	packages := map[string]*catalogItem{}
	for _, file := range files {
		items, err := getBrewPackages(file)
		if err != nil {
			continue
		}
		for _, name := range items {
			entry, ok := packages[name]
			if !ok {
				entry = &catalogItem{
					Name:      name,
					Kind:      catalogKindBrew,
					Sources:   []string{},
					Installed: itemInSet(installed, name),
				}
				packages[name] = entry
			}
			entry.Sources = appendUnique(entry.Sources, filepath.Base(file))
		}
	}

	return flattenCatalogMap(packages), nil
}

func loadFlatpakCatalog() ([]catalogItem, error) {
	files := discoverCatalogFiles(
		[]string{"/etc/flatpak/preinstall.d", "custom/flatpaks", "custom/flatpak"},
		[]string{".preinstall", ".list"},
	)
	if len(files) == 0 {
		return nil, fmt.Errorf("no Flatpak catalog files found in /etc/flatpak/preinstall.d or custom/flatpaks")
	}

	ctx := context.Background()
	installed := listInstalledFlatpakApps(ctx)
	apps := map[string]*catalogItem{}
	for _, file := range files {
		items, err := readFlatpakCatalogFile(file)
		if err != nil {
			continue
		}
		for _, name := range items {
			entry, ok := apps[name]
			if !ok {
				entry = &catalogItem{
					Name:      name,
					Kind:      catalogKindFlatpak,
					Sources:   []string{},
					Installed: itemInSet(installed, name),
				}
				apps[name] = entry
			}
			entry.Sources = appendUnique(entry.Sources, filepath.Base(file))
		}
	}

	return flattenCatalogMap(apps), nil
}

func discoverCatalogFiles(directories []string, suffixes []string) []string {
	files := []string{}
	seen := map[string]struct{}{}

	for _, directory := range directories {
		entries, err := os.ReadDir(directory)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			for _, suffix := range suffixes {
				if strings.HasSuffix(name, suffix) {
					full := filepath.Join(directory, name)
					if _, ok := seen[full]; !ok {
						seen[full] = struct{}{}
						files = append(files, full)
					}
					break
				}
			}
		}
	}

	sort.Strings(files)
	return files
}

func readFlatpakCatalogFile(path string) ([]string, error) {
	if strings.HasSuffix(path, ".preinstall") {
		return getFlatpakApps(path)
	}
	if strings.HasSuffix(path, ".list") {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = file.Close()
		}()

		apps := []string{}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			apps = append(apps, line)
		}
		return apps, nil
	}
	return nil, fmt.Errorf("unsupported flatpak catalog file: %s", path)
}

func flattenCatalogMap(items map[string]*catalogItem) []catalogItem {
	flattened := make([]catalogItem, 0, len(items))
	for _, item := range items {
		sort.Strings(item.Sources)
		flattened = append(flattened, *item)
	}
	sort.Slice(flattened, func(i, j int) bool {
		if flattened[i].Kind != flattened[j].Kind {
			return flattened[i].Kind < flattened[j].Kind
		}
		return flattened[i].Name < flattened[j].Name
	})
	return flattened
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func listInstalledBrewPackages(ctx context.Context) map[string]struct{} {
	installed := map[string]struct{}{}
	if !galexec.CheckCommand("brew") {
		return installed
	}

	parse := func(stdout string) {
		for _, raw := range strings.Split(stdout, "\n") {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			installed[name] = struct{}{}
		}
	}

	formula := galexec.RunSimple(ctx, "brew", "list", "--formula")
	if formula.Err == nil {
		parse(formula.Stdout)
	}
	casks := galexec.RunSimple(ctx, "brew", "list", "--cask")
	if casks.Err == nil {
		parse(casks.Stdout)
	}

	return installed
}

func listInstalledFlatpakApps(ctx context.Context) map[string]struct{} {
	installed := map[string]struct{}{}
	if !galexec.CheckCommand("flatpak") {
		return installed
	}

	parse := func(stdout string) {
		for _, raw := range strings.Split(stdout, "\n") {
			name := strings.TrimSpace(raw)
			if name == "" {
				continue
			}
			installed[name] = struct{}{}
		}
	}

	// Catalogs can include apps and extensions/runtimes (e.g. GTK themes).
	// Gather both user and system installation scopes without filtering to --app.
	queries := [][]string{
		{"list", "--columns=application", "--system"},
		{"list", "--columns=application", "--user"},
	}
	parsedAny := false
	for _, query := range queries {
		result := galexec.RunSimple(ctx, "flatpak", query...)
		if result.Err != nil {
			continue
		}
		parse(result.Stdout)
		parsedAny = true
	}

	// Fallback for environments that don't support explicit scope flags.
	if !parsedAny {
		result := galexec.RunSimple(ctx, "flatpak", "list", "--columns=application")
		if result.Err == nil {
			parse(result.Stdout)
		}
	}

	return installed
}

func itemInSet(set map[string]struct{}, name string) bool {
	_, ok := set[name]
	return ok
}
