package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	galexec "github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var (
	appsInstallBrew    bool
	appsInstallFlatpak bool
	appsInstallAll     bool
)

var appsCmd = &cobra.Command{
	Use:   "apps",
	Short: "Manage catalog applications (Homebrew and Flatpak)",
	RunE:  runApps,
}

var appsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installed vs missing applications from Galena catalogs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showCatalogStatus([]catalogKind{catalogKindBrew, catalogKindFlatpak})
	},
}

var appsInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install applications from catalog definitions",
	RunE:  runAppsInstall,
}

func init() {
	appsCmd.AddCommand(appsStatusCmd)
	appsCmd.AddCommand(appsInstallCmd)

	appsInstallCmd.Flags().BoolVar(&appsInstallBrew, "brew", false, "Install from Brewfile catalogs only")
	appsInstallCmd.Flags().BoolVar(&appsInstallFlatpak, "flatpak", false, "Install from Flatpak catalogs only")
	appsInstallCmd.Flags().BoolVar(&appsInstallAll, "all", false, "Install from both catalogs")
}

func runApps(cmd *cobra.Command, args []string) error {
	ui.StartScreen("APPLICATIONS", "Manage Homebrew and Flatpak applications from Galena catalogs")

	for {
		choice, err := ui.RunMenuWithOptions("APPLICATIONS", "Choose a catalog action", []ui.MenuItem{
			{ID: "status", TitleText: "Catalog Status", Details: "See installed and missing items from Brew and Flatpak lists"},
			{ID: "install-brew", TitleText: "Install Brew Packages", Details: "Select and install missing Homebrew packages"},
			{ID: "install-flatpak", TitleText: "Install Flatpaks", Details: "Select and install missing Flatpak applications"},
			{ID: "install-all", TitleText: "Install Both Catalogs", Details: "Review and install from both Brew and Flatpak catalogs"},
			{ID: "back", TitleText: "Back", Details: "Return to the previous menu"},
		}, ui.WithBackNavigation("Back"))
		if err != nil {
			return appsFallbackMenu()
		}

		switch choice {
		case ui.MenuActionBack, ui.MenuActionQuit, "back":
			return nil
		case "status":
			if err := showCatalogStatus([]catalogKind{catalogKindBrew, catalogKindFlatpak}); err != nil {
				return err
			}
		case "install-brew":
			if err := runCatalogInstallFlow([]catalogKind{catalogKindBrew}); err != nil {
				return err
			}
		case "install-flatpak":
			if err := runCatalogInstallFlow([]catalogKind{catalogKindFlatpak}); err != nil {
				return err
			}
		case "install-all":
			if err := runCatalogInstallFlow([]catalogKind{catalogKindBrew, catalogKindFlatpak}); err != nil {
				return err
			}
		default:
			return nil
		}

		if err := waitForEnter("Press enter to return to Applications"); err != nil {
			return err
		}
	}
}

func appsFallbackMenu() error {
	var choice string
	err := huh.NewSelect[string]().
		Title("Applications").
		Description("Choose a catalog action").
		Options(
			huh.NewOption("Catalog Status", "status"),
			huh.NewOption("Install Brew Packages", "install-brew"),
			huh.NewOption("Install Flatpaks", "install-flatpak"),
			huh.NewOption("Install Both Catalogs", "install-all"),
			huh.NewOption("Back", "back"),
		).
		Value(&choice).
		WithTheme(ui.HuhTheme()).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	switch choice {
	case "status":
		return showCatalogStatus([]catalogKind{catalogKindBrew, catalogKindFlatpak})
	case "install-brew":
		return runCatalogInstallFlow([]catalogKind{catalogKindBrew})
	case "install-flatpak":
		return runCatalogInstallFlow([]catalogKind{catalogKindFlatpak})
	case "install-all":
		return runCatalogInstallFlow([]catalogKind{catalogKindBrew, catalogKindFlatpak})
	default:
		return nil
	}
}

func runAppsInstall(cmd *cobra.Command, args []string) error {
	kinds := []catalogKind{}
	if appsInstallAll || (!appsInstallBrew && !appsInstallFlatpak) {
		kinds = append(kinds, catalogKindBrew, catalogKindFlatpak)
	} else {
		if appsInstallBrew {
			kinds = append(kinds, catalogKindBrew)
		}
		if appsInstallFlatpak {
			kinds = append(kinds, catalogKindFlatpak)
		}
	}
	return runCatalogInstallFlow(kinds)
}

func runCatalogInstallFlow(kinds []catalogKind) error {
	if err := ensureCatalogManagers(kinds); err != nil {
		return err
	}

	items, err := loadCatalogForKinds(kinds)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		fmt.Println(ui.InfoBox.Render("No catalog entries found for the selected source(s)."))
		return nil
	}

	options := make([]huh.Option[string], 0, len(items))
	selectedKeys := make([]string, 0, len(items))
	index := map[string]catalogItem{}

	for _, item := range items {
		key := string(item.Kind) + "::" + item.Name
		status := "missing"
		if item.Installed {
			status = "installed"
		}
		label := fmt.Sprintf("[%s] %s (%s)", strings.ToUpper(string(item.Kind)), item.Name, status)
		options = append(options, huh.NewOption(label, key).Selected(!item.Installed))
		index[key] = item
	}

	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select packages to install").
				Description("Installed items are listed for visibility and are unselected by default.").
				Options(options...).
				Value(&selectedKeys).
				Height(16).
				Filterable(true),
		),
	).WithTheme(ui.HuhTheme()).Run(); err != nil {
		return err
	}

	if len(selectedKeys) == 0 {
		fmt.Println(ui.InfoBox.Render("No packages selected."))
		return nil
	}

	selected := make([]catalogItem, 0, len(selectedKeys))
	for _, key := range selectedKeys {
		item, ok := index[key]
		if !ok {
			continue
		}
		selected = append(selected, item)
	}
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].Kind != selected[j].Kind {
			return selected[i].Kind < selected[j].Kind
		}
		return selected[i].Name < selected[j].Name
	})

	return installCatalogItems(selected)
}

func installCatalogItems(items []catalogItem) error {
	ctx := context.Background()
	installed := 0
	skipped := 0
	failed := []string{}

	for _, item := range items {
		if item.Installed {
			skipped++
			continue
		}

		message := fmt.Sprintf("Installing %s (%s)", item.Name, item.Kind)
		err := ui.RunWithSpinner(message, func() error {
			return installCatalogItem(ctx, item)
		})
		if err != nil {
			failed = append(failed, fmt.Sprintf("%s: %v", item.Name, err))
			continue
		}
		installed++
	}

	fmt.Println()
	fmt.Println("Installation Summary")
	fmt.Printf("Installed: %d\n", installed)
	fmt.Printf("Skipped:   %d (already present)\n", skipped)
	fmt.Printf("Failed:    %d\n", len(failed))

	if len(failed) > 0 {
		fmt.Println()
		fmt.Println("Failures:")
		for _, entry := range failed {
			fmt.Printf("  - %s\n", entry)
		}
		return fmt.Errorf("one or more installs failed")
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render("Selected catalog items installed successfully."))
	return nil
}

func installCatalogItem(ctx context.Context, item catalogItem) error {
	switch item.Kind {
	case catalogKindBrew:
		if err := galexec.RequireCommands("brew"); err != nil {
			return err
		}
		result := galexec.Run(ctx, "brew", []string{"install", item.Name}, galexec.DefaultOptions())
		if result.Err != nil {
			return fmt.Errorf("%w\n%s", result.Err, galexec.LastNLines(result.Stderr, 10))
		}
	case catalogKindFlatpak:
		if err := galexec.RequireCommands("flatpak"); err != nil {
			return err
		}

		remoteAdd := galexec.Run(ctx, "flatpak", []string{
			"remote-add", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo",
		}, galexec.DefaultOptions())
		if remoteAdd.Err != nil {
			return fmt.Errorf("%w\n%s", remoteAdd.Err, galexec.LastNLines(remoteAdd.Stderr, 10))
		}

		result := galexec.Run(ctx, "flatpak", []string{
			"install", "-y", "--system", "flathub", item.Name,
		}, galexec.DefaultOptions())
		if result.Err != nil {
			// Fallback to user scope if system scope is unavailable.
			result = galexec.Run(ctx, "flatpak", []string{
				"install", "-y", "flathub", item.Name,
			}, galexec.DefaultOptions())
		}
		if result.Err != nil {
			return fmt.Errorf("%w\n%s", result.Err, galexec.LastNLines(result.Stderr, 10))
		}
	default:
		return fmt.Errorf("unsupported catalog type: %s", item.Kind)
	}

	return nil
}

func showCatalogStatus(kinds []catalogKind) error {
	items, err := loadCatalogForKinds(kinds)
	if err != nil {
		return err
	}

	brewAvailable := galexec.CheckCommand("brew")
	flatpakAvailable := galexec.CheckCommand("flatpak")

	ui.StartScreen("CATALOG STATUS", "Installed vs missing applications from Brew and Flatpak manifests")

	fmt.Println(ui.Title.Render("Managers"))
	fmt.Printf("  %s brew %s\n", statusGlyph(brewAvailable), managerDetails(brewAvailable))
	fmt.Printf("  %s flatpak %s\n", statusGlyph(flatpakAvailable), managerDetails(flatpakAvailable))

	grouped := map[catalogKind][]catalogItem{
		catalogKindBrew:    {},
		catalogKindFlatpak: {},
	}
	for _, item := range items {
		grouped[item.Kind] = append(grouped[item.Kind], item)
	}

	for _, kind := range []catalogKind{catalogKindBrew, catalogKindFlatpak} {
		entries := grouped[kind]
		if len(entries) == 0 {
			continue
		}

		total := len(entries)
		installed := 0
		for _, entry := range entries {
			if entry.Installed {
				installed++
			}
		}

		fmt.Println()
		fmt.Println(ui.Title.Render(strings.ToUpper(string(kind))))
		fmt.Printf("  Installed: %d/%d\n", installed, total)

		for _, entry := range entries {
			state := ui.StatusPending.String()
			if entry.Installed {
				state = ui.StatusSuccess.String()
			}
			fmt.Printf("  %s %-36s %s\n", state, entry.Name, ui.MutedStyle.Render("["+strings.Join(entry.Sources, ", ")+"]"))
		}
	}

	return nil
}

func statusGlyph(ok bool) string {
	if ok {
		return ui.StatusSuccess.String()
	}
	return ui.StatusError.String()
}

func managerDetails(ok bool) string {
	if ok {
		return ui.MutedStyle.Render("(available)")
	}
	return ui.MutedStyle.Render("(not found)")
}

func ensureCatalogManagers(kinds []catalogKind) error {
	for _, kind := range kinds {
		switch kind {
		case catalogKindBrew:
			if !galexec.CheckCommand("brew") {
				return fmt.Errorf("brew is required for brew catalog installs")
			}
		case catalogKindFlatpak:
			if !galexec.CheckCommand("flatpak") {
				return fmt.Errorf("flatpak is required for flatpak catalog installs")
			}
		}
	}
	return nil
}
