package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configInitForce bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage wt configuration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printCommandHelp(cmd)
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		path := resolveConfigPath(configFlag)
		if err := writeDefaultConfig(path, configInitForce); err != nil {
			return err
		}
		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]string{"path": path, "status": "created"})
		}
		fmt.Printf("Created config file: %s\n", path)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration with sources",
	RunE: func(cmd *cobra.Command, _ []string) error {
		pattern := configShowPatternValue()

		configStatus := "not found"
		if appCfg.ConfigFileFound {
			configStatus = "found"
		}

		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]any{
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
	},
}

func configShowPatternValue() string {
	pattern, err := resolveWorktreePattern()
	if err == nil {
		return pattern
	}

	return "(none)"
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the config file path",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]string{"path": resolveConfigPath(configFlag)})
		}
		fmt.Println(resolveConfigPath(configFlag))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)
}
