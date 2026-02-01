package ui

import (
	"strings"

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

const defaultThemeName = "aurora"

// ThemeNames returns supported palette names.
func ThemeNames() []string {
	return []string{"aurora", "ember", "mono", "galena"}
}

// PaletteByName returns a palette by theme name.
func PaletteByName(name string) Palette {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ember":
		return Palette{
			Name:       "ember",
			Primary:    lipgloss.Color("#F97316"),
			Secondary:  lipgloss.Color("#F43F5E"),
			Accent:     lipgloss.Color("#FACC15"),
			Info:       lipgloss.Color("#38BDF8"),
			Success:    lipgloss.Color("#22C55E"),
			Warning:    lipgloss.Color("#F59E0B"),
			Error:      lipgloss.Color("#EF4444"),
			Muted:      lipgloss.Color("#94A3B8"),
			Background: lipgloss.Color("#0F172A"),
			Foreground: lipgloss.Color("#E2E8F0"),
			Border:     lipgloss.Color("#475569"),
			Highlight:  lipgloss.Color("#FDBA74"),
		}
	case "mono":
		return Palette{
			Name:       "mono",
			Primary:    lipgloss.Color("#E2E8F0"),
			Secondary:  lipgloss.Color("#CBD5F5"),
			Accent:     lipgloss.Color("#94A3B8"),
			Info:       lipgloss.Color("#E2E8F0"),
			Success:    lipgloss.Color("#E2E8F0"),
			Warning:    lipgloss.Color("#94A3B8"),
			Error:      lipgloss.Color("#CBD5F5"),
			Muted:      lipgloss.Color("#94A3B8"),
			Background: lipgloss.Color("#0B1220"),
			Foreground: lipgloss.Color("#E2E8F0"),
			Border:     lipgloss.Color("#64748B"),
			Highlight:  lipgloss.Color("#F8FAFC"),
		}
	case "galena":
		return Palette{
			Name:       "galena",
			Primary:    lipgloss.Color("#FFD24A"),
			Secondary:  lipgloss.Color("#FFB347"),
			Accent:     lipgloss.Color("#FFE08A"),
			Info:       lipgloss.Color("#7FB8FF"),
			Success:    lipgloss.Color("#8BE6B1"),
			Warning:    lipgloss.Color("#FFC857"),
			Error:      lipgloss.Color("#F27D72"),
			Muted:      lipgloss.Color("#A39A86"),
			Background: lipgloss.Color("#0B0C0E"),
			Foreground: lipgloss.Color("#F9F4E6"),
			Border:     lipgloss.Color("#5B4C2B"),
			Highlight:  lipgloss.Color("#FFF1B6"),
		}
	default:
		return Palette{
			Name:       "aurora",
			Primary:    lipgloss.Color("#22D3EE"),
			Secondary:  lipgloss.Color("#A78BFA"),
			Accent:     lipgloss.Color("#38BDF8"),
			Info:       lipgloss.Color("#60A5FA"),
			Success:    lipgloss.Color("#34D399"),
			Warning:    lipgloss.Color("#FBBF24"),
			Error:      lipgloss.Color("#F87171"),
			Muted:      lipgloss.Color("#94A3B8"),
			Background: lipgloss.Color("#0B1120"),
			Foreground: lipgloss.Color("#E2E8F0"),
			Border:     lipgloss.Color("#334155"),
			Highlight:  lipgloss.Color("#7DD3FC"),
		}
	}
}

// DefaultPalette returns the default theme palette.
func DefaultPalette() Palette {
	return PaletteByName(defaultThemeName)
}
