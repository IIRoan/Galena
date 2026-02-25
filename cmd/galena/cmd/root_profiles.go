package cmd

type cliProfile string

const (
	cliProfileManagement cliProfile = "management"
	cliProfileBuild      cliProfile = "build"
)

var activeProfile = cliProfileManagement

func configureRootForProfile(profile cliProfile) {
	activeProfile = profile
	rootCmd.ResetCommands()

	switch profile {
	case cliProfileBuild:
		rootCmd.Use = "galena-build"
		rootCmd.Short = "Build and develop OCI-native OS images"
		rootCmd.Long = "galena-build is a developer CLI for building, validating, and shipping\n" +
			"OCI-native bootable operating system images."
		addBuildCommands()
	default:
		rootCmd.Use = "galena"
		rootCmd.Short = "Manage a Galena device"
		rootCmd.Long = "galena is a device management CLI for day-to-day system operations,\n" +
			"application management, and first-boot workflows."
		addManagementCommands()
	}
}

func addBuildCommands() {
	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(diskCmd)
	rootCmd.AddCommand(vmCmd)
	rootCmd.AddCommand(pushCmd)
	rootCmd.AddCommand(signCmd)
	rootCmd.AddCommand(sbomCmd)
	rootCmd.AddCommand(cliCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(lintCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(settingsCmd)
	rootCmd.AddCommand(ciCmd)
}

func addManagementCommands() {
	rootCmd.AddCommand(appsCmd)
	rootCmd.AddCommand(manageStatusCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(ujustCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(manageBuildToolsCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(settingsCmd)
}
