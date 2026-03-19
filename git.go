package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// repoInfo holds repository metadata resolved from git remote and worktree state.
type repoInfo struct {
	Main  string
	Host  string
	Owner string
	Name  string
}

// worktreeListEntry represents a single entry from git worktree list --porcelain.
type worktreeListEntry struct {
	Path     string `json:"path"`
	HEAD     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	Detached bool   `json:"detached,omitempty"`
	Locked   string `json:"locked,omitempty"`
	Prunable string `json:"prunable,omitempty"`
}

// worktreeCache holds cached worktree list results for the current command.
var worktreeCache struct {
	entries []worktreeListEntry
	valid   bool
}

func resetWorktreeCache() {
	worktreeCache.entries = nil
	worktreeCache.valid = false
}

func getDefaultBase() string {
	cmd := gitCmd.Command("symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	ref := strings.TrimSpace(string(output))
	return strings.TrimPrefix(ref, "refs/remotes/origin/")
}

func getRepoInfo() (repoInfo, error) {
	cmd := gitCmd.Command("rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	var repoRoot string
	isBare := false
	if err == nil {
		repoRoot = strings.TrimSpace(string(output))
	} else {
		cmd = gitCmd.Command("rev-parse", "--is-bare-repository")
		output, err = cmd.Output()
		if err != nil || strings.TrimSpace(string(output)) != "true" {
			return repoInfo{}, fmt.Errorf("not in a git repository")
		}
		isBare = true
		cmd = gitCmd.Command("rev-parse", "--absolute-git-dir")
		output, err = cmd.Output()
		if err != nil {
			return repoInfo{}, fmt.Errorf("not in a git repository")
		}
		repoRoot = strings.TrimSpace(string(output))
	}
	repoName := ""
	remoteURL := ""
	cmd = gitCmd.Command("remote", "get-url", "origin")
	output, err = cmd.Output()
	if err == nil {
		remoteURL = strings.TrimSpace(string(output))
		if parsed, ok := parseRemoteURL(remoteURL); ok {
			repoName = parsed.Name
		}
	}
	if repoName == "" { //nolint:nestif // multi-source repo name resolution is inherently branchy
		repoName = strings.TrimSuffix(filepath.Base(repoRoot), ".git")
		if commonCmd := gitCmd.Command("rev-parse", "--git-common-dir"); commonCmd != nil {
			if commonOutput, commonErr := commonCmd.Output(); commonErr == nil {
				commonDir := strings.TrimSpace(string(commonOutput))
				if commonDir != "" {
					if !filepath.IsAbs(commonDir) {
						commonDir = filepath.Join(repoRoot, commonDir)
					}
					commonDir = filepath.Clean(commonDir)
					base := filepath.Base(commonDir)
					if base == ".git" {
						repoName = filepath.Base(filepath.Dir(commonDir))
					} else {
						repoName = strings.TrimSuffix(base, ".git")
					}
				}
			}
		}
	}
	info := repoInfo{
		Main: getMainWorktreePath(getDefaultBase(), repoName, repoRoot, isBare),
		Name: repoName,
	}

	if remoteURL != "" {
		if parsed, ok := parseRemoteURL(remoteURL); ok {
			info.Host = parsed.Host
			info.Owner = parsed.Owner
		}
	}

	return info, nil
}

func getMainWorktreePath(defaultBranch, repoName, repoRoot string, isBare bool) string {
	entries, err := getWorktreeListPorcelain()
	if err == nil && len(entries) > 0 {
		if defaultBranch != "" {
			for _, e := range entries {
				if e.Branch == defaultBranch {
					return e.Path
				}
			}
		}
		for _, e := range entries {
			if filepath.Base(e.Path) == repoName {
				return e.Path
			}
		}
		for _, e := range entries {
			if stat, err := os.Stat(filepath.Join(e.Path, ".git")); err == nil && stat.IsDir() {
				return e.Path
			}
		}
		return entries[0].Path
	}

	if isBare {
		return filepath.Join(filepath.Dir(repoRoot), repoName)
	}
	return repoRoot
}

func worktreeExists(branch string) (string, bool) {
	entries, err := getWorktreeListPorcelain()
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.Branch == branch {
			return e.Path, true
		}
	}
	return "", false
}

func branchExists(branch string) bool {
	// Check local branch
	cmd := gitCmd.Command("show-ref", "--verify", "--quiet", fmt.Sprintf("refs/heads/%s", branch))
	if cmd.Run() == nil {
		return true
	}

	// Check remote branch
	cmd = gitCmd.Command("show-ref", "--verify", "--quiet", fmt.Sprintf("refs/remotes/origin/%s", branch))
	return cmd.Run() == nil
}

func getWorktreeListPorcelain() ([]worktreeListEntry, error) {
	if worktreeCache.valid {
		return worktreeCache.entries, nil
	}

	cmd := gitCmd.Command("worktree", "list", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	entries := make([]worktreeListEntry, 0)
	current := worktreeListEntry{}

	for _, rawLine := range strings.Split(string(output), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			if current.Path != "" {
				entries = append(entries, current)
				current = worktreeListEntry{}
			}
			continue
		}

		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				entries = append(entries, current)
			}
			current = worktreeListEntry{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		case line == "bare":
			current.Bare = true
		case line == "detached":
			current.Detached = true
		case strings.HasPrefix(line, "locked"):
			current.Locked = strings.TrimSpace(strings.TrimPrefix(line, "locked"))
		case strings.HasPrefix(line, "prunable"):
			current.Prunable = strings.TrimSpace(strings.TrimPrefix(line, "prunable"))
		}
	}

	if current.Path != "" {
		entries = append(entries, current)
	}

	worktreeCache.entries = entries
	worktreeCache.valid = true
	return entries, nil
}

func getAvailableBranches() ([]string, error) {
	// Get local and remote branches
	cmd := gitCmd.Command("branch", "-a", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Use a map to deduplicate
	branchMap := make(map[string]bool)

	for _, line := range strings.Split(string(output), "\n") {
		branch := strings.TrimSpace(line)
		if branch == "" {
			continue
		}

		// Skip remote HEAD pointers and detached HEAD states
		if strings.HasPrefix(branch, "origin/HEAD") || strings.Contains(branch, "->") || strings.Contains(branch, "HEAD") {
			continue
		}

		// For remote branches, strip the origin/ prefix
		branch = strings.TrimPrefix(branch, "origin/")

		// Skip if branch name is just "origin" or other remote names
		if branch == "origin" || branch == "upstream" {
			continue
		}

		// Add to map (deduplicates automatically)
		branchMap[branch] = true
	}

	// Convert map to slice
	branches := []string{}
	for branch := range branchMap {
		branches = append(branches, branch)
	}

	return branches, nil
}

func getExistingWorktreeBranches() ([]string, error) {
	entries, err := getWorktreeListPorcelain()
	if err != nil {
		return nil, err
	}

	if len(entries) <= 1 {
		return nil, nil
	}

	var branches []string
	for _, e := range entries[1:] { // Skip main worktree
		if e.Branch != "" {
			branches = append(branches, e.Branch)
		}
	}
	return branches, nil
}

func getMergedBranches(base string) ([]string, error) {
	cmd := gitCmd.Command("branch", "--merged", base, "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get merged branches: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		// Skip empty lines and base branches
		if branch == "" || branch == base || branch == "main" || branch == "master" {
			continue
		}
		branches = append(branches, branch)
	}
	return branches, nil
}

func isDirEmpty(path string) (bool, error) {
	dir, err := os.Open(path)
	switch {
	case os.IsNotExist(err):
		return true, nil
	case err != nil:
		return false, err
	}
	defer dir.Close()

	_, err = dir.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}
