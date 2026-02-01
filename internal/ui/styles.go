package ui

import (
	_ "embed" // embed banner text
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

var (
	//go:embed banner.txt
	bannerText string

	// Colors holds the active palette.
	Colors Palette

	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Info       lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
	Muted      lipgloss.Color
	Background lipgloss.Color
	Foreground lipgloss.Color
	Border     lipgloss.Color
	Highlight  lipgloss.Color

	Title             lipgloss.Style
	Subtitle          lipgloss.Style
	Tagline           lipgloss.Style
	BrandStyle        lipgloss.Style
	HeaderStyle       lipgloss.Style
	HeaderFill        lipgloss.Style
	SuccessStyle      lipgloss.Style
	WarningStyle      lipgloss.Style
	ErrorStyle        lipgloss.Style
	MutedStyle        lipgloss.Style
	BannerStyle       lipgloss.Style
	InfoBox           lipgloss.Style
	SuccessBox        lipgloss.Style
	ErrorBox          lipgloss.Style
	Panel             lipgloss.Style
	PanelTitle        lipgloss.Style
	KeyStyle          lipgloss.Style
	HintStyle         lipgloss.Style
	TableHeader       lipgloss.Style
	TableCell         lipgloss.Style
	StatusRunning     lipgloss.Style
	StatusSuccess     lipgloss.Style
	StatusError       lipgloss.Style
	StatusPending     lipgloss.Style
	PromptTitle       lipgloss.Style
	PromptDescription lipgloss.Style
	WizardTitle       lipgloss.Style
	WizardStep        lipgloss.Style
	WizardDescription lipgloss.Style
)

func init() {
	ApplyPalette(DefaultPalette())
}

// ApplyPalette updates the active palette and rebuilds styles.
func ApplyPalette(p Palette) {
	Colors = p

	if p.Disabled {
		Primary = ""
		Secondary = ""
		Accent = ""
		Info = ""
		Success = ""
		Warning = ""
		Error = ""
		Muted = ""
		Background = ""
		Foreground = ""
		Border = ""
		Highlight = ""
	} else {
		Primary = p.Primary
		Secondary = p.Secondary
		Accent = p.Accent
		Info = p.Info
		Success = p.Success
		Warning = p.Warning
		Error = p.Error
		Muted = p.Muted
		Background = p.Background
		Foreground = p.Foreground
		Border = p.Border
		Highlight = p.Highlight
	}

	rebuildStyles()
}

func rebuildStyles() {
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Secondary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderLeft(true).
		BorderForeground(Border).
		PaddingLeft(1).
		MarginTop(1)

	Subtitle = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true)

	Tagline = lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true).
		MarginBottom(1)

	BrandStyle = lipgloss.NewStyle().
		Foreground(Background).
		Background(Primary).
		Bold(true).
		Padding(0, 1)

	HeaderStyle = lipgloss.NewStyle().
		Foreground(Background).
		Background(Secondary).
		Bold(true).
		Padding(0, 1)

	HeaderFill = lipgloss.NewStyle().
		Background(Secondary)

	SuccessStyle = lipgloss.NewStyle().
		Foreground(Success).
		Bold(true)

	WarningStyle = lipgloss.NewStyle().
		Foreground(Warning)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(Error).
		Bold(true)

	MutedStyle = lipgloss.NewStyle().
		Foreground(Muted)

	BannerStyle = lipgloss.NewStyle().
		Foreground(Highlight).
		Bold(true).
		MarginBottom(1)

	InfoBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Border).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	SuccessBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Success).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	ErrorBox = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(Error).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(1, 2)

	PanelTitle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	KeyStyle = lipgloss.NewStyle().
		Foreground(Accent).
		Bold(true)

	HintStyle = lipgloss.NewStyle().
		Foreground(Muted)

	TableHeader = lipgloss.NewStyle().
		Bold(true).
		Foreground(Secondary).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(Border)

	TableCell = lipgloss.NewStyle().
		Padding(0, 1)

	StatusRunning = lipgloss.NewStyle().
		Foreground(Info).
		SetString("●")

	StatusSuccess = lipgloss.NewStyle().
		Foreground(Success).
		SetString("✓")

	StatusError = lipgloss.NewStyle().
		Foreground(Error).
		SetString("✗")

	StatusPending = lipgloss.NewStyle().
		Foreground(Muted).
		SetString("○")

	PromptTitle = lipgloss.NewStyle().
		Foreground(Secondary).
		Bold(true).
		MarginBottom(1)

	PromptDescription = lipgloss.NewStyle().
		Foreground(Muted).
		Italic(true).
		MarginBottom(1)

	WizardTitle = lipgloss.NewStyle().
		Foreground(Background).
		Background(Accent).
		Padding(0, 2).
		Bold(true).
		MarginTop(1).
		MarginBottom(0)

	WizardStep = lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		MarginTop(1).
		MarginBottom(0)

	WizardDescription = lipgloss.NewStyle().
		Foreground(Muted).
		MarginTop(0).
		MarginBottom(0)
}

func Banner() string {
	text := strings.TrimRight(bannerText, "\n")
	width := contentWidth()
	bannerWidth := lipgloss.Width(text)
	if width >= bannerWidth {
		return BannerStyle.Width(width).Align(lipgloss.Center).Render(text)
	}
	return BannerStyle.Render(text)
}

func Header(title string) string {
	width := contentWidth()
	brand := BrandStyle.Render(" finctl ")
	section := HeaderStyle.Render(" " + strings.ToUpper(title) + " ")
	line := lipgloss.JoinHorizontal(lipgloss.Top, brand, section)
	lineWidth := lipgloss.Width(line)
	if width > lineWidth {
		line += HeaderFill.Render(strings.Repeat(" ", width-lineWidth))
	}
	return line
}

func contentWidth() int {
	width := terminalWidth()
	if width > 100 {
		return 100
	}
	return width
}

func terminalWidth() int {
	width, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil || width == 0 {
		return 80
	}
	return width
}

func FormatStep(step int, total int, name string, status string) string {
	stepNum := MutedStyle.Render("[" + padInt(step, 2) + "/" + padInt(total, 2) + "]")

	var statusIcon string
	switch status {
	case "running":
		statusIcon = StatusRunning.String()
	case "success":
		statusIcon = StatusSuccess.String()
	case "error":
		statusIcon = StatusError.String()
	default:
		statusIcon = StatusPending.String()
	}

	return stepNum + " " + statusIcon + " " + name
}

func padInt(n, width int) string {
	return fmt.Sprintf("%0*d", width, n)
}
