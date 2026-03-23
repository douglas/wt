package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config represents the wt configuration file structure.
type Config struct {
	Root      string
	Strategy  string
	Pattern   string
	Separator string
	Hooks     Hooks
	CopyFiles CopyFiles
}

// CopyFiles holds the list of file paths to copy from the main worktree
// into newly created worktrees.
type CopyFiles struct {
	Paths []string
}

// Hooks holds pre/post command hook commands.
type Hooks struct {
	PreCreate    []string
	PostCreate   []string
	PreCheckout  []string
	PostCheckout []string
	PreRemove    []string
	PostRemove   []string
	PrePR        []string
	PostPR       []string
	PreMR        []string
	PostMR       []string
}

// configSource tracks where each config value came from.
type configSource struct {
	Root      string
	Strategy  string
	Pattern   string
	Separator string
}

// AppConfig holds all resolved configuration state for the application.
type AppConfig struct {
	Root            string
	Strategy        string
	Pattern         string
	Separator       string
	Hooks           Hooks
	CopyFiles       CopyFiles
	ConfigFilePath  string
	ConfigFileFound bool
	ConfigSources   configSource
	OutputFormat    string
}

// appCfg is the package-level application configuration.
var appCfg = AppConfig{
	OutputFormat: formatText,
}

// configFlag is the --config flag value (set by extractGlobalFlags).
var configFlag string

// defaultConfigTemplate is the content written by `wt config init`.
const defaultConfigTemplate = `# wt configuration file
# See: https://github.com/timvw/wt#configuration

# Root directory for worktrees (default: ~/dev/worktrees)
# root = "~/dev/worktrees"

# Worktree placement strategy
# Options: global, sibling-repo, parent-branches, parent-worktrees,
#          parent-dotdir, inside-dotdir, custom
# strategy = "global"

# Custom pattern (used when strategy = "custom", or to override any strategy's default)
# Available variables: {.worktreeRoot}, {.repo.Name}, {.repo.Main},
#                      {.repo.Owner}, {.repo.Host}, {.branch},
#                      {.env.VARNAME} (access environment variables, e.g. {.env.USER})
# pattern = "{.worktreeRoot}/{.repo.Name}/{.branch}"

# Separator replaces "/" and "\" in template value variables ({.branch}, {.repo.Owner}, {.env.*})
# Default is "/" (no transformation — slashes create subdirectories).
# Set to "-" or "_" for flat paths (e.g. feat/foo -> feat-foo).
# Does NOT affect path variables ({.repo.Main}, {.worktreeRoot}).
# separator = "/"

# Example: group worktrees by a FEATURE environment variable
# strategy = "custom"
# pattern = "{.worktreeRoot}/{.env.FEATURE}/{.repo.Name}"

# Hooks — run commands before/after wt operations
# Security: Hook commands run with your user privileges via sh -c.
# Always quote variables in hooks: "$WT_BRANCH", "$WT_PATH"
# Available env vars in hooks: $WT_PATH, $WT_BRANCH, $WT_MAIN,
#                              $WT_REPO_NAME, $WT_REPO_HOST, $WT_REPO_OWNER
# Pre-hooks abort on failure; post-hooks warn only.
# Set WT_HOOKS_DISABLED=1 to skip all hooks.
#
# [hooks]
# post_create = ["test -f $WT_MAIN/.env && cp $WT_MAIN/.env $WT_PATH/.env || true"]
# post_checkout = ["cd $WT_PATH && npm install"]
# pre_remove = ["echo Removing $WT_PATH"]

# Copy files — copy files from the main worktree into new worktrees.
# Files that don't exist in the main worktree are skipped with a warning.
#
# [copy_files]
# paths = [".env", ".tool-versions", ".envrc"]
`

// configDir returns the directory where wt config files are stored.
func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "wt")
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("APPDATA"); d != "" {
			return filepath.Join(d, "wt")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "wt")
}

// resolveConfigPath determines which config file to use.
// Priority: --config flag > WT_CONFIG env var > default location.
func resolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envPath := os.Getenv("WT_CONFIG"); envPath != "" {
		return envPath
	}
	return filepath.Join(configDir(), "config.toml")
}

// loadWorktreeConfig loads configuration from file and environment variables.
// Precedence: env vars > config file > defaults.
func loadWorktreeConfig() {
	// 1. Start with defaults
	home, _ := os.UserHomeDir()
	defaultRoot := filepath.Join(home, "dev", "worktrees")

	appCfg.Root = defaultRoot
	appCfg.Strategy = "global"
	appCfg.Pattern = ""
	appCfg.Separator = "/"

	appCfg.ConfigSources = configSource{
		Root:      "default",
		Strategy:  "default",
		Pattern:   "default",
		Separator: "default",
	}

	// Reset hooks and copy_files
	appCfg.Hooks = Hooks{}
	appCfg.CopyFiles = CopyFiles{}

	// 2. Load config file
	appCfg.ConfigFilePath = resolveConfigPath(configFlag)
	appCfg.ConfigFileFound = false

	if _, err := os.Stat(appCfg.ConfigFilePath); err == nil {
		appCfg.ConfigFileFound = true
		if cfg, err := parseConfigFile(appCfg.ConfigFilePath); err == nil {
			if cfg.Root != "" {
				appCfg.Root = expandHome(cfg.Root)
				appCfg.ConfigSources.Root = "config file"
			}
			if cfg.Strategy != "" {
				appCfg.Strategy = strings.ToLower(strings.TrimSpace(cfg.Strategy))
				appCfg.ConfigSources.Strategy = "config file"
			}
			if cfg.Pattern != "" {
				appCfg.Pattern = strings.TrimSpace(cfg.Pattern)
				appCfg.ConfigSources.Pattern = "config file"
			}
			if cfg.Separator != "" {
				appCfg.Separator = cfg.Separator
				appCfg.ConfigSources.Separator = "config file"
			}
			appCfg.Hooks = cfg.Hooks
			appCfg.CopyFiles = cfg.CopyFiles
		}
	}

	// 3. Environment variables override config file
	if v := os.Getenv("WORKTREE_ROOT"); v != "" {
		appCfg.Root = v
		appCfg.ConfigSources.Root = "env: WORKTREE_ROOT"
	}
	if v := os.Getenv("WORKTREE_STRATEGY"); v != "" {
		appCfg.Strategy = strings.ToLower(strings.TrimSpace(v))
		appCfg.ConfigSources.Strategy = "env: WORKTREE_STRATEGY"
	}
	if v := os.Getenv("WORKTREE_PATTERN"); v != "" {
		appCfg.Pattern = strings.TrimSpace(v)
		appCfg.ConfigSources.Pattern = "env: WORKTREE_PATTERN"
	}
	if v, ok := os.LookupEnv("WORKTREE_SEPARATOR"); ok {
		appCfg.Separator = v
		appCfg.ConfigSources.Separator = "env: WORKTREE_SEPARATOR"
	}
}

// parseConfigFile reads a TOML config file into a Config struct.
// Handles the limited subset used by wt: string values, section headers,
// and string arrays.
func parseConfigFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	var cfg Config
	section := ""
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		// Key = value
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		// Parse value: string array or quoted string
		if strings.HasPrefix(val, "[") {
			arr := parseStringArray(val)
			switch {
			case section == "hooks":
				setHookField(&cfg.Hooks, section, key, arr)
			case section == "copy_files" && key == "paths":
				cfg.CopyFiles.Paths = arr
			}
		} else {
			s := unquoteString(val)
			if section == "" {
				switch key {
				case "root":
					cfg.Root = s
				case "strategy":
					cfg.Strategy = s
				case "pattern":
					cfg.Pattern = s
				case "separator":
					cfg.Separator = s
				}
			}
		}
	}

	return cfg, scanner.Err()
}

// unquoteString removes surrounding double quotes from a TOML string value.
func unquoteString(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseStringArray parses a TOML inline array of strings like ["a", "b"].
func parseStringArray(s string) []string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil
	}
	s = s[1 : len(s)-1]
	if strings.TrimSpace(s) == "" {
		return nil
	}

	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		result = append(result, unquoteString(item))
	}
	return result
}

// setHookField sets the appropriate Hooks field based on section and key.
func setHookField(hooks *Hooks, section, key string, val []string) {
	if section != "hooks" {
		return
	}
	switch key {
	case "pre_create":
		hooks.PreCreate = val
	case "post_create":
		hooks.PostCreate = val
	case "pre_checkout":
		hooks.PreCheckout = val
	case "post_checkout":
		hooks.PostCheckout = val
	case "pre_remove":
		hooks.PreRemove = val
	case "post_remove":
		hooks.PostRemove = val
	case "pre_pr":
		hooks.PrePR = val
	case "post_pr":
		hooks.PostPR = val
	case "pre_mr":
		hooks.PreMR = val
	case "post_mr":
		hooks.PostMR = val
	}
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// writeDefaultConfig creates a default config file at the given path.
func writeDefaultConfig(path string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists: %s (use --force to overwrite)", path)
		}
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
