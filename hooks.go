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
		return appCfg.Hooks.PreCreate
	case "post_create":
		return appCfg.Hooks.PostCreate
	case "pre_checkout":
		return appCfg.Hooks.PreCheckout
	case "post_checkout":
		return appCfg.Hooks.PostCheckout
	case "pre_remove":
		return appCfg.Hooks.PreRemove
	case "post_remove":
		return appCfg.Hooks.PostRemove
	case "pre_pr":
		return appCfg.Hooks.PrePR
	case "post_pr":
		return appCfg.Hooks.PostPR
	case "pre_mr":
		return appCfg.Hooks.PreMR
	case "post_mr":
		return appCfg.Hooks.PostMR
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
