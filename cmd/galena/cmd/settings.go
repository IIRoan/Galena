package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iiroan/galena/internal/config"
	"github.com/iiroan/galena/internal/ui"
)

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Configure UI preferences and build defaults",
	RunE:  runSettings,
}

func runSettings(cmd *cobra.Command, args []string) error {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	dense := cfg.UI.Dense
	noColorPref := cfg.UI.NoColor
	advancedMode := cfg.UI.Advanced

	variant := cfg.Build.Defaults.Variant
	if variant == "" {
		variant = "main"
	}
	tag := cfg.Build.Defaults.Tag
	if tag == "" {
		tag = "latest"
	}
	buildNumberInput := strconv.Itoa(cfg.Build.Defaults.BuildNumber)

	push := cfg.Build.Defaults.Push
	sign := cfg.Build.Defaults.Sign
	sbom := cfg.Build.Defaults.SBOM
	noCache := cfg.Build.Defaults.NoCache
	rechunk := cfg.Build.Defaults.Rechunk
	dryRun := cfg.Build.Defaults.DryRun
	useJust := cfg.Build.Defaults.UseJust

	buildArgsInput := formatKeyValuePairs(cfg.Build.BuildArgs)
	timeoutInput := cfg.Build.Timeout

	variantOptions := make([]huh.Option[string], 0)
	for _, name := range cfg.ListVariantNames() {
		variantOptions = append(variantOptions, huh.NewOption(name, name))
	}
	if len(variantOptions) == 0 {
		variantOptions = append(variantOptions, huh.NewOption("main", "main"))
	}

	var save bool
	changedUI := false
	changedDefaults := false
	changedFlags := false
	changedAdvanced := false

	ui.StartScreen("SETTINGS", "Select a settings section to edit")

	for {
		choice, err := ui.RunMenuWithOptions("SETTINGS", "Select a settings section", []ui.MenuItem{
			{ID: "ui", TitleText: "Display & Prompts", Details: "Layout density, color mode, and advanced prompts"},
			{ID: "defaults", TitleText: "Build Defaults", Details: "Default variant, tag, and build number"},
			{ID: "flags", TitleText: "Build Flags", Details: "Push/sign/SBOM defaults and cache behavior"},
			{ID: "advanced", TitleText: "Advanced Build Options", Details: "Build args and timeouts"},
			{ID: "save", TitleText: "Save & Exit", Details: "Write updates to galena.yaml"},
			{ID: "exit", TitleText: "Exit", Details: "Leave without saving"},
		}, ui.WithBackNavigation("Back"))
		if err != nil {
			return err
		}

		switch choice {
		case ui.MenuActionBack, ui.MenuActionQuit:
			return nil
		case "ui":
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Dense Layout").
						Description("Reduce vertical spacing in the TUI").
						Value(&dense),
					huh.NewConfirm().
						Title("Disable Colors").
						Description("Use monochrome output").
						Value(&noColorPref),
					huh.NewConfirm().
						Title("Advanced Build Prompts").
						Description("Show advanced options in build and disk wizards").
						Value(&advancedMode),
				),
			).WithTheme(ui.HuhTheme())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return err
			}
			changedUI = true
		case "defaults":
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewSelect[string]().
						Title("Default Variant").
						Description("Select the default build variant").
						Options(variantOptions...).
						Value(&variant),
					huh.NewInput().
						Title("Default Tag").
						Description("Container tag used for builds").
						Value(&tag),
					huh.NewInput().
						Title("Default Build Number").
						Description("Used for versioning schemes").
						Value(&buildNumberInput).
						Validate(func(value string) error {
							if value == "" {
								return nil
							}
							_, err := strconv.Atoi(value)
							if err != nil {
								return fmt.Errorf("enter a valid integer")
							}
							return nil
						}),
				),
			).WithTheme(ui.HuhTheme())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return err
			}
			changedDefaults = true
		case "flags":
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewConfirm().
						Title("Push by Default").
						Description("Push images after a successful build").
						Value(&push),
					huh.NewConfirm().
						Title("Sign by Default").
						Description("Sign images with cosign").
						Value(&sign),
					huh.NewConfirm().
						Title("Generate SBOM by Default").
						Description("Create an SBOM after build").
						Value(&sbom),
					huh.NewConfirm().
						Title("No Cache by Default").
						Description("Disable build cache for podman builds").
						Value(&noCache),
					huh.NewConfirm().
						Title("Rechunk by Default").
						Description("Optimize image chunks after build").
						Value(&rechunk),
					huh.NewConfirm().
						Title("Dry Run by Default").
						Description("Skip actual build execution").
						Value(&dryRun),
					huh.NewConfirm().
						Title("Use Justfile by Default").
						Description("Route builds through existing Justfile recipes").
						Value(&useJust),
				),
			).WithTheme(ui.HuhTheme())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return err
			}
			changedFlags = true
		case "advanced":
			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Build Args").
						Description("Comma-separated KEY=VALUE pairs").
						Placeholder("FEATURE_FLAG=on, CACHE=false").
						Value(&buildArgsInput),
					huh.NewInput().
						Title("Build Timeout").
						Description("Duration (e.g. 45m, 2h)").
						Placeholder("30m").
						Value(&timeoutInput).
						Validate(func(value string) error {
							if value == "" {
								return nil
							}
							_, err := time.ParseDuration(value)
							if err != nil {
								return fmt.Errorf("invalid duration")
							}
							return nil
						}),
				),
			).WithTheme(ui.HuhTheme())

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					continue
				}
				return err
			}
			changedAdvanced = true
		case "save":
			save = true
			goto Save
		case "exit":
			return nil
		default:
			return nil
		}
	}

Save:
	if !save {
		return nil
	}

	if !changedUI && !changedDefaults && !changedFlags && !changedAdvanced {
		return nil
	}

	buildNumber := 0
	if changedDefaults || changedFlags {
		if buildNumberInput != "" {
			parsed, err := strconv.Atoi(buildNumberInput)
			if err != nil {
				return fmt.Errorf("invalid build number: %w", err)
			}
			buildNumber = parsed
		}
	}

	buildArgs := map[string]string{}
	if changedAdvanced {
		parsedArgs, err := parseKeyValueCSV(buildArgsInput)
		if err != nil {
			return err
		}
		buildArgs = parsedArgs
	}

	if changedUI {
		cfg.UI = config.UIConfig{
			Theme:      "space",
			ShowBanner: false,
			Dense:      dense,
			NoColor:    noColorPref,
			Advanced:   advancedMode,
		}
	}

	if changedDefaults || changedFlags {
		cfg.Build.Defaults = config.BuildDefaults{
			Variant:     variant,
			Tag:         tag,
			BuildNumber: buildNumber,
			NoCache:     noCache,
			Push:        push,
			Sign:        sign,
			SBOM:        sbom,
			Rechunk:     rechunk,
			DryRun:      dryRun,
			UseJust:     useJust,
		}
	}

	if changedAdvanced {
		cfg.Build.BuildArgs = buildArgs
		cfg.Build.Timeout = timeoutInput
	}

	path := cfgFile
	if path == "" {
		var err error
		path, err = config.GetConfigPath()
		if err != nil {
			return err
		}
	}

	if err := cfg.Save(path); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	ui.ApplyPreferences(ui.Preferences{
		Theme:      "space",
		ShowBanner: false,
		Dense:      cfg.UI.Dense,
		NoColor:    cfg.UI.NoColor || noColor,
		Advanced:   cfg.UI.Advanced,
	})

	fmt.Println()
	fmt.Println(ui.SuccessBox.Render("Settings saved to " + path))
	return nil
}
