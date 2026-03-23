// Package main implements wt, a git worktree management CLI
// with organized directory structure and multi-strategy support.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
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
			help := captureStdoutStr(printUsage)
			_ = emitJSONSuccess("", map[string]any{"help": help})
			return
		}
		printUsage()
		return
	}

	// Handle --help / -h / help as first arg
	cmdName := args[0]
	if cmdName == "--help" || cmdName == "-h" || cmdName == "help" {
		helpFn := func() {
			if len(args) > 1 {
				if cmd, ok := lookupCommand(args[1]); ok {
					printCommandHelp(cmd)
					return
				}
			}
			printUsage()
		}
		if isJSONOutput() {
			_ = emitJSONSuccess("", map[string]any{"help": captureStdoutStr(helpFn)})
			return
		}
		helpFn()
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
				help := captureStdoutStr(func() { printCommandHelp(cmd) })
				_ = emitJSONSuccess(cmd.name, map[string]any{"help": help})
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

// captureStdoutStr redirects os.Stdout to a pipe, runs fn, and returns
// whatever fn printed. Both pipe ends are properly closed.
func captureStdoutStr(fn func()) string {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		fn() // fallback: print directly
		return ""
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	r.Close()
	return buf.String()
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
