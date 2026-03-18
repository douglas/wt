package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config represents the wt configuration file structure.
type Config struct {
	Root      string `toml:"root"`
	Strategy  string `toml:"strategy"`
	Pattern   string `toml:"pattern"`
	Separator string `toml:"separator"`
	Hooks     Hooks  `toml:"hooks"`
}

// Hooks holds pre/post command hook commands.
type Hooks struct {
	PreCreate    []string `toml:"pre_create"`
	PostCreate   []string `toml:"post_create"`
	PreCheckout  []string `toml:"pre_checkout"`
	PostCheckout []string `toml:"post_checkout"`
	PreRemove    []string `toml:"pre_remove"`
	PostRemove   []string `toml:"post_remove"`
	PrePR        []string `toml:"pre_pr"`
	PostPR       []string `toml:"post_pr"`
	PreMR        []string `toml:"pre_mr"`
	PostMR       []string `toml:"post_mr"`
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
	ConfigFilePath  string
	ConfigFileFound bool
	ConfigSources   configSource
	OutputFormat    string
}

// appCfg is the package-level application configuration.
var appCfg = AppConfig{
	OutputFormat: formatText,
}

// configFlag is the --config flag value (set by cobra).
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
# Available env vars in hooks: $WT_PATH, $WT_BRANCH, $WT_MAIN,
#                              $WT_REPO_NAME, $WT_REPO_HOST, $WT_REPO_OWNER
# Pre-hooks abort on failure; post-hooks warn only.
# Set WT_HOOKS_DISABLED=1 to skip all hooks.
#
# [hooks]
# post_create = ["test -f $WT_MAIN/.env && cp $WT_MAIN/.env $WT_PATH/.env || true"]
# post_checkout = ["cd $WT_PATH && npm install"]
# pre_remove = ["echo Removing $WT_PATH"]
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

	// Reset hooks
	appCfg.Hooks = Hooks{}

	// 2. Load config file
	appCfg.ConfigFilePath = resolveConfigPath(configFlag)
	appCfg.ConfigFileFound = false

	if _, err := os.Stat(appCfg.ConfigFilePath); err == nil {
		appCfg.ConfigFileFound = true
		var cfg Config
		if _, err := toml.DecodeFile(appCfg.ConfigFilePath, &cfg); err == nil {
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

	if err := os.WriteFile(path, []byte(defaultConfigTemplate), 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
