package ui

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	MenuActionBack = "__back__"
	MenuActionQuit = "__quit__"
)

type MenuOption func(*menuConfig)

type menuConfig struct {
	allowBack          bool
	backLabel          string
	initialSelectionID string
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

// WithInitialSelectionID pre-selects an item by ID when the menu opens.
func WithInitialSelectionID(id string) MenuOption {
	return func(cfg *menuConfig) {
		cfg.initialSelectionID = strings.TrimSpace(id)
	}
}

type menuKeyMap struct {
	Select  key.Binding
	Back    key.Binding
	Quit    key.Binding
	Filter  key.Binding
	Jump    key.Binding
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
	jumpKey := key.NewBinding(
		key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8", "9"),
		key.WithHelp("1-9", "quick launch"),
	)
	if allowBack {
		return menuKeyMap{
			Select:  selectKey,
			Filter:  filterKey,
			Jump:    jumpKey,
			Back:    key.NewBinding(key.WithKeys("esc", "q"), key.WithHelp("esc/q", backLabel)),
			Quit:    key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
			hasBack: true,
		}
	}
	return menuKeyMap{
		Select:  selectKey,
		Filter:  filterKey,
		Jump:    jumpKey,
		Quit:    key.NewBinding(key.WithKeys("q", "esc", "ctrl+c"), key.WithHelp("q/esc", "quit")),
		hasBack: false,
	}
}

func (k menuKeyMap) ShortHelp() []key.Binding {
	if k.hasBack {
		return []key.Binding{k.Select, k.Jump, k.Filter, k.Back}
	}
	return []key.Binding{k.Select, k.Jump, k.Filter, k.Quit}
}

func (k menuKeyMap) FullHelp() [][]key.Binding {
	if k.hasBack {
		return [][]key.Binding{{k.Select, k.Jump, k.Filter}, {k.Back, k.Quit}}
	}
	return [][]key.Binding{{k.Select, k.Jump, k.Filter}, {k.Quit}}
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
func (m MenuItem) FilterValue() string { return m.TitleText + " " + m.Details + " " + m.ID }

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

	cliVersion    string
	galenaVersion string
}

type menuLayout struct {
	stacked     bool
	leftWidth   int
	rightWidth  int
	leftHeight  int
	rightHeight int
	listWidth   int
	listHeight  int
}

type launcherDelegate struct {
	slot          lipgloss.Style
	title         lipgloss.Style
	selectedTitle lipgloss.Style
	dimmedTitle   lipgloss.Style
}

func newLauncherDelegate() launcherDelegate {
	return launcherDelegate{
		slot: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(Muted))),
		title: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(Foreground))),
		selectedTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(Primary))).
			Bold(true),
		dimmedTitle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(string(Muted))),
	}
}

func (d launcherDelegate) Height() int { return 1 }

func (d launcherDelegate) Spacing() int { return 0 }

func (d launcherDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d launcherDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	menuItem, ok := item.(MenuItem)
	if !ok || m.Width() <= 0 {
		return
	}

	isSelected := index == m.Index() && m.FilterState() != list.Filtering
	emptyFilter := m.FilterState() == list.Filtering && strings.TrimSpace(m.FilterValue()) == ""

	slot := fmt.Sprintf("%d.", index+1)
	available := max(14, m.Width()-6)
	content := menuItem.TitleText
	if menuItem.Details != "" && m.Width() > 68 {
		content += " - " + menuItem.Details
	}
	content = ansi.Truncate(content, available, "...")

	prefix := "  "
	slotText := d.slot.Render(slot)
	titleText := d.title.Render(content)

	if isSelected {
		prefix = "> "
		slotText = d.selectedTitle.Render(slot)
		titleText = d.selectedTitle.Render(content)
		fmt.Fprint(w, prefix+slotText+" "+titleText) //nolint:errcheck
		return
	}
	if emptyFilter {
		titleText = d.dimmedTitle.Render(content)
		fmt.Fprint(w, prefix+slotText+" "+titleText) //nolint:errcheck
		return
	}
	fmt.Fprint(w, prefix+slotText+" "+titleText) //nolint:errcheck
}

func newMenuModel(title string, subtitle string, items []MenuItem, cfg menuConfig) menuModel {
	delegate := newLauncherDelegate()
	cliVersion, galenaVersion := resolveSystemVersions()

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

	if cfg.initialSelectionID != "" {
		for idx, item := range items {
			if item.ID == cfg.initialSelectionID {
				l.Select(idx)
				break
			}
		}
	}

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
		list:          l,
		title:         title,
		subtitle:      subtitle,
		allowBack:     cfg.allowBack,
		help:          helpModel,
		keys:          keys,
		now:           time.Now(),
		cliVersion:    cliVersion,
		galenaVersion: galenaVersion,
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
		return m, menuTick()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(MenuItem); ok {
				m.choice = item.ID
				return m, tea.Quit
			}
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if m.list.FilterState() != list.Filtering {
				if m.selectByNumber(msg.String()) {
					return m, tea.Quit
				}
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

func (m *menuModel) pageStartIndex() int {
	start := m.list.Index() - m.list.Cursor()
	if start < 0 {
		return 0
	}
	return start
}

func (m *menuModel) selectByNumber(keyNum string) bool {
	if len(keyNum) != 1 {
		return false
	}
	slot := int(keyNum[0]-'1') + 1
	if slot < 1 || slot > 9 {
		return false
	}

	visible := m.list.VisibleItems()
	if len(visible) == 0 {
		return false
	}

	target := m.pageStartIndex() + (slot - 1)
	if target < 0 || target >= len(visible) {
		return false
	}

	m.list.Select(target)
	if item, ok := visible[target].(MenuItem); ok {
		m.choice = item.ID
		return true
	}
	return false
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
	layout := calculateMenuLayout(width, height)
	m.list.SetSize(layout.listWidth, layout.listHeight)
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
	layout := calculateMenuLayout(width, height)

	leftPanel := lipgloss.NewStyle().
		Width(layout.leftWidth).
		Height(layout.leftHeight).
		PaddingRight(1).
		Render(m.renderLeftPanel(layout.leftWidth - 1))

	rightPanel := lipgloss.NewStyle().
		Width(layout.rightWidth).
		Height(layout.rightHeight).
		PaddingLeft(1).
		Render(m.renderRightPanel(layout.rightWidth-1, layout.rightHeight))

	var body string
	if layout.stacked {
		body = lipgloss.JoinVertical(lipgloss.Left, leftPanel, "", rightPanel)
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	}
	footer := m.help.View(m.keys)

	v := tea.NewView(Frame(m.title, m.subtitle, body, footer))
	v.AltScreen = true
	return v
}

func (m menuModel) renderLeftPanel(innerWidth int) string {
	filter := strings.TrimSpace(m.list.FilterValue())
	if filter == "" {
		return m.list.View()
	}
	filterHint := MutedStyle.Render("filter: " + ansi.Truncate(filter, max(10, innerWidth-8), "..."))
	return lipgloss.JoinVertical(lipgloss.Left, m.list.View(), "", filterHint)
}

func (m menuModel) renderRightPanel(innerWidth int, innerHeight int) string {
	item, _ := m.list.SelectedItem().(MenuItem)
	clock := m.now.Format("15:04:05")
	if clock == "00:00:00" {
		clock = time.Now().Format("15:04:05")
	}

	section := []string{
		lipgloss.NewStyle().Foreground(lipgloss.Color(string(Accent))).Bold(true).Render("Selection"),
	}

	if item.TitleText != "" {
		section = append(section,
			PrimaryStyle().Render(ansi.Truncate(item.TitleText, max(8, innerWidth), "...")),
		)
		if strings.TrimSpace(item.Details) != "" {
			section = append(section, MutedStyle.Render(ansi.Truncate(item.Details, max(8, innerWidth), "...")))
		}
		section = append(section, MutedStyle.Render("id: "+item.ID))
	} else {
		section = append(section, MutedStyle.Render("No selection"))
	}

	section = append(section,
		"",
		lipgloss.NewStyle().Foreground(lipgloss.Color(string(Accent))).Bold(true).Render("System"),
		menuInfoLine("CLI", m.cliVersion, innerWidth),
		menuInfoLine("Galena", m.galenaVersion, innerWidth),
		menuInfoLine("Clock", clock, innerWidth),
		"",
		MutedStyle.Render("Enter to run"),
		MutedStyle.Render("1-9 quick launch"),
	)

	if len(section) > innerHeight {
		section = section[:innerHeight]
	}
	return strings.Join(section, "\n")
}

func calculateMenuLayout(width int, height int) menuLayout {
	const (
		gap            = 2
		minLeftWidth   = 40
		minRightWidth  = 20
		stackThreshold = 90
		minPanelHeight = 8
		minListHeight  = 5
		leftListExtraH = 4
	)

	bodyHeight := height - 8
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	stacked := width < stackThreshold
	if !stacked {
		maxLeftWidth := width - minRightWidth - gap
		if maxLeftWidth < minLeftWidth {
			stacked = true
		}
	}

	if stacked {
		leftHeight := (bodyHeight * 3) / 5
		if leftHeight < minPanelHeight {
			leftHeight = minPanelHeight
		}
		rightHeight := bodyHeight - leftHeight
		if rightHeight < minPanelHeight {
			rightHeight = minPanelHeight
			leftHeight = bodyHeight - rightHeight
			if leftHeight < minPanelHeight {
				leftHeight = minPanelHeight
			}
		}

		listWidth := width - 6
		if listWidth < 4 {
			listWidth = 4
		}
		listHeight := leftHeight - leftListExtraH
		if listHeight < minListHeight {
			listHeight = minListHeight
		}

		return menuLayout{
			stacked:     true,
			leftWidth:   max(1, width),
			rightWidth:  max(1, width),
			leftHeight:  leftHeight,
			rightHeight: rightHeight,
			listWidth:   listWidth,
			listHeight:  listHeight,
		}
	}

	maxLeftWidth := width - minRightWidth - gap
	leftWidth := int(float64(width) * 0.62)
	if leftWidth < minLeftWidth {
		leftWidth = minLeftWidth
	}
	if leftWidth > maxLeftWidth {
		leftWidth = maxLeftWidth
	}
	rightWidth := width - leftWidth - gap

	listWidth := leftWidth - 5
	if listWidth < 4 {
		listWidth = 4
	}
	listHeight := bodyHeight - 6
	if listHeight < minListHeight {
		listHeight = minListHeight
	}

	return menuLayout{
		stacked:     false,
		leftWidth:   leftWidth,
		rightWidth:  rightWidth,
		leftHeight:  bodyHeight,
		rightHeight: bodyHeight,
		listWidth:   listWidth,
		listHeight:  listHeight,
	}
}

func menuInfoLine(label string, value string, width int) string {
	if strings.TrimSpace(value) == "" {
		value = "unknown"
	}
	line := fmt.Sprintf("%-7s %s", strings.ToLower(label)+":", value)
	return MutedStyle.Render(ansi.Truncate(line, max(10, width), "..."))
}

func resolveSystemVersions() (cliVersion string, galenaVersion string) {
	cliVersion = "dev"
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			cliVersion = info.Main.Version
		} else {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" && setting.Value != "" {
					revision := setting.Value
					if len(revision) > 7 {
						revision = revision[:7]
					}
					cliVersion = "dev-" + revision
					break
				}
			}
		}
	}

	osRelease := readOSRelease()
	galenaVersion = firstNonEmpty(
		osRelease["IMAGE_VERSION"],
		osRelease["VERSION_ID"],
		"unknown",
	)

	return cliVersion, galenaVersion
}

func readOSRelease() map[string]string {
	files := []string{"/etc/os-release", "/usr/lib/os-release"}
	values := map[string]string{}

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			if key != "" && value != "" {
				values[key] = value
			}
		}
	}

	return values
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
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
