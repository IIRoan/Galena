package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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

type step int

const (
	stepIntro step = iota
	stepBrew
	stepFlatpak
	stepSummary
	stepDeploy
	stepDone
)

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

type model struct {
	currentStep      step
	brewPackages     []string
	flatpakApps      []string
	selectedBrew     []string
	selectedFlatpaks []string
	disableSetup     bool
	form             *huh.Form
	width            int
	height           int
	quitting         bool
	installing       bool
	spinner          spinner.Model
	finished         bool

	// Installation state
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

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = ui.PrimaryStyle()

	m := &model{
		currentStep:  stepIntro,
		brewPackages: brewPackages,
		flatpakApps:  flatpakApps,
		disableSetup: true,
		spinner:      s,
	}

	m.updateForm()

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}
	if fm, ok := finalModel.(*model); ok && fm.finished {
		printSetupSummary(fm)
	}
	return nil
}

func (m *model) updateForm() {
	var group *huh.Group

	switch m.currentStep {
	case stepIntro:
		group = huh.NewGroup(
			huh.NewNote().
				Title("WELCOME TO GALENA").
				Description("\n" + ui.MutedStyle.Render("This wizard will personalize your new installation by setting up user-specific packages and desktop applications.") + "\n\n" +
					ui.PanelTitle.Render("Phase 1: Environment Provisioning") + "\n" +
					"We will discover available tools and let you choose exactly what to install."),
		)
	case stepBrew:
		options := make([]huh.Option[string], 0)
		for _, pkg := range m.brewPackages {
			options = append(options, huh.NewOption(pkg, pkg).Selected(true))
		}
		group = huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("SELECT CLI TOOLS").
				Description("Choose command-line utilities to be managed via Homebrew.").
				Options(options...).
				Value(&m.selectedBrew).
				Height(12).
				Filterable(true),
		)
	case stepFlatpak:
		options := make([]huh.Option[string], 0)
		for _, app := range m.flatpakApps {
			options = append(options, huh.NewOption(app, app).Selected(true))
		}
		group = huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("SELECT APPLICATIONS").
				Description("Choose desktop applications to be installed via Flatpak.").
				Options(options...).
				Value(&m.selectedFlatpaks).
				Height(12).
				Filterable(true),
		)
	case stepSummary:
		group = huh.NewGroup(
			huh.NewConfirm().
				Title("READY TO DEPLOY?").
				Description(fmt.Sprintf("\n%s\n%s\n\nPersist configuration?\n%s",
					ui.AccentStyle().Render(fmt.Sprintf(" • %d CLI tools", len(m.selectedBrew))),
					ui.AccentStyle().Render(fmt.Sprintf(" • %d GUI apps", len(m.selectedFlatpaks))),
					ui.MutedStyle.Render("If yes, this wizard will not show again on next boot."),
				)).
				Value(&m.disableSetup).
				Affirmative("Deploy Now").
				Negative("Keep showing"),
		)
	}

	if group != nil {
		m.form = huh.NewForm(group).WithTheme(ui.HuhTheme())
		m.form.Init()
		m.form.NextField()
		m.form.PrevField()
	}
}

func (m *model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "q":
			if m.currentStep == stepDone {
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
		m.currentStep = stepDone
		m.finished = true
		m.quitting = true
		return m, tea.Quit
	}

	if m.currentStep == stepDeploy && !m.installing {
		m.installing = true
		m.prepareTasks()
		return m, m.nextTask()
	}

	if m.form != nil && m.currentStep < stepDeploy {
		form, cmd := m.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			m.form = f
		}
		cmds = append(cmds, cmd)

		if m.form.State == huh.StateCompleted {
			if m.currentStep == stepSummary {
				m.currentStep = stepDeploy
				if !m.installing {
					m.installing = true
					m.prepareTasks()
					cmds = append(cmds, m.nextTask())
				}
			} else {
				m.currentStep++
				m.updateForm()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *model) prepareTasks() {
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

func (m *model) nextTask() tea.Cmd {
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

func (m *model) finalize() tea.Cmd {
	return func() tea.Msg {
		if m.disableSetup {
			_ = os.MkdirAll("/var/lib/galena", 0755)
			_ = os.WriteFile("/var/lib/galena/setup.done", []byte("done"), 0644)
		}
		return allFinishedMsg{}
	}
}

func (m *model) View() string {
	if m.quitting {
		return ""
	}

	sidebarWidth := 26
	contentWidth := m.width - sidebarWidth - 6
	if contentWidth < 40 {
		contentWidth = 40
	}
	contentHeight := m.height - 4
	if contentHeight < 8 {
		contentHeight = 8
	}
	contentInnerWidth := contentWidth - 8
	if contentInnerWidth < 20 {
		contentInnerWidth = 20
	}

	sidebarStyle := lipgloss.NewStyle().
		Width(sidebarWidth).
		Height(m.height-4).
		Border(lipgloss.NormalBorder(), false, true, false, false).
		BorderForeground(ui.Muted).
		Padding(1, 2)

	contentStyle := lipgloss.NewStyle().
		Width(contentWidth).
		Height(contentHeight).
		Padding(1, 4)

	var sb strings.Builder
	steps := []string{"Introduction", "CLI Utilities", "Applications", "Summary", "Deployment"}
	for i, s := range steps {
		stepIdx := step(i)
		prefix := "  "
		style := ui.MutedStyle
		if m.currentStep == stepIdx {
			prefix = "→ "
			style = ui.PrimaryStyle()
		} else if m.currentStep > stepIdx {
			prefix = "✓ "
			style = ui.SuccessStyle
		}
		sb.WriteString(style.Render(prefix+s) + "\n\n")
	}

	header := ui.Header(" SYSTEM SETUP ")

	var body string
	if m.currentStep == stepDeploy {
		body = m.renderDeploymentView(contentInnerWidth)
	} else if m.form != nil {
		body = m.form.View()
	}

	main := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarStyle.Render(sb.String()),
		contentStyle.Render(body),
	)

	return lipgloss.JoinVertical(lipgloss.Left, header, main)
}

func (m *model) renderDeploymentView(contentWidth int) string {
	var s strings.Builder
	s.WriteString(ui.PanelTitle.Render("PROVISIONING IN PROGRESS") + "\n\n")

	taskLine := fmt.Sprintf("%s Processing: %s", m.spinner.View(), ui.AccentStyle().Render(m.currentTaskName))
	s.WriteString(lipgloss.NewStyle().Width(contentWidth).Render(taskLine) + "\n\n")

	// Progress Bar
	barWidth := contentWidth - 6
	if barWidth > 40 {
		barWidth = 40
	}
	if barWidth < 10 {
		barWidth = 10
	}
	filled := 0
	if m.totalTasks > 0 {
		percent := float64(m.completedTasks) / float64(m.totalTasks)
		filled = int(percent * float64(barWidth))
	}
	bar := lipgloss.NewStyle().Foreground(ui.Success).Render(strings.Repeat("█", filled)) +
		lipgloss.NewStyle().Foreground(ui.Muted).Render(strings.Repeat("░", barWidth-filled))

	progPercent := 0
	if m.totalTasks > 0 {
		progPercent = int(float64(m.completedTasks) / float64(m.totalTasks) * 100)
	}
	s.WriteString(fmt.Sprintf("%s %d%%\n\n", bar, progPercent))

	s.WriteString(ui.MutedStyle.Width(contentWidth).Render("This may take several minutes depending on your connection."))

	return s.String()
}

func printSetupSummary(m *model) {
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
	defer file.Close()
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
	defer file.Close()
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
