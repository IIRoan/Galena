package cmd

import (
	"errors"

	"github.com/charmbracelet/huh"

	"github.com/iiroan/galena/internal/ui"
)

func runManagementTUI() error {
	menuItems := []ui.MenuItem{
		{ID: "apps", TitleText: "Applications", Details: "Manage Homebrew and Flatpak installs from the Galena catalog"},
		{ID: "status", TitleText: "Device Status", Details: "Inspect setup markers, tool availability, and catalog coverage"},
		{ID: "update", TitleText: "System Update", Details: "Run bootc upgrade and optionally reboot"},
		{ID: "ujust", TitleText: "Bluefin Tasks", Details: "Browse and run ujust workflows from the shipped recipes"},
		{ID: "setup", TitleText: "Setup Wizard", Details: "Run the first-boot setup wizard manually"},
		{ID: "exit", TitleText: "Exit", Details: "Close the management console"},
	}
	lastChoice := ""

	for {
		choice, err := ui.RunMenuWithOptions(
			"GALENA MANAGEMENT",
			"Select what you want to manage on this device.",
			menuItems,
			ui.WithInitialSelectionID(lastChoice),
		)
		if err != nil {
			return runManagementFallback()
		}

		if choice == ui.MenuActionQuit || choice == "exit" || choice == "" {
			return nil
		}
		lastChoice = choice

		if err := runManagementChoice(choice); err != nil {
			if errors.Is(err, huh.ErrUserAborted) {
				continue
			}
			return err
		}

		if err := waitForEnter("Press enter to return to Galena Management"); err != nil {
			return err
		}
	}
}

func runManagementFallback() error {
	ui.StartScreen("MANAGEMENT MENU", "Select what you want to manage on this device.")
	var fallbackChoice string
	fallbackErr := huh.NewSelect[string]().
		Title("Galena Management").
		Description("What would you like to do?").
		Options(
			huh.NewOption("Applications", "apps"),
			huh.NewOption("Device Status", "status"),
			huh.NewOption("System Update", "update"),
			huh.NewOption("Bluefin Tasks", "ujust"),
			huh.NewOption("Setup Wizard", "setup"),
			huh.NewOption("Exit", "exit"),
		).
		Value(&fallbackChoice).
		WithTheme(ui.HuhTheme()).
		Run()
	if fallbackErr != nil {
		if errors.Is(fallbackErr, huh.ErrUserAborted) {
			return nil
		}
		return fallbackErr
	}
	return runManagementChoice(fallbackChoice)
}

func runManagementChoice(choice string) error {
	switch choice {
	case "apps":
		return appsCmd.RunE(appsCmd, []string{})
	case "status":
		return manageStatusCmd.RunE(manageStatusCmd, []string{})
	case "update":
		return updateCmd.RunE(updateCmd, []string{})
	case "ujust":
		return ujustCmd.RunE(ujustCmd, []string{})
	case "setup":
		return setupCmd.RunE(setupCmd, []string{})
	case "exit", ui.MenuActionQuit, ui.MenuActionBack, "":
		return nil
	default:
		return nil
	}
}
