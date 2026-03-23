# Changelog

All notable changes to this project will be documented in this file.

## [0.2.0] - 2026-03-23

### Features

- `wt done` — remove the current linked worktree and auto-navigate back to the main checkout
- `[copy_files]` config — automatically copy files (`.env`, `.tool-versions`) from main worktree into new worktrees on create/checkout/pr/mr
- `done` added to bash, zsh, and PowerShell shell completion

### Security

- Fix path traversal in `copy_files` — reject paths containing `..` or escaping the worktree root
- Fix symlink following in `copy_files` — skip symlinks to prevent reading unintended files
- Fix symlink attack in `wt init` — reject symlinked shell rc files
- Fix terminal escape injection — sanitize branch names and PR titles in interactive prompts
- Fix pipe fd leak and truncation in JSON help output
- Fix config file created with 0o600 (owner-only) instead of world-readable 0o644
- Fix `--config` validation — reject non-regular files (e.g., `/dev/stdin`)
- Fix PowerShell `Set-Location` to use `-LiteralPath` for paths with special characters
- Add hook security documentation to config template

### Refactoring

- Replace cobra with stdlib `flag` — zero external CLI framework dependencies
- Split `commands.go` (1,013 lines) into 6 focused `cmd_*.go` files
- Split `mock_test.go` (1,586 lines) into domain-specific test files
- Add custom error types: `ErrCancelled`, `ErrNotInWorktree`, `ConfigError`
- Consolidate `init()` blocks, add package-level godoc
- Fix path-prefix boundary bug in `removeCmd` (`isInsideWorktree` helper)

### Build & Distribution

- Binary size reduced: 4.7 MB → 4.0 MB (-15%) with stdlib flag replacement
- Dependencies removed: cobra, pflag, mousetrap
- Simplified release pipeline for fork (GitHub Releases only)
- Updated all project references to `douglas/wt`

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

- GoReleaser with cross-platform builds
- CI: unit tests, e2e tests (Linux, macOS, Windows), 83% coverage, 29 golangci-lint linters, race detector
