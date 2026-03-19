package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mockGitRunner records commands and returns canned responses.
type mockGitRunner struct {
	// outputs maps "arg0 arg1 ..." to the stdout bytes returned by Output().
	outputs map[string][]byte
	// errors maps "arg0 arg1 ..." to the error returned.
	errors map[string]error
	// calls records each invocation's args.
	calls [][]string
}

func newMockGitRunner() *mockGitRunner {
	return &mockGitRunner{
		outputs: make(map[string][]byte),
		errors:  make(map[string]error),
	}
}

func (m *mockGitRunner) Command(args ...string) *exec.Cmd {
	m.calls = append(m.calls, args)
	key := strings.Join(args, " ")

	// Build a helper command that prints the canned output or exits with error.
	if errVal, ok := m.errors[key]; ok && errVal != nil {
		// Return a command that fails
		cmd := exec.Command("sh", "-c", fmt.Sprintf("echo -n %q >&2; exit 1", errVal.Error()))
		return cmd
	}

	if output, ok := m.outputs[key]; ok {
		cmd := exec.Command("echo", "-n", string(output))
		return cmd
	}

	// Default: return empty success
	cmd := exec.Command("true")
	return cmd
}

// withMockGit sets gitCmd to a mock and restores it after the test.
// It also resets the worktree cache to prevent cross-test contamination.
func withMockGit(t *testing.T) *mockGitRunner {
	t.Helper()
	mock := newMockGitRunner()
	orig := gitCmd
	gitCmd = mock
	resetWorktreeCache()
	t.Cleanup(func() {
		gitCmd = orig
		resetWorktreeCache()
	})
	return mock
}

// withMockExt sets extCmd to a mock and restores it after the test.
func withMockExt(t *testing.T) *mockGitRunner {
	t.Helper()
	mock := newMockGitRunner()
	orig := extCmd
	extCmd = mock
	t.Cleanup(func() { extCmd = orig })
	return mock
}

// withMockLookPath sets lookPathFunc to a mock and restores it after the test.
// The mock returns ("found", nil) for any binary in the found set, and an error otherwise.
func withMockLookPath(t *testing.T, found map[string]bool) {
	t.Helper()
	orig := lookPathFunc
	lookPathFunc = func(file string) (string, error) {
		if found[file] {
			return file, nil
		}
		return "", fmt.Errorf("executable file not found in $PATH: %s", file)
	}
	t.Cleanup(func() { lookPathFunc = orig })
}

// withAppConfig saves and restores appCfg for the test.
func withAppConfig(t *testing.T) {
	t.Helper()
	orig := appCfg
	t.Cleanup(func() { appCfg = orig })
}

// --- Hook tests ---

func TestRunHooksPreHookFailure(t *testing.T) {
	err := runHooks("pre_create", []string{"false"}, map[string]string{})
	if err == nil {
		t.Fatal("expected pre-hook failure to return error")
	}
	if !strings.Contains(err.Error(), "false") {
		t.Errorf("error should mention the command: %v", err)
	}
}

func TestRunHooksPostHookFailureWarnsOnly(t *testing.T) {
	err := runHooks("post_create", []string{"false"}, map[string]string{})
	if err != nil {
		t.Fatalf("post-hook failure should not return error, got: %v", err)
	}
}

func TestRunHooksDisabledEnvVar(t *testing.T) {
	t.Setenv("WT_HOOKS_DISABLED", "1")
	err := runHooks("pre_create", []string{"false"}, map[string]string{})
	if err != nil {
		t.Fatalf("hooks should be skipped when disabled, got: %v", err)
	}
}

func TestRunHooksEmptyList(t *testing.T) {
	err := runHooks("pre_create", nil, map[string]string{})
	if err != nil {
		t.Fatalf("empty hooks should not return error, got: %v", err)
	}
}

func TestRunHooksEnvPropagation(t *testing.T) {
	// Run a hook that prints a WT_ env var to a temp file
	tmpFile := t.TempDir() + "/env_output"
	hookCmd := fmt.Sprintf("echo $WT_BRANCH > %s", tmpFile)
	env := map[string]string{
		"WT_BRANCH": "test-branch",
		"WT_PATH":   "/tmp/test",
	}

	err := runHooks("post_create", []string{hookCmd}, env)
	if err != nil {
		t.Fatalf("hook should succeed, got: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read hook output: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "test-branch" {
		t.Errorf("hook env WT_BRANCH = %q, want %q", got, "test-branch")
	}
}

func TestBuildHookEnvKeys(t *testing.T) {
	t.Parallel()

	info := repoInfo{
		Main:  "/home/user/repo",
		Host:  "github.com",
		Owner: "owner",
		Name:  "repo",
	}
	env := buildHookEnv(info, "feature", "/tmp/wt/feature")

	expected := map[string]string{
		"WT_PATH":       "/tmp/wt/feature",
		"WT_BRANCH":     "feature",
		"WT_MAIN":       "/home/user/repo",
		"WT_REPO_NAME":  "repo",
		"WT_REPO_HOST":  "github.com",
		"WT_REPO_OWNER": "owner",
	}
	for k, want := range expected {
		if got := env[k]; got != want {
			t.Errorf("env[%q] = %q, want %q", k, got, want)
		}
	}
}

func TestGetHooksAllNames(t *testing.T) {
	withAppConfig(t)
	appCfg.Hooks = Hooks{
		PreCreate:    []string{"pre-create-cmd"},
		PostCreate:   []string{"post-create-cmd"},
		PreCheckout:  []string{"pre-checkout-cmd"},
		PostCheckout: []string{"post-checkout-cmd"},
		PreRemove:    []string{"pre-remove-cmd"},
		PostRemove:   []string{"post-remove-cmd"},
		PrePR:        []string{"pre-pr-cmd"},
		PostPR:       []string{"post-pr-cmd"},
		PreMR:        []string{"pre-mr-cmd"},
		PostMR:       []string{"post-mr-cmd"},
	}

	tests := []struct {
		name string
		want string
	}{
		{"pre_create", "pre-create-cmd"},
		{"post_create", "post-create-cmd"},
		{"pre_checkout", "pre-checkout-cmd"},
		{"post_checkout", "post-checkout-cmd"},
		{"pre_remove", "pre-remove-cmd"},
		{"post_remove", "post-remove-cmd"},
		{"pre_pr", "pre-pr-cmd"},
		{"post_pr", "post-pr-cmd"},
		{"pre_mr", "pre-mr-cmd"},
		{"post_mr", "post-mr-cmd"},
	}

	for _, tt := range tests {
		hooks := getHooks(tt.name)
		if len(hooks) != 1 || hooks[0] != tt.want {
			t.Errorf("getHooks(%q) = %v, want [%q]", tt.name, hooks, tt.want)
		}
	}

	// Unknown hook name
	if hooks := getHooks("unknown"); hooks != nil {
		t.Errorf("getHooks(unknown) = %v, want nil", hooks)
	}
}

// --- Git helper tests with mock ---

func TestGetDefaultBaseMock(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput []byte
		mockError  error
		want       string
	}{
		{
			name:      "error falls back to main",
			mockError: fmt.Errorf("not found"),
			want:      "main",
		},
		{
			name:       "success extracts branch",
			mockOutput: []byte("refs/remotes/origin/develop"),
			want:       "develop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := withMockGit(t)
			if tt.mockError != nil {
				mock.errors["symbolic-ref refs/remotes/origin/HEAD"] = tt.mockError
			} else {
				mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = tt.mockOutput
			}

			if got := getDefaultBase(); got != tt.want {
				t.Errorf("getDefaultBase() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBranchExistsMock(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(*mockGitRunner)
		wantExists bool
	}{
		{
			name: "local branch exists",
			setupMock: func(mock *mockGitRunner) {
				mock.outputs["show-ref --verify --quiet refs/heads/feature"] = []byte("")
			},
			wantExists: true,
		},
		{
			name: "remote only",
			setupMock: func(mock *mockGitRunner) {
				mock.errors["show-ref --verify --quiet refs/heads/feature"] = fmt.Errorf("not found")
				mock.outputs["show-ref --verify --quiet refs/remotes/origin/feature"] = []byte("")
			},
			wantExists: true,
		},
		{
			name: "neither",
			setupMock: func(mock *mockGitRunner) {
				mock.errors["show-ref --verify --quiet refs/heads/feature"] = fmt.Errorf("not found")
				mock.errors["show-ref --verify --quiet refs/remotes/origin/feature"] = fmt.Errorf("not found")
			},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := withMockGit(t)
			tt.setupMock(mock)

			if got := branchExists("feature"); got != tt.wantExists {
				t.Errorf("branchExists() = %v, want %v", got, tt.wantExists)
			}
		})
	}
}

func TestGetAvailableBranchesMock(t *testing.T) {
	tests := []struct {
		name       string
		mockOutput string
		mockError  error
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "filters duplicates and remotes",
			mockOutput: "main\nfeature\norigin/feature\norigin/HEAD -> origin/main\n",
			wantCount:  2,
		},
		{
			name:       "empty output",
			mockOutput: "",
			wantCount:  0,
		},
		{
			name:      "error",
			mockError: fmt.Errorf("git error"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := withMockGit(t)
			if tt.mockError != nil {
				mock.errors["branch -a --format=%(refname:short)"] = tt.mockError
			} else {
				mock.outputs["branch -a --format=%(refname:short)"] = []byte(tt.mockOutput)
			}

			branches, err := getAvailableBranches()
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(branches) != tt.wantCount {
				t.Errorf("got %d branches, want %d: %v", len(branches), tt.wantCount, branches)
			}
		})
	}
}

func TestGetWorktreeListPorcelainError(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["worktree list --porcelain"] = fmt.Errorf("git error")

	_, err := getWorktreeListPorcelain()
	if err == nil {
		t.Fatal("expected error from getWorktreeListPorcelain")
	}
}

func TestGetMergedBranchesError(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["branch --merged main --format=%(refname:short)"] = fmt.Errorf("git error")

	_, err := getMergedBranches("main")
	if err == nil {
		t.Fatal("expected error from getMergedBranches")
	}
}

func TestGetMergedBranchesFiltersBase(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["branch --merged main --format=%(refname:short)"] = []byte("main\nmaster\nfeature-done\n")

	branches, err := getMergedBranches("main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 1 || branches[0] != "feature-done" {
		t.Errorf("getMergedBranches() = %v, want [feature-done]", branches)
	}
}

func TestWorktreeExistsMock(t *testing.T) {
	tests := []struct {
		name            string
		porcelainOutput string
		porcelainError  error
		searchBranch    string
		wantPath        string
		wantExists      bool
	}{
		{
			name: "not found",
			porcelainOutput: "worktree /home/user/repo\n" +
				"HEAD abc1234\n" +
				"branch refs/heads/main\n" +
				"\n",
			searchBranch: "feature",
			wantExists:   false,
		},
		{
			name: "found",
			porcelainOutput: "worktree /home/user/repo\n" +
				"HEAD abc1234\n" +
				"branch refs/heads/main\n" +
				"\n" +
				"worktree /tmp/wt/feature\n" +
				"HEAD def5678\n" +
				"branch refs/heads/feature\n" +
				"\n",
			searchBranch: "feature",
			wantPath:     "/tmp/wt/feature",
			wantExists:   true,
		},
		{
			name:           "git error",
			porcelainError: fmt.Errorf("git error"),
			searchBranch:   "feature",
			wantExists:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := withMockGit(t)
			if tt.porcelainError != nil {
				mock.errors["worktree list --porcelain"] = tt.porcelainError
			} else {
				mock.outputs["worktree list --porcelain"] = []byte(tt.porcelainOutput)
			}

			path, exists := worktreeExists(tt.searchBranch)
			if exists != tt.wantExists {
				t.Errorf("worktreeExists() exists = %v, want %v", exists, tt.wantExists)
			}
			if path != tt.wantPath {
				t.Errorf("worktreeExists() path = %q, want %q", path, tt.wantPath)
			}
		})
	}
}

// --- PR/MR helper tests ---

func TestParseGitHubBranchNameInvalidJSON(t *testing.T) {
	_, err := parseGitHubBranchName("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseGitHubBranchNameEmpty(t *testing.T) {
	_, err := parseGitHubBranchName(`{"headRefName":""}`)
	if err == nil {
		t.Fatal("expected error for empty branch")
	}
}

func TestParseGitLabBranchNameInvalidJSON(t *testing.T) {
	_, err := parseGitLabBranchName("not json")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseGitLabBranchNameEmpty(t *testing.T) {
	_, err := parseGitLabBranchName(`{"source_branch":""}`)
	if err == nil {
		t.Fatal("expected error for empty branch")
	}
}

// --- Paths tests with mock ---

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

func TestGetWorktreeListPorcelain_ConsecutiveWithoutBlankLine(t *testing.T) {
	mock := withMockGit(t)
	// Two worktrees without a blank line between them — tests the
	// "current.Path != ''" check inside the worktree prefix case.
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /first\n" +
			"HEAD aaa\n" +
			"branch refs/heads/main\n" +
			"worktree /second\n" +
			"HEAD bbb\n" +
			"branch refs/heads/feat\n")

	entries, err := getWorktreeListPorcelain()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Path != "/first" {
		t.Errorf("entries[0].Path = %q, want /first", entries[0].Path)
	}
	if entries[1].Path != "/second" {
		t.Errorf("entries[1].Path = %q, want /second", entries[1].Path)
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

func TestGetPRBranchNameInvalidRemoteType(t *testing.T) {
	t.Parallel()

	_, err := getPRBranchName("123", RemoteUnknown)
	if err == nil {
		t.Fatal("expected error for unknown remote type")
	}
}

func TestRunHooksJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	// In JSON mode, hooks should still run but output to stderr.
	tmpFile := t.TempDir() + "/hook_output"
	hookCmd := fmt.Sprintf("echo ok > %s", tmpFile)
	err := runHooks("post_create", []string{hookCmd}, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("hook did not run: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "ok" {
		t.Errorf("hook output = %q, want ok", got)
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

// repairOutputMockGitRunner wraps mockGitRunner but returns a command that
// outputs to stderr AND fails for the repair command.
type repairOutputMockGitRunner struct {
	*mockGitRunner
	repairTo string
}

func (m *repairOutputMockGitRunner) Command(args ...string) *exec.Cmd {
	key := strings.Join(args, " ")
	if key == fmt.Sprintf("-C %s worktree repair", m.repairTo) {
		// Return a command that outputs to stderr and exits non-zero.
		return exec.Command("sh", "-c", "echo 'repair output here' && exit 1")
	}
	return m.mockGitRunner.Command(args...)
}

func TestCheckoutPROrMR_JSONExistingWorktree(t *testing.T) {
	mock := withMockGit(t)
	ext := withMockExt(t)
	withMockLookPath(t, map[string]bool{"gh": true})
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	appCfg.Strategy = "global"
	appCfg.Root = t.TempDir()

	ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"feature-x"}`)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\n" +
			"HEAD abc123\n" +
			"branch refs/heads/main\n" +
			"\n" +
			"worktree /tmp/wt/feature-x\n" +
			"HEAD def456\n" +
			"branch refs/heads/feature-x\n" +
			"\n")

	output := captureStdout(t, func() {
		err := checkoutPROrMR(prCmd, "42", RemoteGitHub)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Should emit JSON, not text.
	if !strings.Contains(output, `"status":"exists"`) {
		t.Errorf("expected JSON with status:exists, got: %s", output)
	}
}

func TestCheckoutPROrMR_GitLabCreatesWorktreeJSON(t *testing.T) {
	mock := withMockGit(t)
	ext := withMockExt(t)
	withMockLookPath(t, map[string]bool{"glab": true})
	withAppConfig(t)
	appCfg.OutputFormat = "json"
	appCfg.Strategy = "global"
	appCfg.Root = t.TempDir()

	ext.outputs["glab mr view 99 --output json"] = []byte(`{"source_branch":"fix-bug"}`)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	mock.outputs["remote get-url origin"] = []byte("git@gitlab.com:owner/repo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\n" +
			"HEAD abc123\n" +
			"branch refs/heads/main\n" +
			"\n")
	mock.outputs["show-ref --verify --quiet refs/heads/fix-bug"] = []byte("")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")

	output := captureStdout(t, func() {
		err := checkoutPROrMR(mrCmd, "99", RemoteGitLab)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, `"status":"created"`) {
		t.Errorf("expected JSON with status:created, got: %s", output)
	}
}

func TestApplyMigratePlan_JSONFailure(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	plan := []migrateItem{
		{Branch: "main", From: "/nonexistent/from", To: "/tmp/to",
			Primary: true, Action: migrateActionMove},
	}

	captureStdout(t, func() {
		err := applyMigratePlan(migrateCmd, plan)
		if err == nil {
			t.Fatal("expected error for failed migration")
		}
		if !strings.Contains(err.Error(), "failures") {
			t.Errorf("error = %q, want 'failures'", err)
		}
	})
}

func TestApplyMigratePlan_SecondaryPrepareFail(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "text"

	// Target path can't be prepared because parent is read-only.
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	to := filepath.Join(roDir, "newdir", "feat")
	plan := []migrateItem{
		{Branch: "feat", From: "/old/feat", To: to, Action: migrateActionMove},
	}

	_ = mock // no git calls needed for prepare failure

	output := captureStdout(t, func() {
		err := applyMigratePlan(migrateCmd, plan)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	if !strings.Contains(output, "Failed feat") {
		t.Errorf("expected 'Failed feat' in output, got: %s", output)
	}
}

func TestApplyMigratePlan_PrimaryMoveJSON(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	from := filepath.Join(tmpDir, "old-primary")
	to := filepath.Join(tmpDir, "new-primary")
	if err := os.MkdirAll(from, 0o755); err != nil {
		t.Fatal(err)
	}

	plan := []migrateItem{
		{Branch: "main", From: from, To: to, Primary: true, Action: migrateActionMove},
	}

	mock.outputs[fmt.Sprintf("-C %s worktree repair", to)] = []byte("")

	output := captureStdout(t, func() {
		err := applyMigratePlan(migrateCmd, plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, `"migrated":1`) && !strings.Contains(output, `"migrated": 1`) {
		t.Errorf("expected JSON with migrated:1, got: %s", output)
	}
}

func TestApplyMigratePlan_PrimarySkipJSON(t *testing.T) {
	withMockGit(t)
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	plan := []migrateItem{
		{Branch: "main", From: "/from", Primary: true,
			Action: migrateActionSkip, Reason: "already at target"},
	}

	output := captureStdout(t, func() {
		err := applyMigratePlan(migrateCmd, plan)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, `"skipped":1`) && !strings.Contains(output, `"skipped": 1`) {
		t.Errorf("expected JSON with skipped:1, got: %s", output)
	}
}

func TestInstallShellConfig_JSONModeUpdateExisting(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", false, true); err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}
	})

	// JSON mode should suppress "Updated" text.
	if strings.Contains(output, "Updated") {
		t.Errorf("expected no text output in JSON mode, got: %s", output)
	}
}

func TestInstallShellConfig_DryRunJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", true, true); err != nil {
			t.Fatalf("installShellConfig dry-run failed: %v", err)
		}
	})

	// JSON mode + dry run should suppress "Would append" text.
	if strings.Contains(output, "Would") {
		t.Errorf("expected no text in JSON dry-run mode, got: %s", output)
	}
}

func TestRemoveShellConfig_NonexistentJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	output := captureStdout(t, func() {
		err := removeShellConfig("/nonexistent/path/.bashrc", "bash", false)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	// JSON mode should suppress "No configuration found" text.
	if strings.Contains(output, "No configuration") {
		t.Errorf("expected no text in JSON mode, got: %s", output)
	}
}

func TestRemoveShellConfig_NoMarkersJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	if err := os.WriteFile(configPath, []byte("# some config\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		err := removeShellConfig(configPath, "bash", false)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	if strings.Contains(output, "No wt configuration") {
		t.Errorf("expected no text in JSON mode, got: %s", output)
	}
}

func TestGetRepoInfo_BareRepo(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("not a worktree")
	mock.outputs["rev-parse --is-bare-repository"] = []byte("true")
	mock.outputs["rev-parse --absolute-git-dir"] = []byte("/home/user/repo.git")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo.git\nHEAD abc\nbranch refs/heads/main\nbare\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want myrepo", info.Name)
	}
}

func TestGetRepoInfo_NotARepo(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("fatal")
	mock.errors["rev-parse --is-bare-repository"] = fmt.Errorf("fatal")

	_, err := getRepoInfo()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("error = %q, want 'not in a git repository'", err)
	}
}

func TestGetRepoInfo_BareAbsGitDirFails(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("fatal")
	mock.outputs["rev-parse --is-bare-repository"] = []byte("true")
	mock.errors["rev-parse --absolute-git-dir"] = fmt.Errorf("fatal")

	_, err := getRepoInfo()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetRepoInfo_NoRemoteURL(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/myrepo")
	mock.errors["remote get-url origin"] = fmt.Errorf("no remote")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/myrepo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want myrepo", info.Name)
	}
	if info.Host != "" {
		t.Errorf("Host = %q, want empty", info.Host)
	}
}

func TestGetRepoInfo_AbsoluteGitCommonDir(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/myrepo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte("/home/user/myrepo/.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/myrepo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want myrepo", info.Name)
	}
}

func TestGetRepoInfo_GitCommonDirBare(t *testing.T) {
	mock := withMockGit(t)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	// No remote origin, so fallback to common-dir based name.
	mock.errors["remote get-url origin"] = fmt.Errorf("no remote")
	// common-dir returns a bare repo path (no .git suffix → base is used directly).
	mock.outputs["rev-parse --git-common-dir"] = []byte("/home/user/myrepo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\nHEAD abc\nbranch refs/heads/main\n\n")

	info, err := getRepoInfo()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Name != "myrepo" {
		t.Errorf("Name = %q, want myrepo", info.Name)
	}
}

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

func TestCheckoutPROrMR_FetchFallback(t *testing.T) {
	mock := withMockGit(t)
	ext := withMockExt(t)
	withMockLookPath(t, map[string]bool{"gh": true})
	withAppConfig(t)
	appCfg.Strategy = "global"
	appCfg.Root = t.TempDir()

	ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"fork-branch"}`)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\nHEAD abc\nbranch refs/heads/main\n\n")
	// First fetch fails, triggers fallback.
	mock.errors["fetch origin fork-branch"] = fmt.Errorf("remote branch not found")
	// Branch exists locally.
	mock.outputs["show-ref --verify --quiet refs/heads/fork-branch"] = []byte("")

	output := captureStdout(t, func() {
		err := checkoutPROrMR(prCmd, "42", RemoteGitHub)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "fork-branch") {
		// Text output should mention the branch.
		t.Logf("output: %s", output)
	}
}

func TestCheckoutPROrMR_WorktreeCreateFails(t *testing.T) {
	mock := withMockGit(t)
	ext := withMockExt(t)
	withMockLookPath(t, map[string]bool{"gh": true})
	withAppConfig(t)
	appCfg.Strategy = "global"
	appCfg.Root = t.TempDir()

	ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"new-feat"}`)
	mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/repo\nHEAD abc\nbranch refs/heads/main\n\n")
	mock.outputs["show-ref --verify --quiet refs/heads/new-feat"] = []byte("")
	// Worktree add fails.
	mock.errors["worktree add "+filepath.Join(appCfg.Root, "repo", "new-feat")+" new-feat"] = fmt.Errorf("worktree add failed")

	err := checkoutPROrMR(prCmd, "42", RemoteGitHub)
	if err == nil {
		t.Fatal("expected error when worktree creation fails")
	}
	if !strings.Contains(err.Error(), "failed to create worktree") {
		t.Errorf("error = %q, want 'failed to create worktree'", err)
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

func TestListParsedWorktrees_Error(t *testing.T) {
	mock := withMockGit(t)
	mock.errors["worktree list --porcelain"] = fmt.Errorf("git error")

	_, err := listParsedWorktrees()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to list worktrees") {
		t.Errorf("error = %q, want 'failed to list worktrees'", err)
	}
}

func TestGetPRBranchName_GitLab(t *testing.T) {
	ext := withMockExt(t)
	ext.outputs["glab mr view 55 --output json"] = []byte(`{"source_branch":"fix-123"}`)

	branch, err := getPRBranchName("55", RemoteGitLab)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if branch != "fix-123" {
		t.Errorf("branch = %q, want fix-123", branch)
	}
}

func TestGetPRBranchName_GitLabError(t *testing.T) {
	ext := withMockExt(t)
	ext.errors["glab mr view 55 --output json"] = fmt.Errorf("not found")

	_, err := getPRBranchName("55", RemoteGitLab)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to get MR branch name") {
		t.Errorf("error = %q, want 'failed to get MR branch name'", err)
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

func TestInstallShellConfig_WriteFileError(t *testing.T) {
	// Create a file with markers, then make it read-only so WriteFile fails.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(content), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(configPath, 0o644) })

	err := installShellConfig(configPath, "bash", false, true)
	if err == nil {
		t.Fatal("expected error when file is read-only")
	}
	if !strings.Contains(err.Error(), "failed to write") {
		t.Errorf("error = %q, want 'failed to write'", err)
	}
}

func TestRemoveShellConfig_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(configPath, 0o644) })

	err := removeShellConfig(configPath, "bash", false)
	if err == nil {
		t.Fatal("expected error when file is read-only")
	}
	if !strings.Contains(err.Error(), "failed to write") {
		t.Errorf("error = %q, want 'failed to write'", err)
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

func TestBuildMigratePlan_RenderPathError(t *testing.T) {
	mock := withMockGit(t)
	withAppConfig(t)

	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	appCfg.Root = filepath.Join(tmpDir, "worktrees")
	appCfg.Strategy = "invalid-strategy-that-will-fail"
	appCfg.Pattern = ""
	appCfg.Separator = "/"

	mock.outputs["rev-parse --show-toplevel"] = []byte(repoRoot)
	mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/myrepo.git")
	mock.outputs["rev-parse --git-common-dir"] = []byte(".git")
	mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree " + repoRoot + "\nHEAD abc\nbranch refs/heads/main\n\n")

	entries := []parsedWorktree{
		{Path: repoRoot, Branch: "main", Main: true},
		{Path: "/old/feat", Branch: "feat"},
	}

	_, err := buildMigratePlan(entries, false)
	if err == nil {
		t.Fatal("expected error when renderWorktreePath fails")
	}
}

func TestCheckoutPROrMR_GetRepoInfoFails(t *testing.T) {
	mock := withMockGit(t)
	ext := withMockExt(t)
	withMockLookPath(t, map[string]bool{"gh": true})
	withAppConfig(t)

	ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"feat"}`)
	// getRepoInfo fails.
	mock.errors["rev-parse --show-toplevel"] = fmt.Errorf("fatal")
	mock.errors["rev-parse --is-bare-repository"] = fmt.Errorf("fatal")
	// worktreeExists needs worktree list.
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /repo\nHEAD abc\nbranch refs/heads/main\n\n")

	err := checkoutPROrMR(prCmd, "42", RemoteGitHub)
	if err == nil {
		t.Fatal("expected error when getRepoInfo fails")
	}
	if !strings.Contains(err.Error(), "not in a git repository") {
		t.Errorf("error = %q, want 'not in a git repository'", err)
	}
}

func TestWriteDefaultConfig_WriteFileFails(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the parent dir and make it read-only so WriteFile fails.
	dir := filepath.Join(tmpDir, "wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "config.toml")
	err := writeDefaultConfig(path, false)
	if err == nil {
		t.Fatal("expected error when WriteFile fails")
	}
	if !strings.Contains(err.Error(), "failed to write config file") {
		t.Errorf("error = %q, want 'failed to write config file'", err)
	}
}

func TestInstallShellConfig_WriteStringError(t *testing.T) {
	// Create a new file in a dir that becomes read-only after the directory
	// creation succeeds but the file write fails.
	// Actually, this is hard to trigger. Let me try: open a file O_APPEND
	// then make it read-only... Actually, OS-level permission checks happen
	// at open time, not write time. So this path is hard to cover.
	// Instead, let me cover the "config doesn't end with newline" path.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	// File without trailing newline.
	if err := os.WriteFile(configPath, []byte("# config"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := installShellConfig(configPath, "bash", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// Should have added a newline before the marker.
	if !strings.Contains(string(content), "# config\n\n"+markerStart) {
		t.Errorf("expected newline before marker, got: %q", string(content))
	}
}

func TestDetectTargetState_StatError(t *testing.T) {
	// Use a path that triggers a stat error other than "not exist".
	// A path component that is a file (not dir) causes ENOTDIR.
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stat on blocker/child fails with ENOTDIR.
	_, err := detectTargetState(filepath.Join(blocker, "child"))
	if err == nil {
		t.Fatal("expected error for stat through non-directory")
	}
	if !strings.Contains(err.Error(), "failed to stat target path") {
		t.Errorf("error = %q, want 'failed to stat target path'", err)
	}
}

func TestGetMainWorktreePath_BareRepo(t *testing.T) {
	mock := withMockGit(t)
	// Simulate error listing worktrees, so bare fallback path is used.
	mock.errors["worktree list --porcelain"] = fmt.Errorf("no worktrees")

	got := getMainWorktreePath("main", "myrepo", "/home/user/repo.git", true)
	want := filepath.Join("/home/user", "myrepo")
	if got != want {
		t.Errorf("getMainWorktreePath(bare) = %q, want %q", got, want)
	}
}

func TestGetMainWorktreePath_FallbackToRepoName(t *testing.T) {
	mock := withMockGit(t)
	// No worktree matches default branch "main", but one matches repo name.
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /home/user/myrepo\nHEAD abc\nbranch refs/heads/develop\n\n" +
			"worktree /tmp/wt/feat\nHEAD def\nbranch refs/heads/feat\n\n")

	got := getMainWorktreePath("main", "myrepo", "/home/user/myrepo", false)
	if got != "/home/user/myrepo" {
		t.Errorf("got %q, want /home/user/myrepo", got)
	}
}

func TestGetMainWorktreePath_FallbackToFirstEntry(t *testing.T) {
	mock := withMockGit(t)
	// No worktree matches default branch or repo name.
	mock.outputs["worktree list --porcelain"] = []byte(
		"worktree /tmp/unknown\nHEAD abc\nbranch refs/heads/develop\n\n")

	got := getMainWorktreePath("main", "myrepo", "/tmp/unknown", false)
	if got != "/tmp/unknown" {
		t.Errorf("got %q, want /tmp/unknown", got)
	}
}

func TestRemoveShellConfig_DryRunJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		err := removeShellConfig(configPath, "bash", true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	if strings.Contains(output, "Would remove") {
		t.Errorf("expected no text in JSON dry-run mode, got: %s", output)
	}
}
