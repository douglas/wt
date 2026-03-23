package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Command-level flag variables for management commands.
var (
	removeForce   bool
	cleanupDryRun bool
	cleanupForce  bool
)

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
				return ErrCancelled
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
		inRemovedWorktree := err == nil && isInsideWorktree(cwd, existingPath)

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
		resetWorktreeCache()

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

// isInsideWorktree checks whether cwd is inside the given worktree path,
// using a directory boundary check to prevent false prefix matches.
func isInsideWorktree(cwd, worktreePath string) bool {
	return cwd == worktreePath || strings.HasPrefix(cwd, worktreePath+string(os.PathSeparator))
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
			resetWorktreeCache()

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
		resetWorktreeCache()

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
		resetWorktreeCache()

		if isJSONOutput() {
			return emitJSONSuccess(cmd, map[string]any{"status": "pruned"})
		}

		fmt.Println("✓ Pruned stale worktree administrative files")
		return nil
	},
}
