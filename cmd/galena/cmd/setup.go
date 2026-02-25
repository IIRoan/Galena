package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	lipglossv2 "charm.land/lipgloss/v2"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/ui"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "High-fidelity system setup wizard",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

type installTask struct {
	name string
	kind string // "brew" or "flatpak"
}

type taskStartedMsg installTask

type taskFinishedMsg struct {
	task    installTask
	skipped bool
	err     error
}

type allFinishedMsg struct{}

type deploymentModel struct {
	selectedBrew     []string
	selectedFlatpaks []string
	disableSetup     bool

	width      int
	height     int
	quitting   bool
	installing bool
	spinner    spinner.Model
	finished   bool

	keyboardReportEvents bool

	pendingTasks    []installTask
	completedTasks  int
	totalTasks      int
	currentTaskName string
	skippedItems    []string
	failedItems     []string
}

func runSetup(cmd *cobra.Command, args []string) error {
	stateDir := "/var/lib/galena"
	doneFile := filepath.Join(stateDir, "setup.done")

	if _, err := os.Stat(doneFile); err == nil {
		fmt.Println(ui.SuccessStyle.Render("Setup already completed!"))
		return nil
	}

	brewPackages, _ := getBrewPackages("/usr/share/ublue-os/homebrew/default.Brewfile")
	if len(brewPackages) == 0 {
		brewPackages, _ = getBrewPackages("custom/brew/default.Brewfile")
	}

	flatpakApps, _ := getFlatpakApps("/etc/flatpak/preinstall.d/default.preinstall")
	if len(flatpakApps) == 0 {
		flatpakApps, _ = getFlatpakApps("custom/flatpaks/default.preinstall")
	}

	selectedBrew, selectedFlatpaks, disableSetup, err := promptSetupSelections(brewPackages, flatpakApps)
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil
		}
		return err
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipglossv2.NewStyle().Foreground(lipglossv2.Color(string(ui.Primary)))

	m := &deploymentModel{
		selectedBrew:     selectedBrew,
		selectedFlatpaks: selectedFlatpaks,
		disableSetup:     disableSetup,
		spinner:          s,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(*deploymentModel); ok && fm.finished {
		printSetupSummary(fm)
	}
	return nil
}

func promptSetupSelections(brewPackages []string, flatpakApps []string) ([]string, []string, bool, error) {
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewNote().
				Title("WELCOME TO GALENA").
				Description("\n" + ui.MutedStyle.Render("This wizard personalizes your installation by selecting CLI and GUI packages.") + "\n\n" +
					ui.PanelTitle.Render("Phase 1: Environment Provisioning") + "\n" +
					"Choose exactly what to install before deployment."),
		),
	).WithTheme(ui.HuhTheme()).Run(); err != nil {
		return nil, nil, false, err
	}

	selectedBrew := make([]string, 0, len(brewPackages))
	if len(brewPackages) > 0 {
		options := make([]huh.Option[string], 0, len(brewPackages))
		for _, pkg := range brewPackages {
			options = append(options, huh.NewOption(pkg, pkg).Selected(true))
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("SELECT CLI TOOLS").
					Description("Choose command-line utilities to be managed via Homebrew.").
					Options(options...).
					Value(&selectedBrew).
					Height(12).
					Filterable(true),
			),
		).WithTheme(ui.HuhTheme()).Run(); err != nil {
			return nil, nil, false, err
		}
	}

	selectedFlatpaks := make([]string, 0, len(flatpakApps))
	if len(flatpakApps) > 0 {
		options := make([]huh.Option[string], 0, len(flatpakApps))
		for _, app := range flatpakApps {
			options = append(options, huh.NewOption(app, app).Selected(true))
		}
		if err := huh.NewForm(
			huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("SELECT APPLICATIONS").
					Description("Choose desktop applications to be installed via Flatpak.").
					Options(options...).
					Value(&selectedFlatpaks).
					Height(12).
					Filterable(true),
			),
		).WithTheme(ui.HuhTheme()).Run(); err != nil {
			return nil, nil, false, err
		}
	}

	disableSetup := true
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("READY TO DEPLOY?").
				Description(fmt.Sprintf("\n%s\n%s\n\nPersist setup completion?\n%s",
					ui.AccentStyle().Render(fmt.Sprintf(" • %d CLI tools", len(selectedBrew))),
					ui.AccentStyle().Render(fmt.Sprintf(" • %d GUI apps", len(selectedFlatpaks))),
					ui.MutedStyle.Render("If yes, this wizard will not show again on next boot."),
				)).
				Value(&disableSetup).
				Affirmative("Deploy Now").
				Negative("Keep showing"),
		),
	).WithTheme(ui.HuhTheme()).Run(); err != nil {
		return nil, nil, false, err
	}

	return selectedBrew, selectedFlatpaks, disableSetup, nil
}

func (m *deploymentModel) Init() tea.Cmd {
	m.prepareTasks()
	m.installing = true
	return tea.Batch(
		m.spinner.Tick,
		m.nextTask(),
	)
}

func (m *deploymentModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyboardEnhancementsMsg:
		m.keyboardReportEvents = msg.SupportsEventTypes()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.finished {
				m.quitting = true
				return m, tea.Quit
			}
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case taskStartedMsg:
		m.currentTaskName = fmt.Sprintf("%s (%s)", msg.name, msg.kind)
		return m, nil
	case taskFinishedMsg:
		m.completedTasks++
		if msg.skipped {
			m.skippedItems = append(m.skippedItems, msg.task.name)
		} else if msg.err != nil {
			m.failedItems = append(m.failedItems, fmt.Sprintf("%s: %v", msg.task.name, msg.err))
		}
		return m, m.nextTask()
	case allFinishedMsg:
		m.finished = true
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m *deploymentModel) prepareTasks() {
	m.pendingTasks = make([]installTask, 0)
	for _, pkg := range m.selectedBrew {
		m.pendingTasks = append(m.pendingTasks, installTask{name: pkg, kind: "brew"})
	}
	for _, app := range m.selectedFlatpaks {
		m.pendingTasks = append(m.pendingTasks, installTask{name: app, kind: "flatpak"})
	}
	m.totalTasks = len(m.pendingTasks)
	m.completedTasks = 0
}

func (m *deploymentModel) nextTask() tea.Cmd {
	if len(m.pendingTasks) == 0 {
		return m.finalize()
	}

	task := m.pendingTasks[0]
	m.pendingTasks = m.pendingTasks[1:]

	return tea.Batch(
		func() tea.Msg { return taskStartedMsg(task) },
		func() tea.Msg {
			var err error
			skipped := false
			if task.kind == "brew" {
				if exec.Command("brew", "list", task.name).Run() == nil {
					skipped = true
				} else {
					err = exec.Command("brew", "install", task.name).Run()
				}
			} else {
				if exec.Command("flatpak", "info", task.name).Run() == nil {
					skipped = true
				} else {
					err = exec.Command("flatpak", "install", "-y", "--system", "flathub", task.name).Run()
				}
			}
			return taskFinishedMsg{task: task, skipped: skipped, err: err}
		},
	)
}

func (m *deploymentModel) finalize() tea.Cmd {
	return func() tea.Msg {
		if m.disableSetup {
			_ = os.MkdirAll("/var/lib/galena", 0o755)
			_ = os.WriteFile("/var/lib/galena/setup.done", []byte("done"), 0o644)
		}
		return allFinishedMsg{}
	}
}

func (m *deploymentModel) View() tea.View {
	if m.quitting {
		return tea.View{}
	}

	width := m.width
	if width <= 0 {
		width = 100
	}
	height := m.height
	if height <= 0 {
		height = 30
	}

	sidebarWidth := 28
	contentWidth := width - sidebarWidth - 6
	if contentWidth < 40 {
		contentWidth = 40
	}
	contentHeight := height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	contentInnerWidth := contentWidth - 8
	if contentInnerWidth < 20 {
		contentInnerWidth = 20
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(height-4).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(ui.Muted).
		Padding(1, 2)

	contentStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Padding(1, 4)

	var sb strings.Builder
	sb.WriteString(ui.PanelTitle.Render("DEPLOYMENT") + "\n\n")
	sb.WriteString(ui.MutedStyle.Render(fmt.Sprintf("CLI tools: %d", len(m.selectedBrew))) + "\n")
	sb.WriteString(ui.MutedStyle.Render(fmt.Sprintf("GUI apps:  %d", len(m.selectedFlatpaks))) + "\n\n")
	sb.WriteString(ui.AccentStyle().Render(fmt.Sprintf("Progress: %d/%d", m.completedTasks, m.totalTasks)) + "\n\n")
	if m.keyboardReportEvents {
		sb.WriteString(ui.SuccessStyle.Render("Keyboard enhancements: active") + "\n")
	} else {
		sb.WriteString(ui.MutedStyle.Render("Keyboard enhancements: basic") + "\n")
	}

	header := ui.Header(" SYSTEM SETUP ")
	main := lipgloss.JoinHorizontal(
		lipgloss.Top,
		sidebarStyle.Render(sb.String()),
		contentStyle.Render(m.renderDeploymentView(contentInnerWidth)),
	)

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, header, main))
	v.AltScreen = true
	v.WindowTitle = "Galena Setup"
	v.KeyboardEnhancements.ReportEventTypes = true

	progressPercent := 0
	if m.totalTasks > 0 {
		progressPercent = int(float64(m.completedTasks) / float64(m.totalTasks) * 100)
	}
	state := tea.ProgressBarDefault
	if len(m.failedItems) > 0 {
		state = tea.ProgressBarWarning
	}
	v.ProgressBar = tea.NewProgressBar(state, progressPercent)

	return v
}

func (m *deploymentModel) renderDeploymentView(contentWidth int) string {
	var s strings.Builder
	s.WriteString(ui.PanelTitle.Render("PROVISIONING IN PROGRESS") + "\n\n")

	taskLabel := m.currentTaskName
	if taskLabel == "" {
		taskLabel = "Preparing tasks..."
	}
	taskLine := fmt.Sprintf("%s Processing: %s", m.spinner.View(), ui.AccentStyle().Render(taskLabel))
	s.WriteString(lipgloss.NewStyle().Width(contentWidth).Render(taskLine) + "\n\n")

	barWidth := contentWidth - 6
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 10 {
		barWidth = 10
	}

	filled := 0
	if m.totalTasks > 0 {
		filled = int(float64(m.completedTasks) / float64(m.totalTasks) * float64(barWidth))
	}
	bar := lipgloss.NewStyle().Foreground(ui.Success).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(ui.Muted).Render(strings.Repeat("░", barWidth-filled))

	progPercent := 0
	if m.totalTasks > 0 {
		progPercent = int(float64(m.completedTasks) / float64(m.totalTasks) * 100)
	}
	_, _ = fmt.Fprintf(&s, "%s %d%%\n\n", bar, progPercent)
	s.WriteString(ui.MutedStyle.Width(contentWidth).Render("This may take several minutes depending on your connection."))

	return s.String()
}

func printSetupSummary(m *deploymentModel) {
	installed := m.totalTasks - len(m.skippedItems) - len(m.failedItems)

	fmt.Println()
	fmt.Println("Provisioning Summary")
	fmt.Printf("Installed: %d items\n", installed)
	if len(m.skippedItems) > 0 {
		fmt.Printf("Skipped: %d items (already present)\n", len(m.skippedItems))
	}
	if len(m.failedItems) > 0 {
		fmt.Printf("Failed: %d items\n", len(m.failedItems))
	}

	if len(m.skippedItems) > 0 {
		fmt.Println()
		fmt.Println("Skipped (up to date):")
		for _, item := range m.skippedItems {
			fmt.Printf("  - %s\n", item)
		}
	}

	if len(m.failedItems) > 0 {
		fmt.Println()
		fmt.Println("Failures:")
		for _, item := range m.failedItems {
			fmt.Printf("  - %s\n", item)
		}
	}

	fmt.Println()
	if m.disableSetup {
		fmt.Println("Setup persistence: enabled (won't show next boot).")
	} else {
		fmt.Println("Setup persistence: disabled (will show on next boot).")
	}
	fmt.Println("Your system is ready.")
}

func getBrewPackages(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	packages := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "brew ") {
			parts := strings.Split(line, "\"")
			if len(parts) < 2 {
				parts = strings.Split(line, "'")
			}
			if len(parts) >= 2 {
				packages = append(packages, parts[1])
			}
		}
	}
	return packages, nil
}

func getFlatpakApps(path string) ([]string, error) {
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
		if strings.HasPrefix(line, "[Flatpak Preinstall ") && strings.HasSuffix(line, "]") {
			id := strings.TrimPrefix(line, "[Flatpak Preinstall ")
			id = strings.TrimSuffix(id, "]")
			apps = append(apps, id)
		}
	}
	return apps, nil
}
