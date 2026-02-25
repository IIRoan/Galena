package ui

import (
	"fmt"

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

type menuModel struct {
	list      list.Model
	title     string
	subtitle  string
	choice    string
	quitting  bool
	allowBack bool
	help      help.Model
	keys      menuKeyMap
}

func newMenuModel(title string, subtitle string, items []MenuItem, cfg menuConfig) menuModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)
	baseTitle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Foreground)))
	delegate.Styles.NormalTitle = baseTitle
	delegate.Styles.SelectedTitle = baseTitle.Foreground(lipgloss.Color(string(Primary))).Bold(true)
	delegate.Styles.DimmedTitle = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.NormalDesc = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.SelectedDesc = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.DimmedDesc = baseTitle.Foreground(lipgloss.Color(string(Muted)))
	delegate.Styles.FilterMatch = lipgloss.NewStyle().Underline(true)

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
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Muted)))
	l.Styles.HelpStyle = hintStyle
	l.Styles.PaginationStyle = hintStyle

	helpModel := help.New()
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(string(Accent))).Bold(true)
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
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := clampSize(msg.Width-2, 40)
		height := clampSize(msg.Height-8, 10)
		m.list.SetSize(width, height)
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

func (m menuModel) View() tea.View {
	if m.quitting {
		return tea.View{}
	}

	content := m.list.View()
	footer := m.help.View(m.keys)
	v := tea.NewView(Frame(m.title, m.subtitle, content, footer))
	v.AltScreen = true
	return v
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
