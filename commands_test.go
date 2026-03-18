package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// captureRunE calls cmd.RunE and captures stdout output.
func captureRunE(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	runErr := cmd.RunE(cmd, args)

	w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	return buf.String(), runErr
}

// parseJSON unmarshals output into a generic map for assertion.
func parseJSON(t *testing.T, output string) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON %q: %v", output, err)
	}
	return result
}

// --- versionCmd ---

func TestVersionCmd_Text(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	out, err := captureRunE(t, versionCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "wt version") {
		t.Errorf("expected output to contain 'wt version', got %q", out)
	}
}

func TestVersionCmd_JSON(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	out, err := captureRunE(t, versionCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["version"] == nil {
		t.Errorf("expected data.version field, got %v", j["data"])
	}
}

// --- listCmd ---

func TestListCmd_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	// listCmd in text mode calls .Run() with Stdout = os.Stdout,
	// so the mock's echo command prints to our captured stdout.
	mock.outputs["worktree list"] = []byte("/tmp/repo abc123 [main]\n/tmp/wt/feature def456 [feature]")

	out, err := captureRunE(t, listCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "/tmp/repo") {
		t.Errorf("expected output to contain worktree path, got %q", out)
	}
}

func TestListCmd_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			"worktree /tmp/wt/feature\nHEAD def456\nbranch refs/heads/feature\n\n")

	out, err := captureRunE(t, listCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got %v", j["data"])
	}
	worktrees, ok := data["worktrees"].([]any)
	if !ok || len(worktrees) == 0 {
		t.Errorf("expected non-empty worktrees array, got %v", data["worktrees"])
	}
}

// --- pruneCmd ---

func TestPruneCmd_Text(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	out, err := captureRunE(t, pruneCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Pruned") {
		t.Errorf("expected output to contain 'Pruned', got %q", out)
	}
}

func TestPruneCmd_JSON(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	out, err := captureRunE(t, pruneCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["status"] != "pruned" {
		t.Errorf("expected data.status='pruned', got %v", j["data"])
	}
}

// --- infoCmd ---

func TestInfoCmd_Text(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	appCfg.Root = "/tmp/worktrees"
	appCfg.Strategy = "global"
	appCfg.Pattern = ""
	appCfg.ConfigFilePath = "/tmp/config.toml"
	appCfg.ConfigFileFound = true
	appCfg.Separator = "/"

	out, err := captureRunE(t, infoCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"Strategy:", "Pattern:", "Root:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got %q", want, out)
		}
	}
}

func TestInfoCmd_JSON(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	appCfg.Root = "/tmp/worktrees"
	appCfg.Strategy = "global"
	appCfg.Pattern = ""
	appCfg.ConfigFilePath = "/tmp/config.toml"
	appCfg.ConfigFileFound = true
	appCfg.Separator = "/"

	out, err := captureRunE(t, infoCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil")
	}
	if data["config"] == nil {
		t.Errorf("expected data.config field, got %v", data)
	}
	if data["strategies"] == nil {
		t.Errorf("expected data.strategies field, got %v", data)
	}
}

// --- configPathCmd ---

func TestConfigPathCmd_Text(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	appCfg.ConfigFilePath = "/tmp/config.toml"

	out, err := captureRunE(t, configPathCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// configPathCmd calls resolveConfigPath(configFlag)
	if !strings.Contains(out, ".toml") && !strings.Contains(out, "config") {
		t.Errorf("expected output to contain config file path, got %q", out)
	}
}

func TestConfigPathCmd_JSON(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	out, err := captureRunE(t, configPathCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["path"] == nil {
		t.Errorf("expected data.path field, got %v", j["data"])
	}
}

// --- checkoutCmd: branch already exists as worktree ---

func setupCheckoutMocks(mock *mockGitRunner) {
	mock.outputs["rev-parse --show-toplevel"] = []byte("/tmp/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			"worktree /tmp/wt/feature\nHEAD def456\nbranch refs/heads/feature\n\n")
	mock.outputs["worktree list"] = []byte(
		"/tmp/repo abc123 [main]\n/tmp/wt/feature def456 [feature]\n")
}

func TestCheckoutCmd_BranchExists_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	setupCheckoutMocks(mock)

	out, err := captureRunE(t, checkoutCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected output to contain 'already exists', got %q", out)
	}
}

func TestCheckoutCmd_BranchExists_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	setupCheckoutMocks(mock)

	out, err := captureRunE(t, checkoutCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["status"] != "exists" {
		t.Errorf("expected data.status='exists', got %v", j["data"])
	}
}

// --- checkoutCmd: branch doesn't exist ---

func TestCheckoutCmd_BranchDoesNotExist(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	setupCheckoutMocks(mock)
	// Override porcelain to NOT include new-branch
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")
	resetWorktreeCache()
	// Branch doesn't exist locally or remotely
	mock.errors["show-ref --verify --quiet refs/heads/new-branch"] = fmt.Errorf("not found")
	mock.errors["show-ref --verify --quiet refs/remotes/origin/new-branch"] = fmt.Errorf("not found")

	_, err := captureRunE(t, checkoutCmd, []string{"new-branch"})
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected error to contain 'does not exist', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "wt create") {
		t.Errorf("expected error to contain 'wt create', got %q", err.Error())
	}
}

// --- createCmd: worktree already exists ---

func TestCreateCmd_AlreadyExists_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	setupCheckoutMocks(mock)

	out, err := captureRunE(t, createCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "already exists") {
		t.Errorf("expected output to contain 'already exists', got %q", out)
	}
}

func TestCreateCmd_AlreadyExists_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	setupCheckoutMocks(mock)

	out, err := captureRunE(t, createCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["status"] != "exists" {
		t.Errorf("expected data.status='exists', got %v", j["data"])
	}
}

// --- createCmd: new branch (success) ---

func TestCreateCmd_NewBranch_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	appCfg.Strategy = "global"
	appCfg.Pattern = ""
	tmpDir := t.TempDir()
	appCfg.Root = tmpDir

	setupCheckoutMocks(mock)
	// Override porcelain to NOT include new-branch
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")
	resetWorktreeCache()

	// buildWorktreePath will produce <root>/repo/new-branch
	expectedPath := filepath.Join(tmpDir, "repo", "new-branch")
	mockKey := fmt.Sprintf("worktree add %s -b new-branch main", expectedPath)
	mock.outputs[mockKey] = []byte("")

	out, err := captureRunE(t, createCmd, []string{"new-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Worktree created") {
		t.Errorf("expected output to contain 'Worktree created', got %q", out)
	}
}

func TestCreateCmd_NewBranch_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	appCfg.Strategy = "global"
	appCfg.Pattern = ""
	tmpDir := t.TempDir()
	appCfg.Root = tmpDir

	setupCheckoutMocks(mock)
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")
	resetWorktreeCache()

	expectedPath := filepath.Join(tmpDir, "repo", "new-branch")
	mockKey := fmt.Sprintf("worktree add %s -b new-branch main", expectedPath)
	mock.outputs[mockKey] = []byte("")

	out, err := captureRunE(t, createCmd, []string{"new-branch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["status"] != "created" {
		t.Errorf("expected data.status='created', got %v", j["data"])
	}
}

// --- removeCmd: branch not found ---

func TestRemoveCmd_BranchNotFound(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")

	_, err := captureRunE(t, removeCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent branch")
	}
	if !strings.Contains(err.Error(), "no worktree found") {
		t.Errorf("expected error to contain 'no worktree found', got %q", err.Error())
	}
}

// --- removeCmd: success ---

func TestRemoveCmd_Success_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	// Use a real TempDir for the worktree path so cleanupWorktreePath works
	tmpDir := t.TempDir()
	appCfg.Root = tmpDir
	featurePath := filepath.Join(tmpDir, "repo", "feature")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Mock getRepoInfo calls
	mock.outputs["rev-parse --show-toplevel"] = []byte("/tmp/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature\n\n", featurePath))
	mock.outputs["worktree list"] = []byte(
		fmt.Sprintf("/tmp/repo abc123 [main]\n%s def456 [feature]\n", featurePath))

	// Mock the remove command
	mockKey := fmt.Sprintf("worktree remove %s", featurePath)
	mock.outputs[mockKey] = []byte("")

	out, err := captureRunE(t, removeCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("expected output to contain 'Removed worktree', got %q", out)
	}
}

func TestRemoveCmd_Success_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	appCfg.Root = tmpDir
	featurePath := filepath.Join(tmpDir, "repo", "feature")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mock.outputs["rev-parse --show-toplevel"] = []byte("/tmp/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature\n\n", featurePath))
	mock.outputs["worktree list"] = []byte(
		fmt.Sprintf("/tmp/repo abc123 [main]\n%s def456 [feature]\n", featurePath))

	mockKey := fmt.Sprintf("worktree remove %s", featurePath)
	mock.outputs[mockKey] = []byte("")

	out, err := captureRunE(t, removeCmd, []string{"feature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil || data["status"] != "removed" {
		t.Errorf("expected data.status='removed', got %v", j["data"])
	}
}

// --- cleanupCmd: no candidates ---

func TestCleanupCmd_NoCandidates_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No worktrees found for merged branches") {
		t.Errorf("expected 'No worktrees found for merged branches', got %q", out)
	}
}

func TestCleanupCmd_NoCandidates_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n")

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil")
	}
	if removed, ok := data["removed"].(float64); !ok || removed != 0 {
		t.Errorf("expected removed:0, got %v", data["removed"])
	}
}

// --- cleanupCmd: dry-run with candidates ---

func TestCleanupCmd_DryRun_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	// Save and restore cleanupDryRun
	origDryRun := cleanupDryRun
	cleanupDryRun = true
	t.Cleanup(func() { cleanupDryRun = origDryRun })

	tmpDir := t.TempDir()
	featurePath := filepath.Join(tmpDir, "feature-done")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\nfeature-done\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature-done\n\n", featurePath))

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Would remove") {
		t.Errorf("expected output to contain 'Would remove', got %q", out)
	}
}

func TestCleanupCmd_DryRun_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	origDryRun := cleanupDryRun
	cleanupDryRun = true
	t.Cleanup(func() { cleanupDryRun = origDryRun })

	tmpDir := t.TempDir()
	featurePath := filepath.Join(tmpDir, "feature-done")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\nfeature-done\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature-done\n\n", featurePath))

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil")
	}
	if data["dry_run"] != true {
		t.Errorf("expected dry_run:true, got %v", data["dry_run"])
	}
}

// --- cleanupCmd: force mode ---

func TestCleanupCmd_Force_Text(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	origForce := cleanupForce
	cleanupForce = true
	t.Cleanup(func() { cleanupForce = origForce })

	tmpDir := t.TempDir()
	appCfg.Root = tmpDir
	featurePath := filepath.Join(tmpDir, "feature-done")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\nfeature-done\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature-done\n\n", featurePath))

	// Mock worktree remove
	mockKey := fmt.Sprintf("worktree remove %s", featurePath)
	mock.outputs[mockKey] = []byte("")

	// Mock worktree prune (called at end of cleanup)
	mock.outputs["worktree prune"] = []byte("")

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Removed worktree") {
		t.Errorf("expected output to contain 'Removed worktree', got %q", out)
	}
}

func TestCleanupCmd_Force_JSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	origForce := cleanupForce
	cleanupForce = true
	t.Cleanup(func() { cleanupForce = origForce })

	tmpDir := t.TempDir()
	appCfg.Root = tmpDir
	featurePath := filepath.Join(tmpDir, "feature-done")
	if err := os.MkdirAll(featurePath, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\nfeature-done\n")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/repo\nHEAD abc123\nbranch refs/heads/main\n\n" +
			fmt.Sprintf("worktree %s\nHEAD def456\nbranch refs/heads/feature-done\n\n", featurePath))

	mockKey := fmt.Sprintf("worktree remove %s", featurePath)
	mock.outputs[mockKey] = []byte("")
	mock.outputs["worktree prune"] = []byte("")

	out, err := captureRunE(t, cleanupCmd, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	j := parseJSON(t, out)
	if j["ok"] != true {
		t.Errorf("expected ok:true, got %v", j["ok"])
	}
	data, _ := j["data"].(map[string]any)
	if data == nil {
		t.Fatalf("expected data field, got nil")
	}
	if removed, ok := data["removed"].(float64); !ok || removed != 1 {
		t.Errorf("expected removed:1, got %v", data["removed"])
	}
}
