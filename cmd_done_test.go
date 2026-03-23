package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindCurrentWorktree(t *testing.T) {
	t.Parallel()
	entries := []worktreeListEntry{
		{Path: "/home/user/repo", Branch: "main"},
		{Path: "/home/user/worktrees/feat", Branch: "feat"},
		{Path: "/home/user/worktrees/fix", Branch: "fix"},
	}

	tests := []struct {
		name       string
		cwd        string
		wantFound  bool
		wantBranch string
	}{
		{"exact match", "/home/user/worktrees/feat", true, "feat"},
		{"child dir", "/home/user/worktrees/feat/src/pkg", true, "feat"},
		{"main worktree not matched", "/home/user/repo", false, ""},
		{"main child not matched", "/home/user/repo/src", false, ""},
		{"unrelated dir", "/tmp/other", false, ""},
		{"prefix but not child", "/home/user/worktrees/features", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry, found := findCurrentWorktree(entries, tt.cwd)
			if found != tt.wantFound {
				t.Errorf("findCurrentWorktree(%q) found = %v, want %v", tt.cwd, found, tt.wantFound)
			}
			if found && entry.Branch != tt.wantBranch {
				t.Errorf("findCurrentWorktree(%q) branch = %q, want %q", tt.cwd, entry.Branch, tt.wantBranch)
			}
		})
	}
}

func TestFindCurrentWorktree_SingleEntry(t *testing.T) {
	t.Parallel()
	entries := []worktreeListEntry{
		{Path: "/home/user/repo", Branch: "main"},
	}
	_, found := findCurrentWorktree(entries, "/home/user/repo")
	if found {
		t.Error("should not find linked worktree when only main exists")
	}
}

func TestDoneCmd_NotInWorktree(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	// Empty worktree list → not inside a git worktree
	mock.outputs["worktree list --porcelain"] = []byte("")

	_, err := captureRunE(t, doneCmd, nil)
	if err == nil {
		t.Fatal("expected error when not in a worktree")
	}
	if !strings.Contains(err.Error(), "not inside") {
		t.Errorf("error should mention 'not inside': %v", err)
	}
}

func TestDoneCmd_InMainWorktree(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	// Create a temp dir to simulate being in the main worktree
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(mainPath, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(mainPath)

	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree " + mainPath + "\nHEAD abc123\nbranch refs/heads/main\n\n",
	)

	_, err := captureRunE(t, doneCmd, nil)
	if err == nil {
		t.Fatal("expected error when in main worktree")
	}
	if !strings.Contains(err.Error(), "cannot remove the main worktree") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDoneCmd_HappyPath(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktrees", "feat")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mainPath, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(wtPath)

	porcelain := "worktree " + mainPath + "\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def456\nbranch refs/heads/feat\n\n"
	mock.outputs["worktree list --porcelain"] = []byte(porcelain)

	// Mock the remote URL for getRepoInfo
	mock.outputs["remote get-url origin"] = []byte("https://github.com/user/repo.git")
	mock.outputs["rev-parse --path-format=absolute --git-common-dir"] = []byte(mainPath + "/.git")

	output, err := captureRunE(t, doneCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Removed worktree") {
		t.Errorf("expected removal message, got: %s", output)
	}
	if !strings.Contains(output, "wt navigating to: "+mainPath) {
		t.Errorf("expected navigation marker for main path, got: %s", output)
	}
}

func TestDoneCmd_DetachedHead(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktrees", "detached")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mainPath, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(wtPath)

	porcelain := "worktree " + mainPath + "\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def456\ndetached\n\n"
	mock.outputs["worktree list --porcelain"] = []byte(porcelain)

	_, err := captureRunE(t, doneCmd, nil)
	if err == nil {
		t.Fatal("expected error for detached HEAD")
	}
	if !strings.Contains(err.Error(), "no branch") {
		t.Errorf("expected detached HEAD error, got: %v", err)
	}
}

func TestDoneCmd_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = formatJSON

	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "repo")
	wtPath := filepath.Join(tmpDir, "worktrees", "feat")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(mainPath, 0o755); err != nil {
		t.Fatal(err)
	}

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(wtPath)

	porcelain := "worktree " + mainPath + "\nHEAD abc123\nbranch refs/heads/main\n\n" +
		"worktree " + wtPath + "\nHEAD def456\nbranch refs/heads/feat\n\n"
	mock.outputs["worktree list --porcelain"] = []byte(porcelain)
	mock.outputs["remote get-url origin"] = []byte("https://github.com/user/repo.git")
	mock.outputs["rev-parse --path-format=absolute --git-common-dir"] = []byte(mainPath + "/.git")

	output, err := captureRunE(t, doneCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, output)
	if j["ok"] != true {
		t.Errorf("expected ok=true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data["branch"] != "feat" {
		t.Errorf("expected branch=feat, got %v", data["branch"])
	}
	if data["navigate_to"] != mainPath {
		t.Errorf("expected navigate_to=%s, got %v", mainPath, data["navigate_to"])
	}
}

func TestIsInsideWorktree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		cwd, path string
		want      bool
	}{
		{"/home/user/wt/feat", "/home/user/wt/feat", true},
		{"/home/user/wt/feat/src", "/home/user/wt/feat", true},
		{"/home/user/wt/features", "/home/user/wt/feat", false},
		{"/other", "/home/user/wt/feat", false},
	}
	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			t.Parallel()
			got := isInsideWorktree(tt.cwd, tt.path)
			if got != tt.want {
				t.Errorf("isInsideWorktree(%q, %q) = %v, want %v", tt.cwd, tt.path, got, tt.want)
			}
		})
	}
}

func TestErrNotInWorktree(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrNotInWorktree, ErrNotInWorktree) {
		t.Error("ErrNotInWorktree should match itself with errors.Is")
	}
}
