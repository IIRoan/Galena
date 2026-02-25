package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	galexec "github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

type ujustRecipe struct {
	Name        string
	Params      []string
	Group       string
	Description string
	Source      string
}

var ujustCmd = &cobra.Command{
	Use:   "ujust [recipe] [args...]",
	Short: "Run Bluefin/ujust tasks from Galena",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUJustTasks,
}

func runUJustTasks(cmd *cobra.Command, args []string) error {
	if err := galexec.RequireCommands("ujust"); err != nil {
		return fmt.Errorf("ujust is required: %w", err)
	}

	if len(args) > 0 {
		return runAttachedCommand("ujust", args)
	}

	recipes, _ := loadUJustRecipes()
	if len(recipes) == 0 {
		return runAttachedCommand("ujust", nil)
	}

	menuItems := make([]ui.MenuItem, 0, len(recipes)+2)
	menuItems = append(menuItems,
		ui.MenuItem{
			ID:        "open-menu",
			TitleText: "Open Interactive ujust Menu",
			Details:   "Launch native ujust selection UI with all system recipes",
		},
	)

	for _, recipe := range recipes {
		title := recipe.Name
		if recipe.Group != "" {
			title = fmt.Sprintf("[%s] %s", recipe.Group, recipe.Name)
		}
		details := recipe.Description
		if details == "" {
			details = "Run recipe from " + recipe.Source
		}
		menuItems = append(menuItems, ui.MenuItem{
			ID:        "recipe::" + recipe.Name,
			TitleText: title,
			Details:   details,
		})
	}
	menuItems = append(menuItems, ui.MenuItem{
		ID:        "back",
		TitleText: "Back",
		Details:   "Return to the previous menu",
	})

	choice, err := ui.RunMenuWithOptions("BLUEFIN TASKS", "Choose a ujust task to run", menuItems, ui.WithBackNavigation("Back"))
	if err != nil {
		return runUJustFallback(recipes)
	}

	if choice == ui.MenuActionBack || choice == ui.MenuActionQuit || choice == "back" {
		return nil
	}

	if choice == "open-menu" {
		return runAttachedCommand("ujust", nil)
	}

	if !strings.HasPrefix(choice, "recipe::") {
		return nil
	}
	name := strings.TrimPrefix(choice, "recipe::")
	recipe, ok := findRecipeByName(recipes, name)
	if !ok {
		return fmt.Errorf("recipe %q not found", name)
	}

	params, err := promptRecipeParameters(recipe)
	if err != nil {
		return err
	}

	runArgs := append([]string{recipe.Name}, params...)
	return runAttachedCommand("ujust", runArgs)
}

func runUJustFallback(recipes []ujustRecipe) error {
	options := []huh.Option[string]{huh.NewOption("Open Interactive ujust Menu", "open-menu")}
	for _, recipe := range recipes {
		label := recipe.Name
		if recipe.Group != "" {
			label = fmt.Sprintf("[%s] %s", recipe.Group, recipe.Name)
		}
		options = append(options, huh.NewOption(label, "recipe::"+recipe.Name))
	}
	options = append(options, huh.NewOption("Back", "back"))

	var choice string
	if err := huh.NewSelect[string]().
		Title("Bluefin Tasks").
		Description("Choose a ujust task to run").
		Options(options...).
		Value(&choice).
		WithTheme(ui.HuhTheme()).
		Run(); err != nil {
		return err
	}

	if choice == "back" {
		return nil
	}
	if choice == "open-menu" {
		return runAttachedCommand("ujust", nil)
	}
	if !strings.HasPrefix(choice, "recipe::") {
		return nil
	}
	name := strings.TrimPrefix(choice, "recipe::")
	recipe, ok := findRecipeByName(recipes, name)
	if !ok {
		return fmt.Errorf("recipe %q not found", name)
	}
	params, err := promptRecipeParameters(recipe)
	if err != nil {
		return err
	}
	return runAttachedCommand("ujust", append([]string{recipe.Name}, params...))
}

func promptRecipeParameters(recipe ujustRecipe) ([]string, error) {
	if len(recipe.Params) == 0 {
		return nil, nil
	}

	values := make([]string, len(recipe.Params))
	groups := make([]*huh.Group, 0, len(recipe.Params))
	for idx, param := range recipe.Params {
		i := idx
		name := param
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Value for %s", name)).
				Value(&values[i]),
		))
	}

	if err := huh.NewForm(groups...).WithTheme(ui.HuhTheme()).Run(); err != nil {
		return nil, err
	}
	return values, nil
}

func findRecipeByName(recipes []ujustRecipe, name string) (ujustRecipe, bool) {
	for _, recipe := range recipes {
		if recipe.Name == name {
			return recipe, true
		}
	}
	return ujustRecipe{}, false
}

func loadUJustRecipes() ([]ujustRecipe, error) {
	files := discoverCatalogFiles(
		[]string{"/usr/share/ublue-os/just", "custom/ujust"},
		[]string{".just"},
	)
	if len(files) == 0 {
		return nil, fmt.Errorf("no ujust recipe files found")
	}

	recipes := []ujustRecipe{}
	seen := map[string]struct{}{}
	for _, file := range files {
		parsed, err := parseUJustRecipes(file)
		if err != nil {
			continue
		}
		for _, recipe := range parsed {
			if _, ok := seen[recipe.Name]; ok {
				continue
			}
			seen[recipe.Name] = struct{}{}
			recipes = append(recipes, recipe)
		}
	}

	sort.Slice(recipes, func(i, j int) bool {
		if recipes[i].Group != recipes[j].Group {
			return recipes[i].Group < recipes[j].Group
		}
		return recipes[i].Name < recipes[j].Name
	})

	return recipes, nil
}

func parseUJustRecipes(path string) ([]ujustRecipe, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	recipes := []ujustRecipe{}
	scanner := bufio.NewScanner(file)
	currentGroup := ""
	pendingDescription := ""
	nextPrivate := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			pendingDescription = ""
			continue
		}
		if strings.HasPrefix(line, "#") {
			if pendingDescription == "" {
				pendingDescription = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			}
			continue
		}
		if strings.HasPrefix(line, "[group(") {
			currentGroup = parseJustGroup(line)
			nextPrivate = false
			continue
		}
		if line == "[private]" {
			nextPrivate = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			continue
		}
		if strings.Contains(line, ":=") || strings.HasPrefix(line, "set ") {
			pendingDescription = ""
			continue
		}
		if strings.Contains(line, ":") {
			recipeDef := strings.TrimSpace(strings.SplitN(line, ":", 2)[0])
			fields := strings.Fields(recipeDef)
			if len(fields) == 0 {
				continue
			}
			name := fields[0]
			if nextPrivate || strings.HasPrefix(name, "_") {
				nextPrivate = false
				pendingDescription = ""
				continue
			}

			recipes = append(recipes, ujustRecipe{
				Name:        name,
				Params:      fields[1:],
				Group:       currentGroup,
				Description: pendingDescription,
				Source:      filepath.Base(path),
			})
			nextPrivate = false
			pendingDescription = ""
		}
	}

	return recipes, nil
}

func parseJustGroup(line string) string {
	start := strings.IndexAny(line, `'"`)
	if start == -1 || start+1 >= len(line) {
		return ""
	}
	quote := line[start]
	end := strings.IndexRune(line[start+1:], rune(quote))
	if end == -1 {
		return ""
	}
	return line[start+1 : start+1+end]
}
