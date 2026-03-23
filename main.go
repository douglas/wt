// Package main implements wt, a git worktree management CLI
// with organized directory structure and multi-strategy support.
package main

import (
	"fmt"
	"os"
	"strings"
)

// version is set at build time via -ldflags.
var version = "dev"

func main() {
	args := extractGlobalFlags(os.Args[1:])

	// Load config (may be overridden by --config flag)
	loadWorktreeConfig()
	if configFlag != "" {
		loadWorktreeConfig()
	}

	if err := validateOutputFormat(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// No command given — print help
	if len(args) == 0 {
		if isJSONOutput() {
			var help strings.Builder
			origStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			printUsage()
			w.Close()
			os.Stdout = origStdout
			buf := make([]byte, 64*1024)
			n, _ := r.Read(buf)
			help.Write(buf[:n])
			_ = emitJSONSuccess("", map[string]any{"help": help.String()})
			return
		}
		printUsage()
		return
	}

	// Handle --help / -h / help as first arg
	cmdName := args[0]
	if cmdName == "--help" || cmdName == "-h" || cmdName == "help" { //nolint:nestif // help dispatch logic is inherently branchy
		if isJSONOutput() {
			var help strings.Builder
			origStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w
			if len(args) > 1 {
				if cmd, ok := lookupCommand(args[1]); ok {
					printCommandHelp(cmd)
				} else {
					printUsage()
				}
			} else {
				printUsage()
			}
			w.Close()
			os.Stdout = origStdout
			buf := make([]byte, 64*1024)
			n, _ := r.Read(buf)
			help.Write(buf[:n])
			_ = emitJSONSuccess("", map[string]any{"help": help.String()})
			return
		}
		if len(args) > 1 {
			if cmd, ok := lookupCommand(args[1]); ok {
				printCommandHelp(cmd)
				return
			}
		}
		printUsage()
		return
	}

	cmd, ok := lookupCommand(cmdName)
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\nRun 'wt --help' for usage.\n", cmdName)
		os.Exit(1)
	}

	// Check for --help on the command itself
	cmdArgs := args[1:]
	for _, a := range cmdArgs {
		if a == "--help" || a == "-h" {
			if isJSONOutput() {
				var help strings.Builder
				origStdout := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w
				printCommandHelp(cmd)
				w.Close()
				os.Stdout = origStdout
				buf := make([]byte, 64*1024)
				n, _ := r.Read(buf)
				help.Write(buf[:n])
				_ = emitJSONSuccess(cmd.name, map[string]any{"help": help.String()})
				return
			}
			printCommandHelp(cmd)
			return
		}
	}

	// Parse command-specific flags
	cmd.flags.SetOutput(os.Stderr)
	if err := cmd.flags.Parse(cmdArgs); err != nil {
		os.Exit(1)
	}

	if err := cmd.run(cmd.flags.Args()); err != nil {
		if isJSONOutput() {
			_ = emitJSONError(cmd.name, err)
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
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
