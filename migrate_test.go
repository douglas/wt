package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommandRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "migrate" {
			found = true
			break
		}
	}

	if !found {
		t.Error("migrate command not registered with root command")
	}
}

func TestMigrateCommandFlags(t *testing.T) {
	var migrateCommandFound bool
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "migrate" {
			migrateCommandFound = true

			if applyFlag := cmd.Flags().Lookup("apply"); applyFlag == nil {
				t.Error("migrate command missing --apply flag")
			}

			forceFlag := cmd.Flags().Lookup("force")
			if forceFlag == nil {
				t.Error("migrate command missing --force flag")
			} else if forceFlag.Shorthand != "f" {
				t.Errorf("migrate --force flag shorthand = %q, want %q", forceFlag.Shorthand, "f")
			}

			break
		}
	}

	if !migrateCommandFound {
		t.Fatal("migrate command not found")
	}
}

func TestMigratePreviewAndApplyMovesWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migrate integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	worktreeRoot := filepath.Join(tmpDir, "worktrees")
	legacyRoot := filepath.Join(tmpDir, "legacy")

	setupTestRepo(t, repoDir)
	wtBinary := buildWtBinary(t, tmpDir)

	branch := "migrate-branch"
	runGitCommand(t, repoDir, "branch", branch)

	oldPath := filepath.Join(legacyRoot, branch)
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("Failed to create legacy root: %v", err)
	}
	runGitCommand(t, repoDir, "worktree", "add", oldPath, branch)

	targetPath := filepath.Join(worktreeRoot, "test-repo", branch)
	env := []string{"WORKTREE_ROOT=" + worktreeRoot}

	previewCmd := exec.Command(wtBinary, "migrate")
	previewCmd.Dir = repoDir
	previewCmd.Env = append(os.Environ(), env...)
	previewOutput, previewErr := previewCmd.CombinedOutput()
	if previewErr != nil {
		t.Fatalf("migrate preview failed: %v\nOutput: %s", previewErr, previewOutput)
	}
	if !strings.Contains(string(previewOutput), branch) {
		t.Fatalf("migrate preview output does not mention branch %q:\n%s", branch, previewOutput)
	}

	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("expected old worktree path to still exist after preview: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("expected target path not to exist after preview, got err: %v", err)
	}

	applyCmd := exec.Command(wtBinary, "migrate", "--apply")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), env...)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate --apply failed: %v\nOutput: %s", applyErr, applyOutput)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old worktree path to be removed after apply, got err: %v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected target worktree path to exist after apply: %v", err)
	}
}

func TestMigrateSkipsNonEmptyTargetWithoutForce(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migrate integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	worktreeRoot := filepath.Join(tmpDir, "worktrees")
	legacyRoot := filepath.Join(tmpDir, "legacy")

	setupTestRepo(t, repoDir)
	wtBinary := buildWtBinary(t, tmpDir)

	branch := "migrate-skip"
	runGitCommand(t, repoDir, "branch", branch)

	oldPath := filepath.Join(legacyRoot, branch)
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("Failed to create legacy root: %v", err)
	}
	runGitCommand(t, repoDir, "worktree", "add", oldPath, branch)

	targetPath := filepath.Join(worktreeRoot, "test-repo", branch)
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("Failed to create target path: %v", err)
	}
	conflictFile := filepath.Join(targetPath, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("conflict"), 0o644); err != nil {
		t.Fatalf("Failed to create conflict file: %v", err)
	}

	applyCmd := exec.Command(wtBinary, "migrate", "--apply")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate --apply failed: %v\nOutput: %s", applyErr, applyOutput)
	}

	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("expected old path to remain when target is non-empty: %v", err)
	}
	if _, err := os.Stat(conflictFile); err != nil {
		t.Fatalf("expected conflict file to remain when not forced: %v", err)
	}
}

func TestMigrateForceReplacesNonEmptyTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migrate integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	worktreeRoot := filepath.Join(tmpDir, "worktrees")
	legacyRoot := filepath.Join(tmpDir, "legacy")

	setupTestRepo(t, repoDir)
	wtBinary := buildWtBinary(t, tmpDir)

	branch := "migrate-force"
	runGitCommand(t, repoDir, "branch", branch)

	oldPath := filepath.Join(legacyRoot, branch)
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("Failed to create legacy root: %v", err)
	}
	runGitCommand(t, repoDir, "worktree", "add", oldPath, branch)

	targetPath := filepath.Join(worktreeRoot, "test-repo", branch)
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("Failed to create target path: %v", err)
	}
	conflictFile := filepath.Join(targetPath, "conflict.txt")
	if err := os.WriteFile(conflictFile, []byte("conflict"), 0o644); err != nil {
		t.Fatalf("Failed to create conflict file: %v", err)
	}

	applyCmd := exec.Command(wtBinary, "migrate", "--apply", "--force")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate --apply --force failed: %v\nOutput: %s", applyErr, applyOutput)
	}

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old worktree path to be removed after forced apply, got err: %v", err)
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("expected target path to exist after forced apply: %v", err)
	}
	if _, err := os.Stat(conflictFile); !os.IsNotExist(err) {
		t.Fatalf("expected conflict file to be removed by forced migration, got err: %v", err)
	}
}
