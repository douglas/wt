package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type migrateAction string

const (
	migrateActionMove      migrateAction = "move"
	migrateActionMoveForce migrateAction = "move-force"
	migrateActionSkip      migrateAction = "skip"
)

type parsedWorktree struct {
	Path     string
	Branch   string
	Detached bool
	Main     bool
}

type migrateItem struct {
	Branch string
	From   string
	To     string
	Action migrateAction
	Reason string
}

type targetState int

const (
	targetMissing targetState = iota
	targetFile
	targetDirEmpty
	targetDirNonEmpty
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate existing worktrees to configured paths",
	Long: `Migrate existing linked worktrees to the currently configured location strategy.

By default this command previews what would change. Use --apply to perform moves.
Main worktree is never moved.

Examples:
  wt migrate
  wt migrate --apply
  wt migrate --apply --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := listParsedWorktrees()
		if err != nil {
			return err
		}

		plan, err := buildMigratePlan(entries, migrateForce)
		if err != nil {
			return err
		}

		if !migrateApply {
			printMigratePreview(plan)
			return nil
		}

		return applyMigratePlan(plan)
	},
}

func listParsedWorktrees() ([]parsedWorktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	var entries []parsedWorktree
	var current *parsedWorktree

	flush := func() {
		if current == nil {
			return
		}
		entries = append(entries, *current)
		current = nil
	}

	for _, raw := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			flush()
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			flush()
			current = &parsedWorktree{Path: strings.TrimPrefix(line, "worktree ")}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(line, "branch ") {
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
			continue
		}

		if line == "detached" {
			current.Detached = true
		}
	}

	flush()

	if len(entries) > 0 {
		entries[0].Main = true
	}

	return entries, nil
}

func buildMigratePlan(entries []parsedWorktree, force bool) ([]migrateItem, error) {
	info, err := getRepoInfo()
	if err != nil {
		return nil, err
	}

	var plan []migrateItem

	for _, wt := range entries {
		if wt.Main {
			plan = append(plan, migrateItem{
				Branch: wt.Branch,
				From:   filepath.Clean(wt.Path),
				Action: migrateActionSkip,
				Reason: "primary checkout remains in place",
			})
			continue
		}
		if wt.Detached || strings.TrimSpace(wt.Branch) == "" {
			plan = append(plan, migrateItem{
				Branch: wt.Branch,
				From:   wt.Path,
				To:     "",
				Action: migrateActionSkip,
				Reason: "detached or branchless worktree",
			})
			continue
		}

		targetPath, err := renderWorktreePath(info, wt.Branch)
		if err != nil {
			return nil, err
		}

		from := filepath.Clean(wt.Path)
		to := filepath.Clean(targetPath)

		if from == to {
			plan = append(plan, migrateItem{
				Branch: wt.Branch,
				From:   from,
				To:     to,
				Action: migrateActionSkip,
				Reason: "already in configured path",
			})
			continue
		}

		state, err := detectTargetState(to)
		if err != nil {
			return nil, err
		}

		item := migrateItem{
			Branch: wt.Branch,
			From:   from,
			To:     to,
			Action: migrateActionMove,
		}

		switch state {
		case targetMissing:
			// move
		case targetDirEmpty:
			item.Reason = "target path exists but is empty"
		case targetDirNonEmpty:
			if force {
				item.Action = migrateActionMoveForce
				item.Reason = "target path exists and is non-empty (force)"
			} else {
				item.Action = migrateActionSkip
				item.Reason = "target path exists and is non-empty"
			}
		case targetFile:
			if force {
				item.Action = migrateActionMoveForce
				item.Reason = "target path exists as file (force)"
			} else {
				item.Action = migrateActionSkip
				item.Reason = "target path exists as file"
			}
		}

		plan = append(plan, item)
	}

	return plan, nil
}

func detectTargetState(target string) (targetState, error) {
	info, err := os.Stat(target)
	switch {
	case os.IsNotExist(err):
		return targetMissing, nil
	case err != nil:
		return targetMissing, fmt.Errorf("failed to stat target path %s: %w", target, err)
	}

	if !info.IsDir() {
		return targetFile, nil
	}

	empty, err := isDirEmpty(target)
	if err != nil {
		return targetMissing, fmt.Errorf("failed to inspect target path %s: %w", target, err)
	}
	if empty {
		return targetDirEmpty, nil
	}

	return targetDirNonEmpty, nil
}

func printMigratePreview(plan []migrateItem) {
	moveCount := 0
	skipCount := 0
	for _, item := range plan {
		if item.Action == migrateActionMove || item.Action == migrateActionMoveForce {
			moveCount++
		} else {
			skipCount++
		}
	}

	if len(plan) == 0 {
		fmt.Println("No linked worktrees found to migrate")
		return
	}

	fmt.Println("Migration preview (no changes made):")
	for _, item := range plan {
		switch item.Action {
		case migrateActionMove:
			fmt.Printf("  - would move %s\n    from: %s\n    to:   %s\n", item.Branch, item.From, item.To)
			if item.Reason != "" {
				fmt.Printf("    note: %s\n", item.Reason)
			}
		case migrateActionMoveForce:
			fmt.Printf("  - would force-move %s\n    from: %s\n    to:   %s\n", item.Branch, item.From, item.To)
			if item.Reason != "" {
				fmt.Printf("    note: %s\n", item.Reason)
			}
		case migrateActionSkip:
			fmt.Printf("  - skip %s\n", item.Branch)
			if item.From != "" {
				fmt.Printf("    path: %s\n", item.From)
			}
			if item.Reason != "" {
				fmt.Printf("    reason: %s\n", item.Reason)
			}
		}
	}

	fmt.Printf("\nSummary: %d move(s), %d skip(s)\n", moveCount, skipCount)
	fmt.Println("Run 'wt migrate --apply' to perform these moves")
}

func applyMigratePlan(plan []migrateItem) error {
	moveCount := 0
	skipCount := 0
	failCount := 0

	for _, item := range plan {
		switch item.Action {
		case migrateActionSkip:
			fmt.Printf("Skipped %s: %s\n", item.Branch, item.Reason)
			skipCount++
			continue
		case migrateActionMove, migrateActionMoveForce:
			force := item.Action == migrateActionMoveForce
			if err := prepareMigrateTarget(item.To, force); err != nil {
				fmt.Printf("Failed %s: %v\n", item.Branch, err)
				failCount++
				continue
			}

			cmd := exec.Command("git", "worktree", "move", item.From, item.To)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Printf("Failed %s: %v\n", item.Branch, err)
				failCount++
				continue
			}

			fmt.Printf("Moved %s: %s -> %s\n", item.Branch, item.From, item.To)
			moveCount++
		}
	}

	fmt.Printf("\nMigration complete: %d moved, %d skipped, %d failed\n", moveCount, skipCount, failCount)
	if failCount > 0 {
		return fmt.Errorf("migration completed with %d failures", failCount)
	}

	return nil
}

func prepareMigrateTarget(target string, force bool) error {
	state, err := detectTargetState(target)
	if err != nil {
		return err
	}

	switch state {
	case targetMissing:
		// nothing to remove
	case targetDirEmpty:
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove empty target path %s: %w", target, err)
		}
	case targetDirNonEmpty:
		if !force {
			return fmt.Errorf("target path %s exists and is non-empty", target)
		}
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("failed to remove target path %s: %w", target, err)
		}
	case targetFile:
		if !force {
			return fmt.Errorf("target path %s exists as file", target)
		}
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove target file %s: %w", target, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("failed to create target parent directory for %s: %w", target, err)
	}

	return nil
}
