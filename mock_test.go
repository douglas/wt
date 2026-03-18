package main

import (
	"fmt"
	"os"
	"os/exec"
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

func TestGetPRBranchNameInvalidRemoteType(t *testing.T) {
	_, err := getPRBranchName("123", RemoteUnknown)
	if err == nil {
		t.Fatal("expected error for unknown remote type")
	}
}
