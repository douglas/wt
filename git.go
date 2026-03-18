package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type repoInfo struct {
	Main  string
	Host  string
	Owner string
	Name  string
}

type worktreeListEntry struct {
	Path     string `json:"path"`
	HEAD     string `json:"head,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Bare     bool   `json:"bare,omitempty"`
	Detached bool   `json:"detached,omitempty"`
	Locked   string `json:"locked,omitempty"`
	Prunable string `json:"prunable,omitempty"`
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
	if repoName == "" {
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
	cmd := gitCmd.Command("worktree", "list")
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}

	lines := strings.Split(string(output), "\n")
	searchPattern := fmt.Sprintf("[%s]", branch)
	for _, line := range lines {
		if strings.Contains(line, searchPattern) {
			// Extract the path (first field)
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], true
			}
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
	cmd := gitCmd.Command("worktree", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	branches := []string{}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines[1:] { // Skip first line (main worktree)
		if line == "" {
			continue
		}
		// Extract branch name from [branch] format
		if matches := regexp.MustCompile(`\[([^\]]+)\]`).FindStringSubmatch(line); matches != nil {
			branches = append(branches, matches[1])
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
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
