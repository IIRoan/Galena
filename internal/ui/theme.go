package ui

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// HuhTheme returns the active form theme built from the palette.
func HuhTheme() *huh.Theme {
	t := huh.ThemeBase()

	t.Focused.Base = t.Focused.Base.BorderForeground(Border)
	t.Focused.Title = t.Focused.Title.Foreground(Highlight).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(Highlight).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(Muted)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(Error)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(Error)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(Accent)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(Accent)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(Accent)
	t.Focused.Option = t.Focused.Option.Foreground(Foreground)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(Accent)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(Accent)
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(Accent)
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(Foreground)
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(Muted)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(Background).Background(Primary).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(Foreground).Background(lipgloss.Color(""))

	t.Focused.TextInput.Cursor = t.Focused.TextInput.Cursor.Foreground(Info)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(Muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(Accent)

	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	return t
}
