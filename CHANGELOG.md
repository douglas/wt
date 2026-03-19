# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0] - 2026-03-18

First tagged release. `wt` has been in active development since its initial commit and is stable for daily use.

### Features

- Git worktree management: create, checkout, remove, list, and cleanup worktrees
- Interactive selection prompts with ESC/Ctrl-C cancellation (raw-mode TTY input)
- Shell integration with auto-cd for bash, zsh, and PowerShell (`wt init`)
- Configurable worktree path strategies via TOML config (`wt config init`)
- Template patterns with environment variable support and branch separators
- GitHub PR and GitLab MR checkout as worktrees with upstream tracking
- `wt cleanup` to remove worktrees for merged branches
- `wt migrate` to reconcile worktree paths after config changes
- Pre/post command hooks
- Machine-readable JSON output (`--format json`)
- `wt examples` command for usage reference
- Cross-platform: Linux, macOS, Windows

### Build & Distribution

- GoReleaser with UPX compression, Homebrew tap, Scoop bucket, AUR, WinGet, deb/rpm/archlinux packages
- CI: unit tests, e2e tests (Linux, macOS, Windows), 83.5% coverage, 29 golangci-lint linters, race detector
