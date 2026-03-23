package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFilesToWorktree_HappyPath(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	// Create source files
	if err := os.WriteFile(filepath.Join(mainDir, ".env"), []byte("SECRET=123"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mainDir, ".tool-versions"), []byte("go 1.26"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := copyFilesToWorktree(mainDir, wtDir, []string{".env", ".tool-versions"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify files were copied
	got, err := os.ReadFile(filepath.Join(wtDir, ".env"))
	if err != nil {
		t.Fatalf("failed to read copied .env: %v", err)
	}
	if string(got) != "SECRET=123" {
		t.Errorf("copied .env content = %q, want %q", got, "SECRET=123")
	}

	got, err = os.ReadFile(filepath.Join(wtDir, ".tool-versions"))
	if err != nil {
		t.Fatalf("failed to read copied .tool-versions: %v", err)
	}
	if string(got) != "go 1.26" {
		t.Errorf("copied .tool-versions content = %q, want %q", got, "go 1.26")
	}
}

func TestCopyFilesToWorktree_MissingFile(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	// Create only one of two files
	if err := os.WriteFile(filepath.Join(mainDir, ".env"), []byte("OK"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := copyFilesToWorktree(mainDir, wtDir, []string{".env", ".nonexistent"})
	if err != nil {
		t.Fatalf("missing file should warn, not error: %v", err)
	}

	// .env should still be copied
	if _, err := os.Stat(filepath.Join(wtDir, ".env")); err != nil {
		t.Error("expected .env to be copied despite missing .nonexistent")
	}

	// .nonexistent should not exist
	if _, err := os.Stat(filepath.Join(wtDir, ".nonexistent")); !os.IsNotExist(err) {
		t.Error("expected .nonexistent to not exist in worktree")
	}
}

func TestCopyFilesToWorktree_NestedPath(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	// Create nested source file
	nested := filepath.Join(mainDir, ".config", "settings.toml")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("key = true"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := copyFilesToWorktree(mainDir, wtDir, []string{".config/settings.toml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(wtDir, ".config", "settings.toml"))
	if err != nil {
		t.Fatalf("failed to read nested copied file: %v", err)
	}
	if string(got) != "key = true" {
		t.Errorf("content = %q, want %q", got, "key = true")
	}
}

func TestCopyFilesToWorktree_EmptyPaths(t *testing.T) {
	t.Parallel()
	err := copyFilesToWorktree("/main", "/wt", nil)
	if err != nil {
		t.Fatalf("empty paths should be no-op: %v", err)
	}
	err = copyFilesToWorktree("/main", "/wt", []string{})
	if err != nil {
		t.Fatalf("empty slice should be no-op: %v", err)
	}
}

func TestCopyFilesToWorktree_PreservesPermissions(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	src := filepath.Join(mainDir, "script.sh")
	if err := os.WriteFile(src, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := copyFilesToWorktree(mainDir, wtDir, []string{"script.sh"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(filepath.Join(wtDir, "script.sh"))
	if err != nil {
		t.Fatalf("failed to stat copied file: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("permission = %o, want 0755", info.Mode().Perm())
	}
}

func TestParseCopyFilesConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	content := `root = "~/worktrees"
strategy = "global"

[copy_files]
paths = [".env", ".tool-versions", ".envrc"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("parseConfigFile error: %v", err)
	}

	want := []string{".env", ".tool-versions", ".envrc"}
	if len(cfg.CopyFiles.Paths) != len(want) {
		t.Fatalf("CopyFiles.Paths = %v, want %v", cfg.CopyFiles.Paths, want)
	}
	for i, p := range want {
		if cfg.CopyFiles.Paths[i] != p {
			t.Errorf("CopyFiles.Paths[%d] = %q, want %q", i, cfg.CopyFiles.Paths[i], p)
		}
	}
}

func TestCopyFilesToWorktree_PathTraversal(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	// Create a file that the traversal path would try to reach
	if err := os.WriteFile(filepath.Join(mainDir, "safe.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Path traversal should be skipped (not error, just warning)
	err := copyFilesToWorktree(mainDir, wtDir, []string{"../../etc/passwd", "safe.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The traversal path should NOT have been copied
	if _, err := os.Stat(filepath.Join(wtDir, "../../etc/passwd")); !os.IsNotExist(err) {
		t.Error("path traversal file should not have been copied")
	}

	// The safe file should still be copied
	if _, err := os.Stat(filepath.Join(wtDir, "safe.txt")); err != nil {
		t.Error("safe.txt should have been copied despite traversal skip")
	}
}

func TestCopyFilesToWorktree_SymlinkSkipped(t *testing.T) {
	t.Parallel()
	mainDir := t.TempDir()
	wtDir := t.TempDir()

	// Create a real file and a symlink to it
	realFile := filepath.Join(mainDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("real"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realFile, filepath.Join(mainDir, "link.txt")); err != nil {
		t.Skip("symlinks not supported on this platform")
	}

	err := copyFilesToWorktree(mainDir, wtDir, []string{"link.txt", "real.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Symlink should NOT have been copied
	if _, err := os.Stat(filepath.Join(wtDir, "link.txt")); !os.IsNotExist(err) {
		t.Error("symlink should not have been copied")
	}

	// Real file should have been copied
	if _, err := os.Stat(filepath.Join(wtDir, "real.txt")); err != nil {
		t.Error("real.txt should have been copied")
	}
}

func TestSanitizeForTerminal(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  string
	}{
		{"normal text", "normal text"},
		{"feat\x1b[2JPWNED", "feat[2JPWNED"},   // ESC stripped
		{"hello\x00world", "helloworld"},       // null stripped
		{"tab\there", "tabhere"},               // tab stripped
		{"newline\nhere", "newlinehere"},       // newline stripped
		{"unicode: résumé", "unicode: résumé"}, // UTF-8 preserved
		{"\x7f delete", " delete"},             // DEL stripped
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			got := sanitizeForTerminal(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeForTerminal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
