package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWorktreePath_ParentStatError(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.Strategy = "global"
	appCfg.Root = "/nonexistent/root"
	appCfg.Pattern = ""
	appCfg.Separator = "/"

	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	// buildWorktreePath will try to stat/create /nonexistent/root/myrepo
	// which will fail with MkdirAll.
	_, err = buildWorktreePath(info, "feat")
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
	if !strings.Contains(err.Error(), "failed to create worktree directory") {
		t.Errorf("error = %q, want 'failed to create worktree directory'", err)
	}
}

func TestBuildWorktreePath_ParentIsFile(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	tmpDir := t.TempDir()
	// Make the repo-name directory a file so parent-is-not-a-dir triggers.
	blocker := filepath.Join(tmpDir, "myrepo")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	appCfg.Strategy = "global"
	appCfg.Root = tmpDir
	appCfg.Pattern = ""
	appCfg.Separator = "/"

	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	_, err = buildWorktreePath(info, "feat")
	if err == nil {
		t.Fatal("expected error when parent is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error = %q, want 'not a directory'", err)
	}
}

func TestRenderWorktreePath_EmptyPattern(t *testing.T) {
	withAppConfig(t)
	appCfg.Strategy = "custom"
	appCfg.Pattern = ""

	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	_, err = renderWorktreePath(info, "feat")
	if err == nil {
		t.Fatal("expected error for empty pattern with custom strategy")
	}
	if !strings.Contains(err.Error(), "WORKTREE_PATTERN is required") {
		t.Errorf("error = %q, want 'WORKTREE_PATTERN is required'", err)
	}
}

func TestRenderWorktreePath_InvalidTemplate(t *testing.T) {
	withAppConfig(t)
	appCfg.Strategy = "global"
	appCfg.Pattern = "{.invalid{nested}"
	appCfg.Separator = "/"

	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	_, err = renderWorktreePath(info, "feat")
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "invalid worktree pattern") {
		t.Errorf("error = %q, want 'invalid worktree pattern'", err)
	}
}

func TestRenderWorktreePath_MissingTemplateVar(t *testing.T) {
	withAppConfig(t)
	appCfg.Strategy = "global"
	appCfg.Pattern = "{.nonexistent.field}"
	appCfg.Separator = "/"

	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	_, err = renderWorktreePath(info, "feat")
	if err == nil {
		t.Fatal("expected error for missing template variable")
	}
	if !strings.Contains(err.Error(), "pattern variables missing values") {
		t.Errorf("error = %q, want 'pattern variables missing values'", err)
	}
}

func TestRenderWorktreePath_RelativePath(t *testing.T) {
	withAppConfig(t)
	appCfg.Strategy = "global"
	// Pattern that produces a relative path.
	appCfg.Pattern = "relative/{.branch}"
	appCfg.Root = "/absolute/root"
	appCfg.Separator = "/"

	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("getRepoInfo failed: %v", err)
	}

	got, err := renderWorktreePath(info, "feat")
	if err != nil {
		t.Fatalf("renderWorktreePath failed: %v", err)
	}
	// Relative path should be joined with root.
	want := filepath.Join("/absolute/root", "relative", "feat")
	if got != want {
		t.Errorf("renderWorktreePath = %q, want %q", got, want)
	}
}

func TestResolveWorktreePattern_AllStrategies(t *testing.T) {
	withAppConfig(t)
	appCfg.Pattern = ""

	strategies := []string{
		"sibling-repo", "sibling",
		"parent-worktrees", "parent-centered",
		"parent-branches", "repo-root",
		"parent-dotdir", "local-root",
		"inside-dotdir", "nested-local",
	}

	for _, s := range strategies {
		appCfg.Strategy = s
		p, err := resolveWorktreePattern()
		if err != nil {
			t.Errorf("resolveWorktreePattern(%q) error: %v", s, err)
		}
		if p == "" {
			t.Errorf("resolveWorktreePattern(%q) returned empty pattern", s)
		}
	}
}

func TestResolveWorktreePattern_UnsupportedStrategy(t *testing.T) {
	withAppConfig(t)
	appCfg.Strategy = "invalid-strategy"
	appCfg.Pattern = ""

	_, err := resolveWorktreePattern()
	if err == nil {
		t.Fatal("expected error for unsupported strategy")
	}
	if !strings.Contains(err.Error(), "unsupported WORKTREE_STRATEGY") {
		t.Errorf("error = %q, want 'unsupported WORKTREE_STRATEGY'", err)
	}
}

func TestCleanupWorktreePath_EmptyString(t *testing.T) {
	err := cleanupWorktreePath("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupWorktreePath_RemovesEmptyParent(t *testing.T) {
	withAppConfig(t)
	tmpDir := t.TempDir()
	appCfg.Root = tmpDir

	// Create repo dir with one worktree dir inside.
	repoDir := filepath.Join(tmpDir, "myrepo")
	wtDir := filepath.Join(repoDir, "feat")
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := cleanupWorktreePath(wtDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// wtDir should be removed.
	if _, err := os.Stat(wtDir); !os.IsNotExist(err) {
		t.Error("expected worktree dir to be removed")
	}
	// repoDir should also be removed (was empty after worktree removal).
	if _, err := os.Stat(repoDir); !os.IsNotExist(err) {
		t.Error("expected empty parent repo dir to be removed")
	}
}

func TestCleanupWorktreePath_ParentNotUnderRoot(t *testing.T) {
	withAppConfig(t)
	tmpDir := t.TempDir()
	appCfg.Root = filepath.Join(tmpDir, "root")

	// Create worktree dir outside of root.
	outsideDir := filepath.Join(tmpDir, "outside", "feat")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := cleanupWorktreePath(outsideDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// outsideDir should be removed but parent should remain
	// (it's outside root so empty-parent cleanup doesn't apply).
	if _, err := os.Stat(outsideDir); !os.IsNotExist(err) {
		t.Error("expected worktree dir to be removed")
	}
	parent := filepath.Dir(outsideDir)
	if _, err := os.Stat(parent); err != nil {
		t.Error("expected parent outside root to remain")
	}
}

func TestCleanupWorktreePath_NonEmptyParent(t *testing.T) {
	withAppConfig(t)
	tmpDir := t.TempDir()
	appCfg.Root = tmpDir

	// Create repo dir with two worktree dirs.
	repoDir := filepath.Join(tmpDir, "myrepo")
	wtDir1 := filepath.Join(repoDir, "feat1")
	wtDir2 := filepath.Join(repoDir, "feat2")
	if err := os.MkdirAll(wtDir1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wtDir2, 0o755); err != nil {
		t.Fatal(err)
	}

	// Remove feat1 — repoDir still has feat2, so should NOT be removed.
	err := cleanupWorktreePath(wtDir1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(repoDir); err != nil {
		t.Error("expected non-empty parent repo dir to remain")
	}
}

func TestIsDirEmpty_OpenError(t *testing.T) {
	// Pass a path that exists but can't be opened as a dir.
	tmpDir := t.TempDir()
	f := filepath.Join(tmpDir, "afile")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := isDirEmpty(f)
	if err == nil {
		t.Fatal("expected error when opening a file as directory")
	}
}
