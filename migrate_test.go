package main

import (
	"encoding/json"
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

func TestMigrateMovesPrimaryCheckoutOutOfWorktreeRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migrate integration test in short mode")
	}

	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "home")
	worktreeRoot := filepath.Join(homeDir, "dev", "worktrees")
	primaryPath := filepath.Join(worktreeRoot, "test-repo")
	legacyPath := filepath.Join(tmpDir, "legacy", "feature-move")

	if err := os.MkdirAll(primaryPath, 0o755); err != nil {
		t.Fatalf("Failed to create primary checkout path: %v", err)
	}

	setupTestRepo(t, primaryPath)
	runGitCommand(t, primaryPath, "remote", "add", "origin", "https://github.com/acme/test-repo.git")
	runGitCommand(t, primaryPath, "branch", "feature-move")

	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("Failed to create legacy root: %v", err)
	}
	runGitCommand(t, primaryPath, "worktree", "add", legacyPath, "feature-move")

	wtBinary := buildWtBinary(t, tmpDir)

	applyCmd := exec.Command(wtBinary, "migrate")
	applyCmd.Dir = primaryPath
	applyCmd.Env = append(os.Environ(), "HOME="+homeDir, "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate failed: %v\nOutput: %s", applyErr, applyOutput)
	}

	expectedPrimaryPath := filepath.Join(homeDir, "src", "acme", "test-repo")
	if _, err := os.Stat(expectedPrimaryPath); err != nil {
		t.Fatalf("expected primary checkout at %s: %v\nOutput: %s", expectedPrimaryPath, err, applyOutput)
	}
	if _, err := os.Stat(filepath.Join(primaryPath, ".git")); !os.IsNotExist(err) {
		t.Fatalf("expected old primary path to no longer be a primary checkout, got err: %v", err)
	}

	expectedFeaturePath := filepath.Join(worktreeRoot, "test-repo", "feature-move")
	if _, err := os.Stat(expectedFeaturePath); err != nil {
		t.Fatalf("expected feature worktree at %s: %v", expectedFeaturePath, err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy feature path to be removed, got err: %v", err)
	}
}

func TestMigrateMovesWorktree(t *testing.T) {
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

	applyCmd := exec.Command(wtBinary, "migrate")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), env...)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate failed: %v\nOutput: %s", applyErr, applyOutput)
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

	applyCmd := exec.Command(wtBinary, "migrate")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate failed: %v\nOutput: %s", applyErr, applyOutput)
	}
	if !strings.Contains(string(applyOutput), "Skipped "+branch) {
		t.Fatalf("expected migrate output to mention skip for %q, got:\n%s", branch, applyOutput)
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

	applyCmd := exec.Command(wtBinary, "migrate", "--force")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate --force failed: %v\nOutput: %s", applyErr, applyOutput)
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

func TestMigrateJSONOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migrate integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	worktreeRoot := filepath.Join(tmpDir, "worktrees")
	legacyRoot := filepath.Join(tmpDir, "legacy")

	setupTestRepo(t, repoDir)
	wtBinary := buildWtBinary(t, tmpDir)

	branch := "migrate-json"
	runGitCommand(t, repoDir, "branch", branch)

	oldPath := filepath.Join(legacyRoot, branch)
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("Failed to create legacy root: %v", err)
	}
	runGitCommand(t, repoDir, "worktree", "add", oldPath, branch)

	applyCmd := exec.Command(wtBinary, "--format", "json", "migrate")
	applyCmd.Dir = repoDir
	applyCmd.Env = append(os.Environ(), "WORKTREE_ROOT="+worktreeRoot)
	applyOutput, applyErr := applyCmd.CombinedOutput()
	if applyErr != nil {
		t.Fatalf("migrate json failed: %v\nOutput: %s", applyErr, applyOutput)
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Force    bool `json:"force"`
			Total    int  `json:"total"`
			Migrated int  `json:"migrated"`
			Skipped  int  `json:"skipped"`
			Failed   int  `json:"failed"`
		} `json:"data"`
	}

	if err := json.Unmarshal(applyOutput, &payload); err != nil {
		t.Fatalf("failed to parse migrate json output: %v\noutput=%q", err, applyOutput)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true in migrate json output, got false: %s", applyOutput)
	}
	if payload.Command != "wt migrate" {
		t.Fatalf("expected command wt migrate, got %q", payload.Command)
	}
	if payload.Data.Total == 0 {
		t.Fatalf("expected migrate json total > 0, got %d", payload.Data.Total)
	}
	if payload.Data.Migrated == 0 {
		t.Fatalf("expected migrate json migrated > 0, got %d", payload.Data.Migrated)
	}
}

func TestDetectTargetState(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("missing path", func(t *testing.T) {
		state, err := detectTargetState(filepath.Join(tmpDir, "does-not-exist"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != targetMissing {
			t.Errorf("detectTargetState() = %d, want targetMissing (%d)", state, targetMissing)
		}
	})

	t.Run("file target", func(t *testing.T) {
		f := filepath.Join(tmpDir, "afile")
		if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		state, err := detectTargetState(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != targetFile {
			t.Errorf("detectTargetState() = %d, want targetFile (%d)", state, targetFile)
		}
	})

	t.Run("empty dir", func(t *testing.T) {
		d := filepath.Join(tmpDir, "emptydir")
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		state, err := detectTargetState(d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != targetDirEmpty {
			t.Errorf("detectTargetState() = %d, want targetDirEmpty (%d)", state, targetDirEmpty)
		}
	})

	t.Run("non-empty dir", func(t *testing.T) {
		d := filepath.Join(tmpDir, "fulldir")
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "file.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		state, err := detectTargetState(d)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != targetDirNonEmpty {
			t.Errorf("detectTargetState() = %d, want targetDirNonEmpty (%d)", state, targetDirNonEmpty)
		}
	})
}

func TestIsPathWithinRoot(t *testing.T) {
	tests := []struct {
		name string
		path string
		root string
		want bool
	}{
		{
			name: "child path within root",
			path: "/a/b/c",
			root: "/a/b",
			want: true,
		},
		{
			name: "same path",
			path: "/a/b",
			root: "/a/b",
			want: true,
		},
		{
			name: "sibling not within root",
			path: "/a/c",
			root: "/a/b",
			want: false,
		},
		{
			name: "parent not within child",
			path: "/a",
			root: "/a/b",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPathWithinRoot(tt.path, tt.root)
			if got != tt.want {
				t.Errorf("isPathWithinRoot(%q, %q) = %v, want %v",
					tt.path, tt.root, got, tt.want)
			}
		})
	}
}

func TestCanonicalExistingPath(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("existing absolute path", func(t *testing.T) {
		got := canonicalExistingPath(tmpDir)
		if !filepath.IsAbs(got) {
			t.Errorf("canonicalExistingPath(%q) = %q, expected absolute", tmpDir, got)
		}
		if got != filepath.Clean(tmpDir) && got != tmpDir {
			// May differ by symlink resolution; both should be clean absolute
			if !filepath.IsAbs(got) {
				t.Errorf("expected absolute path, got %q", got)
			}
		}
	})

	t.Run("relative path within existing dir", func(t *testing.T) {
		// canonicalExistingPath uses filepath.Abs which resolves relative to cwd
		got := canonicalExistingPath(".")
		if !filepath.IsAbs(got) {
			t.Errorf("canonicalExistingPath(\".\") = %q, expected absolute", got)
		}
	})
}

func TestResolvePrimaryCheckoutTarget(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		name string
		info repoInfo
		want string
	}{
		{
			name: "with owner",
			info: repoInfo{Owner: "acme", Name: "repo"},
			want: filepath.Join(home, "src", "acme", "repo"),
		},
		{
			name: "without owner",
			info: repoInfo{Name: "repo"},
			want: filepath.Join(home, "src", "repo"),
		},
		{
			name: "with slashed owner",
			info: repoInfo{Owner: "/org/", Name: "repo"},
			want: filepath.Join(home, "src", "org", "repo"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePrimaryCheckoutTarget(tt.info)
			if got != tt.want {
				t.Errorf("resolvePrimaryCheckoutTarget(%+v) = %q, want %q",
					tt.info, got, tt.want)
			}
		})
	}
}

func TestPrepareMigrateTarget(t *testing.T) {
	t.Run("missing target creates parent", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "sub", "target")
		if err := prepareMigrateTarget(target, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		parent := filepath.Dir(target)
		if _, err := os.Stat(parent); err != nil {
			t.Errorf("expected parent dir to exist: %v", err)
		}
	})

	t.Run("empty dir removed", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "emptydir")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := prepareMigrateTarget(target, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Error("expected empty dir to be removed")
		}
	})

	t.Run("non-empty dir without force errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "fulldir")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := prepareMigrateTarget(target, false)
		if err == nil {
			t.Fatal("expected error for non-empty dir without force")
		}
		if !strings.Contains(err.Error(), "non-empty") {
			t.Errorf("error should mention non-empty: %v", err)
		}
	})

	t.Run("non-empty dir with force removed", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "fulldir2")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := prepareMigrateTarget(target, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Error("expected non-empty dir to be removed with force")
		}
	})

	t.Run("file without force errors", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "afile")
		if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := prepareMigrateTarget(target, false)
		if err == nil {
			t.Fatal("expected error for file target without force")
		}
		if !strings.Contains(err.Error(), "file") {
			t.Errorf("error should mention file: %v", err)
		}
	})

	t.Run("file with force removed", func(t *testing.T) {
		tmpDir := t.TempDir()
		target := filepath.Join(tmpDir, "afile2")
		if err := os.WriteFile(target, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := prepareMigrateTarget(target, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Error("expected file to be removed with force")
		}
	})
}

func TestListParsedWorktrees(t *testing.T) {
	mock := withMockGit(t)

	porcelainOutput := strings.Join([]string{
		"worktree /home/user/repo",
		"HEAD abc1234567890",
		"branch refs/heads/main",
		"",
		"worktree /tmp/wt/feature",
		"HEAD def5678901234",
		"branch refs/heads/feature",
		"",
		"worktree /tmp/wt/detached",
		"HEAD 1111111111111",
		"detached",
		"",
	}, "\n")
	mock.outputs["worktree list --porcelain"] = []byte(porcelainOutput)

	entries, err := listParsedWorktrees()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// First entry should be marked as Main
	if !entries[0].Main {
		t.Error("expected first entry to be Main")
	}
	if entries[0].Path != "/home/user/repo" {
		t.Errorf("entries[0].Path = %q, want /home/user/repo", entries[0].Path)
	}
	if entries[0].Branch != "main" {
		t.Errorf("entries[0].Branch = %q, want main", entries[0].Branch)
	}

	// Second entry
	if entries[1].Main {
		t.Error("expected second entry not to be Main")
	}
	if entries[1].Branch != "feature" {
		t.Errorf("entries[1].Branch = %q, want feature", entries[1].Branch)
	}

	// Third entry: detached
	if !entries[2].Detached {
		t.Error("expected third entry to be Detached")
	}
	if entries[2].Main {
		t.Error("expected third entry not to be Main")
	}
}
