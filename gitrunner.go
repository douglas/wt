package main

import "os/exec"

// GitRunner abstracts git command execution for testability.
type GitRunner interface {
	// Command creates an *exec.Cmd for git with the given arguments.
	// The caller can set Stdout/Stderr/Env before running.
	Command(args ...string) *exec.Cmd
}

// execGitRunner is the default GitRunner that shells out to the git binary.
type execGitRunner struct{}

func (r *execGitRunner) Command(args ...string) *exec.Cmd {
	return exec.Command("git", args...)
}

// gitCmd is the package-level GitRunner used by all git operations.
var gitCmd GitRunner = &execGitRunner{}

// execRunner is a general-purpose command runner for external CLIs (gh, glab).
type execRunner struct{}

func (r *execRunner) Command(args ...string) *exec.Cmd {
	return exec.Command(args[0], args[1:]...) //nolint:gosec // args come from hardcoded call sites, not user input
}

// extCmd is the package-level runner for external CLI commands.
var extCmd GitRunner = &execRunner{}

// lookPathFunc is a mockable wrapper around exec.LookPath.
var lookPathFunc = exec.LookPath
