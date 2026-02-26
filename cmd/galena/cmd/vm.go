package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/build"
)

var (
	vmImage   string
	vmMemory  string
	vmCPUs    int
	vmDisplay string
	vmSSHPort int
	vmNoKVM   bool
	vmNoBIOS  bool
	vmUseJust bool
)

var vmCmd = &cobra.Command{
	Use:   "vm <command>",
	Short: "Manage virtual machines for testing",
	Long: `Run and manage virtual machines for testing disk images.

Subcommands:
  run    - Start a VM with a disk image
  ssh    - Connect to a running VM via SSH

Examples:
  # Run a VM with the most recent disk image
  galena-build vm run

  # Run a VM with a specific image
  galena-build vm run --image ./output/disk.qcow2

  # Run with custom resources
  galena-build vm run --memory 8G --cpus 4

  # Connect to VM via SSH
  galena-build vm ssh`,
}

var vmRunCmd = &cobra.Command{
	Use:   "run [image]",
	Short: "Start a VM with a disk image",
	Long: `Start a virtual machine using QEMU with the specified disk image.

If no image is specified, it will look for the most recent disk image
in the output directory.

Examples:
  galena-build vm run
  galena-build vm run ./output/disk.qcow2
  galena-build vm run --memory 8G --cpus 4
  galena-build vm run --display vnc`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVMRun,
}

var vmSSHCmd = &cobra.Command{
	Use:   "ssh [user]",
	Short: "Connect to a running VM via SSH",
	Long: `Connect to a running VM via SSH.

By default, connects to localhost on port 2222 with the user 'galena'.

Examples:
  galena-build vm ssh
  galena-build vm ssh root
  galena-build vm ssh --port 2223`,
	Args: cobra.MaximumNArgs(1),
	RunE: runVMSSH,
}

func init() {
	vmCmd.AddCommand(vmRunCmd)
	vmCmd.AddCommand(vmSSHCmd)

	// vm run flags
	vmRunCmd.Flags().StringVar(&vmImage, "image", "", "Disk image path (default: auto-detect)")
	vmRunCmd.Flags().StringVarP(&vmMemory, "memory", "m", "4G", "VM memory (e.g., 4G, 8192M)")
	vmRunCmd.Flags().IntVarP(&vmCPUs, "cpus", "c", 2, "Number of CPUs")
	vmRunCmd.Flags().StringVar(&vmDisplay, "display", "gtk", "Display type (gtk, sdl, vnc, none)")
	vmRunCmd.Flags().IntVar(&vmSSHPort, "ssh-port", 2222, "SSH port forwarding")
	vmRunCmd.Flags().BoolVar(&vmNoKVM, "no-kvm", false, "Disable KVM acceleration")
	vmRunCmd.Flags().BoolVar(&vmNoBIOS, "no-uefi", false, "Use legacy BIOS instead of UEFI")
	vmRunCmd.Flags().BoolVar(&vmUseJust, "just", false, "Use existing Justfile recipes")

	// vm ssh flags
	vmSSHCmd.Flags().IntVar(&vmSSHPort, "port", 2222, "SSH port")
}

func runVMRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	vmRunner := build.NewVMRunner(cfg, rootDir, logger)

	// Use just if requested
	if vmUseJust {
		image := "galena"
		if len(args) > 0 {
			image = args[0]
		}
		return vmRunner.RunViaJust(ctx, image)
	}

	// Find image path
	imagePath := vmImage
	if imagePath == "" && len(args) > 0 {
		imagePath = args[0]
	}
	if imagePath == "" {
		var err error
		imagePath, err = vmRunner.FindDiskImage("")
		if err != nil {
			return fmt.Errorf("no disk image found: %w\nRun 'galena-build disk qcow2' first to create one", err)
		}
		logger.Info("auto-detected disk image", "path", imagePath)
	}

	opts := build.VMOptions{
		ImagePath: imagePath,
		Memory:    vmMemory,
		CPUs:      vmCPUs,
		Display:   vmDisplay,
		SSH:       true,
		SSHPort:   vmSSHPort,
		KVM:       !vmNoKVM,
		UEFI:      !vmNoBIOS,
	}

	return vmRunner.Run(ctx, opts)
}

func runVMSSH(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	rootDir, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("finding project root: %w", err)
	}

	vmRunner := build.NewVMRunner(cfg, rootDir, logger)

	user := "galena"
	if len(args) > 0 {
		user = args[0]
	}

	return vmRunner.SSH(ctx, vmSSHPort, user)
}
