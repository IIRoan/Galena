package cmd

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/huh"
)

// newHuhBackOnQKeyMap keeps default Huh bindings and adds q as a quit/back key.
func newHuhBackOnQKeyMap() *huh.KeyMap {
	keyMap := huh.NewDefaultKeyMap()
	keyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c", "q"),
		key.WithHelp("q", "back"),
	)
	return keyMap
}
