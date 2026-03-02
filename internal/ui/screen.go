package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

func StartScreen(title string, subtitle string) {
	ClearScreen()
	fmt.Println(Header(title))
	if subtitle != "" {
		fmt.Println(Tagline.Render(subtitle))
	}
	if !CurrentPreferences.Dense {
		fmt.Println()
	}
}

func ClearScreen() {
	if !IsInteractiveTerminal() {
		return
	}
	fmt.Print("\033[2J\033[H")
}

func IsInteractiveTerminal() bool {
	if os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "" {
		return false
	}
	if os.Getenv("TERM") == "" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

// Frame renders a full-screen TUI layout.
func Frame(title string, subtitle string, body string, footer string) string {
	parts := make([]string, 0, 5)
	parts = append(parts, Header(title))
	if subtitle != "" {
		parts = append(parts, Tagline.Render(subtitle))
	}
	parts = append(parts, body)
	if footer != "" {
		parts = append(parts, footer)
	}
	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}
