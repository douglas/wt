package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	formatText = "text"
	formatJSON = "json"
)

// jsonEnvelope wraps all JSON-mode output with a uniform ok/error structure.
type jsonEnvelope struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

func isJSONOutput() bool {
	return strings.EqualFold(strings.TrimSpace(appCfg.OutputFormat), formatJSON)
}

func validateOutputFormat() error {
	trimmed := strings.ToLower(strings.TrimSpace(appCfg.OutputFormat))
	switch trimmed {
	case formatText, formatJSON:
		appCfg.OutputFormat = trimmed
		return nil
	default:
		return fmt.Errorf("unsupported --format value %q (supported: text, json)", appCfg.OutputFormat)
	}
}

func commandPath(name string) string {
	if name == "" {
		return "wt"
	}
	return "wt " + name
}

func emitJSONSuccess(cmdName string, data any) error {
	if !isJSONOutput() {
		return nil
	}
	return emitJSON(jsonEnvelope{OK: true, Command: commandPath(cmdName), Data: data})
}

func emitJSONError(cmdName string, err error) error {
	if !isJSONOutput() {
		return nil
	}
	return emitJSON(jsonEnvelope{OK: false, Command: commandPath(cmdName), Error: err.Error()})
}

func emitJSON(payload jsonEnvelope) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(payload)
}
