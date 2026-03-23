package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// RemoteType identifies the hosting platform for PR/MR operations.
type RemoteType int

const (
	RemoteGitHub RemoteType = iota
	RemoteGitLab
	RemoteUnknown
)

func parseRemoteURL(remoteURL string) (repoInfo, bool) {
	trimmed := strings.TrimSpace(remoteURL)
	if trimmed == "" {
		return repoInfo{}, false
	}

	if strings.Contains(trimmed, "://") {
		parsed, err := url.Parse(trimmed)
		if err != nil || parsed.Hostname() == "" {
			return repoInfo{}, false
		}
		host := parsed.Hostname()
		path := strings.Trim(parsed.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) < 2 {
			return repoInfo{}, false
		}
		repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
		owner := strings.Join(parts[:len(parts)-1], "/")
		return repoInfo{Host: host, Owner: owner, Name: repo}, true
	}

	if scpLike := strings.SplitN(trimmed, ":", 2); len(scpLike) == 2 {
		hostPart := scpLike[0]
		path := scpLike[1]
		if atIdx := strings.LastIndex(hostPart, "@"); atIdx != -1 {
			hostPart = hostPart[atIdx+1:]
		}
		host := hostPart
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 2 {
			return repoInfo{}, false
		}
		repo := strings.TrimSuffix(parts[len(parts)-1], ".git")
		owner := strings.Join(parts[:len(parts)-1], "/")
		return repoInfo{Host: host, Owner: owner, Name: repo}, true
	}

	return repoInfo{}, false
}

func getPRNumber(input string) (string, error) {
	// Check if it's a GitHub PR URL
	githubRegex := regexp.MustCompile(`^https://github\.com/.*/pull/([0-9]+)`)
	if matches := githubRegex.FindStringSubmatch(input); matches != nil {
		return matches[1], nil
	}

	// Check if it's a GitLab MR URL
	gitlabRegex := regexp.MustCompile(`^https://gitlab\.com/.*/-/merge_requests/([0-9]+)`)
	if matches := gitlabRegex.FindStringSubmatch(input); matches != nil {
		return matches[1], nil
	}

	// Check if it's just a number
	numRegex := regexp.MustCompile(`^[0-9]+$`)
	if numRegex.MatchString(input) {
		return input, nil
	}

	return "", fmt.Errorf("invalid PR/MR number or URL: %s", input)
}

func parsePROutput(output string) ([]string, []string) {
	var numbers []string
	var labels []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			numbers = append(numbers, parts[0])
			labels = append(labels, fmt.Sprintf("#%s: %s", parts[0], parts[1]))
		}
	}
	return numbers, labels
}

func getOpenPRs() ([]string, []string, error) {
	cmd := extCmd.Command("gh", "pr", "list", "--json", "number,title", "--jq", ".[] | \"\\(.number)\\t\\(.title)\"")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	numbers, labels := parsePROutput(string(output))
	return numbers, labels, nil
}

func parseMROutput(output string) ([]string, []string) {
	var numbers []string
	var labels []string
	// Parse glab output: !123  title  (branch) ← (target)
	mrRegex := regexp.MustCompile(`^!(\d+)\s+[^\s]+\s+(.+?)\s+\(`)
	for _, line := range strings.Split(output, "\n") {
		if matches := mrRegex.FindStringSubmatch(line); matches != nil {
			numbers = append(numbers, matches[1])
			labels = append(labels, fmt.Sprintf("!%s: %s", matches[1], strings.TrimSpace(matches[2])))
		}
	}
	return numbers, labels
}

func getOpenMRs() ([]string, []string, error) {
	cmd := extCmd.Command("glab", "mr", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, nil, err
	}

	numbers, labels := parseMROutput(string(output))
	return numbers, labels, nil
}

// parseGitHubBranchName extracts the branch name from gh pr view JSON output.
func parseGitHubBranchName(jsonOutput string) (string, error) {
	var result struct {
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return "", fmt.Errorf("failed to parse GitHub PR JSON: %w", err)
	}
	if result.HeadRefName == "" {
		return "", fmt.Errorf("empty branch name in GitHub PR response")
	}
	return result.HeadRefName, nil
}

// parseGitLabBranchName extracts the branch name from glab mr view JSON output.
func parseGitLabBranchName(jsonOutput string) (string, error) {
	var result struct {
		SourceBranch string `json:"source_branch"`
	}
	if err := json.Unmarshal([]byte(jsonOutput), &result); err != nil {
		return "", fmt.Errorf("failed to parse GitLab MR JSON: %w", err)
	}
	if result.SourceBranch == "" {
		return "", fmt.Errorf("empty branch name in GitLab MR response")
	}
	return result.SourceBranch, nil
}

// getPRBranchName looks up the actual branch name for a PR/MR using the gh/glab CLI.
func getPRBranchName(prNumber string, remoteType RemoteType) (string, error) {
	switch remoteType {
	case RemoteGitHub:
		cmd := extCmd.Command("gh", "pr", "view", prNumber, "--json", "headRefName")
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get PR branch name: %w", err)
		}
		return parseGitHubBranchName(string(output))
	case RemoteGitLab:
		cmd := extCmd.Command("glab", "mr", "view", prNumber, "--output", "json")
		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get MR branch name: %w", err)
		}
		return parseGitLabBranchName(string(output))
	case RemoteUnknown:
		return "", fmt.Errorf("invalid remote type")
	}
	return "", fmt.Errorf("invalid remote type")
}

func checkoutPROrMR(cmdName string, input string, remoteType RemoteType) error {
	jsonMode := isJSONOutput()
	prNumber, err := getPRNumber(input)
	if err != nil {
		return err
	}

	var refSpec, prefix string

	switch remoteType {
	case RemoteGitHub:
		refSpec = fmt.Sprintf("pull/%s/head", prNumber)
		prefix = "pr"
		if _, err := lookPathFunc("gh"); err != nil {
			return fmt.Errorf("'gh' CLI not found. Install it from https://cli.github.com")
		}
	case RemoteGitLab:
		refSpec = fmt.Sprintf("merge-requests/%s/head", prNumber)
		prefix = "mr"
		if _, err := lookPathFunc("glab"); err != nil {
			return fmt.Errorf("'glab' CLI not found. Install it from https://gitlab.com/gitlab-org/cli")
		}
	case RemoteUnknown:
		return fmt.Errorf("invalid remote type")
	}

	// Look up the actual branch name from the PR/MR
	branch, err := getPRBranchName(prNumber, remoteType)
	if err != nil {
		return fmt.Errorf("failed to look up branch for %s #%s: %w", strings.ToUpper(prefix), prNumber, err)
	}

	info, err := getRepoInfo()
	if err != nil {
		return err
	}

	// Check if worktree already exists for this branch
	if existingPath, exists := worktreeExists(branch); exists {
		if jsonMode {
			return emitJSONSuccess(cmdName, map[string]any{
				"status":      "exists",
				"id":          prNumber,
				"kind":        prefix,
				"branch":      branch,
				"path":        existingPath,
				"navigate_to": existingPath,
			})
		}
		fmt.Printf("✓ Worktree already exists: %s\n", existingPath)
		printCDMarker(existingPath)
		return nil
	}

	path, err := buildWorktreePath(info, branch)
	if err != nil {
		return err
	}

	// Determine hook name based on remote type
	hookPrefix := "pr"
	if remoteType == RemoteGitLab {
		hookPrefix = "mr"
	}
	hookEnv := buildHookEnv(info, branch, path)

	// Run pre-pr/pre-mr hooks
	preHookName := "pre_" + hookPrefix
	if err := runHooks(preHookName, getHooks(preHookName), hookEnv); err != nil {
		return fmt.Errorf("%s hook failed: %w", preHookName, err)
	}

	// Try fetching the branch directly from origin
	fetchCmd := gitCmd.Command("fetch", "origin", branch)
	fetchCmd.Stderr = os.Stderr
	if err := fetchCmd.Run(); err != nil {
		// Fallback: fetch via PR/MR refspec (e.g. for fork PRs)
		fallbackCmd := gitCmd.Command("fetch", "origin", fmt.Sprintf("%s:%s", refSpec, branch))
		fallbackCmd.Stderr = os.Stderr
		_ = fallbackCmd.Run()
	}

	// Create worktree — prefer the remote-tracking branch, fall back to local
	addCmd := gitCmd.Command("worktree", "add", path, branch)
	if !branchExists(branch) {
		addCmd = gitCmd.Command("worktree", "add", path, "-b", branch, fmt.Sprintf("origin/%s", branch))
	}
	if !jsonMode {
		addCmd.Stdout = os.Stdout
		addCmd.Stderr = os.Stderr
	}
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}
	resetWorktreeCache()

	// Ensure upstream tracking is set (best-effort, may fail for fork PRs)
	upstreamCmd := gitCmd.Command("branch", "--set-upstream-to",
		fmt.Sprintf("origin/%s", branch), branch)
	_ = upstreamCmd.Run()

	// Run post-pr/post-mr hooks (warn only)
	postHookName := "post_" + hookPrefix
	_ = runHooks(postHookName, getHooks(postHookName), hookEnv)

	// Copy files from main worktree if configured
	if len(appCfg.CopyFiles.Paths) > 0 {
		_ = copyFilesToWorktree(info.Main, path, appCfg.CopyFiles.Paths)
	}

	if jsonMode {
		return emitJSONSuccess(cmdName, map[string]any{
			"status":      "created",
			"id":          prNumber,
			"kind":        prefix,
			"branch":      branch,
			"path":        path,
			"navigate_to": path,
		})
	}

	fmt.Printf("✓ %s #%s (%s) checked out at: %s\n", strings.ToUpper(prefix), prNumber, branch, path)

	printCDMarker(path)
	return nil
}
