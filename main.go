// Package main implements wt, a git worktree management CLI
// with organized directory structure and multi-strategy support.
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time via -ldflags.
var version = "dev"

func init() {
	// Step 1: Load config from file/env (must happen before buildRootCmdLong).
	loadWorktreeConfig()
	rootCmd.Long = buildRootCmdLong()

	// Step 2: Register persistent flags.
	rootCmd.PersistentFlags().StringVar(&configFlag, "config", "", "Path to config file (default: ~/.config/wt/config.toml)")
	rootCmd.PersistentFlags().StringVar(&appCfg.OutputFormat, "format", formatText, "Output format: text or json")

	// Step 3: Custom help function that supports JSON output.
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if !isJSONOutput() {
			defaultHelp(cmd, args)
			return
		}

		buf := bytes.NewBuffer(nil)
		origOut := cmd.OutOrStdout()
		origErr := cmd.ErrOrStderr()
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		defaultHelp(cmd, args)
		cmd.SetOut(origOut)
		cmd.SetErr(origErr)

		_ = emitJSONSuccess(cmd, map[string]any{"help": buf.String()})
	})

	// Step 4: Register all commands.
	rootCmd.AddCommand(checkoutCmd)
	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(prCmd)
	rootCmd.AddCommand(mrCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(pruneCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(migrateCmd)
	rootCmd.AddCommand(shellenvCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(examplesCmd)
	rootCmd.AddCommand(doneCmd)

	// Step 5: Register command-specific flags.
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal even if worktree has modifications")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Preview what would be removed without making changes")
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Remove all merged worktrees without confirmation")
	migrateCmd.Flags().BoolVarP(&migrateForce, "force", "f", false, "Force migration when target path exists and is non-empty")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Preview changes without modifying files")
	initCmd.Flags().BoolVar(&initUninstall, "uninstall", false, "Remove wt configuration from shell")
	initCmd.Flags().BoolVar(&initNoPrompt, "no-prompt", false, "Skip activation instructions (for automated installs)")
	configInitCmd.Flags().BoolVar(&configInitForce, "force", false, "Overwrite existing config file")
}

func main() {
	// Re-load config after cobra parses flags so --config is available
	rootCmd.PersistentPreRunE = func(_ *cobra.Command, _ []string) error {
		if err := validateOutputFormat(); err != nil {
			return err
		}
		if configFlag != "" {
			loadWorktreeConfig()
			rootCmd.Long = buildRootCmdLong()
		}
		return nil
	}
	if err := rootCmd.Execute(); err != nil {
		if isJSONOutput() {
			_ = emitJSONError(rootCmd, err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// rootCmd is the top-level cobra command for the wt CLI.
var rootCmd = &cobra.Command{
	Use:           "wt",
	Short:         "Git worktree helper with organized directory structure",
	Long:          "",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return printCommandHelp(cmd)
	},
}

func printCommandHelp(cmd *cobra.Command) error {
	return cmd.Help()
}

func buildRootCmdLong() string {
	pattern, err := resolveWorktreePattern()
	if err != nil {
		pattern = appCfg.Pattern
		if pattern == "" {
			pattern = "unknown"
		}
	}

	return fmt.Sprintf(`Git-like worktree management with organized directory structure.

Strategy: %s
Pattern:  %s
Root:     %s

Run 'wt info' to see available strategies and pattern variables.
Set WORKTREE_ROOT, WORKTREE_STRATEGY, and WORKTREE_PATTERN to customize.`,
		appCfg.Strategy,
		pattern,
		appCfg.Root,
	)
}
