# wt - Git Worktree Manager

[![CI](https://github.com/douglas/wt/actions/workflows/ci.yml/badge.svg)](https://github.com/douglas/wt/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/v/release/douglas/wt)](https://github.com/douglas/wt/releases)

A fast, zero-dependency Git worktree helper written in Go. Originally forked from [timvw/wt](https://github.com/timvw/wt), with significant refactoring, new features, and security hardening.

## Features

- Configurable worktree placement strategies: `global`, `sibling-repo`, `parent-branches`, and more
- **`wt done`** — remove current worktree and auto-navigate back to main
- **`[copy_files]`** — automatically copy files (`.env`, `.tool-versions`) into new worktrees
- **Interactive selection menus** for checkout, remove, pr, and mr commands
- GitHub PR support via `wt pr` (uses `gh` CLI) — checks out the PR's actual branch name
- GitLab MR support via `wt mr` (uses `glab` CLI) — checks out the MR's actual branch name
- **Pre/post command hooks** — run custom scripts on create/checkout/remove/pr/mr
- Shell integration with auto-cd and tab completion (bash, zsh, PowerShell)
- Machine-readable JSON output (`--format json`) for all commands
- Zero external CLI framework dependencies — stdlib only

## Installation

### From GitHub Releases

Download the latest binary for your platform from the [releases page](https://github.com/douglas/wt/releases).

```bash
# Linux (amd64)
curl -Lo wt https://github.com/douglas/wt/releases/latest/download/wt_linux_amd64.tar.gz
tar xzf wt_linux_amd64.tar.gz
sudo mv wt /usr/local/bin/

# macOS (Apple Silicon)
curl -Lo wt https://github.com/douglas/wt/releases/latest/download/wt_darwin_arm64.tar.gz
tar xzf wt_darwin_arm64.tar.gz
sudo mv wt /usr/local/bin/

wt init  # Configure shell integration
```

### From Source

```bash
git clone https://github.com/douglas/wt.git
cd wt

# Using just (recommended)
just build            # builds to bin/wt
just install          # installs to /usr/local/bin (requires sudo)
just install-user     # installs to ~/bin (no sudo)

# Or build directly with go
go build -ldflags="-s -w" -o wt .
sudo mv wt /usr/local/bin/

# Configure shell integration
wt init
```

### Shell Integration

The `wt init` command configures shell integration for your shell:

```bash
wt init              # Auto-detect shell and configure
wt init bash         # Configure for bash specifically
wt init zsh          # Configure for zsh specifically
wt init --dry-run    # Preview changes without modifying files
wt init --uninstall  # Remove wt configuration from shell
```

After running `wt init`, restart your shell or run:

```bash
source ~/.bashrc   # for bash
source "${ZDOTDIR:-$HOME}/.zshrc"    # for zsh
```

Shell integration enables:

- Automatic `cd` to worktree after `checkout`/`create`/`pr`/`mr` commands
- Tab completion for commands and branch names

**Manual setup** (alternative to `wt init`): Add this to the **END** of your shell config:

```bash
eval "$(wt shellenv)"
```

**Note for zsh users:** Place this after `compinit` in your config file.


## Usage

### Commands

```bash
# Checkout existing branch in new worktree
wt checkout feature-branch
wt co feature-branch              # short alias
wt co                             # interactive: select from available branches

# Create new branch in worktree (defaults to main/master as base)
wt create my-feature
wt create my-feature develop      # specify base branch

# Remove current worktree and navigate back to main
wt done                           # from inside a linked worktree
wt done --force                   # force removal with uncommitted changes

# Checkout GitHub PR in worktree (requires gh CLI)
wt pr 123
wt pr https://github.com/org/repo/pull/123
wt pr                             # interactive: select from open PRs

# Checkout GitLab MR in worktree (requires glab CLI)
wt mr 123
wt mr https://gitlab.com/org/repo/-/merge_requests/123
wt mr                             # interactive: select from open MRs

# List all worktrees
wt list
wt ls                             # short alias

# Remove a worktree
wt remove old-branch
wt rm old-branch                  # short alias
wt rm                             # interactive: select from existing worktrees

# Clean up worktrees for merged branches
wt cleanup
wt cleanup --dry-run              # preview what would be removed
wt cleanup --force                # remove all without confirmation

# Migrate existing worktrees to configured paths
wt migrate
wt migrate --force                # force when target path exists

# Clean up stale worktree administrative files
wt prune

# Configure shell integration
wt init
wt init --uninstall

# Show worktree location configuration
wt info

# Manage configuration file
wt config init          # Create a default config file
wt config show          # Show effective configuration with sources
wt config path          # Print the config file path

# Show practical examples
wt examples

# Show version
wt version

# Machine-readable JSON output
wt --format json version
wt --format json list
```

### JSON Output (`--format json`)

All commands support machine-readable JSON output with a uniform envelope:

```json
{"ok": true, "command": "wt version", "data": {"version": "0.2.0"}}
```

In JSON mode, shell integration does **not** auto-navigate. Use the `navigate_to` field from the response.

### Interactive Selection

When you run `wt co`, `wt rm`, `wt pr`, or `wt mr` without arguments, you get a numbered selection menu. Press ESC or Ctrl-C to cancel.

## Configuration

### Configuration File

`wt` supports an optional TOML configuration file:

```bash
wt config init          # Create a default config file
wt config init --force  # Overwrite existing config file
wt config show          # Show effective configuration with sources
wt config path          # Print the config file path
```

**File location** (in order of priority):

1. `--config` flag: `wt --config /path/to/config.toml <command>`
2. Default: `~/.config/wt/config.toml` (respects `$XDG_CONFIG_HOME`; `%AppData%\wt\config.toml` on Windows)

**Example config file** (`~/.config/wt/config.toml`):

```toml
root = "~/projects/worktrees"
strategy = "sibling-repo"
separator = "-"

[hooks]
post_create = ["cd \"$WT_PATH\" && npm install"]

[copy_files]
paths = [".env", ".tool-versions", ".envrc"]
```

### Precedence

Configuration values are resolved in this order (highest priority first):

1. **Environment variables** (`WORKTREE_ROOT`, `WORKTREE_STRATEGY`, `WORKTREE_PATTERN`, `WORKTREE_SEPARATOR`)
2. **Config file** (`~/.config/wt/config.toml`)
3. **Built-in defaults**

Run `wt config show` to see the effective value and source of each setting.

### Worktree Strategies

| Strategy | Default pattern |
| --- | --- |
| `global` (default) | `{.worktreeRoot}/{.repo.Name}/{.branch}` |
| `sibling-repo` | `{.repo.Main}/../{.repo.Name}-{.branch}` |
| `parent-branches` | `{.repo.Main}/../{.branch}` |
| `parent-worktrees` | `{.repo.Main}/../{.repo.Name}.worktrees/{.branch}` |
| `parent-dotdir` | `{.repo.Main}/../.worktrees/{.branch}` |
| `inside-dotdir` | `{.repo.Main}/.worktrees/{.branch}` |
| `custom` | User-defined via `pattern` setting |

**Pattern variables:** `{.repo.Name}`, `{.repo.Main}`, `{.repo.Owner}`, `{.repo.Host}`, `{.branch}`, `{.worktreeRoot}`, `{.env.VARNAME}`

### Separator

Controls how `/` in branch names is replaced in paths:

| Separator | `feat/foo` becomes |
| --- | --- |
| `/` (default) | `feat/foo` (nested dirs) |
| `-` | `feat-foo` (flat) |
| `_` | `feat_foo` (flat) |

### Copy Files

Automatically copy files from the main worktree into new worktrees:

```toml
[copy_files]
paths = [".env", ".tool-versions", ".envrc", ".env.local"]
```

Files are copied on `checkout`, `create`, `pr`, and `mr`. Missing files are skipped with a warning. Symlinks are not followed.

### Hooks

Run custom commands before or after operations:

```toml
[hooks]
post_create = ["cd \"$WT_PATH\" && npm install"]
post_checkout = ["cd \"$WT_PATH\" && bundle install"]
pre_remove = ["echo Removing \"$WT_PATH\""]
```

**Available hooks:** `pre_create`, `post_create`, `pre_checkout`, `post_checkout`, `pre_remove`, `post_remove`, `pre_pr`, `post_pr`, `pre_mr`, `post_mr`

**Environment variables in hooks:** `$WT_PATH`, `$WT_BRANCH`, `$WT_MAIN`, `$WT_REPO_NAME`, `$WT_REPO_HOST`, `$WT_REPO_OWNER`

Pre-hooks abort on failure; post-hooks warn only. Set `WT_HOOKS_DISABLED=1` to skip all hooks.

### Multi-Repo Workflows

Group worktrees by feature across repositories:

```toml
strategy = "custom"
pattern = "{.worktreeRoot}/{.branch}/{.repo.Name}"
```

```bash
cd ~/src/shared-lib && wt create feat/PROJ-123
cd ~/src/main-app && wt create feat/PROJ-123
# Result: ~/dev/worktrees/feat/PROJ-123/{shared-lib,main-app}/
```

## Development

Prerequisites: Go 1.26+, [just](https://github.com/casey/just) task runner.

```bash
just              # Show available recipes
just build        # Build the binary
just test         # Run unit tests
just e2e          # Run E2E tests
just test-all     # Run unit + E2E tests
just lint         # Run linters
just build-all    # Cross-compile for all platforms
just dev-shellenv # Shell integration for running from source
```

### Running from Source

```bash
eval "$(just dev-shellenv)"
```

This replaces the `wt` shell function to call `go run` against your local checkout. Auto-cd and tab completion work normally.

## Requirements

- Git
- `gh` CLI (optional, for `wt pr`)
- `glab` CLI (optional, for `wt mr`)
- Go 1.26+ (for building from source)

## License

MIT

## Credits

Originally forked from [timvw/wt](https://github.com/timvw/wt). Inspired by [tree-me](https://github.com/haacked/dotfiles/blob/main/bin/tree-me) by Phil Haack.
