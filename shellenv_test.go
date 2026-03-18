package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestShellenvInteractiveModeOutputCapture tests that the shell function
// captures output for interactive commands (co/checkout/rm/remove/pr/mr with no args).
// This is critical for auto-cd functionality.
//
// BUG: Currently fails because interactive mode doesn't capture output
func TestShellenvInteractiveModeOutputCapture(t *testing.T) {
	// Get the shellenv output
	cmd := exec.Command("go", "run", ".", "shellenv")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run wt shellenv: %v", err)
	}
	shellenv := string(output)

	// The BUG: Interactive mode runs "command wt" directly without capturing output
	// This means the navigation marker is never captured and auto-cd doesn't work
	if strings.Contains(shellenv, "# Run interactively without capturing output") {
		t.Fatal("BUG DETECTED: Shell function has special case for interactive mode that skips output capture.\n" +
			"This prevents auto-cd from working when running 'wt co', 'wt rm', etc. without arguments.\n" +
			"The navigation marker is printed but never captured by the shell function.\n" +
			"EXPECTED: All commands should capture output using 'output=$(command wt \"$@\")'")
	}

	// After fix: The simplified function should always capture output
	// There should be NO special case handling for interactive mode
	hasSpecialCase := strings.Contains(shellenv, "if [ \"$#\" -eq 1 ]; then") &&
		strings.Contains(shellenv, "co|checkout|rm|remove|pr|mr)")

	if hasSpecialCase {
		t.Fatal("BUG DETECTED: Shell function still has special case handling for interactive commands.\n" +
			"This code path doesn't capture output, breaking auto-cd functionality.\n" +
			"EXPECTED: Remove the special case and let all commands use the same output capture logic.")
	}

	// Verify the fix: uses simple command substitution to capture output
	if !strings.Contains(shellenv, "output=$(command wt") {
		t.Error("Shell function must capture output via command substitution")
	}

	// Verify the fix: should extract cd_path from output
	if !strings.Contains(shellenv, "cd_path=$(printf") {
		t.Error("Shell function must extract cd_path from navigation marker in output")
	}

	// Verify no script(1)/PTY complexity remains
	if strings.Contains(shellenv, "script -q") {
		t.Error("Shell function must not use script(1) for PTY allocation")
	}
}

// TestShellenvZshCompdefProtection tests that compdef is only called
// when it's available, preventing "command not found: compdef" errors.
//
// BUG: Currently fails because compdef is called unconditionally
func TestShellenvZshCompdefProtection(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "shellenv")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run wt shellenv: %v", err)
	}
	shellenv := string(output)

	// The BUG: compdef is called unconditionally, failing when zsh completion isn't loaded
	hasUnprotectedCompdef := strings.Contains(shellenv, "compdef _wt_complete_zsh wt") &&
		!strings.Contains(shellenv, "if (( $+functions[compdef] ))")

	if hasUnprotectedCompdef {
		t.Fatal("BUG DETECTED: compdef is called unconditionally in zsh completion.\n" +
			"This causes 'command not found: compdef' error when sourcing shellenv before compinit.\n" +
			"EXPECTED: Check if compdef is available before calling it: if (( $+functions[compdef] )); then compdef ...; fi")
	}

	// Verify the fix: should have protection check
	if !strings.Contains(shellenv, "if (( $+functions[compdef] ))") {
		t.Error("Zsh completion must check if compdef is available before calling it")
	}

	// Verify compdef is still present (for when it IS available)
	if !strings.Contains(shellenv, "compdef _wt_complete_zsh wt") {
		t.Error("Zsh completion should still call compdef when available")
	}
}

// TestShellenvZshCompdefError tests that the shellenv can be sourced
// in zsh without errors when compdef is not available (integration test)
func TestShellenvZshCompdefError(t *testing.T) {
	// Run shellenv and try to source it in a fresh zsh shell (without compinit)
	// This simulates the real-world error condition
	cmd := exec.Command("zsh", "-c", "source <(go run . shellenv) 2>&1 && type wt")
	output, err := cmd.CombinedOutput()

	// Check for compdef error - this is the BUG we're testing for
	if strings.Contains(string(output), "command not found: compdef") {
		t.Error("BUG: Shellenv produces 'command not found: compdef' error in zsh without compinit.\n" +
			"This happens when user sources shellenv before running compinit in their .zshrc")
	}

	// Should still define the wt function even if completion setup fails
	if err == nil && !strings.Contains(string(output), "wt is a shell function") {
		t.Log("Warning: Shell function should be defined even when compdef is not available")
	}
}

// TestShellenvCompletionCheckoutUsesAllBranches verifies that tab completion
// for checkout/co/create completes from all branches (not just existing worktrees).
func TestShellenvCompletionCheckoutUsesAllBranches(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "shellenv")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run wt shellenv: %v", err)
	}
	shellenv := string(output)

	// Bash: checkout/co/create should use git branch -a, not git worktree list
	if !strings.Contains(shellenv, `checkout|co|create)`) {
		t.Error("Bash completion should handle checkout, co, and create together")
	}

	// Zsh: same grouping
	if !strings.Contains(shellenv, `checkout|co|create)`) {
		t.Error("Zsh completion should handle checkout, co, and create together")
	}

	// Should use git branch -a for checkout/co/create completions
	if !strings.Contains(shellenv, `git branch -a --format='%(refname:short)'`) {
		t.Error("Completion for checkout/co/create should use 'git branch -a' to list all branches")
	}

	// Should dynamically get remote names for prefix stripping
	if !strings.Contains(shellenv, `git remote`) {
		t.Error("Completion should query 'git remote' to strip remote prefixes dynamically")
	}
}

// TestShellenvCompletionRemoveUsesWorktreeList verifies that tab completion
// for remove/rm only completes from existing worktrees (not all branches).
func TestShellenvCompletionRemoveUsesWorktreeList(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "shellenv")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run wt shellenv: %v", err)
	}
	shellenv := string(output)

	// Bash: remove/rm should still use git worktree list
	if !strings.Contains(shellenv, `remove|rm)`) {
		t.Error("Bash completion should handle remove and rm together")
	}

	// The worktree list pattern should still be present (for remove/rm)
	if !strings.Contains(shellenv, `git worktree list 2>/dev/null | tail -n +2`) {
		t.Error("Completion for remove/rm should use 'git worktree list' to list existing worktrees")
	}
}

// TestShellenvSimpleWrapper ensures the shell wrapper uses simple command
// substitution without PTY/script(1) complexity.
func TestShellenvSimpleWrapper(t *testing.T) {
	cmd := exec.Command("go", "run", ".", "shellenv")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to run wt shellenv: %v", err)
	}
	shellenv := string(output)

	// Must use simple command substitution
	if !strings.Contains(shellenv, `output=$(command wt "$@")`) {
		t.Fatal("shell wrapper must use command substitution to capture output")
	}

	// Must not use script(1) or temp files
	if strings.Contains(shellenv, "script -q") || strings.Contains(shellenv, "mktemp") {
		t.Fatal("shell wrapper must not use script(1) or temp files")
	}
}
