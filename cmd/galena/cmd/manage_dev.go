package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	galexec "github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

type devProfile struct {
	ID          string   `yaml:"id"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Template    string   `yaml:"template"`
	Brew        []string `yaml:"brew_packages"`
	Flatpak     []string `yaml:"flatpak_apps"`
}

type devProfileCatalog struct {
	Profiles []devProfile `yaml:"profiles"`
}

const (
	defaultDevProfileID = "app-dev-core"
)

var (
	devWorkspace string
	devInitForce bool
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Manage devcontainer-first development workflows",
	RunE:  runDev,
}

var devListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List running and stopped devcontainers discovered via Podman",
	RunE:    runDevList,
}

var devInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .devcontainer/devcontainer.json from the default Galena template",
	RunE:  runDevInit,
}

var devUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Start the devcontainer for the workspace",
	RunE:  runDevUp,
}

var devShellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open an interactive shell inside the devcontainer",
	RunE:  runDevShell,
}

var devExecCmd = &cobra.Command{
	Use:   "exec -- <command...>",
	Short: "Run a command inside the devcontainer",
	Args:  cobra.ArbitraryArgs,
	RunE:  runDevExec,
}

var devStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the devcontainer for the workspace",
	RunE:  runDevStop,
}

var devStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show host and devcontainer workspace status",
	RunE:  runDevStatus,
}

func init() {
	devCmd.PersistentFlags().StringVar(&devWorkspace, "workspace", "", "Workspace folder (default: current directory)")
	devInitCmd.Flags().BoolVar(&devInitForce, "force", false, "Overwrite existing .devcontainer/devcontainer.json")

	devCmd.AddCommand(devListCmd)
	devCmd.AddCommand(devInitCmd)
	devCmd.AddCommand(devUpCmd)
	devCmd.AddCommand(devShellCmd)
	devCmd.AddCommand(devExecCmd)
	devCmd.AddCommand(devStopCmd)
	devCmd.AddCommand(devStatusCmd)
}

func runDev(cmd *cobra.Command, args []string) error {
	ui.StartScreen("DEVELOPMENT", "Devcontainer-first workflows for Galena")

	for {
		choice, err := ui.RunMenuWithOptions("DEVELOPMENT", "Choose a development action", []ui.MenuItem{
			{ID: "list", TitleText: "List Containers", Details: "Show running and stopped devcontainers discovered on this host"},
			{ID: "init", TitleText: "Initialize Workspace", Details: "Create .devcontainer/devcontainer.json from the default template"},
			{ID: "up", TitleText: "Start Devcontainer", Details: "Run devcontainer up for this workspace"},
			{ID: "shell", TitleText: "Open Shell", Details: "Open an interactive shell inside the devcontainer"},
			{ID: "exec", TitleText: "Run Command", Details: "Run a one-off command in the devcontainer"},
			{ID: "stop", TitleText: "Stop Devcontainer", Details: "Run devcontainer down"},
			{ID: "status", TitleText: "Dev Status", Details: "Check host readiness and workspace state"},
			{ID: "back", TitleText: "Back", Details: "Return to the previous menu"},
		}, ui.WithBackNavigation("Back"))
		if err != nil {
			fallbackErr := runDevFallback()
			if fallbackErr == nil || errors.Is(fallbackErr, huh.ErrUserAborted) {
				return nil
			}
			fmt.Println(ui.ErrorStyle.Render("Error: development menu unavailable: " + fallbackErr.Error()))
			fmt.Println(ui.MutedStyle.Render("Hint: try non-interactive commands, e.g. 'galena dev status' or 'galena dev list'."))
			_ = waitForEnter("Press enter to return")
			return nil
		}

		switch choice {
		case ui.MenuActionBack, ui.MenuActionQuit, "back":
			return nil
		case "list":
			if !runDevActionWithPause(func() error { return runDevList(devListCmd, nil) }) {
				return nil
			}
		case "init":
			if !runDevActionWithPause(func() error { return runDevInit(devInitCmd, nil) }) {
				return nil
			}
		case "up":
			if !runDevActionWithPause(func() error { return runDevUp(devUpCmd, nil) }) {
				return nil
			}
		case "shell":
			if !runDevActionWithPause(func() error { return runDevShell(devShellCmd, nil) }) {
				return nil
			}
		case "exec":
			if !runDevActionWithPause(runDevExecPrompt) {
				return nil
			}
		case "stop":
			if !runDevActionWithPause(func() error { return runDevStop(devStopCmd, nil) }) {
				return nil
			}
		case "status":
			if !runDevActionWithPause(func() error { return runDevStatus(devStatusCmd, nil) }) {
				return nil
			}
		}
	}
}

func runDevActionWithPause(action func() error) bool {
	err := action()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return true
		}
		fmt.Println(ui.ErrorStyle.Render("Error: " + err.Error()))
		_ = waitForEnter("Press enter to return to Development")
		return true
	}
	if err := waitForEnter("Press enter to return to Development"); err != nil {
		fmt.Println(ui.WarningStyle.Render("Could not pause for input: " + err.Error()))
		return false
	}
	return true
}

func runDevFallback() error {
	var choice string
	err := huh.NewSelect[string]().
		Title("Development").
		Description("Choose a development action").
		Options(
			huh.NewOption("List Containers", "list"),
			huh.NewOption("Initialize Workspace", "init"),
			huh.NewOption("Start Devcontainer", "up"),
			huh.NewOption("Open Shell", "shell"),
			huh.NewOption("Run Command", "exec"),
			huh.NewOption("Stop Devcontainer", "stop"),
			huh.NewOption("Dev Status", "status"),
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
	case "list":
		return runDevList(devListCmd, nil)
	case "init":
		return runDevInit(devInitCmd, nil)
	case "up":
		return runDevUp(devUpCmd, nil)
	case "shell":
		return runDevShell(devShellCmd, nil)
	case "exec":
		return runDevExecPrompt()
	case "stop":
		return runDevStop(devStopCmd, nil)
	case "status":
		return runDevStatus(devStatusCmd, nil)
	default:
		return nil
	}
}

type podmanContainerEntry struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	State  string            `json:"State"`
	Status string            `json:"Status"`
	Labels map[string]string `json:"Labels"`
}

type discoveredDevcontainer struct {
	ID        string
	Name      string
	State     string
	Status    string
	Workspace string
	Config    string
}

func runDevList(cmd *cobra.Command, args []string) error {
	ui.StartScreen("DEV CONTAINERS", "Running and stopped devcontainers on this host")

	containers, err := discoverDevcontainers(context.Background())
	if err != nil {
		fmt.Println(ui.WarningStyle.Render("Could not discover devcontainers: " + err.Error()))
		return nil
	}
	printDiscoveredDevcontainers(containers)
	return nil
}

func runDevInit(cmd *cobra.Command, args []string) error {
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	targetPath, err := initDevcontainerWorkspace(workspace, defaultDevProfileID, devInitForce)
	if err != nil {
		return err
	}

	fmt.Println(ui.SuccessBox.Render(fmt.Sprintf("Devcontainer config ready at %s", targetPath)))
	fmt.Printf("Next step: %s\n", ui.AccentStyle().Render("galena dev up --workspace "+workspace))
	return nil
}

func runDevUp(cmd *cobra.Command, args []string) error {
	if err := galexec.RequireCommands("devcontainer", "podman"); err != nil {
		return err
	}
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	if err := ensureWorkspaceHasDevcontainer(workspace); err != nil {
		return err
	}

	ui.StartScreen("DEVCONTAINER UP", "Starting development container")
	result := galexec.RunStreaming(context.Background(), "devcontainer", []string{"up", "--workspace-folder", workspace}, galexec.DefaultOptions())
	if result.Err != nil {
		return fmt.Errorf("devcontainer up failed: %w", result.Err)
	}
	fmt.Println(ui.SuccessBox.Render("Devcontainer is ready."))
	return nil
}

func runDevShell(cmd *cobra.Command, args []string) error {
	if err := galexec.RequireCommands("devcontainer"); err != nil {
		return err
	}
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	if err := ensureWorkspaceHasDevcontainer(workspace); err != nil {
		return err
	}

	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "bash"
	}

	ui.StartScreen("DEV SHELL", "Opening shell inside the devcontainer")
	return runAttachedCommand("devcontainer", []string{"exec", "--workspace-folder", workspace, shell})
}

func runDevExec(cmd *cobra.Command, args []string) error {
	if err := galexec.RequireCommands("devcontainer"); err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("provide a command after '--', e.g. galena dev exec -- make test")
	}
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	if err := ensureWorkspaceHasDevcontainer(workspace); err != nil {
		return err
	}
	running, err := runningDevcontainersForWorkspace(context.Background(), workspace)
	if err != nil {
		return err
	}
	if len(running) == 0 {
		return fmt.Errorf("no running devcontainer found for workspace %s (run 'galena dev up --workspace %s' first)", workspace, workspace)
	}

	runArgs := append([]string{"exec", "--workspace-folder", workspace}, args...)
	return runAttachedCommand("devcontainer", runArgs)
}

func runDevExecPrompt() error {
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	if err := ensureWorkspaceHasDevcontainer(workspace); err != nil {
		return err
	}
	running, err := runningDevcontainersForWorkspace(context.Background(), workspace)
	if err != nil {
		return err
	}
	if len(running) == 0 {
		fmt.Println(ui.WarningStyle.Render("No running devcontainer found for this workspace."))
		fmt.Println(ui.InfoBox.Render("Start one first with: galena dev up --workspace " + workspace))
		return nil
	}

	var commandLine string
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Command to run inside devcontainer").
				Placeholder("go test ./... or pnpm test").
				Value(&commandLine),
		),
	).WithTheme(ui.HuhTheme()).WithKeyMap(newHuhBackOnQKeyMap()).Run(); err != nil {
		return err
	}

	parts := strings.Fields(strings.TrimSpace(commandLine))
	if len(parts) == 0 {
		return nil
	}
	return runDevExec(devExecCmd, parts)
}

func runDevStop(cmd *cobra.Command, args []string) error {
	if err := galexec.RequireCommands("podman"); err != nil {
		return err
	}
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}

	ui.StartScreen("DEVCONTAINER STOP", "Stopping development containers for this workspace")

	containers, err := devcontainersForWorkspace(context.Background(), workspace)
	if err != nil {
		return err
	}
	if len(containers) == 0 {
		fmt.Println(ui.InfoBox.Render("No discovered devcontainers for this workspace."))
		return nil
	}

	stopped := 0
	alreadyStopped := 0
	failed := []string{}
	for _, entry := range containers {
		if !strings.EqualFold(entry.State, "running") {
			alreadyStopped++
			continue
		}
		result := galexec.RunSimple(context.Background(), "podman", "stop", entry.ID)
		if result.Err != nil {
			stderr := strings.TrimSpace(galexec.LastNLines(result.Stderr, 5))
			if stderr == "" {
				stderr = result.Err.Error()
			}
			failed = append(failed, fmt.Sprintf("%s (%s): %s", entry.Name, entry.ID, stderr))
			continue
		}
		stopped++
	}

	fmt.Println()
	fmt.Printf("Workspace containers: %d\n", len(containers))
	fmt.Printf("Stopped now:          %d\n", stopped)
	fmt.Printf("Already stopped:      %d\n", alreadyStopped)
	fmt.Printf("Failed:               %d\n", len(failed))
	if len(failed) > 0 {
		fmt.Println()
		fmt.Println("Failures:")
		for _, line := range failed {
			fmt.Printf("  - %s\n", line)
		}
		return fmt.Errorf("one or more containers failed to stop")
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render("Workspace devcontainer(s) stopped."))
	return nil
}

func runDevStatus(cmd *cobra.Command, args []string) error {
	workspace, err := resolveWorkspace()
	if err != nil {
		return err
	}
	devFile := filepath.Join(workspace, ".devcontainer", "devcontainer.json")

	ui.StartScreen("DEV STATUS", "Host readiness and devcontainer state")
	fmt.Println(ui.Title.Render("Host Tools"))
	printTool("devcontainer")
	printTool("podman")
	printTool("git")

	fmt.Println()
	fmt.Println(ui.Title.Render("Workspace"))
	printKV("Path", workspace)
	printKV("Config", markerStatus(devFile))

	containers, err := discoverDevcontainers(context.Background())
	if err != nil {
		printKV("Container Discovery", ui.WarningStyle.Render(err.Error()))
		return nil
	}

	matches, err := devcontainersForWorkspace(context.Background(), workspace)
	if err != nil {
		printKV("Workspace Containers", ui.WarningStyle.Render(err.Error()))
		matches = nil
	}

	running := 0
	stopped := 0
	for _, entry := range containers {
		if strings.EqualFold(entry.State, "running") {
			running++
		} else {
			stopped++
		}
	}
	printKV("Known Devcontainers", fmt.Sprintf("%d total (%d running, %d stopped)", len(containers), running, stopped))

	if len(matches) == 0 {
		printKV("Workspace Containers", ui.MutedStyle.Render("none found for this workspace"))
	} else {
		printKV("Workspace Containers", fmt.Sprintf("%d found", len(matches)))
		for _, entry := range matches {
			state := ui.MutedStyle.Render(entry.State)
			if strings.EqualFold(entry.State, "running") {
				state = ui.SuccessStyle.Render(entry.State)
			}
			fmt.Printf("  %s %-20s %s\n", state, entry.Name, ui.MutedStyle.Render(entry.Status))
		}
	}

	fmt.Println()
	fmt.Println(ui.Title.Render("All Discovered Devcontainers"))
	printDiscoveredDevcontainers(containers)

	return nil
}

func discoverDevcontainers(ctx context.Context) ([]discoveredDevcontainer, error) {
	if err := galexec.RequireCommands("podman"); err != nil {
		return nil, err
	}

	result := galexec.RunSimple(ctx, "podman", "ps", "-a", "--format", "json")
	if result.Err != nil {
		stderr := strings.TrimSpace(galexec.LastNLines(result.Stderr, 8))
		if stderr == "" {
			stderr = result.Err.Error()
		}
		return nil, fmt.Errorf("podman ps failed: %s", stderr)
	}

	entries := make([]podmanContainerEntry, 0)
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		return nil, fmt.Errorf("parsing podman container list: %w", err)
	}

	containers := make([]discoveredDevcontainer, 0)
	for _, entry := range entries {
		name := firstName(entry.Names)
		workspace := firstNonEmpty(entry.Labels["devcontainer.local_folder"], entry.Labels["vsch.local.folder"])
		config := firstNonEmpty(entry.Labels["devcontainer.config_file"], entry.Labels["vsch.local.config_file"])
		isDevcontainer := workspace != "" || config != "" || strings.HasPrefix(name, "vsc-")
		if !isDevcontainer {
			continue
		}
		containers = append(containers, discoveredDevcontainer{
			ID:        trimID(entry.ID),
			Name:      name,
			State:     entry.State,
			Status:    entry.Status,
			Workspace: workspace,
			Config:    config,
		})
	}

	sort.SliceStable(containers, func(i, j int) bool {
		iRunning := strings.EqualFold(containers[i].State, "running")
		jRunning := strings.EqualFold(containers[j].State, "running")
		if iRunning != jRunning {
			return iRunning
		}
		if containers[i].Workspace != containers[j].Workspace {
			return containers[i].Workspace < containers[j].Workspace
		}
		return containers[i].Name < containers[j].Name
	})

	return containers, nil
}

func runningDevcontainersForWorkspace(ctx context.Context, workspace string) ([]discoveredDevcontainer, error) {
	containers, err := devcontainersForWorkspace(ctx, workspace)
	if err != nil {
		return nil, err
	}
	running := make([]discoveredDevcontainer, 0)
	for _, entry := range containers {
		if !strings.EqualFold(entry.State, "running") {
			continue
		}
		running = append(running, entry)
	}
	return running, nil
}

func devcontainersForWorkspace(ctx context.Context, workspace string) ([]discoveredDevcontainer, error) {
	containers, err := discoverDevcontainers(ctx)
	if err != nil {
		return nil, err
	}
	normalizedWorkspace := normalizePath(workspace)
	matches := make([]discoveredDevcontainer, 0)
	for _, entry := range containers {
		if samePath(normalizedWorkspace, normalizePath(entry.Workspace)) {
			matches = append(matches, entry)
			continue
		}
		configPath := normalizePath(entry.Config)
		if configPath == "" {
			continue
		}
		prefix := normalizedWorkspace + string(os.PathSeparator)
		if strings.HasPrefix(configPath, prefix) {
			matches = append(matches, entry)
		}
	}
	return matches, nil
}

func printDiscoveredDevcontainers(containers []discoveredDevcontainer) {
	if len(containers) == 0 {
		fmt.Println(ui.InfoBox.Render("No devcontainers discovered via Podman labels (running or stopped)."))
		return
	}

	for _, entry := range containers {
		state := ui.MutedStyle.Render(entry.State)
		if strings.EqualFold(entry.State, "running") {
			state = ui.SuccessStyle.Render(entry.State)
		}
		workspace := entry.Workspace
		if strings.TrimSpace(workspace) == "" {
			workspace = "(workspace label unavailable)"
		}
		fmt.Printf("  %s %-20s %s\n", state, entry.Name, ui.MutedStyle.Render(entry.ID+" - "+entry.Status))
		fmt.Printf("      workspace: %s\n", workspace)
		if strings.TrimSpace(entry.Config) != "" {
			fmt.Printf("      config:    %s\n", ui.MutedStyle.Render(entry.Config))
		}
		fmt.Println()
	}
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return names[0]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func trimID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) <= 12 {
		return id
	}
	return id[:12]
}

func normalizePath(path string) string {
	value := strings.TrimSpace(path)
	if value == "" {
		return ""
	}
	absPath, err := filepath.Abs(value)
	if err == nil {
		value = absPath
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		value = resolved
	}
	return filepath.Clean(value)
}

func samePath(a string, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func resolveWorkspace() (string, error) {
	candidate := strings.TrimSpace(devWorkspace)
	if candidate == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		candidate = cwd
	}
	absPath, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		absPath = resolved
	}
	return absPath, nil
}

func ensureWorkspaceHasDevcontainer(workspace string) error {
	path := filepath.Join(workspace, ".devcontainer", "devcontainer.json")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s not found. Run 'galena dev init' first", path)
		}
		return err
	}
	return nil
}

func initDevcontainerWorkspace(workspace string, profileID string, force bool) (string, error) {
	profiles, baseDir, err := loadDevProfiles()
	if err != nil {
		return "", err
	}
	profile, ok := findDevProfile(profiles, profileID)
	if !ok {
		return "", fmt.Errorf("profile %q not found", profileID)
	}

	templatePath := filepath.Join(baseDir, "templates", profile.Template)
	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("reading profile template %s: %w", templatePath, err)
	}
	if !json.Valid(templateData) {
		return "", fmt.Errorf("template %s is not valid JSON", templatePath)
	}

	devDir := filepath.Join(workspace, ".devcontainer")
	targetPath := filepath.Join(devDir, "devcontainer.json")
	if _, err := os.Stat(targetPath); err == nil && !force {
		return "", fmt.Errorf("%s already exists (use --force to overwrite)", targetPath)
	}

	if err := os.MkdirAll(devDir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(targetPath, templateData, 0o644); err != nil {
		return "", err
	}
	return targetPath, nil
}

func findDevProfile(profiles []devProfile, id string) (devProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true
		}
	}
	return devProfile{}, false
}

func loadDevProfiles() ([]devProfile, string, error) {
	catalogPaths := []string{
		"/usr/share/galena/devcontainer/profiles.yaml",
		"custom/devcontainer/profiles.yaml",
	}

	for _, path := range catalogPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var catalog devProfileCatalog
		if err := yaml.Unmarshal(data, &catalog); err != nil {
			return nil, "", fmt.Errorf("parsing %s: %w", path, err)
		}
		if len(catalog.Profiles) == 0 {
			return nil, "", fmt.Errorf("no profiles defined in %s", path)
		}
		if err := validateDevProfiles(catalog.Profiles); err != nil {
			return nil, "", err
		}
		sort.SliceStable(catalog.Profiles, func(i, j int) bool {
			return catalog.Profiles[i].Title < catalog.Profiles[j].Title
		})
		return catalog.Profiles, filepath.Dir(path), nil
	}

	return nil, "", fmt.Errorf("no devcontainer profile catalog found")
}

func validateDevProfiles(profiles []devProfile) error {
	seen := map[string]struct{}{}
	for _, profile := range profiles {
		if strings.TrimSpace(profile.ID) == "" {
			return fmt.Errorf("dev profile missing id")
		}
		if strings.TrimSpace(profile.Title) == "" {
			return fmt.Errorf("dev profile %q missing title", profile.ID)
		}
		if strings.TrimSpace(profile.Template) == "" {
			return fmt.Errorf("dev profile %q missing template", profile.ID)
		}
		if _, exists := seen[profile.ID]; exists {
			return fmt.Errorf("duplicate dev profile id %q", profile.ID)
		}
		seen[profile.ID] = struct{}{}
	}
	return nil
}

func containerPreferredBrewSet() map[string]struct{} {
	preferred := map[string]struct{}{}
	profiles, _, err := loadDevProfiles()
	if err != nil {
		return preferred
	}
	for _, profile := range profiles {
		for _, pkg := range profile.Brew {
			name := strings.TrimSpace(pkg)
			if name == "" {
				continue
			}
			preferred[name] = struct{}{}
		}
	}
	return preferred
}

func runDevcontainerCommandStreaming(args ...string) error {
	result := galexec.RunStreaming(context.Background(), "devcontainer", args, galexec.DefaultOptions())
	if result.Err != nil {
		return result.Err
	}
	return nil
}

func bootstrapSetupDevcontainer(profileID string) error {
	if err := galexec.RequireCommands("devcontainer", "podman"); err != nil {
		return err
	}

	workspace, err := resolveSetupWorkspace()
	if err != nil {
		return err
	}
	if _, err := initDevcontainerWorkspace(workspace, profileID, false); err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
	}
	return runDevcontainerCommandStreaming("up", "--workspace-folder", workspace)
}

func resolveSetupWorkspace() (string, error) {
	if wd, err := os.Getwd(); err == nil && wd != "/" {
		if abs, absErr := filepath.Abs(wd); absErr == nil {
			return abs, nil
		}
		return wd, nil
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home, nil
	}
	return "", fmt.Errorf("unable to determine workspace directory")
}
