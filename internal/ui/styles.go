// Package ui provides Charm-based UI components for finctl
package ui

import (
	"fmt"
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

	// Prompt styles
	PromptTitle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true).
			MarginBottom(1)

	PromptDescription = lipgloss.NewStyle().
				Foreground(Muted).
				Italic(true).
				MarginBottom(1)

	// Modern UI elements
	GradientPrimary = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	GradientSecondary = lipgloss.AdaptiveColor{Light: "#06B6D4", Dark: "#22D3EE"}

	WizardTitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Padding(0, 2).
			Bold(true).
			MarginTop(2).
			MarginBottom(0)

	WizardStep = lipgloss.NewStyle().
			Foreground(Secondary).
			Bold(true).
			MarginTop(1).
			MarginBottom(0)

	WizardDescription = lipgloss.NewStyle().
				Foreground(Muted).
				MarginTop(0).
				MarginBottom(0)

	HeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(Primary).
			Padding(0, 1).
			Bold(true).
			Width(60).
			Align(lipgloss.Center)

	AppStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary)
)

// Banner returns the finctl ASCII banner
func Banner() string {
	banner := `
  ▄▀▀▀▀▄   ▄▀▀█▄   ▄▀▀        ▄▀▀▀▀▄  ▄▀▀▄ █  ▄▀▀█▄  
 █      █ █  █ ▀▄ █   █      █      █ █  █ ▄ █  █ ▀▄ 
 █      █ █  █  █ ▐  █       █      █ █  █   █  █  █ 
 ▀▄    ▄▀ █   ▀ ▄    █   ▄   ▀▄    ▄▀ █  █   █   ▀ ▄ 
   ▀▀▀▀   ▀      ▀▀▀▀▀▀▀▀      ▀▀▀▀   ▀ ▀    ▀      `
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
	if n >= 10 {
		return fmt.Sprintf("%02d", n)
	}
	return fmt.Sprintf("%02d", n)
}
