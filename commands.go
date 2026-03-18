package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	removeForce   bool
	cleanupDryRun bool
	cleanupForce  bool
	migrateForce  bool
)

var checkoutCmd = &cobra.Command{
	Use:     "checkout [branch]",
	Aliases: []string{"co"},
	Short:   "Checkout existing branch in new worktree",
	Args:    cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var branch string

		// Interactive selection if no branch provided
		if len(args) == 0 {
			if isJSONOutput() {
				return fmt.Errorf("wt checkout with --format json requires an explicit branch argument")
			}
			branches, err := getAvailableBranches()
			if err != nil {
				return fmt.Errorf("failed to get branches: %w", err)
			}
			if len(branches) == 0 {
				return fmt.Errorf("no available branches to checkout")
			}

			_, result, err := selectItem("Select branch to checkout", branches)
			if err != nil {
				return fmt.Errorf("selection cancelled")
			}
			branch = result
		} else {
			branch = args[0]
		}
		info, err := getRepoInfo()
		if err != nil {
			return err
		}

		// Check if worktree already exists
		if existingPath, exists := worktreeExists(branch); exists {
			if isJSONOutput() {
				return emitJSONSuccess(cmd, map[string]any{
					"status":      "exists",
					"branch":      branch,
					"path":        existingPath,
					"navigate_to": existingPath,
				})
			}
			fmt.Printf("✓ Worktree already exists: %s\n", existingPath)
			printCDMarker(existingPath)
			return nil
		}

		// Check if branch exists
		if !branchExists(branch) {
			return fmt.Errorf("branch '%s' does not exist\nUse 'wt create %s' to create a new branch", branch, branch)
		}

		path, err := buildWorktreePath(info, branch)
		if err != nil {
			return err
		}

		hookEnv := buildHookEnv(info, branch, path)

		// Run pre-checkout hooks
		if err := runHooks("pre_checkout", getHooks("pre_checkout"), hookEnv); err != nil {
			return fmt.Errorf("pre-checkout hook failed: %w", err)
		}

		// Create worktree
		gitCmd := gitCmd.Command("worktree", "add", path, branch)
		if !isJSONOutput() {
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
		}
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Run post-checkout hooks (warn only)
		_ = runHooks("post_checkout", getHooks("post_checkout"), hookEnv)

		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]any{
				"status":      "created",
				"branch":      branch,
				"path":        path,
				"navigate_to": path,
			})
		}

		fmt.Printf("✓ Worktree created at: %s\n", path)

		printCDMarker(path)
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create <branch> [base-branch]",
	Short: "Create new branch in worktree (default: main/master)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		branch := args[0]
		base := getDefaultBase()
		if len(args) > 1 {
			base = args[1]
		}

		info, err := getRepoInfo()
		if err != nil {
			return err
		}

		// Check if worktree already exists
		if existingPath, exists := worktreeExists(branch); exists {
			if isJSONOutput() {
				return emitJSONSuccess(cmd, map[string]any{
					"status":      "exists",
					"branch":      branch,
					"base":        base,
					"path":        existingPath,
					"navigate_to": existingPath,
				})
			}
			fmt.Printf("✓ Worktree already exists: %s\n", existingPath)
			printCDMarker(existingPath)
			return nil
		}

		path, err := buildWorktreePath(info, branch)
		if err != nil {
			return err
		}

		hookEnv := buildHookEnv(info, branch, path)

		// Run pre-create hooks
		if err := runHooks("pre_create", getHooks("pre_create"), hookEnv); err != nil {
			return fmt.Errorf("pre-create hook failed: %w", err)
		}

		// Create new branch and worktree
		gitCmd := gitCmd.Command("worktree", "add", path, "-b", branch, base)
		if !isJSONOutput() {
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
		}
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		// Run post-create hooks (warn only)
		_ = runHooks("post_create", getHooks("post_create"), hookEnv)

		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]any{
				"status":      "created",
				"branch":      branch,
				"base":        base,
				"path":        path,
				"navigate_to": path,
			})
		}

		fmt.Printf("✓ Worktree created at: %s\n", path)

		printCDMarker(path)
		return nil
	},
}

var prCmd = &cobra.Command{
	Use:   "pr [number|url]",
	Short: "Checkout GitHub PR in worktree (uses gh CLI)",
	Long: `Checkout a GitHub Pull Request in a worktree.

Looks up the PR's actual branch name using the 'gh' CLI, then creates
a worktree with that branch name — just like 'wt checkout <branch>'.
For GitLab Merge Requests, use 'wt mr' instead.

Examples:
  wt pr                                        # Interactive PR selection
  wt pr 123                                    # GitHub PR number
  wt pr https://github.com/org/repo/pull/123   # GitHub PR URL`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input string

		// Interactive selection if no PR provided
		if len(args) == 0 {
			if isJSONOutput() {
				return fmt.Errorf("wt pr with --format json requires an explicit PR number or URL")
			}
			numbers, labels, err := getOpenPRs()
			if err != nil {
				return fmt.Errorf("failed to get PRs: %w (is 'gh' CLI installed?)", err)
			}
			if len(labels) == 0 {
				return fmt.Errorf("no open PRs found")
			}

			idx, _, err := selectItem("Select Pull Request", labels)
			if err != nil {
				return fmt.Errorf("selection cancelled")
			}
			input = numbers[idx]
		} else {
			input = args[0]
		}

		return checkoutPROrMR(cmd, input, RemoteGitHub)
	},
}

var mrCmd = &cobra.Command{
	Use:   "mr [number|url]",
	Short: "Checkout GitLab MR in worktree (uses glab CLI)",
	Long: `Checkout a GitLab Merge Request in a worktree.

Looks up the MR's actual branch name using the 'glab' CLI, then creates
a worktree with that branch name — just like 'wt checkout <branch>'.
For GitHub Pull Requests, use 'wt pr' instead.

Examples:
  wt mr                                        # Interactive MR selection
  wt mr 123                                    # GitLab MR number
  wt mr https://gitlab.com/org/repo/-/merge_requests/123  # GitLab MR URL`,
	Args: cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input string

		// Interactive selection if no MR provided
		if len(args) == 0 {
			if isJSONOutput() {
				return fmt.Errorf("wt mr with --format json requires an explicit MR number or URL")
			}
			numbers, labels, err := getOpenMRs()
			if err != nil {
				return fmt.Errorf("failed to get MRs: %w (is 'glab' CLI installed?)", err)
			}
			if len(labels) == 0 {
				return fmt.Errorf("no open MRs found")
			}

			idx, _, err := selectItem("Select Merge Request", labels)
			if err != nil {
				return fmt.Errorf("selection cancelled")
			}
			input = numbers[idx]
		} else {
			input = args[0]
		}

		return checkoutPROrMR(cmd, input, RemoteGitLab)
	},
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all worktrees",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if isJSONOutput() {
			entries, err := getWorktreeListPorcelain()
			if err != nil {
				return err
			}
			return emitJSONSuccess(cmd, map[string]any{"worktrees": entries})
		}

		gitCmd := gitCmd.Command("worktree", "list")
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return err
		}
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:     "remove [branch]",
	Aliases: []string{"rm"},
	Short:   "Remove a worktree",
	Args:    cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var branch string
		jsonMode := isJSONOutput()

		// Interactive selection if no branch provided
		if len(args) == 0 {
			if jsonMode {
				return fmt.Errorf("wt remove with --format json requires an explicit branch argument")
			}
			branches, err := getExistingWorktreeBranches()
			if err != nil {
				return fmt.Errorf("failed to get worktrees: %w", err)
			}
			if len(branches) == 0 {
				return fmt.Errorf("no worktrees to remove")
			}

			_, result, err := selectItem("Select worktree to remove", branches)
			if err != nil {
				return fmt.Errorf("selection cancelled")
			}
			branch = result
		} else {
			branch = args[0]
		}

		existingPath, exists := worktreeExists(branch)
		if !exists {
			return fmt.Errorf("no worktree found for branch: %s", branch)
		}

		// Build hook env for remove hooks
		info, _ := getRepoInfo()
		hookEnv := buildHookEnv(info, branch, existingPath)

		// Run pre-remove hooks
		if err := runHooks("pre_remove", getHooks("pre_remove"), hookEnv); err != nil {
			return fmt.Errorf("pre-remove hook failed: %w", err)
		}

		// Check if we're currently in the worktree being removed
		cwd, err := os.Getwd()
		inRemovedWorktree := err == nil && strings.HasPrefix(cwd, existingPath)

		// Find the main worktree path (for cd after removal)
		var mainWorktreePath string
		if inRemovedWorktree {
			listCmd := gitCmd.Command("worktree", "list")
			output, err := listCmd.Output()
			if err == nil {
				lines := strings.Split(string(output), "\n")
				if len(lines) > 0 {
					// First line is always the main worktree
					fields := strings.Fields(lines[0])
					if len(fields) > 0 {
						mainWorktreePath = fields[0]
					}
				}
			}
		}

		gitArgs := []string{"worktree", "remove"}
		if removeForce {
			gitArgs = append(gitArgs, "--force")
		}
		gitArgs = append(gitArgs, existingPath)

		gitCmd := gitCmd.Command(gitArgs...)
		if !jsonMode {
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
		}
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		if err := cleanupWorktreePath(existingPath); err != nil {
			return err
		}

		// Run post-remove hooks (warn only)
		_ = runHooks("post_remove", getHooks("post_remove"), hookEnv)

		if jsonMode {
			return emitJSONSuccess(cmd, map[string]any{
				"status":      "removed",
				"branch":      branch,
				"path":        existingPath,
				"navigate_to": mainWorktreePath,
			})
		}

		fmt.Printf("✓ Removed worktree: %s\n", existingPath)

		// If we were in the removed worktree, navigate to main
		if inRemovedWorktree && mainWorktreePath != "" {
			printCDMarker(mainWorktreePath)
		}

		return nil
	},
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove worktrees for merged branches",
	Long: `Remove worktrees for branches that have been merged into the base branch.

This command finds all worktrees whose branches have been merged into main/master,
and removes them. Use --dry-run to preview what would be removed.

Examples:
  wt cleanup              # Interactive confirmation for each worktree
  wt cleanup --dry-run    # Preview what would be removed
  wt cleanup --force      # Remove all without confirmation`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		base := getDefaultBase()
		jsonMode := isJSONOutput()

		// Get merged branches
		mergedBranches, err := getMergedBranches(base)
		if err != nil {
			return err
		}

		// Get existing worktree branches
		worktreeBranches, err := getExistingWorktreeBranches()
		if err != nil {
			return fmt.Errorf("failed to get worktrees: %w", err)
		}

		// Create a set of merged branches for quick lookup
		mergedSet := make(map[string]bool)
		for _, b := range mergedBranches {
			mergedSet[b] = true
		}

		// Find worktrees that are for merged branches
		var toRemove []string
		for _, branch := range worktreeBranches {
			if mergedSet[branch] {
				toRemove = append(toRemove, branch)
			}
		}

		if len(toRemove) == 0 {
			if jsonMode {
				return emitJSONSuccess(cmd, map[string]any{"removed": 0, "skipped": 0, "base": base, "worktrees": []string{}})
			}
			fmt.Println("No worktrees found for merged branches")
			return nil
		}

		if jsonMode && !cleanupDryRun && !cleanupForce {
			return fmt.Errorf("wt cleanup with --format json requires --force or --dry-run")
		}

		// Dry run mode - just show what would be removed
		if cleanupDryRun {
			if jsonMode {
				planned := make([]map[string]string, 0, len(toRemove))
				for _, branch := range toRemove {
					if path, exists := worktreeExists(branch); exists {
						planned = append(planned, map[string]string{"branch": branch, "path": path})
					}
				}
				return emitJSONSuccess(cmd, map[string]any{"dry_run": true, "base": base, "worktrees": planned})
			}
			fmt.Printf("Would remove %d worktree(s) for merged branches:\n", len(toRemove))
			for _, branch := range toRemove {
				if path, exists := worktreeExists(branch); exists {
					fmt.Printf("  - %s (%s)\n", branch, path)
				}
			}
			return nil
		}

		// Track results
		removed := 0
		skipped := 0

		for _, branch := range toRemove {
			existingPath, exists := worktreeExists(branch)
			if !exists {
				continue
			}

			// If not force mode, ask for confirmation
			if !cleanupForce {
				ok, err := confirmPrompt(fmt.Sprintf("Remove worktree for merged branch '%s'?", branch))
				if err != nil || !ok {
					fmt.Printf("  Skipped: %s\n", branch)
					skipped++
					continue
				}
			}

			// Remove the worktree
			gitCmd := gitCmd.Command("worktree", "remove", existingPath)
			if !jsonMode {
				gitCmd.Stdout = os.Stdout
				gitCmd.Stderr = os.Stderr
			}
			if err := gitCmd.Run(); err != nil {
				if jsonMode {
					skipped++
					continue
				}
				fmt.Printf("  Failed to remove %s: %v\n", branch, err)
				continue
			}

			if err := cleanupWorktreePath(existingPath); err != nil {
				if jsonMode {
					continue
				}
				fmt.Printf("  Warning: failed to cleanup path for %s: %v\n", branch, err)
			}

			if !jsonMode {
				fmt.Printf("✓ Removed worktree: %s\n", branch)
			}
			removed++
		}

		// Run prune at the end
		pruneGitCmd := gitCmd.Command("worktree", "prune")
		_ = pruneGitCmd.Run()

		if jsonMode {
			return emitJSONSuccess(cmd, map[string]any{"dry_run": false, "base": base, "removed": removed, "skipped": skipped})
		}

		fmt.Printf("\nCleanup complete: %d removed, %d skipped\n", removed, skipped)
		return nil
	},
}

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove worktree administrative files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		gitCmd := gitCmd.Command("worktree", "prune")
		if !isJSONOutput() {
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr
		}
		if err := gitCmd.Run(); err != nil {
			return err
		}

		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]any{"status": "pruned"})
		}

		fmt.Println("✓ Pruned stale worktree administrative files")
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show worktree location configuration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		jsonMode := isJSONOutput()
		pattern, err := resolveWorktreePattern()
		if err != nil {
			pattern = appCfg.Pattern
			if pattern == "" {
				pattern = "unknown"
			}
		}

		configStatus := "not found, using defaults"
		if appCfg.ConfigFileFound {
			configStatus = "found"
		}

		hooks := map[string][]string{
			"pre_create":    appCfg.Hooks.PreCreate,
			"post_create":   appCfg.Hooks.PostCreate,
			"pre_checkout":  appCfg.Hooks.PreCheckout,
			"post_checkout": appCfg.Hooks.PostCheckout,
			"pre_remove":    appCfg.Hooks.PreRemove,
			"post_remove":   appCfg.Hooks.PostRemove,
			"pre_pr":        appCfg.Hooks.PrePR,
			"post_pr":       appCfg.Hooks.PostPR,
			"pre_mr":        appCfg.Hooks.PreMR,
			"post_mr":       appCfg.Hooks.PostMR,
		}

		if jsonMode {
			return emitJSONSuccess(cmd, map[string]any{
				"config": map[string]string{
					"path":      appCfg.ConfigFilePath,
					"status":    configStatus,
					"strategy":  appCfg.Strategy,
					"pattern":   pattern,
					"root":      appCfg.Root,
					"separator": appCfg.Separator,
				},
				"strategies": []map[string]string{
					{"name": "global", "pattern": "{.worktreeRoot}/{.repo.Name}/{.branch}"},
					{"name": "sibling-repo", "pattern": "{.repo.Main}/../{.repo.Name}-{.branch}"},
					{"name": "parent-branches", "pattern": "{.repo.Main}/../{.branch}"},
					{"name": "parent-worktrees", "pattern": "{.repo.Main}/../{.repo.Name}.worktrees/{.branch}"},
					{"name": "parent-dotdir", "pattern": "{.repo.Main}/../.worktrees/{.branch}"},
					{"name": "inside-dotdir", "pattern": "{.repo.Main}/.worktrees/{.branch}"},
					{"name": "custom", "pattern": "requires pattern setting"},
				},
				"pattern_variables": []string{"{.repo.Name}", "{.repo.Main}", "{.repo.Owner}", "{.repo.Host}", "{.branch}", "{.worktreeRoot}", "{.env.VARNAME}"},
				"hooks":             hooks,
			})
		}

		fmt.Printf(`Config:    %s (%s)

Strategy:  %s
Pattern:   %s
Root:      %s
Separator: %q

Strategies:
  global           -> {.worktreeRoot}/{.repo.Name}/{.branch}
  sibling-repo     -> {.repo.Main}/../{.repo.Name}-{.branch}
  parent-branches  -> {.repo.Main}/../{.branch}
  parent-worktrees -> {.repo.Main}/../{.repo.Name}.worktrees/{.branch}
  parent-dotdir    -> {.repo.Main}/../.worktrees/{.branch}
  inside-dotdir    -> {.repo.Main}/.worktrees/{.branch}
  custom           -> requires pattern setting

Pattern variables: {.repo.Name}, {.repo.Main}, {.repo.Owner}, {.repo.Host}, {.branch}, {.worktreeRoot}, {.env.VARNAME}
Note: The separator setting controls how "/" and "\" in value variables are replaced.
      Default "/" preserves slashes (nested dirs). Set to "-" or "_" for flat paths.
      Path variables ({.repo.Main}, {.worktreeRoot}) are never transformed.
Note: {.env.VARNAME} accesses the environment variable VARNAME (e.g. {.env.HOME}).
`, appCfg.ConfigFilePath, configStatus, appCfg.Strategy, pattern, appCfg.Root, appCfg.Separator)

		// Show configured hooks
		hookNames := []struct {
			name  string
			hooks []string
		}{
			{"pre_create", appCfg.Hooks.PreCreate},
			{"post_create", appCfg.Hooks.PostCreate},
			{"pre_checkout", appCfg.Hooks.PreCheckout},
			{"post_checkout", appCfg.Hooks.PostCheckout},
			{"pre_remove", appCfg.Hooks.PreRemove},
			{"post_remove", appCfg.Hooks.PostRemove},
			{"pre_pr", appCfg.Hooks.PrePR},
			{"post_pr", appCfg.Hooks.PostPR},
			{"pre_mr", appCfg.Hooks.PreMR},
			{"post_mr", appCfg.Hooks.PostMR},
		}
		hasHooks := false
		for _, h := range hookNames {
			if len(h.hooks) > 0 {
				hasHooks = true
				break
			}
		}
		if hasHooks {
			fmt.Println("Hooks:")
			for _, h := range hookNames {
				if len(h.hooks) > 0 {
					for _, cmd := range h.hooks {
						fmt.Printf("  %-15s %s\n", h.name+":", cmd)
					}
				}
			}
			fmt.Println()
		} else {
			fmt.Println("Hooks:    (none configured)")
			fmt.Println()
		}

		return nil
	},
}

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

var shellenvCmd = &cobra.Command{
	Use:   "shellenv",
	Short: "Output shell function for auto-cd (source this)",
	Long: `Output shell integration code for automatic directory navigation.

Add this to the END of your ~/.bashrc or ~/.zshrc:
  source <(wt shellenv)

For PowerShell, add this to your $PROFILE:
  Invoke-Expression (& wt shellenv)

Note: For zsh, place this AFTER compinit to enable tab completion.

This enables:
- Automatic cd to worktree after checkout/create/pr/mr commands
- Tab completion for commands and branch names`,
	Run: func(cmd *cobra.Command, _ []string) {
		if isJSONOutput() {
			_ = emitJSONSuccess(cmd, map[string]string{
				"note": "shellenv outputs shell script text; run without --format json to source it",
			})
			return
		}
		// Output OS-specific shell integration
		// On Windows, default to PowerShell. On Unix, output bash/zsh.
		if runtime.GOOS == "windows" {
			// PowerShell integration for Windows
			fmt.Print(`# PowerShell integration (Windows)
# Detected via runtime.GOOS, compatible with $PSVersionTable
# NOTE: Requires wt.exe to be in PATH or current directory

function wt {
    # Call wt.exe explicitly to avoid recursive function call
    # PowerShell will find wt.exe in PATH or current directory
    $output = & wt.exe @args
    $exitCode = $LASTEXITCODE
    Write-Output $output

    # In JSON mode, keep stdout machine-readable and skip auto-navigation.
    $isJson = $false
    for ($i = 0; $i -lt $args.Count; $i++) {
        if ($args[$i] -eq '--format' -and $i + 1 -lt $args.Count -and $args[$i + 1] -eq 'json') {
            $isJson = $true
        }
        if ($args[$i] -eq '--format=json') {
            $isJson = $true
        }
    }
    if ($isJson) {
        $global:LASTEXITCODE = $exitCode
        return
    }

    if ($exitCode -eq 0) {
        $cdPath = $output | Select-String -Pattern "^wt navigating to: " | ForEach-Object { $_.Line.Substring(18) }
        if ($cdPath) {
            Set-Location $cdPath
        }
    }
    $global:LASTEXITCODE = $exitCode
}

# PowerShell completion
Register-ArgumentCompleter -CommandName wt -ScriptBlock {
    param($commandName, $wordToComplete, $commandAst, $fakeBoundParameters)

    $commands = @('checkout', 'co', 'create', 'pr', 'mr', 'list', 'ls', 'remove', 'rm', 'cleanup', 'migrate', 'prune', 'help', 'shellenv', 'init', 'info', 'config', 'examples', 'version')

    # Get the position in the command line
    $position = $commandAst.CommandElements.Count - 1

    if ($position -eq 0) {
        # Complete commands
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    } elseif ($position -eq 1) {
        $subCommand = $commandAst.CommandElements[1].Value
        if ($subCommand -in @('checkout', 'co', 'create')) {
            # Complete branch names from all local and remote branches
            $remotes = (git remote 2>$null) -join '|'
            $branches = git branch -a --format='%(refname:short)' 2>$null | Where-Object { $_ -notmatch 'HEAD' } | ForEach-Object { $_ -replace "^($remotes)/", '' } | Sort-Object -Unique
            $branches | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        } elseif ($subCommand -in @('remove', 'rm')) {
            # Complete branch names from existing worktrees
            $branches = git worktree list 2>$null | Select-Object -Skip 1 | ForEach-Object {
                if ($_ -match '\[([^\]]+)\]') { $matches[1] }
            }
            $branches | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        } elseif ($subCommand -eq 'config') {
            @('init', 'show', 'path') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
}
`)
			return
		}

		// Bash/Zsh integration for Unix systems
		os.Stdout.WriteString(`wt() {
    # In JSON mode, keep stdout machine-readable and skip auto-navigation.
    case " $* " in
        *" --format json "*|*" --format=json "*)
            command wt "$@"
            return $?
            ;;
    esac

    local output exit_code cd_path
    output=$(command wt "$@")
    exit_code=$?
    printf '%s\n' "$output"
    cd_path=$(printf '%s\n' "$output" | grep '^wt navigating to: ' | tail -1 | sed 's/^wt navigating to: //')
    if [ $exit_code -eq 0 ] && [ -n "$cd_path" ]; then
        cd "$cd_path"
    fi
    return $exit_code
}

# Bash completion
if [ -n "$BASH_VERSION" ]; then
    _wt_complete() {
        local cur prev commands
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        commands="checkout co create pr mr list ls remove rm cleanup migrate prune help shellenv init info config examples version"

        # Complete commands if first argument
        if [ $COMP_CWORD -eq 1 ]; then
            COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
            return 0
        fi

        # Complete branch names for checkout/co/create and worktree branches for remove/rm
        case "$prev" in
            checkout|co|create)
                local branches remotes
                remotes=$(git remote 2>/dev/null | paste -sd'|' -)
                branches=$(git branch -a --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed -E "s#^($remotes)/##" | sort -u)
                COMPREPLY=( $(compgen -W "$branches" -- "$cur") )
                return 0
                ;;
            remove|rm)
                local branches
                branches=$(git worktree list 2>/dev/null | tail -n +2 | sed -n 's/.*\[\([^]]*\)\].*/\1/p')
                COMPREPLY=( $(compgen -W "$branches" -- "$cur") )
                return 0
                ;;
            config)
                COMPREPLY=( $(compgen -W "init show path" -- "$cur") )
                return 0
                ;;
        esac
    }
    complete -F _wt_complete wt
fi

# Zsh completion
if [ -n "$ZSH_VERSION" ]; then
    _wt_complete_zsh() {
        local -a commands branches
        commands=(
            'checkout:Checkout existing branch in new worktree'
            'co:Checkout existing branch in new worktree'
            'create:Create new branch in worktree'
            'pr:Checkout GitHub PR in worktree'
            'mr:Checkout GitLab MR in worktree'
            'list:List all worktrees'
            'ls:List all worktrees'
            'remove:Remove a worktree'
            'rm:Remove a worktree'
            'cleanup:Remove worktrees for merged branches'
            'migrate:Migrate existing worktrees to configured paths'
            'prune:Remove worktree administrative files'
            'help:Show help'
            'shellenv:Output shell function for auto-cd'
            'init:Initialize shell integration'
            'info:Show worktree location configuration'
            'config:Manage wt configuration'
            'examples:Show practical command examples'
            'version:Show version information'
        )

        if (( CURRENT == 2 )); then
            _describe 'command' commands
        elif (( CURRENT == 3 )); then
            case "$words[2]" in
                checkout|co|create)
                    local remotes
                    remotes=$(git remote 2>/dev/null | paste -sd'|' -)
                    branches=(${(f)"$(git branch -a --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed -E "s#^($remotes)/##" | sort -u)"})
                    _describe 'branch' branches
                    ;;
                remove|rm)
                    branches=(${(f)"$(git worktree list 2>/dev/null | tail -n +2 | sed -n 's/.*\[\([^]]*\)\].*/\1/p')"})
                    _describe 'branch' branches
                    ;;
                config)
                    local -a config_cmds
                    config_cmds=(
                        'init:Create a default configuration file'
                        'show:Show effective configuration with sources'
                        'path:Print the config file path'
                    )
                    _describe 'config command' config_cmds
                    ;;
            esac
        fi
    }
    # Only register completion if compdef is available
    if (( $+functions[compdef] )); then
        compdef _wt_complete_zsh wt
    fi
fi
`)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]string{"version": version})
		}
		fmt.Printf("wt version %s\n", version)
		return nil
	},
}
