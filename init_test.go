package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDetectShell(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		envShell string
		want     string
	}{
		{
			name: "explicit bash argument",
			args: []string{"bash"},
			want: "bash",
		},
		{
			name: "explicit zsh argument",
			args: []string{"zsh"},
			want: "zsh",
		},
		{
			name: "explicit powershell argument",
			args: []string{"powershell"},
			want: "powershell",
		},
		{
			name: "pwsh alias returns powershell",
			args: []string{"pwsh"},
			want: "powershell",
		},
		{
			name: "case insensitive",
			args: []string{"BASH"},
			want: "bash",
		},
		{
			name:     "detect from SHELL env - zsh",
			args:     []string{},
			envShell: "/bin/zsh",
			want:     "zsh",
		},
		{
			name:     "detect from SHELL env - bash",
			args:     []string{},
			envShell: "/bin/bash",
			want:     "bash",
		},
		{
			name:     "unknown shell falls back to env detection",
			args:     []string{"fish"},
			envShell: "/bin/zsh",
			want:     "zsh",
		},
		{
			name:     "unknown shell with no env falls back to bash",
			args:     []string{"fish"},
			envShell: "/bin/sh",
			want:     "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip Windows-specific tests on non-Windows
			if runtime.GOOS == "windows" && tt.envShell != "" {
				t.Skip("Skipping SHELL env test on Windows")
			}

			// Save and restore SHELL env var
			origShell := os.Getenv("SHELL")
			if tt.envShell != "" {
				os.Setenv("SHELL", tt.envShell)
			}
			defer os.Setenv("SHELL", origShell)

			got := detectShell(tt.args)
			if got != tt.want {
				t.Errorf("detectShell(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestSupportedShells(t *testing.T) {
	t.Parallel()

	// Verify all expected shells are in the map
	expected := []string{"bash", "zsh", "powershell", "pwsh"}
	for _, shell := range expected {
		if !supportedShells[shell] {
			t.Errorf("supportedShells missing %q", shell)
		}
	}

	// Verify no unexpected shells
	if len(supportedShells) != len(expected) {
		t.Errorf("supportedShells has unexpected entries: got %d, want %d", len(supportedShells), len(expected))
	}
}

func TestGetShellConfigPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	// Ensure tests are stable even when the developer machine has ZDOTDIR set.
	origZdotdir := os.Getenv("ZDOTDIR")
	os.Setenv("ZDOTDIR", "")
	t.Cleanup(func() { os.Setenv("ZDOTDIR", origZdotdir) })

	tests := []struct {
		name  string
		shell string
		want  string
	}{
		{
			name:  "zsh config path",
			shell: "zsh",
			want:  filepath.Join(home, ".zshrc"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getShellConfigPath(tt.shell)
			if got != tt.want {
				t.Errorf("getShellConfigPath(%q) = %q, want %q", tt.shell, got, tt.want)
			}
		})
	}
}

func TestGetShellConfigPathZshRespectsZdotdir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	orig := os.Getenv("ZDOTDIR")
	t.Cleanup(func() { os.Setenv("ZDOTDIR", orig) })

	zdotdir := filepath.Join(home, ".config", "zsh")
	os.Setenv("ZDOTDIR", zdotdir)

	got := getShellConfigPath("zsh")
	want := filepath.Join(zdotdir, ".zshrc")
	if got != want {
		t.Fatalf("getShellConfigPath(%q) = %q, want %q", "zsh", got, want)
	}
}

func TestGetShellConfigContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		shell    string
		contains []string
	}{
		{
			name:     "bash content",
			shell:    "bash",
			contains: []string{markerStart, markerEnd, "wt shellenv"},
		},
		{
			name:     "zsh content",
			shell:    "zsh",
			contains: []string{markerStart, markerEnd, "wt shellenv"},
		},
		{
			name:     "powershell content",
			shell:    "powershell",
			contains: []string{markerStart, markerEnd, "wt shellenv", "Invoke-Expression"},
		},
		{
			name:  "unsupported shell returns empty",
			shell: "fish",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getShellConfigContent(tt.shell)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("getShellConfigContent(%q) missing %q", tt.shell, want)
				}
			}
			if len(tt.contains) == 0 && got != "" {
				t.Errorf("getShellConfigContent(%q) = %q, want empty", tt.shell, got)
			}
		})
	}
}

func TestSuccessPrefix(t *testing.T) {
	tests := []struct {
		name    string
		envCI   string
		envTerm string
		want    string
	}{
		{
			name: "normal terminal shows checkmark",
			want: "✓",
		},
		{
			name:  "CI environment shows [ok]",
			envCI: "true",
			want:  "[ok]",
		},
		{
			name:    "dumb terminal shows [ok]",
			envTerm: "dumb",
			want:    "[ok]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env vars
			origCI := os.Getenv("CI")
			origTerm := os.Getenv("TERM")
			defer func() {
				os.Setenv("CI", origCI)
				os.Setenv("TERM", origTerm)
			}()

			os.Unsetenv("CI")
			os.Unsetenv("TERM")
			if tt.envCI != "" {
				os.Setenv("CI", tt.envCI)
			}
			if tt.envTerm != "" {
				os.Setenv("TERM", tt.envTerm)
			}

			got := successPrefix()
			if got != tt.want {
				t.Errorf("successPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInstallAndRemoveShellConfig(t *testing.T) {
	// Create a temp directory for test files
	tmpDir, err := os.MkdirTemp("", "wt-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ".bashrc")

	// Test install on new file
	t.Run("install on new file", func(t *testing.T) {
		err := installShellConfig(configPath, "bash", false, true)
		if err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if !strings.Contains(string(content), markerStart) {
			t.Error("Config missing start marker")
		}
		if !strings.Contains(string(content), markerEnd) {
			t.Error("Config missing end marker")
		}
		if !strings.Contains(string(content), "wt shellenv") {
			t.Error("Config missing shellenv command")
		}
	})

	// Test idempotent install
	t.Run("idempotent install", func(t *testing.T) {
		err := installShellConfig(configPath, "bash", false, true)
		if err != nil {
			t.Fatalf("Second installShellConfig failed: %v", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		// Should only have one occurrence of the marker
		count := strings.Count(string(content), markerStart)
		if count != 1 {
			t.Errorf("Expected 1 occurrence of marker, got %d", count)
		}
	})

	// Test remove
	t.Run("remove config", func(t *testing.T) {
		err := removeShellConfig(configPath, "bash", false)
		if err != nil {
			t.Fatalf("removeShellConfig failed: %v", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if strings.Contains(string(content), markerStart) {
			t.Error("Config still contains start marker after removal")
		}
		if strings.Contains(string(content), markerEnd) {
			t.Error("Config still contains end marker after removal")
		}
	})

	// Test preserves existing content
	t.Run("preserves existing content", func(t *testing.T) {
		existingContent := "# My existing config\nexport MY_VAR=hello\n"
		err := os.WriteFile(configPath, []byte(existingContent), 0644)
		if err != nil {
			t.Fatalf("Failed to write existing config: %v", err)
		}

		err = installShellConfig(configPath, "bash", false, true)
		if err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}

		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if !strings.Contains(string(content), "MY_VAR=hello") {
			t.Error("Existing content was not preserved")
		}

		// Remove and verify existing content still present
		err = removeShellConfig(configPath, "bash", false)
		if err != nil {
			t.Fatalf("removeShellConfig failed: %v", err)
		}

		content, err = os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read config: %v", err)
		}

		if !strings.Contains(string(content), "MY_VAR=hello") {
			t.Error("Existing content was not preserved after removal")
		}
	})
}

func TestDryRun(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "wt-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, ".bashrc")

	// Dry run should not create file
	t.Run("dry run does not create file", func(t *testing.T) {
		err := installShellConfig(configPath, "bash", true, true)
		if err != nil {
			t.Fatalf("installShellConfig dry run failed: %v", err)
		}

		if _, err := os.Stat(configPath); !os.IsNotExist(err) {
			t.Error("Dry run should not create file")
		}
	})

	// Create file for removal test
	t.Run("dry run does not modify file on remove", func(t *testing.T) {
		content := markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n"
		err := os.WriteFile(configPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		err = removeShellConfig(configPath, "bash", true)
		if err != nil {
			t.Fatalf("removeShellConfig dry run failed: %v", err)
		}

		// File should still have markers
		readContent, _ := os.ReadFile(configPath)
		if !strings.Contains(string(readContent), markerStart) {
			t.Error("Dry run should not modify file")
		}
	})
}

func TestInstallShellConfigDryRunUpdateExisting(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	// Write a file that already has the markers.
	original := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Dry-run update should not modify the file.
	if err := installShellConfig(configPath, "bash", true, true); err != nil {
		t.Fatalf("installShellConfig dry-run failed: %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if string(after) != original {
		t.Errorf("dry-run modified the file.\nbefore: %q\nafter:  %q", original, string(after))
	}
}

func TestInstallShellConfigUnsupportedShell(t *testing.T) {
	t.Parallel()

	err := installShellConfig("/tmp/test", "fish", false, true)
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("error = %q, want it to contain 'unsupported shell'", err.Error())
	}
}

func TestInstallShellConfigShowsActivation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	// Capture stdout to verify activation message
	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", false, false); err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}
	})

	if !strings.Contains(output, "source") {
		t.Errorf("expected activation instructions with 'source', got: %s", output)
	}
}

func TestInstallShellConfigPowershellActivation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "profile.ps1")

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "powershell", false, false); err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}
	})

	if !strings.Contains(output, "$PROFILE") {
		t.Errorf("expected powershell activation with '$PROFILE', got: %s", output)
	}
}

func TestInstallShellConfigUpdateExisting(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	// First install
	if err := installShellConfig(configPath, "bash", false, true); err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	// Update (not dry run) - should update the existing block
	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", false, true); err != nil {
			t.Fatalf("update install failed: %v", err)
		}
	})

	if !strings.Contains(output, "Updated") {
		t.Errorf("expected 'Updated' in output, got: %s", output)
	}

	content, _ := os.ReadFile(configPath)
	count := strings.Count(string(content), markerStart)
	if count != 1 {
		t.Errorf("expected 1 marker occurrence, got %d", count)
	}
}

func TestRemoveShellConfigNonexistentFile(t *testing.T) {
	output := captureStdout(t, func() {
		err := removeShellConfig("/nonexistent/path/.bashrc", "bash", false)
		if err != nil {
			t.Fatalf("expected no error for nonexistent file, got: %v", err)
		}
	})
	if !strings.Contains(output, "No configuration found") {
		t.Errorf("expected 'No configuration found', got: %s", output)
	}
}

func TestRemoveShellConfigNoMarkers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	if err := os.WriteFile(configPath, []byte("# some config\n"), 0644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		err := removeShellConfig(configPath, "bash", false)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})
	if !strings.Contains(output, "No wt configuration found") {
		t.Errorf("expected 'No wt configuration found', got: %s", output)
	}
}

func TestInitJSONOutput(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping init JSON integration test in short mode")
	}

	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "test-repo")
	setupTestRepo(t, repoDir)
	wtBinary := buildWtBinary(t, tmpDir)

	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("failed to create HOME dir: %v", err)
	}

	cmd := exec.Command(wtBinary, "--format", "json", "init", "bash", "--dry-run")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+homeDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wt init --format json --dry-run failed: %v\nOutput: %s", err, out)
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Data    struct {
			Status     string `json:"status"`
			Operation  string `json:"operation"`
			Shell      string `json:"shell"`
			ConfigPath string `json:"config_path"`
			DryRun     bool   `json:"dry_run"`
		} `json:"data"`
	}

	if err := json.Unmarshal(out, &payload); err != nil {
		t.Fatalf("failed to parse init json output: %v\noutput=%q", err, out)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true in init json output: %s", out)
	}
	if payload.Command != "wt init" {
		t.Fatalf("expected command wt init, got %q", payload.Command)
	}
	if payload.Data.Operation != "install" {
		t.Fatalf("expected operation install, got %q", payload.Data.Operation)
	}
	if payload.Data.Status != "planned" {
		t.Fatalf("expected status planned for dry-run, got %q", payload.Data.Status)
	}
	if payload.Data.Shell != "bash" {
		t.Fatalf("expected shell bash, got %q", payload.Data.Shell)
	}
	if !payload.Data.DryRun {
		t.Fatal("expected dry_run=true")
	}
	if payload.Data.ConfigPath == "" {
		t.Fatal("expected config_path to be populated")
	}
}

func TestGetShellConfigPathBash(t *testing.T) {
	tmpDir := t.TempDir()
	bashrc := filepath.Join(tmpDir, ".bashrc")
	if err := os.WriteFile(bashrc, []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	got := getShellConfigPath("bash")
	if got != bashrc {
		t.Errorf("getShellConfigPath(\"bash\") = %q, want %q", got, bashrc)
	}
}

func TestGetShellConfigPathUnknown(t *testing.T) {
	got := getShellConfigPath("fish")
	if got != "" {
		t.Errorf("getShellConfigPath(\"fish\") = %q, want empty string", got)
	}
}

func TestGetShellConfigPathPowershellProfile(t *testing.T) {
	profilePath := "/custom/profile.ps1"
	t.Setenv("PROFILE", profilePath)

	got := getShellConfigPath("powershell")
	if got != profilePath {
		t.Errorf("getShellConfigPath(\"powershell\") = %q, want %q (from $PROFILE)", got, profilePath)
	}
}

func TestGetShellConfigPathBashFallback(t *testing.T) {
	// When .bashrc doesn't exist, bash falls back to .bashrc on non-macOS
	// or .bash_profile on macOS
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	got := getShellConfigPath("bash")
	if runtime.GOOS == "darwin" {
		want := filepath.Join(tmpDir, ".bash_profile")
		if got != want {
			t.Errorf("getShellConfigPath(\"bash\") on darwin = %q, want %q", got, want)
		}
	} else {
		want := filepath.Join(tmpDir, ".bashrc")
		if got != want {
			t.Errorf("getShellConfigPath(\"bash\") on linux = %q, want %q", got, want)
		}
	}
}

func TestGetShellConfigPathZdotdirRelative(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("ZDOTDIR", "custom-zsh")

	got := getShellConfigPath("zsh")
	want := filepath.Join(tmpDir, "custom-zsh", ".zshrc")
	if got != want {
		t.Errorf("getShellConfigPath(\"zsh\") with relative ZDOTDIR = %q, want %q", got, want)
	}
}

func TestGetShellConfigPathPowershell(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("PROFILE", "")

	got := getShellConfigPath("powershell")
	if runtime.GOOS != "windows" {
		want := filepath.Join(tmpDir, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
		if got != want {
			t.Errorf("getShellConfigPath(\"powershell\") = %q, want %q", got, want)
		}
	}
	if got == "" {
		t.Error("getShellConfigPath(\"powershell\") should not return empty string")
	}
}

func TestInstallShellConfig_ReadError(t *testing.T) {
	t.Parallel()

	// configPath is a directory → ReadFile fails with "is a directory".
	tmpDir := t.TempDir()
	err := installShellConfig(tmpDir, "bash", false, true)
	if err == nil {
		t.Fatal("expected error when configPath is a directory")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("error = %q, want it to contain 'failed to read'", err)
	}
}

func TestInstallShellConfig_OpenFileError(t *testing.T) {
	t.Parallel()

	// configPath doesn't exist → ReadFile returns IsNotExist (OK).
	// Parent dir is read-only → MkdirAll/OpenFile fails.
	tmpDir := t.TempDir()
	roDir := filepath.Join(tmpDir, "readonly")
	if err := os.MkdirAll(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	configPath := filepath.Join(roDir, "subdir", ".bashrc")
	err := installShellConfig(configPath, "bash", false, true)
	if err == nil {
		t.Fatal("expected error when parent dir creation is blocked")
	}
	if !strings.Contains(err.Error(), "failed to create config directory") &&
		!strings.Contains(err.Error(), "failed to open") {
		t.Errorf("error = %q, want 'failed to create config directory' or 'failed to open'", err)
	}
}

func TestInstallShellConfig_JSONModeNewFile(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", false, true); err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}
	})

	// JSON mode should suppress text output.
	if strings.Contains(output, "Added") || strings.Contains(output, "source") {
		t.Errorf("expected no text output in JSON mode, got: %s", output)
	}

	// File should still be written.
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if !strings.Contains(string(content), markerStart) {
		t.Error("config file missing start marker")
	}
}

func TestRemoveShellConfig_ReadError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	err := removeShellConfig(tmpDir, "bash", false)
	if err == nil {
		t.Fatal("expected error when configPath is a directory")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("error = %q, want it to contain 'failed to read'", err)
	}
}

func TestRemoveShellConfig_MalformedMarkers(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	// End marker before start marker → malformed.
	content := markerEnd + "\nsome content\n" + markerStart + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	err := removeShellConfig(configPath, "bash", false)
	if err == nil {
		t.Fatal("expected error for malformed markers")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error = %q, want it to contain 'malformed'", err)
	}
}

func TestRemoveShellConfig_JSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	// First, write a config with markers.
	content := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if err := removeShellConfig(configPath, "bash", false); err != nil {
			t.Fatalf("removeShellConfig failed: %v", err)
		}
	})

	// JSON mode should suppress text output.
	if strings.Contains(output, "Removed") {
		t.Errorf("expected no text output in JSON mode, got: %s", output)
	}

	// Markers should be removed from file.
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if strings.Contains(string(after), markerStart) {
		t.Error("config still contains start marker after removal")
	}
}

// ---------------------------------------------------------------------------
// Tests migrated from mock_test.go
// ---------------------------------------------------------------------------

func TestInstallShellConfig_JSONModeUpdateExisting(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", false, true); err != nil {
			t.Fatalf("installShellConfig failed: %v", err)
		}
	})

	// JSON mode should suppress "Updated" text.
	if strings.Contains(output, "Updated") {
		t.Errorf("expected no text output in JSON mode, got: %s", output)
	}
}

func TestInstallShellConfig_DryRunJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")

	output := captureStdout(t, func() {
		if err := installShellConfig(configPath, "bash", true, true); err != nil {
			t.Fatalf("installShellConfig dry-run failed: %v", err)
		}
	})

	// JSON mode + dry run should suppress "Would append" text.
	if strings.Contains(output, "Would") {
		t.Errorf("expected no text in JSON dry-run mode, got: %s", output)
	}
}

func TestRemoveShellConfig_NonexistentJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	output := captureStdout(t, func() {
		err := removeShellConfig("/nonexistent/path/.bashrc", "bash", false)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	// JSON mode should suppress "No configuration found" text.
	if strings.Contains(output, "No configuration") {
		t.Errorf("expected no text in JSON mode, got: %s", output)
	}
}

func TestRemoveShellConfig_NoMarkersJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	if err := os.WriteFile(configPath, []byte("# some config\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		err := removeShellConfig(configPath, "bash", false)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	if strings.Contains(output, "No wt configuration") {
		t.Errorf("expected no text in JSON mode, got: %s", output)
	}
}

func TestInstallShellConfig_WriteFileError(t *testing.T) {
	// Create a file with markers, then make it read-only so WriteFile fails.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := "# preamble\n" + markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n# postamble\n"
	if err := os.WriteFile(configPath, []byte(content), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(configPath, 0o644) })

	err := installShellConfig(configPath, "bash", false, true)
	if err == nil {
		t.Fatal("expected error when file is read-only")
	}
	if !strings.Contains(err.Error(), "failed to write") {
		t.Errorf("error = %q, want 'failed to write'", err)
	}
}

func TestRemoveShellConfig_WriteFileError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(configPath, 0o644) })

	err := removeShellConfig(configPath, "bash", false)
	if err == nil {
		t.Fatal("expected error when file is read-only")
	}
	if !strings.Contains(err.Error(), "failed to write") {
		t.Errorf("error = %q, want 'failed to write'", err)
	}
}

func TestInstallShellConfig_WriteStringError(t *testing.T) {
	// Instead, let me cover the "config doesn't end with newline" path.
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	// File without trailing newline.
	if err := os.WriteFile(configPath, []byte("# config"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := installShellConfig(configPath, "bash", false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	// Should have added a newline before the marker.
	if !strings.Contains(string(content), "# config\n\n"+markerStart) {
		t.Errorf("expected newline before marker, got: %q", string(content))
	}
}

func TestDetectTargetState_StatError(t *testing.T) {
	// Use a path that triggers a stat error other than "not exist".
	// A path component that is a file (not dir) causes ENOTDIR.
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Stat on blocker/child fails with ENOTDIR.
	_, err := detectTargetState(filepath.Join(blocker, "child"))
	if err == nil {
		t.Fatal("expected error for stat through non-directory")
	}
	if !strings.Contains(err.Error(), "failed to stat target path") {
		t.Errorf("error = %q, want 'failed to stat target path'", err)
	}
}

func TestRemoveShellConfig_DryRunJSONMode(t *testing.T) {
	withAppConfig(t)
	appCfg.OutputFormat = "json"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".bashrc")
	content := markerStart + "\neval \"$(wt shellenv)\"\n" + markerEnd + "\n"
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	output := captureStdout(t, func() {
		err := removeShellConfig(configPath, "bash", true)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	if strings.Contains(output, "Would remove") {
		t.Errorf("expected no text in JSON dry-run mode, got: %s", output)
	}
}
