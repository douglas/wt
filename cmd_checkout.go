package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
				return ErrCancelled
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
		resetWorktreeCache()

		// Run post-checkout hooks (warn only)
		_ = runHooks("post_checkout", getHooks("post_checkout"), hookEnv)

		// Copy files from main worktree if configured
		if len(appCfg.CopyFiles.Paths) > 0 {
			_ = copyFilesToWorktree(info.Main, path, appCfg.CopyFiles.Paths)
		}

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
		resetWorktreeCache()

		// Run post-create hooks (warn only)
		_ = runHooks("post_create", getHooks("post_create"), hookEnv)

		// Copy files from main worktree if configured
		if len(appCfg.CopyFiles.Paths) > 0 {
			_ = copyFilesToWorktree(info.Main, path, appCfg.CopyFiles.Paths)
		}

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
