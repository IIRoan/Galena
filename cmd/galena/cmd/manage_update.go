package cmd

import (
	"context"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	galexec "github.com/iiroan/galena/internal/exec"
	"github.com/iiroan/galena/internal/ui"
)

var (
	updateReboot bool
	updateYes    bool
	updateCheck  bool
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update the system with bootc",
	Long: `Run a system update using bootc. This stages the latest image and,
optionally, reboots the machine to apply it.`,
	RunE: runSystemUpdate,
}

func init() {
	updateCmd.Flags().BoolVar(&updateReboot, "reboot", false, "Reboot automatically after a successful upgrade")
	updateCmd.Flags().BoolVarP(&updateYes, "yes", "y", false, "Skip confirmation prompt")
	updateCmd.Flags().BoolVar(&updateCheck, "check", false, "Show current bootc status without upgrading")
}

func runSystemUpdate(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	if err := galexec.RequireCommands("bootc"); err != nil {
		return fmt.Errorf("bootc is required for updates: %w", err)
	}

	ui.StartScreen("SYSTEM UPDATE", "Manage OS updates with bootc")

	if updateCheck {
		result := galexec.RunStreaming(ctx, "bootc", []string{"status"}, galexec.DefaultOptions())
		if result.Err != nil {
			return fmt.Errorf("bootc status failed: %w", result.Err)
		}
		return nil
	}

	if !updateYes {
		confirm := false
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("Run system update now?").
					Description("This will run bootc upgrade and stage the next deployment.").
					Value(&confirm),
			),
		).WithTheme(ui.HuhTheme()).Run()
		if err != nil {
			return err
		}
		if !confirm {
			fmt.Println(ui.InfoBox.Render("Update canceled."))
			return nil
		}
	}

	updateName, updateArgs := commandWithPrivilege("bootc", "upgrade")
	result := galexec.RunStreaming(ctx, updateName, updateArgs, galexec.DefaultOptions())
	if result.Err != nil {
		return fmt.Errorf("bootc upgrade failed: %w", result.Err)
	}

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render("System update staged successfully."))

	if !updateReboot {
		fmt.Println(ui.InfoBox.Render("Reboot to apply the updated deployment."))
		return nil
	}

	rebootName, rebootArgs := commandWithPrivilege("systemctl", "reboot")
	reboot := galexec.RunStreaming(ctx, rebootName, rebootArgs, galexec.DefaultOptions())
	if reboot.Err != nil {
		return fmt.Errorf("reboot command failed: %w", reboot.Err)
	}

	return nil
}
