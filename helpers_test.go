package main

import (
	"fmt"
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
