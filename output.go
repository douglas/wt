package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

const (
	formatText = "text"
	formatJSON = "json"
)

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

func commandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return "wt"
	}
	return cmd.CommandPath()
}

func emitJSONSuccess(cmd *cobra.Command, data any) error {
	if !isJSONOutput() {
		return nil
	}
	return emitJSON(jsonEnvelope{OK: true, Command: commandPath(cmd), Data: data})
}

func emitJSONError(cmd *cobra.Command, err error) error {
	if !isJSONOutput() {
		return nil
	}
	return emitJSON(jsonEnvelope{OK: false, Command: commandPath(cmd), Error: err.Error()})
}

func emitJSON(payload jsonEnvelope) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(payload)
}
