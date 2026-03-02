package ui

// Preferences controls runtime UI settings.
type Preferences struct {
	Theme      string
	ShowBanner bool
	Dense      bool
	NoColor    bool
	Advanced   bool
}

// CurrentPreferences holds the active UI preferences.
var CurrentPreferences = Preferences{
	Theme:      defaultThemeName,
	ShowBanner: false,
	Dense:      false,
	NoColor:    false,
	Advanced:   false,
}

// ApplyPreferences updates UI preferences and active palette.
func ApplyPreferences(p Preferences) {
	p.Theme = defaultThemeName
	p.ShowBanner = false
	CurrentPreferences = p
	ApplyTheme(defaultThemeName, p.NoColor)
}

// ApplyTheme switches the color palette for the TUI.
func ApplyTheme(theme string, noColor bool) {
	palette := PaletteByName(theme)
	palette.Disabled = noColor
	ApplyPalette(palette)
}
