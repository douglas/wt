package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// getWorktreeListPorcelain
// ---------------------------------------------------------------------------

func TestGetWorktreeListPorcelainMultiEntry(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\n" +
			"HEAD abc123def456\n" +
			"branch refs/heads/main\n" +
			"\n" +
			"worktree /tmp/wt/feature\n" +
			"HEAD def789abc012\n" +
			"branch refs/heads/feature\n" +
			"\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}

	if entries[0].Path != "/home/user/repo" {
		t.Errorf("entries[0].Path = %q, want %q", entries[0].Path, "/home/user/repo")
	}
	if entries[0].HEAD != "abc123def456" {
		t.Errorf("entries[0].HEAD = %q, want %q", entries[0].HEAD, "abc123def456")
	}
	if entries[0].Branch != "main" {
		t.Errorf("entries[0].Branch = %q, want %q", entries[0].Branch, "main")
	}

	if entries[1].Path != "/tmp/wt/feature" {
		t.Errorf("entries[1].Path = %q, want %q", entries[1].Path, "/tmp/wt/feature")
	}
	if entries[1].Branch != "feature" {
		t.Errorf("entries[1].Branch = %q, want %q", entries[1].Branch, "feature")
	}
}

func TestGetWorktreeListPorcelainBare(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo.git\n" +
			"HEAD abc123\n" +
			"bare\n" +
			"\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].Bare {
		t.Error("expected Bare to be true")
	}
	if entries[0].Branch != "" {
		t.Errorf("bare entry should have no branch, got %q", entries[0].Branch)
	}
}

func TestGetWorktreeListPorcelainDetached(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/wt/detached\n" +
			"HEAD deadbeef\n" +
			"detached\n" +
			"\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].Detached {
		t.Error("expected Detached to be true")
	}
	if entries[0].HEAD != "deadbeef" {
		t.Errorf("HEAD = %q, want %q", entries[0].HEAD, "deadbeef")
	}
}

func TestGetWorktreeListPorcelainLocked(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/wt/locked-wt\n" +
			"HEAD aaa111\n" +
			"branch refs/heads/locked-branch\n" +
			"locked reason text here\n" +
			"\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Locked != "reason text here" {
		t.Errorf("Locked = %q, want %q", entries[0].Locked, "reason text here")
	}
}

func TestGetWorktreeListPorcelainPrunable(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/wt/prunable-wt\n" +
			"HEAD bbb222\n" +
			"branch refs/heads/old-branch\n" +
			"prunable gitdir file points to non-existent location\n" +
			"\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if entries[0].Prunable != "gitdir file points to non-existent location" {
		t.Errorf("Prunable = %q, want %q",
			entries[0].Prunable,
			"gitdir file points to non-existent location")
	}
}

func TestGetWorktreeListPorcelainEmpty(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte("")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

// ---------------------------------------------------------------------------
// getRepoInfo
// ---------------------------------------------------------------------------

func TestGetRepoInfoNormalRepo(t *testing.T) {
	mock := withMockGit(t)

	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	// worktree list for getMainWorktreePath
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\n" +
			"HEAD abc123\n" +
			"branch refs/heads/main\n" +
			"\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want %q", info.Name, "myrepo")
	}
	if info.Host != "github.com" {
		t.Errorf("Host = %q, want %q", info.Host, "github.com")
	}
	if info.Owner != "owner" {
		t.Errorf("Owner = %q, want %q", info.Owner, "owner")
	}
	if info.Main != "/home/user/repo" {
		t.Errorf("Main = %q, want %q", info.Main, "/home/user/repo")
	}
}

func TestGetRepoInfoBareRepo(t *testing.T) {
	mock := withMockGit(t)

	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("fatal: not a working tree")
	mock.outputs["rev-parse --is-bare-repository"] = []byte("true")
	mock.outputs["rev-parse --absolute-git-dir"] = []byte("/home/user/repos/myrepo.git")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repos/myrepo.git\n" +
			"HEAD abc123\n" +
			"bare\n" +
			"\n" +
			"worktree /home/user/repos/main\n" +
			"HEAD abc123\n" +
			"branch refs/heads/main\n" +
			"\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want %q", info.Name, "myrepo")
	}
	if info.Main != "/home/user/repos/main" {
		t.Errorf("Main = %q, want %q", info.Main, "/home/user/repos/main")
	}
}

func TestGetRepoInfoNoRemote(t *testing.T) {
	mock := withMockGit(t)

	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/localrepo")
	mock.errors["remote get-url origin"] = fmt.Errorf("fatal: No such remote 'origin'")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/localrepo\n" +
			"HEAD abc123\n" +
			"branch refs/heads/main\n" +
			"\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// repoName derived from common-dir → base of parent of .git
	if info.Name != "localrepo" {
		t.Errorf("Name = %q, want %q", info.Name, "localrepo")
	}
	if info.Host != "" {
		t.Errorf("Host = %q, want empty", info.Host)
	}
	if info.Owner != "" {
		t.Errorf("Owner = %q, want empty", info.Owner)
	}
}

func TestGetRepoInfoNotGitRepo(t *testing.T) {
	mock := withMockGit(t)

	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("not a git repo")
	mock.errors["rev-parse --is-bare-repository"] = fmt.Errorf("not a git repo")

	_, err := getRepoInfo()
	if err == nil {
		t.Fatal("expected error for non-git directory")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "not in a git repository")
	}
}

// ---------------------------------------------------------------------------
// getMainWorktreePath
// ---------------------------------------------------------------------------

func TestGetMainWorktreePathMatchesDefaultBranch(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repos/myrepo.git\n" +
			"HEAD aaa\n" +
			"bare\n" +
			"\n" +
			"worktree /home/user/repos/main\n" +
			"HEAD bbb\n" +
			"branch refs/heads/main\n" +
			"\n" +
			"worktree /home/user/repos/feature\n" +
			"HEAD ccc\n" +
			"branch refs/heads/feature\n" +
			"\n")

	got := getMainWorktreePath("main", "myrepo", "/home/user/repos/myrepo.git", true)
	if got != "/home/user/repos/main" {
		t.Errorf("got %q, want %q", got, "/home/user/repos/main")
	}
}

func TestGetMainWorktreePathFallsBackToRepoName(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repos/myrepo\n" +
			"HEAD aaa\n" +
			"branch refs/heads/develop\n" +
			"\n" +
			"worktree /home/user/repos/feature\n" +
			"HEAD bbb\n" +
			"branch refs/heads/feature\n" +
			"\n")

	// default branch "main" won't match any entry, but basename "myrepo" will
	got := getMainWorktreePath("main", "myrepo", "/somewhere", false)
	if got != "/home/user/repos/myrepo" {
		t.Errorf("got %q, want %q", got, "/home/user/repos/myrepo")
	}
}

func TestGetMainWorktreePathFallsBackToGitDir(t *testing.T) {
	// Create a temp dir with a .git directory inside to match the fallback
	tmpDir := t.TempDir()
	wtWithGit := filepath.Join(tmpDir, "wt-with-git")
	if err := os.MkdirAll(filepath.Join(wtWithGit, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		fmt.Sprintf("worktree %s\nHEAD aaa\nbranch refs/heads/develop\n\n", wtWithGit) +
			fmt.Sprintf("worktree %s/other\nHEAD bbb\nbranch refs/heads/feature\n\n", tmpDir))

	// No branch match, no basename match → falls back to .git directory check
	got := getMainWorktreePath("main", "nomatch", "/somewhere", false)
	if got != wtWithGit {
		t.Errorf("got %q, want %q", got, wtWithGit)
	}
}

func TestGetMainWorktreePathFallsBackToFirstEntry(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/first\n" +
			"HEAD aaa\n" +
			"branch refs/heads/develop\n" +
			"\n" +
			"worktree /tmp/second\n" +
			"HEAD bbb\n" +
			"branch refs/heads/feature\n" +
			"\n")

	// Nothing matches branch, basename, or .git dir → first entry
	got := getMainWorktreePath("main", "nomatch", "/somewhere", false)
	if got != "/tmp/first" {
		t.Errorf("got %q, want %q", got, "/tmp/first")
	}
}

func TestGetMainWorktreePathErrorNonBare(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["worktree list --porcelain"] = fmt.Errorf("git error")

	got := getMainWorktreePath("main", "repo", "/home/user/repo", false)
	if got != "/home/user/repo" {
		t.Errorf("got %q, want %q", got, "/home/user/repo")
	}
}

func TestGetMainWorktreePathErrorBare(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["worktree list --porcelain"] = fmt.Errorf("git error")

	got := getMainWorktreePath("main", "repo", "/home/user/repos/repo.git", true)
	want := filepath.Join("/home/user/repos", "repo")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// isDirEmpty
// ---------------------------------------------------------------------------

func TestIsDirEmpty(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		wantEmpty bool
		wantErr   bool
	}{
		{
			name: "empty directory",
			setup: func(t *testing.T) string {
				return t.TempDir()
			},
			wantEmpty: true,
			wantErr:   false,
		},
		{
			name: "non-empty directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(
					filepath.Join(dir, "file.txt"),
					[]byte("content"), 0o644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantEmpty: false,
			wantErr:   false,
		},
		{
			name: "non-existent path",
			setup: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			wantEmpty: true,
			wantErr:   false,
		},
		{
			name: "path is a file not directory",
			setup: func(t *testing.T) string {
				dir := t.TempDir()
				f := filepath.Join(dir, "afile")
				if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				return f
			},
			wantEmpty: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			empty, err := isDirEmpty(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("isDirEmpty(%q) error = %v, wantErr %v", path, err, tt.wantErr)
			}
			if empty != tt.wantEmpty {
				t.Errorf("isDirEmpty(%q) = %v, want %v", path, empty, tt.wantEmpty)
			}
		})
	}
}
