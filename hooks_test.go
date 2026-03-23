package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

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
