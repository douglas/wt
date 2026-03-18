package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestValidateOutputFormat(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })

	appCfg.OutputFormat = "JSON"
	if err := validateOutputFormat(); err != nil {
		t.Fatalf("validateOutputFormat() unexpected error: %v", err)
	}
	if appCfg.OutputFormat != "json" {
		t.Fatalf("validateOutputFormat() normalized format = %q, want %q", appCfg.OutputFormat, "json")
	}

	appCfg.OutputFormat = "yaml"
	if err := validateOutputFormat(); err == nil {
		t.Fatal("validateOutputFormat() expected error for unsupported format")
	}
}

func TestPrintCDMarkerSkipsJSONOutput(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	appCfg.OutputFormat = "json"
	printCDMarker("/tmp/worktree")

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected no output in json mode, got %q", buf.String())
	}
}

func TestEmitJSONSuccess(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })
	appCfg.OutputFormat = "json"

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err = emitJSONSuccess(rootCmd, map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("emitJSONSuccess() unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	var payload struct {
		OK      bool              `json:"ok"`
		Command string            `json:"command"`
		Data    map[string]string `json:"data"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if !payload.OK {
		t.Fatalf("expected ok=true, got false")
	}
	if payload.Command != "wt" {
		t.Fatalf("expected command wt, got %q", payload.Command)
	}
	if payload.Data["hello"] != "world" {
		t.Fatalf("expected data.hello=world, got %q", payload.Data["hello"])
	}
}

func TestRootHelpUsesJSONFormat(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })
	appCfg.OutputFormat = "json"

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	if err := rootCmd.Help(); err != nil {
		t.Fatalf("help returned error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("expected JSON help envelope, got: %s", out)
	}
}

func TestConfigHelpUsesJSONFormat(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })
	appCfg.OutputFormat = "json"

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	if err := configCmd.Help(); err != nil {
		t.Fatalf("help returned error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"ok":true`) {
		t.Fatalf("expected JSON help envelope, got: %s", out)
	}
	if !strings.Contains(out, `"command":"wt config"`) {
		t.Fatalf("expected wt config command in JSON, got: %s", out)
	}
}

func TestEmitJSONError(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })
	appCfg.OutputFormat = "json"

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err = emitJSONError(rootCmd, fmt.Errorf("something broke"))
	if err != nil {
		t.Fatalf("emitJSONError() unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	var payload struct {
		OK      bool   `json:"ok"`
		Command string `json:"command"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if payload.OK {
		t.Fatalf("expected ok=false, got true")
	}
	if payload.Command != "wt" {
		t.Fatalf("expected command wt, got %q", payload.Command)
	}
	if payload.Error != "something broke" {
		t.Fatalf("expected error='something broke', got %q", payload.Error)
	}
}

func TestEmitJSONErrorTextMode(t *testing.T) {
	original := appCfg.OutputFormat
	t.Cleanup(func() { appCfg.OutputFormat = original })
	appCfg.OutputFormat = "text"

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	err = emitJSONError(rootCmd, fmt.Errorf("something broke"))
	if err != nil {
		t.Fatalf("emitJSONError() unexpected error: %v", err)
	}

	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if strings.TrimSpace(buf.String()) != "" {
		t.Fatalf("expected no output in text mode, got %q", buf.String())
	}
}
