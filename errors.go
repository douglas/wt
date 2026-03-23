package main

import (
	"errors"
	"fmt"
)

// ErrCancelled is returned when the user cancels an interactive prompt
// via ESC, Ctrl-C, or empty input.
var ErrCancelled = errors.New("selection cancelled")

// ErrNotInWorktree is returned by the done command when the current
// directory is not inside a linked worktree.
var ErrNotInWorktree = errors.New("not inside a linked worktree")

// ConfigError represents a configuration file error.
type ConfigError struct {
	Path string
	Err  error
}

func (e *ConfigError) Error() string { return fmt.Sprintf("%s: %v", e.Path, e.Err) }
func (e *ConfigError) Unwrap() error { return e.Err }
