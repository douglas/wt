package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetPRBranchNameMock(t *testing.T) {
	tests := []struct {
		name       string
		prNumber   string
		remoteType RemoteType
		setupExt   func(*mockGitRunner)
		want       string
		wantErr    bool
	}{
		{
			name:       "GitHub success",
			prNumber:   "42",
			remoteType: RemoteGitHub,
			setupExt: func(ext *mockGitRunner) {
				ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"feature-branch"}`)
			},
			want: "feature-branch",
		},
		{
			name:       "GitLab success",
			prNumber:   "99",
			remoteType: RemoteGitLab,
			setupExt: func(ext *mockGitRunner) {
				ext.outputs["glab mr view 99 --output json"] = []byte(`{"source_branch":"fix-bug"}`)
			},
			want: "fix-bug",
		},
		{
			name:       "GitHub command error",
			prNumber:   "42",
			remoteType: RemoteGitHub,
			setupExt: func(ext *mockGitRunner) {
				ext.errors["gh pr view 42 --json headRefName"] = fmt.Errorf("not found")
			},
			wantErr: true,
		},
		{
			name:       "GitLab command error",
			prNumber:   "99",
			remoteType: RemoteGitLab,
			setupExt: func(ext *mockGitRunner) {
				ext.errors["glab mr view 99 --output json"] = fmt.Errorf("not found")
			},
			wantErr: true,
		},
		{
			name:       "unknown remote type",
			prNumber:   "1",
			remoteType: RemoteUnknown,
			setupExt:   func(_ *mockGitRunner) {},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := withMockExt(t)
			tt.setupExt(ext)

			got, err := getPRBranchName(tt.prNumber, tt.remoteType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("getPRBranchName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("getPRBranchName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetOpenPRsMock(t *testing.T) {
	ext := withMockExt(t)
	ext.outputs["gh pr list --json number,title --jq .[] | \"\\(.number)\\t\\(.title)\""] = []byte("123\tFix bug\n456\tAdd feature\n")

	numbers, labels, err := getOpenPRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(numbers) != 2 {
		t.Fatalf("got %d PRs, want 2", len(numbers))
	}
	if numbers[0] != "123" || numbers[1] != "456" {
		t.Errorf("numbers = %v, want [123, 456]", numbers)
	}
	if labels[0] != "#123: Fix bug" {
		t.Errorf("labels[0] = %q, want %q", labels[0], "#123: Fix bug")
	}
}

func TestGetOpenPRsError(t *testing.T) {
	ext := withMockExt(t)
	ext.errors["gh pr list --json number,title --jq .[] | \"\\(.number)\\t\\(.title)\""] = fmt.Errorf("auth required")

	_, _, err := getOpenPRs()
	if err == nil {
		t.Fatal("expected error from getOpenPRs")
	}
}

func TestGetOpenMRsMock(t *testing.T) {
	ext := withMockExt(t)
	ext.outputs["glab mr list"] = []byte("!10  open  Fix login   (fix-login) ← (main)\n!20  open  Add tests   (add-tests) ← (main)\n")

	numbers, labels, err := getOpenMRs()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(numbers) != 2 {
		t.Fatalf("got %d MRs, want 2", len(numbers))
	}
	if numbers[0] != "10" || numbers[1] != "20" {
		t.Errorf("numbers = %v, want [10, 20]", numbers)
	}
	if labels[0] != "!10: Fix login" {
		t.Errorf("labels[0] = %q, want %q", labels[0], "!10: Fix login")
	}
}

func TestGetOpenMRsError(t *testing.T) {
	ext := withMockExt(t)
	ext.errors["glab mr list"] = fmt.Errorf("auth required")

	_, _, err := getOpenMRs()
	if err == nil {
		t.Fatal("expected error from getOpenMRs")
	}
}

func TestCheckoutPROrMR(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		remoteType RemoteType
		setupGit   func(*mockGitRunner)
		setupExt   func(*mockGitRunner)
		lookPath   map[string]bool
		wantErr    bool
		errContain string
	}{
		{
			name:       "GitHub existing worktree",
			input:      "42",
			remoteType: RemoteGitHub,
			lookPath:   map[string]bool{"gh": true},
			setupExt: func(ext *mockGitRunner) {
				ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"feature-x"}`)
			},
			setupGit: func(mock *mockGitRunner) {
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
			},
		},
		{
			name:       "gh CLI not found",
			input:      "42",
			remoteType: RemoteGitHub,
			lookPath:   map[string]bool{},
			setupExt:   func(_ *mockGitRunner) {},
			setupGit:   func(_ *mockGitRunner) {},
			wantErr:    true,
			errContain: "'gh' CLI not found",
		},
		{
			name:       "glab CLI not found",
			input:      "99",
			remoteType: RemoteGitLab,
			lookPath:   map[string]bool{},
			setupExt:   func(_ *mockGitRunner) {},
			setupGit:   func(_ *mockGitRunner) {},
			wantErr:    true,
			errContain: "'glab' CLI not found",
		},
		{
			name:       "invalid remote type",
			input:      "42",
			remoteType: RemoteUnknown,
			lookPath:   map[string]bool{},
			setupExt:   func(_ *mockGitRunner) {},
			setupGit:   func(_ *mockGitRunner) {},
			wantErr:    true,
			errContain: "invalid remote type",
		},
		{
			name:       "invalid PR input",
			input:      "not-a-number",
			remoteType: RemoteGitHub,
			lookPath:   map[string]bool{},
			setupExt:   func(_ *mockGitRunner) {},
			setupGit:   func(_ *mockGitRunner) {},
			wantErr:    true,
			errContain: "invalid PR/MR number",
		},
		{
			name:       "branch lookup fails",
			input:      "42",
			remoteType: RemoteGitHub,
			lookPath:   map[string]bool{"gh": true},
			setupExt: func(ext *mockGitRunner) {
				ext.errors["gh pr view 42 --json headRefName"] = fmt.Errorf("not found")
			},
			setupGit:   func(_ *mockGitRunner) {},
			wantErr:    true,
			errContain: "failed to look up branch",
		},
		{
			name:       "GitHub creates new worktree",
			input:      "42",
			remoteType: RemoteGitHub,
			lookPath:   map[string]bool{"gh": true},
			setupExt: func(ext *mockGitRunner) {
				ext.outputs["gh pr view 42 --json headRefName"] = []byte(`{"headRefName":"new-feature"}`)
			},
			setupGit: func(mock *mockGitRunner) {
				mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
				mock.outputs["remote get-url origin"] = []byte("git@github.com:owner/repo.git")
				mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
				// No new-feature worktree in list
				mock.outputs["worktree list --porcelain"] = []byte(
					"worktree /home/user/repo\n" +
						"HEAD abc123\n" +
						"branch refs/heads/main\n" +
						"\n")
				// Branch exists locally
				mock.outputs["show-ref --verify --quiet refs/heads/new-feature"] = []byte("")
			},
		},
		{
			name:       "GitLab creates new worktree for new branch",
			input:      "99",
			remoteType: RemoteGitLab,
			lookPath:   map[string]bool{"glab": true},
			setupExt: func(ext *mockGitRunner) {
				ext.outputs["glab mr view 99 --output json"] = []byte(`{"source_branch":"fix-bug"}`)
			},
			setupGit: func(mock *mockGitRunner) {
				mock.outputs["rev-parse --show-toplevel"] = []byte("/home/user/repo")
				mock.outputs["remote get-url origin"] = []byte("git@gitlab.com:owner/repo.git")
				mock.outputs["symbolic-ref refs/remotes/origin/HEAD"] = []byte("refs/remotes/origin/main")
				mock.outputs["worktree list --porcelain"] = []byte(
					"worktree /home/user/repo\n" +
						"HEAD abc123\n" +
						"branch refs/heads/main\n" +
						"\n")
				// Branch does not exist locally or remotely
				mock.errors["show-ref --verify --quiet refs/heads/fix-bug"] = fmt.Errorf("not found")
				mock.errors["show-ref --verify --quiet refs/remotes/origin/fix-bug"] = fmt.Errorf("not found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := withMockGit(t)
			ext := withMockExt(t)
			withMockLookPath(t, tt.lookPath)
			withAppConfig(t)
			appCfg.Strategy = "global"
			appCfg.Root = t.TempDir()

			tt.setupGit(mock)
			tt.setupExt(ext)

			err := checkoutPROrMR(nil, tt.input, tt.remoteType)
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkoutPROrMR() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errContain != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContain) {
					t.Errorf("error = %q, want it to contain %q", err.Error(), tt.errContain)
				}
			}
		})
	}
}
