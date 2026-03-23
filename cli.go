package main

import (
	"flag"
	"fmt"
	"strings"
)

// command represents a CLI subcommand with its metadata and handler.
type command struct {
	name    string
	aliases []string
	short   string
	long    string
	usage   string // args portion, e.g. "[branch]"
	run     func(args []string) error
	flags   *flag.FlagSet
}

// commands maps primary command names to their definitions.
var commands = make(map[string]*command)

// aliasMap maps alias names to primary command names.
var aliasMap = make(map[string]string)

// commandOrder preserves registration order for help output.
var commandOrder []string

// registerCommand adds a command to the registry.
func registerCommand(cmd *command) {
	commands[cmd.name] = cmd
	commandOrder = append(commandOrder, cmd.name)
	for _, alias := range cmd.aliases {
		aliasMap[alias] = cmd.name
	}
	if cmd.flags == nil {
		cmd.flags = flag.NewFlagSet(cmd.name, flag.ContinueOnError)
	}
}

// lookupCommand finds a command by name or alias.
func lookupCommand(name string) (*command, bool) {
	if cmd, ok := commands[name]; ok {
		return cmd, true
	}
	if primary, ok := aliasMap[name]; ok {
		return commands[primary], true
	}
	return nil, false
}

// extractGlobalFlags separates global flags (--config, --format) from
// the argument list. Returns remaining args with global flags consumed.
// Supports flags appearing before or after the command name.
func extractGlobalFlags(args []string) []string {
	var remaining []string
	i := 0
	for i < len(args) {
		arg := args[i]
		switch {
		case arg == "--config" && i+1 < len(args):
			configFlag = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--config="):
			configFlag = strings.TrimPrefix(arg, "--config=")
			i++
		case arg == "--format" && i+1 < len(args):
			appCfg.OutputFormat = args[i+1]
			i += 2
		case strings.HasPrefix(arg, "--format="):
			appCfg.OutputFormat = strings.TrimPrefix(arg, "--format=")
			i++
		default:
			remaining = append(remaining, arg)
			i++
		}
	}
	return remaining
}

// printUsage prints the top-level help text.
func printUsage() {
	long := buildRootCmdLong()
	fmt.Println(long)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  wt <command> [flags]")
	fmt.Println()
	fmt.Println("Commands:")

	// Calculate max name width for alignment
	type entry struct {
		display string
		short   string
	}
	entries := make([]entry, 0, len(commandOrder))
	maxWidth := 0

	seen := make(map[string]bool)
	for _, name := range commandOrder {
		if seen[name] {
			continue
		}
		seen[name] = true
		cmd := commands[name]
		display := name
		if len(cmd.aliases) > 0 {
			display = name + ", " + strings.Join(cmd.aliases, ", ")
		}
		if len(display) > maxWidth {
			maxWidth = len(display)
		}
		entries = append(entries, entry{display, cmd.short})
	}

	for _, e := range entries {
		fmt.Printf("  %-*s   %s\n", maxWidth, e.display, e.short)
	}

	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --config string   Path to config file (default: ~/.config/wt/config.toml)")
	fmt.Printf("  --format string   Output format: text or json (default %q)\n", formatText)
	fmt.Println()
	fmt.Println("Use \"wt <command> --help\" for more information about a command.")
}

// printCommandHelp prints help for a specific command.
func printCommandHelp(cmd *command) {
	if cmd.long != "" {
		fmt.Println(cmd.long)
	} else {
		fmt.Println(cmd.short)
	}
	fmt.Println()

	usage := cmd.name
	if cmd.usage != "" {
		usage += " " + cmd.usage
	}
	fmt.Printf("Usage:\n  wt %s [flags]\n", usage)

	if len(cmd.aliases) > 0 {
		fmt.Printf("\nAliases:\n  %s\n", strings.Join(cmd.aliases, ", "))
	}

	// Print command-specific flags if any are defined
	var flagHelp strings.Builder
	cmd.flags.SetOutput(&flagHelp)
	cmd.flags.PrintDefaults()
	if flagHelp.Len() > 0 {
		fmt.Printf("\nFlags:\n%s", flagHelp.String())
	}

	fmt.Println()
	fmt.Println("Global Flags:")
	fmt.Println("  --config string   Path to config file (default: ~/.config/wt/config.toml)")
	fmt.Printf("  --format string   Output format: text or json (default %q)\n", formatText)
}
