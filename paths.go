package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

func buildWorktreePath(info repoInfo, branch string) (string, error) {
	rendered, err := renderWorktreePath(info, branch)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(rendered)
	infoStat, err := os.Stat(parent)
	switch {
	case err == nil:
		if !infoStat.IsDir() {
			return "", fmt.Errorf("worktree path %s is not a directory", parent)
		}
	case os.IsNotExist(err):
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", fmt.Errorf("failed to create worktree directory %s: %w", parent, err)
		}
	default:
		return "", fmt.Errorf("failed to access worktree directory %s: %w", parent, err)
	}

	return rendered, nil
}

func renderWorktreePath(info repoInfo, branch string) (string, error) {
	pattern, err := resolveWorktreePattern()
	if err != nil {
		return "", err
	}

	sep := appCfg.Separator

	// transformValue replaces "/" and "\" with the configured separator.
	transformValue := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(s, "/", sep), "\\", sep)
	}

	envMap := map[string]string{}
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = transformValue(parts[1])
		}
	}

	context := map[string]any{
		"repo": repoInfo{
			Main:  info.Main, // full path — NOT transformed
			Host:  info.Host,
			Owner: transformValue(info.Owner),
			Name:  info.Name,
		},
		"branch":       strings.TrimSpace(transformValue(branch)),
		"worktreeRoot": appCfg.Root, // full path — NOT transformed
		"env":          envMap,
	}

	if pattern == "" {
		return "", fmt.Errorf("worktree pattern cannot be empty")
	}

	tpl, err := template.New("worktreePattern").
		Delims("{", "}").
		Option("missingkey=error").
		Parse(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid worktree pattern: %w", err)
	}

	var renderedBuf bytes.Buffer
	if err := tpl.Execute(&renderedBuf, context); err != nil {
		return "", fmt.Errorf("pattern variables missing values: %w", err)
	}

	rendered := renderedBuf.String()
	rendered = filepath.FromSlash(rendered)
	if !filepath.IsAbs(rendered) {
		rendered = filepath.Join(appCfg.Root, rendered)
	}

	rendered = filepath.Clean(rendered)
	return rendered, nil
}

func cleanupWorktreePath(worktreePath string) error {
	if worktreePath == "" {
		return nil
	}

	if err := os.RemoveAll(worktreePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove worktree directory %s: %w", worktreePath, err)
	}

	absRoot, err := filepath.Abs(appCfg.Root)
	if err != nil {
		return nil
	}

	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return nil
	}

	repoDir := filepath.Dir(absWorktreePath)
	if strings.HasPrefix(repoDir, absRoot) {
		if empty, err := isDirEmpty(repoDir); err == nil && empty {
			_ = os.Remove(repoDir)
		}
	}

	return nil
}

func resolveWorktreePattern() (string, error) {
	if appCfg.Pattern != "" {
		return appCfg.Pattern, nil
	}
	if appCfg.Strategy == "custom" {
		return "", fmt.Errorf("WORKTREE_PATTERN is required when WORKTREE_STRATEGY is 'custom'")
	}

	switch appCfg.Strategy {
	case "global":
		return "{.worktreeRoot}/{.repo.Name}/{.branch}", nil
	case "sibling-repo", "sibling":
		return "{.repo.Main}/../{.repo.Name}-{.branch}", nil
	case "parent-worktrees", "parent-centered":
		return "{.repo.Main}/../{.repo.Name}.worktrees/{.branch}", nil
	case "parent-branches", "repo-root":
		return "{.repo.Main}/../{.branch}", nil
	case "parent-dotdir", "local-root":
		return "{.repo.Main}/../.worktrees/{.branch}", nil
	case "inside-dotdir", "nested-local":
		return "{.repo.Main}/.worktrees/{.branch}", nil
	default:
		return "", fmt.Errorf("unsupported WORKTREE_STRATEGY: %s", appCfg.Strategy)
	}
}

func printCDMarker(path string) {
	if isJSONOutput() {
		return
	}
	fmt.Printf("wt navigating to: %s\n", path)
}
