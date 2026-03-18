package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// getHooks returns the hook commands for a given hook name.
func getHooks(hookName string) []string {
	switch hookName {
	case "pre_create":
		return worktreeHooks.PreCreate
	case "post_create":
		return worktreeHooks.PostCreate
	case "pre_checkout":
		return worktreeHooks.PreCheckout
	case "post_checkout":
		return worktreeHooks.PostCheckout
	case "pre_remove":
		return worktreeHooks.PreRemove
	case "post_remove":
		return worktreeHooks.PostRemove
	case "pre_pr":
		return worktreeHooks.PrePR
	case "post_pr":
		return worktreeHooks.PostPR
	case "pre_mr":
		return worktreeHooks.PreMR
	case "post_mr":
		return worktreeHooks.PostMR
	default:
		return nil
	}
}

// buildHookEnv creates the environment variables map for hook commands.
func buildHookEnv(info repoInfo, branch, worktreePath string) map[string]string {
	return map[string]string{
		"WT_PATH":       worktreePath,
		"WT_BRANCH":     branch,
		"WT_MAIN":       info.Main,
		"WT_REPO_NAME":  info.Name,
		"WT_REPO_HOST":  info.Host,
		"WT_REPO_OWNER": info.Owner,
	}
}

// runHooks executes hook commands. For pre-hooks (hookName starts with "pre_"),
// any command failure aborts the operation. For post-hooks, failures are warned
// but do not fail the overall operation.
func runHooks(hookName string, hookCommands []string, env map[string]string) error {
	if os.Getenv("WT_HOOKS_DISABLED") == "1" {
		return nil
	}
	if len(hookCommands) == 0 {
		return nil
	}

	isPre := strings.HasPrefix(hookName, "pre_")

	// Build environment slice from current env + hook vars
	environ := os.Environ()
	for k, v := range env {
		environ = append(environ, fmt.Sprintf("%s=%s", k, v))
	}

	for _, cmdStr := range hookCommands {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cmdStr)
		} else {
			cmd = exec.Command("sh", "-c", cmdStr)
		}
		cmd.Env = environ
		if isJSONOutput() {
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
		} else {
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}

		if err := cmd.Run(); err != nil {
			if isPre {
				return fmt.Errorf("command %q failed: %w", cmdStr, err)
			}
			fmt.Fprintf(os.Stderr, "\u26a0 %s hook failed: command %q: %v\n", hookName, cmdStr, err)
		}
	}
	return nil
}
