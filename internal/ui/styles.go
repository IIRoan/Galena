// Package ui provides Charm-based UI components for finctl
package ui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Color palette
	Primary   = lipgloss.Color("#7C3AED") // Purple
	Secondary = lipgloss.Color("#06B6D4") // Cyan
	Success   = lipgloss.Color("#10B981") // Green
	Warning   = lipgloss.Color("#F59E0B") // Amber
	Error     = lipgloss.Color("#EF4444") // Red
	Muted     = lipgloss.Color("#6B7280") // Gray

	// Text styles
	Bold = lipgloss.NewStyle().Bold(true)

	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary).
		MarginBottom(1)

	Subtitle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

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

	// Box styles
	InfoBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Padding(0, 1).
		MarginTop(1).
		MarginBottom(1)

	SuccessBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Success).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	ErrorBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Error).
			Padding(0, 1).
			MarginTop(1).
			MarginBottom(1)

	// Table styles
	TableHeader = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Muted)

	TableCell = lipgloss.NewStyle().
			Padding(0, 1)

	// Status indicators
	StatusRunning = lipgloss.NewStyle().
			Foreground(Secondary).
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
)

// Banner returns the finctl ASCII banner
func Banner() string {
	banner := `
 _____ _            _   _
|  ___(_)_ __   ___| |_| |
| |_  | | '_ \ / __| __| |
|  _| | | | | | (__| |_| |
|_|   |_|_| |_|\___|\__|_|
`
	return lipgloss.NewStyle().
		Foreground(Primary).
		Bold(true).
		Render(banner)
}

// FormatStep formats a build step with status
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
	s := ""
	for i := 0; i < width; i++ {
		s += " "
	}
	ns := s + string(rune('0'+n%10))
	if n >= 10 {
		ns = string(rune('0'+n/10)) + string(rune('0'+n%10))
	} else {
		ns = " " + string(rune('0'+n))
	}
	return ns
}
