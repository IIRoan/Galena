package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
)

const (
	MenuActionBack = "__back__"
	MenuActionQuit = "__quit__"
)

type MenuOption func(*menuConfig)

type menuConfig struct {
	allowBack bool
	backLabel string
}

func defaultMenuConfig() menuConfig {
	return menuConfig{
		allowBack: false,
		backLabel: "Back",
	}
}

func WithBackNavigation(label string) MenuOption {
	return func(cfg *menuConfig) {
		cfg.allowBack = true
		if label != "" {
			cfg.backLabel = label
		}
	}
}

type menuKeyMap struct {
	Select  key.Binding
	Back    key.Binding
	Quit    key.Binding
	Filter  key.Binding
	hasBack bool
}

func newMenuKeyMap(allowBack bool, backLabel string) menuKeyMap {
	selectKey := key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "select"),
	)
	filterKey := key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "filter"),
	)
	if allowBack {
		return menuKeyMap{
			Select:  selectKey,
			Filter:  filterKey,
			Back:    key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", backLabel)),
			Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
			hasBack: true,
		}
	}
	return menuKeyMap{
		Select:  selectKey,
		Filter:  filterKey,
		Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "quit")),
		hasBack: false,
	}
}

func (k menuKeyMap) ShortHelp() []key.Binding {
	if k.hasBack {
		return []key.Binding{k.Select, k.Filter, k.Back}
	}
	return []key.Binding{k.Select, k.Filter, k.Quit}
}

func (k menuKeyMap) FullHelp() [][]key.Binding {
	if k.hasBack {
		return [][]key.Binding{{k.Select, k.Filter}, {k.Back, k.Quit}}
	}
	return [][]key.Binding{{k.Select, k.Filter}, {k.Quit}}
}

// MenuItem represents a selectable item in a TUI list.
type MenuItem struct {
	ID        string
	TitleText string
	Details   string
}

// Title returns the menu label.
func (m MenuItem) Title() string { return m.TitleText }

// Description returns the menu details.
func (m MenuItem) Description() string { return m.Details }

// FilterValue returns the filterable text.
func (m MenuItem) FilterValue() string { return m.TitleText }

type menuTickMsg time.Time

type menuModel struct {
	list      list.Model
	title     string
	subtitle  string
	choice    string
	quitting  bool
	allowBack bool
	help      help.Model
	keys      menuKeyMap

	width  int
	height int
	now    time.Time
	pulse  int
}

func newMenuModel(title string, subtitle string, items []MenuItem, cfg menuConfig) menuModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(1)
	baseTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Foreground)))
	delegate.Styles.NormalTitle = baseTitle
	delegate.Styles.SelectedTitle = baseTitle.Foreground(lipgloss.Color(string(Accent))).Bold(true)
	delegate.Styles.DimmedTitle = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.NormalDesc = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.SelectedDesc = baseTitle.Foreground(lipgloss.Color(string(Highlight)))
	delegate.Styles.DimmedDesc = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.FilterMatch = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color(string(Primary)))

	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	l := list.New(listItems, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.DisableQuitKeybindings()
	l.Styles.HelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(string(Muted)))
	l.Styles.PaginationStyle = l.Styles.HelpStyle

	helpModel := help.New()
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Accent))).Bold(true)
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Muted)))
	helpModel.Styles.ShortKey = keyStyle
	helpModel.Styles.ShortDesc = hintStyle
	helpModel.Styles.FullKey = keyStyle
	helpModel.Styles.FullDesc = hintStyle
	helpModel.Styles.Ellipsis = hintStyle

	keys := newMenuKeyMap(cfg.allowBack, cfg.backLabel)

	return menuModel{
		list:      l,
		title:     title,
		subtitle:  subtitle,
		allowBack: cfg.allowBack,
		help:      helpModel,
		keys:      keys,
		now:       time.Now(),
	}
}

func menuTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return menuTickMsg(t)
	})
}

func (m menuModel) Init() tea.Cmd {
	return menuTick()
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeList()
	case menuTickMsg:
		m.now = time.Time(msg)
		m.pulse++
		return m, menuTick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(MenuItem); ok {
				m.choice = item.ID
				return m, tea.Quit
			}
		case "q", "esc":
			m.quitting = true
			if m.allowBack {
				m.choice = MenuActionBack
			} else {
				m.choice = MenuActionQuit
			}
			return m, tea.Quit
		case "ctrl+c":
			m.quitting = true
			m.choice = MenuActionQuit
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *menuModel) resizeList() {
	width := m.width
	height := m.height
	if width <= 0 {
		width = terminalWidth()
	}
	if height <= 0 {
		height = 26
	}

	left := int(float64(width) * 0.58)
	if left < 42 {
		left = 42
	}
	if left > width-28 {
		left = width - 28
	}
	if left < 30 {
		left = 30
	}

	listWidth := left - 7
	if listWidth < 24 {
		listWidth = 24
	}
	listHeight := height - 12
	if listHeight < 8 {
		listHeight = 8
	}
	m.list.SetSize(listWidth, listHeight)
}

func (m menuModel) View() tea.View {
	if m.quitting {
		return tea.View{}
	}

	width := m.width
	height := m.height
	if width <= 0 {
		width = terminalWidth()
	}
	if height <= 0 {
		height = 26
	}

	leftWidth := int(float64(width) * 0.58)
	if leftWidth < 42 {
		leftWidth = 42
	}
	if leftWidth > width-28 {
		leftWidth = width - 28
	}
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := width - leftWidth - 3
	if rightWidth < 24 {
		rightWidth = 24
	}

	bodyHeight := height - 9
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	leftPanel := Panel.
		Width(leftWidth).
		Height(bodyHeight).
		Render(m.renderLeftPanel(leftWidth - 4))

	rightPanel := Panel.
		Width(rightWidth).
		Height(bodyHeight).
		Render(m.renderRightPanel(rightWidth - 4))

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, " ", rightPanel)
	footer := m.help.View(m.keys)

	v := tea.NewView(Frame(m.title, m.subtitle, body, footer))
	v.AltScreen = true
	return v
}

func (m menuModel) renderLeftPanel(innerWidth int) string {
	header := AccentStyle().Render("ORBIT NAV")
	line := lipgloss.NewStyle().
		Foreground(lipgloss.Color(string(Border))).
		Render(strings.Repeat("─", clampSize(innerWidth, 20)))
	return lipgloss.JoinVertical(lipgloss.Left, header, line, m.list.View())
}

func (m menuModel) renderRightPanel(innerWidth int) string {
	item, _ := m.list.SelectedItem().(MenuItem)
	indicator := "◐"
	if m.pulse%2 == 1 {
		indicator = "◓"
	}

	clock := m.now.Format("15:04:05")
	if clock == "00:00:00" {
		clock = time.Now().Format("15:04:05")
	}

	section := []string{
		AccentStyle().Render("SECTOR DATA"),
		MutedStyle.Render(strings.Repeat("─", clampSize(innerWidth, 20))),
		fmt.Sprintf("%s  %s", PrimaryStyle().Render(indicator), MutedStyle.Render("sync "+clock)),
		"",
	}

	if item.TitleText != "" {
		section = append(section,
			PrimaryStyle().Render(item.TitleText),
			MutedStyle.Width(innerWidth).Render(item.Details),
			"",
			fmt.Sprintf("%s %s", KeyStyle.Render("ID"), item.ID),
		)
	} else {
		section = append(section, MutedStyle.Render("No selection"))
	}

	section = append(section,
		"",
		AccentStyle().Render("KEYS"),
		MutedStyle.Render("↑/↓ move"),
		MutedStyle.Render("enter select"),
		MutedStyle.Render("/ filter"),
	)

	return strings.Join(section, "\n")
}

func clampSize(value int, min int) int {
	if value < min {
		return min
	}
	return value
}

// RunMenu displays a TUI list and returns the selected item ID.
func RunMenu(title string, subtitle string, items []MenuItem) (string, error) {
	if !IsInteractiveTerminal() {
		return "", fmt.Errorf("non-interactive terminal")
	}
	model := newMenuModel(title, subtitle, items, defaultMenuConfig())
	return runMenuModel(model)
}

func RunMenuWithOptions(title string, subtitle string, items []MenuItem, options ...MenuOption) (string, error) {
	if !IsInteractiveTerminal() {
		return "", fmt.Errorf("non-interactive terminal")
	}
	cfg := defaultMenuConfig()
	for _, opt := range options {
		opt(&cfg)
	}
	model := newMenuModel(title, subtitle, items, cfg)
	return runMenuModel(model)
}

func runMenuModel(model menuModel) (string, error) {
	program := tea.NewProgram(model)
	result, err := program.Run()
	if err != nil {
		return "", err
	}
	if finalModel, ok := result.(menuModel); ok {
		return finalModel.choice, nil
	}
	return "", nil
}
