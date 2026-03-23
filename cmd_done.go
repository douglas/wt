package main

import (
	"flag"
	"fmt"
	"os"
)

var doneForce bool

func init() {
	doneFlagSet := flag.NewFlagSet("done", flag.ContinueOnError)
	doneFlagSet.BoolVar(&doneForce, "force", false, "Force removal even if worktree has modifications")
	doneFlagSet.BoolVar(&doneForce, "f", false, "Force removal even if worktree has modifications")

	registerCommand(&command{
		name:  "done",
		short: "Remove current worktree and navigate back",
		long: `Remove the linked worktree you're currently in and navigate back to the main checkout.

This is a convenience command that detects which worktree contains your current directory,
removes it (running pre/post remove hooks), and auto-navigates back to the main repo.

Must be run from inside a linked worktree, not the main checkout.

Examples:
  cd ~/dev/worktrees/myrepo/feature-branch
  wt done           # Remove this worktree, cd back to main
  wt done --force   # Force removal even with uncommitted changes`,
		flags: doneFlagSet,
		run: func(_ []string) error {
			jsonMode := isJSONOutput()

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			entries, err := getWorktreeListPorcelain()
			if err != nil {
				return fmt.Errorf("failed to list worktrees: %w", err)
			}

			if len(entries) == 0 {
				return fmt.Errorf("not inside a git worktree")
			}

			// Check if we're in the main worktree (entry[0])
			mainPath := entries[0].Path
			if isInsideWorktree(cwd, mainPath) {
				return fmt.Errorf("cannot remove the main worktree; use 'wt remove <branch>' from a linked worktree")
			}

			// Find which linked worktree we're in
			entry, found := findCurrentWorktree(entries, cwd)
			if !found {
				return ErrNotInWorktree
			}

			if entry.Branch == "" {
				return fmt.Errorf("current worktree has no branch (detached HEAD)")
			}

			// Build hook env
			info, _ := getRepoInfo()
			hookEnv := buildHookEnv(info, entry.Branch, entry.Path)

			// Run pre-remove hooks
			if err := runHooks("pre_remove", getHooks("pre_remove"), hookEnv); err != nil {
				return fmt.Errorf("pre-remove hook failed: %w", err)
			}

			// Remove the worktree
			gitArgs := []string{"worktree", "remove"}
			if doneForce {
				gitArgs = append(gitArgs, "--force")
			}
			gitArgs = append(gitArgs, entry.Path)

			gitCmd := gitCmd.Command(gitArgs...)
			if !jsonMode {
				gitCmd.Stdout = os.Stdout
				gitCmd.Stderr = os.Stderr
			}
			if err := gitCmd.Run(); err != nil {
				return fmt.Errorf("failed to remove worktree: %w", err)
			}
			resetWorktreeCache()

			if err := cleanupWorktreePath(entry.Path); err != nil {
				return err
			}

			// Run post-remove hooks (warn only)
			_ = runHooks("post_remove", getHooks("post_remove"), hookEnv)

			if jsonMode {
				return emitJSONSuccess("done", map[string]any{
					"status":      "removed",
					"branch":      entry.Branch,
					"path":        entry.Path,
					"navigate_to": mainPath,
				})
			}

			fmt.Printf("✓ Removed worktree: %s\n", entry.Path)
			printCDMarker(mainPath)
			return nil
		},
	})
}

// findCurrentWorktree finds the linked worktree that contains cwd.
// It skips the first entry (main worktree) and uses a directory boundary
// check to prevent false prefix matches (e.g., /tmp/work matching /tmp/worktree).
func findCurrentWorktree(entries []worktreeListEntry, cwd string) (worktreeListEntry, bool) {
	if len(entries) <= 1 {
		return worktreeListEntry{}, false
	}
	for _, entry := range entries[1:] {
		if isInsideWorktree(cwd, entry.Path) {
			return entry, true
		}
	}
	return worktreeListEntry{}, false
}
