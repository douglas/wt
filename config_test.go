package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestConfigDir(t *testing.T) {
	// Save and restore XDG_CONFIG_HOME
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		os.Setenv("XDG_CONFIG_HOME", origXDG)
	})

	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		got := configDir()
		want := filepath.Join("/custom/config", "wt")
		if got != want {
			t.Errorf("configDir() = %q, want %q", got, want)
		}
	})

	t.Run("uses default when XDG_CONFIG_HOME is empty", func(t *testing.T) {
		os.Setenv("XDG_CONFIG_HOME", "")
		got := configDir()
		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "wt")
		if runtime.GOOS == "windows" {
			if appdata := os.Getenv("APPDATA"); appdata != "" {
				want = filepath.Join(appdata, "wt")
			}
		}
		if got != want {
			t.Errorf("configDir() = %q, want %q", got, want)
		}
	})
}

func TestResolveConfigPath(t *testing.T) {
	origEnv := os.Getenv("WT_CONFIG")
	t.Cleanup(func() {
		os.Setenv("WT_CONFIG", origEnv)
	})

	t.Run("flag takes highest priority", func(t *testing.T) {
		os.Setenv("WT_CONFIG", "/env/config.toml")
		got := resolveConfigPath("/flag/config.toml")
		if got != "/flag/config.toml" {
			t.Errorf("resolveConfigPath() = %q, want /flag/config.toml", got)
		}
	})

	t.Run("env var used when no flag", func(t *testing.T) {
		os.Setenv("WT_CONFIG", "/env/config.toml")
		got := resolveConfigPath("")
		if got != "/env/config.toml" {
			t.Errorf("resolveConfigPath() = %q, want /env/config.toml", got)
		}
	})

	t.Run("default path when no flag and no env", func(t *testing.T) {
		os.Setenv("WT_CONFIG", "")
		got := resolveConfigPath("")
		if !strings.HasSuffix(got, filepath.Join("wt", "config.toml")) {
			t.Errorf("resolveConfigPath() = %q, want suffix wt/config.toml", got)
		}
	})
}

func TestExpandHome(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "expands ~/path",
			path: "~/projects/worktrees",
			want: filepath.Join(home, "projects", "worktrees"),
		},
		{
			name: "expands ~ alone",
			path: "~",
			want: home,
		},
		{
			name: "does not expand ~user",
			path: "~otheruser/path",
			want: "~otheruser/path",
		},
		{
			name: "absolute path unchanged",
			path: "/absolute/path",
			want: "/absolute/path",
		},
		{
			name: "relative path unchanged",
			path: "relative/path",
			want: "relative/path",
		},
		{
			name: "empty string unchanged",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := expandHome(tt.path)
			if got != tt.want {
				t.Errorf("expandHome(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestWriteDefaultConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates config file", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "wt", "config.toml")

		err := writeDefaultConfig(path, false)
		if err != nil {
			t.Fatalf("writeDefaultConfig() error = %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read config file: %v", err)
		}

		if !strings.Contains(string(content), "wt configuration file") {
			t.Error("config file does not contain expected header")
		}
		if !strings.Contains(string(content), "strategy") {
			t.Error("config file does not contain strategy setting")
		}
	})

	t.Run("fails if file exists without force", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.toml")

		if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := writeDefaultConfig(path, false)
		if err == nil {
			t.Fatal("expected error when file exists, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' in error, got: %v", err)
		}
	})

	t.Run("overwrites with force", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.toml")

		if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
			t.Fatal(err)
		}

		err := writeDefaultConfig(path, true)
		if err != nil {
			t.Fatalf("writeDefaultConfig(force=true) error = %v", err)
		}

		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "wt configuration file") {
			t.Error("config file not overwritten with default content")
		}
	})

	t.Run("mkdirall failure", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		// Place a file where MkdirAll needs a directory.
		blocker := filepath.Join(tmpDir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(blocker, "nested", "config.toml")
		err := writeDefaultConfig(path, false)
		if err == nil {
			t.Fatal("expected error when MkdirAll fails")
		}
		if !strings.Contains(err.Error(), "failed to create config directory") {
			t.Errorf("error = %q, want 'failed to create config directory'", err)
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "deep", "nested", "config.toml")

		err := writeDefaultConfig(path, false)
		if err != nil {
			t.Fatalf("writeDefaultConfig() error = %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Fatalf("config file not created at %s", path)
		}
	})
}

func TestLoadWorktreeConfig(t *testing.T) {
	// Save original state
	origRoot := appCfg.Root
	origStrategy := appCfg.Strategy
	origPattern := appCfg.Pattern
	origConfigFlag := configFlag
	origConfigFilePath := appCfg.ConfigFilePath
	origConfigFileFound := appCfg.ConfigFileFound
	origConfigSources := appCfg.ConfigSources
	origEnvRoot := os.Getenv("WORKTREE_ROOT")
	origEnvStrategy := os.Getenv("WORKTREE_STRATEGY")
	origEnvPattern := os.Getenv("WORKTREE_PATTERN")
	origEnvConfig := os.Getenv("WT_CONFIG")

	t.Cleanup(func() {
		appCfg.Root = origRoot
		appCfg.Strategy = origStrategy
		appCfg.Pattern = origPattern
		configFlag = origConfigFlag
		appCfg.ConfigFilePath = origConfigFilePath
		appCfg.ConfigFileFound = origConfigFileFound
		appCfg.ConfigSources = origConfigSources
		os.Setenv("WORKTREE_ROOT", origEnvRoot)
		os.Setenv("WORKTREE_STRATEGY", origEnvStrategy)
		os.Setenv("WORKTREE_PATTERN", origEnvPattern)
		os.Setenv("WT_CONFIG", origEnvConfig)
	})

	t.Run("loads defaults when no config file", func(t *testing.T) {
		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", "/nonexistent/config.toml")
		configFlag = ""

		loadWorktreeConfig()

		home, _ := os.UserHomeDir()
		expectedRoot := filepath.Join(home, "dev", "worktrees")
		if appCfg.Root != expectedRoot {
			t.Errorf("worktreeRoot = %q, want %q", appCfg.Root, expectedRoot)
		}
		if appCfg.Strategy != "global" {
			t.Errorf("worktreeStrategy = %q, want global", appCfg.Strategy)
		}
		if appCfg.Pattern != "" {
			t.Errorf("worktreePattern = %q, want empty", appCfg.Pattern)
		}
		if appCfg.ConfigFileFound {
			t.Error("configFileFound should be false for nonexistent file")
		}
		if appCfg.ConfigSources.Root != "default" {
			t.Errorf("configSources.Root = %q, want default", appCfg.ConfigSources.Root)
		}
	})

	t.Run("loads from config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.toml")
		cfgContent := `root = "/custom/worktrees"
strategy = "sibling-repo"
pattern = "{.worktreeRoot}/custom/{.branch}"
`
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", cfgPath)
		configFlag = ""

		loadWorktreeConfig()

		if appCfg.Root != "/custom/worktrees" {
			t.Errorf("worktreeRoot = %q, want /custom/worktrees", appCfg.Root)
		}
		if appCfg.Strategy != "sibling-repo" {
			t.Errorf("worktreeStrategy = %q, want sibling-repo", appCfg.Strategy)
		}
		if appCfg.Pattern != "{.worktreeRoot}/custom/{.branch}" {
			t.Errorf("worktreePattern = %q, want {.worktreeRoot}/custom/{.branch}", appCfg.Pattern)
		}
		if !appCfg.ConfigFileFound {
			t.Error("configFileFound should be true")
		}
		if appCfg.ConfigSources.Root != "config file" {
			t.Errorf("configSources.Root = %q, want 'config file'", appCfg.ConfigSources.Root)
		}
		if appCfg.ConfigSources.Strategy != "config file" {
			t.Errorf("configSources.Strategy = %q, want 'config file'", appCfg.ConfigSources.Strategy)
		}
	})

	t.Run("env vars override config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.toml")
		cfgContent := `root = "/config/worktrees"
strategy = "global"
`
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "/env/worktrees")
		os.Setenv("WORKTREE_STRATEGY", "parent-branches")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", cfgPath)
		configFlag = ""

		loadWorktreeConfig()

		if appCfg.Root != "/env/worktrees" {
			t.Errorf("worktreeRoot = %q, want /env/worktrees", appCfg.Root)
		}
		if appCfg.Strategy != "parent-branches" {
			t.Errorf("worktreeStrategy = %q, want parent-branches", appCfg.Strategy)
		}
		if appCfg.ConfigSources.Root != "env: WORKTREE_ROOT" {
			t.Errorf("configSources.Root = %q, want 'env: WORKTREE_ROOT'", appCfg.ConfigSources.Root)
		}
		if appCfg.ConfigSources.Strategy != "env: WORKTREE_STRATEGY" {
			t.Errorf("configSources.Strategy = %q, want 'env: WORKTREE_STRATEGY'", appCfg.ConfigSources.Strategy)
		}
	})

	t.Run("config flag overrides WT_CONFIG env", func(t *testing.T) {
		tmpDir := t.TempDir()

		envCfg := filepath.Join(tmpDir, "env-config.toml")
		if err := os.WriteFile(envCfg, []byte(`strategy = "global"`), 0o644); err != nil {
			t.Fatal(err)
		}

		flagCfg := filepath.Join(tmpDir, "flag-config.toml")
		if err := os.WriteFile(flagCfg, []byte(`strategy = "sibling-repo"`), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", envCfg)
		configFlag = flagCfg

		loadWorktreeConfig()

		if appCfg.Strategy != "sibling-repo" {
			t.Errorf("worktreeStrategy = %q, want sibling-repo (from flag config)", appCfg.Strategy)
		}
		if appCfg.ConfigFilePath != flagCfg {
			t.Errorf("configFilePath = %q, want %q", appCfg.ConfigFilePath, flagCfg)
		}
	})

	t.Run("config file with tilde expansion in root", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.toml")
		cfgContent := `root = "~/my-worktrees"
`
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", cfgPath)
		configFlag = ""

		loadWorktreeConfig()

		home, _ := os.UserHomeDir()
		expected := filepath.Join(home, "my-worktrees")
		if appCfg.Root != expected {
			t.Errorf("worktreeRoot = %q, want %q", appCfg.Root, expected)
		}
	})

	t.Run("WORKTREE_SEPARATOR env var override", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.toml")
		cfgContent := `separator = "/"
`
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WORKTREE_SEPARATOR", "-")
		os.Setenv("WT_CONFIG", cfgPath)
		configFlag = ""
		t.Cleanup(func() { os.Unsetenv("WORKTREE_SEPARATOR") })

		loadWorktreeConfig()

		if appCfg.Separator != "-" {
			t.Errorf("Separator = %q, want -", appCfg.Separator)
		}
		if appCfg.ConfigSources.Separator != "env: WORKTREE_SEPARATOR" {
			t.Errorf("ConfigSources.Separator = %q, want 'env: WORKTREE_SEPARATOR'",
				appCfg.ConfigSources.Separator)
		}
	})

	t.Run("WORKTREE_PATTERN env var override", func(t *testing.T) {
		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "{.worktreeRoot}/env/{.branch}")
		os.Setenv("WT_CONFIG", "/nonexistent/config.toml")
		configFlag = ""

		loadWorktreeConfig()

		if appCfg.Pattern != "{.worktreeRoot}/env/{.branch}" {
			t.Errorf("Pattern = %q, want '{.worktreeRoot}/env/{.branch}'", appCfg.Pattern)
		}
		if appCfg.ConfigSources.Pattern != "env: WORKTREE_PATTERN" {
			t.Errorf("ConfigSources.Pattern = %q, want 'env: WORKTREE_PATTERN'",
				appCfg.ConfigSources.Pattern)
		}
	})

	t.Run("strategy is lowercased and trimmed", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.toml")
		cfgContent := `strategy = "  Sibling-Repo  "
`
		if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
			t.Fatal(err)
		}

		os.Setenv("WORKTREE_ROOT", "")
		os.Setenv("WORKTREE_STRATEGY", "")
		os.Setenv("WORKTREE_PATTERN", "")
		os.Setenv("WT_CONFIG", cfgPath)
		configFlag = ""

		loadWorktreeConfig()

		if appCfg.Strategy != "sibling-repo" {
			t.Errorf("worktreeStrategy = %q, want sibling-repo", appCfg.Strategy)
		}
	})
}

func TestConfigShowPatternParityBetweenTextAndJSON_Config(t *testing.T) {
	origRoot := appCfg.Root
	origStrategy := appCfg.Strategy
	origPattern := appCfg.Pattern
	origSeparator := appCfg.Separator
	origConfigFilePath := appCfg.ConfigFilePath
	origConfigFileFound := appCfg.ConfigFileFound
	origConfigSources := appCfg.ConfigSources
	origOutputFormat := appCfg.OutputFormat

	t.Cleanup(func() {
		appCfg.Root = origRoot
		appCfg.Strategy = origStrategy
		appCfg.Pattern = origPattern
		appCfg.Separator = origSeparator
		appCfg.ConfigFilePath = origConfigFilePath
		appCfg.ConfigFileFound = origConfigFileFound
		appCfg.ConfigSources = origConfigSources
		appCfg.OutputFormat = origOutputFormat
	})

	runConfigShow := func(t *testing.T, format string) string {
		t.Helper()

		origStdout := os.Stdout
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("failed to create pipe: %v", err)
		}
		os.Stdout = w
		defer func() {
			os.Stdout = origStdout
		}()

		appCfg.OutputFormat = format
		if err := runConfigShow(nil); err != nil {
			t.Fatalf("config show failed for format %s: %v", format, err)
		}

		if err := w.Close(); err != nil {
			t.Fatalf("failed to close write pipe: %v", err)
		}

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, r); err != nil {
			t.Fatalf("failed to read command output: %v", err)
		}

		return buf.String()
	}

	tests := []struct {
		name          string
		strategy      string
		workPattern   string
		patternSource string
		expected      string
	}{
		{
			name:          "strategy default pattern",
			strategy:      "global",
			workPattern:   "",
			patternSource: "strategy default",
			expected:      "{.worktreeRoot}/{.repo.Name}/{.branch}",
		},
		{
			name:          "explicit configured pattern",
			strategy:      "global",
			workPattern:   "{.worktreeRoot}/custom/{.branch}",
			patternSource: "config file",
			expected:      "{.worktreeRoot}/custom/{.branch}",
		},
		{
			name:          "custom strategy without explicit pattern",
			strategy:      "custom",
			workPattern:   "",
			patternSource: "default",
			expected:      "(none)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appCfg.Root = "/tmp/worktrees"
			appCfg.Strategy = tt.strategy
			appCfg.Pattern = tt.workPattern
			appCfg.Separator = "-"
			appCfg.ConfigFilePath = "/tmp/config.toml"
			appCfg.ConfigFileFound = true
			appCfg.ConfigSources = configSource{
				Root:      "config file",
				Strategy:  "config file",
				Pattern:   tt.patternSource,
				Separator: "default",
			}

			textOut := runConfigShow(t, "text")
			jsonOut := runConfigShow(t, "json")

			textPatternRe := regexp.MustCompile(`(?m)^\s*pattern\s*=\s*(.*?)\s+\(`)
			textMatch := textPatternRe.FindStringSubmatch(textOut)
			if len(textMatch) != 2 {
				t.Fatalf("failed to parse pattern from text output: %q", textOut)
			}
			textPattern := textMatch[1]

			var payload struct {
				Data struct {
					Effective struct {
						Pattern struct {
							Value string `json:"value"`
						} `json:"pattern"`
					} `json:"effective"`
				} `json:"data"`
			}
			if err := json.Unmarshal([]byte(jsonOut), &payload); err != nil {
				t.Fatalf("failed to parse json output: %v\noutput=%q", err, jsonOut)
			}

			if payload.Data.Effective.Pattern.Value != textPattern {
				t.Fatalf("pattern mismatch between text and json: text=%q json=%q", textPattern, payload.Data.Effective.Pattern.Value)
			}

			expectedPattern := configShowPatternValue()
			if expectedPattern != tt.expected {
				t.Fatalf("resolved test expectation mismatch: got=%q want=%q", expectedPattern, tt.expected)
			}
			if textPattern != expectedPattern {
				t.Fatalf("text output pattern should use resolved value: got=%q want=%q", textPattern, expectedPattern)
			}
			if payload.Data.Effective.Pattern.Value != expectedPattern {
				t.Fatalf("json output pattern should use resolved value: got=%q want=%q", payload.Data.Effective.Pattern.Value, expectedPattern)
			}
		})
	}
}

func TestParseConfigFileWithHooksAndSections(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	content := `root = "/custom/root"
strategy = "global"
separator = "-"

[hooks]
pre_create = ["echo pre"]
post_create = ["echo post"]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("parseConfigFile() error = %v", err)
	}
	if cfg.Root != "/custom/root" {
		t.Errorf("Root = %q, want %q", cfg.Root, "/custom/root")
	}
	if cfg.Strategy != "global" {
		t.Errorf("Strategy = %q, want %q", cfg.Strategy, "global")
	}
	if cfg.Separator != "-" {
		t.Errorf("Separator = %q, want %q", cfg.Separator, "-")
	}
	if len(cfg.Hooks.PreCreate) != 1 || cfg.Hooks.PreCreate[0] != "echo pre" {
		t.Errorf("Hooks.PreCreate = %v, want [echo pre]", cfg.Hooks.PreCreate)
	}
	if len(cfg.Hooks.PostCreate) != 1 || cfg.Hooks.PostCreate[0] != "echo post" {
		t.Errorf("Hooks.PostCreate = %v, want [echo post]", cfg.Hooks.PostCreate)
	}
}

func TestParseConfigFileIgnoresInvalidLines(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.toml")
	content := `# comment
root = "/root"
this line has no equals sign
strategy = "global"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("parseConfigFile() error = %v", err)
	}
	if cfg.Root != "/root" {
		t.Errorf("Root = %q, want %q", cfg.Root, "/root")
	}
	if cfg.Strategy != "global" {
		t.Errorf("Strategy = %q, want %q", cfg.Strategy, "global")
	}
}

func TestParseConfigFileNonexistent(t *testing.T) {
	t.Parallel()

	_, err := parseConfigFile("/nonexistent/path/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParseStringArray(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "multiple elements",
			input: `["a", "b", "c"]`,
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "single element",
			input: `["single"]`,
			want:  []string{"single"},
		},
		{
			name:  "empty array",
			input: `[]`,
			want:  nil,
		},
		{
			name:  "empty string no brackets",
			input: ``,
			want:  nil,
		},
		{
			name:  "inner quotes preserved",
			input: `["quoted \"value\""]`,
			want:  []string{`quoted \"value\"`},
		},
		{
			name:  "trimmed around but not inside quotes",
			input: `[  "  spaced  "  ]`,
			want:  []string{"  spaced  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseStringArray(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseStringArray(%q) = %v (len %d), want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseStringArray(%q)[%d] = %q, want %q",
						tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSetHookField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		section string
		key     string
		val     []string
		check   func(t *testing.T, h Hooks)
	}{
		{
			name:    "hooks pre_create",
			section: "hooks",
			key:     "pre_create",
			val:     []string{"cmd1"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PreCreate) != 1 || h.PreCreate[0] != "cmd1" {
					t.Errorf("PreCreate = %v, want [cmd1]", h.PreCreate)
				}
			},
		},
		{
			name:    "hooks post_checkout",
			section: "hooks",
			key:     "post_checkout",
			val:     []string{"cmd2"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PostCheckout) != 1 || h.PostCheckout[0] != "cmd2" {
					t.Errorf("PostCheckout = %v, want [cmd2]", h.PostCheckout)
				}
			},
		},
		{
			name:    "hooks post_create",
			section: "hooks",
			key:     "post_create",
			val:     []string{"cmd3"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PostCreate) != 1 || h.PostCreate[0] != "cmd3" {
					t.Errorf("PostCreate = %v, want [cmd3]", h.PostCreate)
				}
			},
		},
		{
			name:    "hooks pre_checkout",
			section: "hooks",
			key:     "pre_checkout",
			val:     []string{"cmd4"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PreCheckout) != 1 || h.PreCheckout[0] != "cmd4" {
					t.Errorf("PreCheckout = %v, want [cmd4]", h.PreCheckout)
				}
			},
		},
		{
			name:    "hooks pre_remove",
			section: "hooks",
			key:     "pre_remove",
			val:     []string{"cmd5"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PreRemove) != 1 || h.PreRemove[0] != "cmd5" {
					t.Errorf("PreRemove = %v, want [cmd5]", h.PreRemove)
				}
			},
		},
		{
			name:    "hooks post_remove",
			section: "hooks",
			key:     "post_remove",
			val:     []string{"cmd6"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PostRemove) != 1 || h.PostRemove[0] != "cmd6" {
					t.Errorf("PostRemove = %v, want [cmd6]", h.PostRemove)
				}
			},
		},
		{
			name:    "hooks pre_pr",
			section: "hooks",
			key:     "pre_pr",
			val:     []string{"cmd7"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PrePR) != 1 || h.PrePR[0] != "cmd7" {
					t.Errorf("PrePR = %v, want [cmd7]", h.PrePR)
				}
			},
		},
		{
			name:    "hooks post_pr",
			section: "hooks",
			key:     "post_pr",
			val:     []string{"cmd8"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PostPR) != 1 || h.PostPR[0] != "cmd8" {
					t.Errorf("PostPR = %v, want [cmd8]", h.PostPR)
				}
			},
		},
		{
			name:    "hooks pre_mr",
			section: "hooks",
			key:     "pre_mr",
			val:     []string{"cmd9"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PreMR) != 1 || h.PreMR[0] != "cmd9" {
					t.Errorf("PreMR = %v, want [cmd9]", h.PreMR)
				}
			},
		},
		{
			name:    "hooks post_mr",
			section: "hooks",
			key:     "post_mr",
			val:     []string{"cmd10"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if len(h.PostMR) != 1 || h.PostMR[0] != "cmd10" {
					t.Errorf("PostMR = %v, want [cmd10]", h.PostMR)
				}
			},
		},
		{
			name:    "wrong section ignored",
			section: "other",
			key:     "pre_create",
			val:     []string{"cmd11"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				if h.PreCreate != nil {
					t.Errorf("PreCreate = %v, want nil (wrong section)", h.PreCreate)
				}
			},
		},
		{
			name:    "unknown key ignored",
			section: "hooks",
			key:     "unknown_key",
			val:     []string{"cmd12"},
			check: func(t *testing.T, h Hooks) {
				t.Helper()
				// All fields should remain zero-value
				if h.PreCreate != nil || h.PostCreate != nil ||
					h.PreCheckout != nil || h.PostCheckout != nil ||
					h.PreRemove != nil || h.PostRemove != nil ||
					h.PrePR != nil || h.PostPR != nil ||
					h.PreMR != nil || h.PostMR != nil {
					t.Error("hooks should be unchanged for unknown key")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var hooks Hooks
			setHookField(&hooks, tt.section, tt.key, tt.val)
			tt.check(t, hooks)
		})
	}
}

func TestParseStringArrayWhitespaceOnly(t *testing.T) {
	t.Parallel()

	got := parseStringArray("[  ]")
	if got != nil {
		t.Errorf("parseStringArray(\"[  ]\") = %v, want nil", got)
	}
}

func TestUnquoteStringEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty content between quotes",
			input: `""`,
			want:  ``,
		},
		{
			name:  "single quote char returned as-is",
			input: `"`,
			want:  `"`,
		},
		{
			name:  "no quotes",
			input: `abc`,
			want:  `abc`,
		},
		{
			name:  "single quotes not handled",
			input: `'single'`,
			want:  `'single'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := unquoteString(tt.input)
			if got != tt.want {
				t.Errorf("unquoteString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests migrated from mock_test.go
// ---------------------------------------------------------------------------

func TestWriteDefaultConfig_WriteFileFails(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the parent dir and make it read-only so WriteFile fails.
	dir := filepath.Join(tmpDir, "wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	path := filepath.Join(dir, "config.toml")
	err := writeDefaultConfig(path, false)
	if err == nil {
		t.Fatal("expected error when WriteFile fails")
	}
	if !strings.Contains(err.Error(), "failed to write config file") {
		t.Errorf("error = %q, want 'failed to write config file'", err)
	}
}
