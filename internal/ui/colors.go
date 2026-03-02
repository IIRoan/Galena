package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Palette defines the TUI color palette.
type Palette struct {
	Name       string
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
	Disabled   bool
}

const defaultThemeName = "space"

// ThemeNames returns supported palette names.
func ThemeNames() []string {
	return []string{defaultThemeName}
}

// PaletteByName returns a palette by theme name.
func PaletteByName(_ string) Palette {
	return Palette{
		Name:       defaultThemeName,
		Primary:    lipgloss.Color("#7C9BFF"),
		Secondary:  lipgloss.Color("#93B2FF"),
		Accent:     lipgloss.Color("#6DE6FF"),
		Info:       lipgloss.Color("#6AA1FF"),
		Success:    lipgloss.Color("#64E8C2"),
		Warning:    lipgloss.Color("#FFD57A"),
		Error:      lipgloss.Color("#FF8A9B"),
		Muted:      lipgloss.Color("#8A94B8"),
		Background: lipgloss.Color("#060B17"),
		Foreground: lipgloss.Color("#E8EEFF"),
		Border:     lipgloss.Color("#26385F"),
		Highlight:  lipgloss.Color("#9BC7FF"),
	}
}

// DefaultPalette returns the default theme palette.
func DefaultPalette() Palette {
	return PaletteByName(defaultThemeName)
}
