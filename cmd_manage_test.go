package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMovePrimaryCheckout_RenameFailure(t *testing.T) {
	withMockGit(t)

	err := movePrimaryCheckout("/nonexistent/from", "/tmp/impossible/to", false)
	if err == nil {
		t.Fatal("expected error when rename fails")
	}
	if !strings.Contains(err.Error(), "failed to move primary checkout") {
		t.Errorf("error = %q, want 'failed to move primary checkout'", err)
	}
}

func TestMovePrimaryCheckout_RepairWithStderrOutput(t *testing.T) {
	// Use a real shell command that prints to stderr AND fails.
	origGit := gitCmd
	mock := &repairOutputMockGitRunner{
		mockGitRunner: newMockGitRunner(),
	}
	gitCmd = mock
	t.Cleanup(func() { gitCmd = origGit; resetWorktreeCache() })
	resetWorktreeCache()

	tmpDir := t.TempDir()
	from := filepath.Join(tmpDir, "old")
	to := filepath.Join(tmpDir, "new")
	if err := os.MkdirAll(from, 0o755); err != nil {
		t.Fatal(err)
	}

	mock.repairTo = to

	err := movePrimaryCheckout(from, to, false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "repair output here") {
		t.Errorf("error = %q, want mention of 'repair output here'", err)
	}
}
