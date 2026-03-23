package main

import (
	"flag"
	"fmt"
)

var configInitForce bool

func configShowPatternValue() string {
	pattern, err := resolveWorktreePattern()
	if err == nil {
		return pattern
	}

	return "(none)"
}

func runConfigInit(_ []string) error {
	path := resolveConfigPath(configFlag)
	if err := writeDefaultConfig(path, configInitForce); err != nil {
		return err
	}
	if isJSONOutput() {
		return emitJSONSuccess("config", map[string]string{"path": path, "status": "created"})
	}
	fmt.Printf("Created config file: %s\n", path)
	return nil
}

func runConfigShow(_ []string) error {
	pattern := configShowPatternValue()

	configStatus := "not found"
	if appCfg.ConfigFileFound {
		configStatus = "found"
	}

	if isJSONOutput() {
		return emitJSONSuccess("config", map[string]any{
			"config_file": map[string]string{
				"path":   appCfg.ConfigFilePath,
				"status": configStatus,
			},
			"effective": map[string]any{
				"root":      map[string]string{"value": appCfg.Root, "source": appCfg.ConfigSources.Root},
				"strategy":  map[string]string{"value": appCfg.Strategy, "source": appCfg.ConfigSources.Strategy},
				"pattern":   map[string]string{"value": pattern, "source": appCfg.ConfigSources.Pattern},
				"separator": map[string]string{"value": appCfg.Separator, "source": appCfg.ConfigSources.Separator},
			},
		})
	}

	fmt.Printf("Config file: %s (%s)\n\n", appCfg.ConfigFilePath, configStatus)
	fmt.Printf("Effective configuration:\n")
	fmt.Printf("  %-10s = %-40s (%s)\n", "root", appCfg.Root, appCfg.ConfigSources.Root)
	fmt.Printf("  %-10s = %-40s (%s)\n", "strategy", appCfg.Strategy, appCfg.ConfigSources.Strategy)
	fmt.Printf("  %-10s = %-40s (%s)\n", "pattern", pattern, appCfg.ConfigSources.Pattern)
	fmt.Printf("  %-10s = %-40s (%s)\n", "separator", fmt.Sprintf("%q", appCfg.Separator), appCfg.ConfigSources.Separator)
	return nil
}

func runConfigPath(_ []string) error {
	if isJSONOutput() {
		return emitJSONSuccess("config", map[string]string{"path": resolveConfigPath(configFlag)})
	}
	fmt.Println(resolveConfigPath(configFlag))
	return nil
}

func init() {
	configFlagSet := flag.NewFlagSet("config", flag.ContinueOnError)

	registerCommand(&command{
		name:  "config",
		short: "Manage wt configuration",
		usage: "<init|show|path>",
		flags: configFlagSet,
		run: func(args []string) error {
			if len(args) == 0 {
				cmd, _ := lookupCommand("config")
				printCommandHelp(cmd)
				return nil
			}

			subcmd := args[0]
			subArgs := args[1:]

			switch subcmd {
			case "init":
				// Parse init-specific flags
				initFlagSet := flag.NewFlagSet("config init", flag.ContinueOnError)
				initFlagSet.BoolVar(&configInitForce, "force", false, "Overwrite existing config file")
				initFlagSet.BoolVar(&configInitForce, "f", false, "Overwrite existing config file")
				if err := initFlagSet.Parse(subArgs); err != nil {
					return err
				}
				return runConfigInit(initFlagSet.Args())
			case "show":
				return runConfigShow(subArgs)
			case "path":
				return runConfigPath(subArgs)
			default:
				return fmt.Errorf("unknown config subcommand: %s", subcmd)
			}
		},
	})
}
