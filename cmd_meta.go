package main

import (
	"fmt"
	"os"
	"runtime"
)

func init() {
	registerCommand(&command{
		name:  "info",
		short: "Show worktree location configuration",
		run: func(_ []string) error {
			jsonMode := isJSONOutput()
			pattern, err := resolveWorktreePattern()
			if err != nil {
				pattern = appCfg.Pattern
				if pattern == "" {
					pattern = "unknown"
				}
			}

			configStatus := "not found, using defaults"
			if appCfg.ConfigFileFound {
				configStatus = "found"
			}

			hooks := map[string][]string{
				"pre_create":    appCfg.Hooks.PreCreate,
				"post_create":   appCfg.Hooks.PostCreate,
				"pre_checkout":  appCfg.Hooks.PreCheckout,
				"post_checkout": appCfg.Hooks.PostCheckout,
				"pre_remove":    appCfg.Hooks.PreRemove,
				"post_remove":   appCfg.Hooks.PostRemove,
				"pre_pr":        appCfg.Hooks.PrePR,
				"post_pr":       appCfg.Hooks.PostPR,
				"pre_mr":        appCfg.Hooks.PreMR,
				"post_mr":       appCfg.Hooks.PostMR,
			}

			if jsonMode {
				return emitJSONSuccess("info", map[string]any{
					"config": map[string]string{
						"path":      appCfg.ConfigFilePath,
						"status":    configStatus,
						"strategy":  appCfg.Strategy,
						"pattern":   pattern,
						"root":      appCfg.Root,
						"separator": appCfg.Separator,
					},
					"strategies": []map[string]string{
						{"name": "global", "pattern": "{.worktreeRoot}/{.repo.Name}/{.branch}"},
						{"name": "sibling-repo", "pattern": "{.repo.Main}/../{.repo.Name}-{.branch}"},
						{"name": "parent-branches", "pattern": "{.repo.Main}/../{.branch}"},
						{"name": "parent-worktrees", "pattern": "{.repo.Main}/../{.repo.Name}.worktrees/{.branch}"},
						{"name": "parent-dotdir", "pattern": "{.repo.Main}/../.worktrees/{.branch}"},
						{"name": "inside-dotdir", "pattern": "{.repo.Main}/.worktrees/{.branch}"},
						{"name": "custom", "pattern": "requires pattern setting"},
					},
					"pattern_variables": []string{"{.repo.Name}", "{.repo.Main}", "{.repo.Owner}", "{.repo.Host}", "{.branch}", "{.worktreeRoot}", "{.env.VARNAME}"},
					"hooks":             hooks,
				})
			}

			fmt.Printf(`Config:    %s (%s)

Strategy:  %s
Pattern:   %s
Root:      %s
Separator: %q

Strategies:
  global           -> {.worktreeRoot}/{.repo.Name}/{.branch}
  sibling-repo     -> {.repo.Main}/../{.repo.Name}-{.branch}
  parent-branches  -> {.repo.Main}/../{.branch}
  parent-worktrees -> {.repo.Main}/../{.repo.Name}.worktrees/{.branch}
  parent-dotdir    -> {.repo.Main}/../.worktrees/{.branch}
  inside-dotdir    -> {.repo.Main}/.worktrees/{.branch}
  custom           -> requires pattern setting

Pattern variables: {.repo.Name}, {.repo.Main}, {.repo.Owner}, {.repo.Host}, {.branch}, {.worktreeRoot}, {.env.VARNAME}
Note: The separator setting controls how "/" and "\" in value variables are replaced.
      Default "/" preserves slashes (nested dirs). Set to "-" or "_" for flat paths.
      Path variables ({.repo.Main}, {.worktreeRoot}) are never transformed.
Note: {.env.VARNAME} accesses the environment variable VARNAME (e.g. {.env.HOME}).
`, appCfg.ConfigFilePath, configStatus, appCfg.Strategy, pattern, appCfg.Root, appCfg.Separator)

			// Show configured hooks
			hookNames := []struct {
				name  string
				hooks []string
			}{
				{"pre_create", appCfg.Hooks.PreCreate},
				{"post_create", appCfg.Hooks.PostCreate},
				{"pre_checkout", appCfg.Hooks.PreCheckout},
				{"post_checkout", appCfg.Hooks.PostCheckout},
				{"pre_remove", appCfg.Hooks.PreRemove},
				{"post_remove", appCfg.Hooks.PostRemove},
				{"pre_pr", appCfg.Hooks.PrePR},
				{"post_pr", appCfg.Hooks.PostPR},
				{"pre_mr", appCfg.Hooks.PreMR},
				{"post_mr", appCfg.Hooks.PostMR},
			}
			hasHooks := false
			for _, h := range hookNames {
				if len(h.hooks) > 0 {
					hasHooks = true
					break
				}
			}
			if hasHooks {
				fmt.Println("Hooks:")
				for _, h := range hookNames {
					if len(h.hooks) > 0 {
						for _, cmd := range h.hooks {
							fmt.Printf("  %-15s %s\n", h.name+":", cmd)
						}
					}
				}
				fmt.Println()
			} else {
				fmt.Println("Hooks:    (none configured)")
				fmt.Println()
			}

			return nil
		},
	})

	registerCommand(&command{
		name:  "shellenv",
		short: "Output shell function for auto-cd (source this)",
		long: `Output shell integration code for automatic directory navigation.

Add this to the END of your ~/.bashrc or ~/.zshrc:
  source <(wt shellenv)

For PowerShell, add this to your $PROFILE:
  Invoke-Expression (& wt shellenv)

Note: For zsh, place this AFTER compinit to enable tab completion.

This enables:
- Automatic cd to worktree after checkout/create/pr/mr commands
- Tab completion for commands and branch names`,
		run: func(_ []string) error {
			if isJSONOutput() {
				_ = emitJSONSuccess("shellenv", map[string]string{
					"note": "shellenv outputs shell script text; run without --format json to source it",
				})
				return nil
			}
			// Output OS-specific shell integration
			// On Windows, default to PowerShell. On Unix, output bash/zsh.
			if runtime.GOOS == "windows" {
				// PowerShell integration for Windows
				fmt.Print(`# PowerShell integration (Windows)
# Detected via runtime.GOOS, compatible with $PSVersionTable
# NOTE: Requires wt.exe to be in PATH or current directory

function wt {
    # Call wt.exe explicitly to avoid recursive function call
    # PowerShell will find wt.exe in PATH or current directory
    $output = & wt.exe @args
    $exitCode = $LASTEXITCODE
    Write-Output $output

    # In JSON mode, keep stdout machine-readable and skip auto-navigation.
    $isJson = $false
    for ($i = 0; $i -lt $args.Count; $i++) {
        if ($args[$i] -eq '--format' -and $i + 1 -lt $args.Count -and $args[$i + 1] -eq 'json') {
            $isJson = $true
        }
        if ($args[$i] -eq '--format=json') {
            $isJson = $true
        }
    }
    if ($isJson) {
        $global:LASTEXITCODE = $exitCode
        return
    }

    if ($exitCode -eq 0) {
        $cdPath = $output | Select-String -Pattern "^wt navigating to: " | ForEach-Object { $_.Line.Substring(18) }
        if ($cdPath) {
            Set-Location $cdPath
        }
    }
    $global:LASTEXITCODE = $exitCode
}

# PowerShell completion
Register-ArgumentCompleter -CommandName wt -ScriptBlock {
    param($commandName, $wordToComplete, $commandAst, $fakeBoundParameters)

    $commands = @('checkout', 'co', 'create', 'done', 'pr', 'mr', 'list', 'ls', 'remove', 'rm', 'cleanup', 'migrate', 'prune', 'help', 'shellenv', 'init', 'info', 'config', 'examples', 'version')

    # Get the position in the command line
    $position = $commandAst.CommandElements.Count - 1

    if ($position -eq 0) {
        # Complete commands
        $commands | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
            [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
        }
    } elseif ($position -eq 1) {
        $subCommand = $commandAst.CommandElements[1].Value
        if ($subCommand -in @('checkout', 'co', 'create')) {
            # Complete branch names from all local and remote branches
            $remotes = (git remote 2>$null) -join '|'
            $branches = git branch -a --format='%(refname:short)' 2>$null | Where-Object { $_ -notmatch 'HEAD' } | ForEach-Object { $_ -replace "^($remotes)/", '' } | Sort-Object -Unique
            $branches | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        } elseif ($subCommand -in @('remove', 'rm')) {
            # Complete branch names from existing worktrees
            $branches = git worktree list 2>$null | Select-Object -Skip 1 | ForEach-Object {
                if ($_ -match '\[([^\]]+)\]') { $matches[1] }
            }
            $branches | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        } elseif ($subCommand -eq 'config') {
            @('init', 'show', 'path') | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
                [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterValue', $_)
            }
        }
    }
}
`)
				return nil
			}

			// Bash/Zsh integration for Unix systems
			os.Stdout.WriteString(`wt() {
    # In JSON mode, keep stdout machine-readable and skip auto-navigation.
    case " $* " in
        *" --format json "*|*" --format=json "*)
            command wt "$@"
            return $?
            ;;
    esac

    local output exit_code cd_path
    output=$(command wt "$@")
    exit_code=$?
    printf '%s\n' "$output"
    cd_path=$(printf '%s\n' "$output" | grep '^wt navigating to: ' | tail -1 | sed 's/^wt navigating to: //')
    if [ $exit_code -eq 0 ] && [ -n "$cd_path" ]; then
        cd "$cd_path"
    fi
    return $exit_code
}

# Bash completion
if [ -n "$BASH_VERSION" ]; then
    _wt_complete() {
        local cur prev commands
        COMPREPLY=()
        cur="${COMP_WORDS[COMP_CWORD]}"
        prev="${COMP_WORDS[COMP_CWORD-1]}"
        commands="checkout co create done pr mr list ls remove rm cleanup migrate prune help shellenv init info config examples version"

        # Complete commands if first argument
        if [ $COMP_CWORD -eq 1 ]; then
            COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
            return 0
        fi

        # Complete branch names for checkout/co/create and worktree branches for remove/rm
        case "$prev" in
            checkout|co|create)
                local branches remotes
                remotes=$(git remote 2>/dev/null | paste -sd'|' -)
                branches=$(git branch -a --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed -E "s#^($remotes)/##" | sort -u)
                COMPREPLY=( $(compgen -W "$branches" -- "$cur") )
                return 0
                ;;
            remove|rm)
                local branches
                branches=$(git worktree list 2>/dev/null | tail -n +2 | sed -n 's/.*\[\([^]]*\)\].*/\1/p')
                COMPREPLY=( $(compgen -W "$branches" -- "$cur") )
                return 0
                ;;
            config)
                COMPREPLY=( $(compgen -W "init show path" -- "$cur") )
                return 0
                ;;
        esac
    }
    complete -F _wt_complete wt
fi

# Zsh completion
if [ -n "$ZSH_VERSION" ]; then
    _wt_complete_zsh() {
        local -a commands branches
        commands=(
            'checkout:Checkout existing branch in new worktree'
            'co:Checkout existing branch in new worktree'
            'create:Create new branch in worktree'
            'done:Remove current worktree and navigate back'
            'pr:Checkout GitHub PR in worktree'
            'mr:Checkout GitLab MR in worktree'
            'list:List all worktrees'
            'ls:List all worktrees'
            'remove:Remove a worktree'
            'rm:Remove a worktree'
            'cleanup:Remove worktrees for merged branches'
            'migrate:Migrate existing worktrees to configured paths'
            'prune:Remove worktree administrative files'
            'help:Show help'
            'shellenv:Output shell function for auto-cd'
            'init:Initialize shell integration'
            'info:Show worktree location configuration'
            'config:Manage wt configuration'
            'examples:Show practical command examples'
            'version:Show version information'
        )

        if (( CURRENT == 2 )); then
            _describe 'command' commands
        elif (( CURRENT == 3 )); then
            case "$words[2]" in
                checkout|co|create)
                    local remotes
                    remotes=$(git remote 2>/dev/null | paste -sd'|' -)
                    branches=(${(f)"$(git branch -a --format='%(refname:short)' 2>/dev/null | grep -v 'HEAD' | sed -E "s#^($remotes)/##" | sort -u)"})
                    _describe 'branch' branches
                    ;;
                remove|rm)
                    branches=(${(f)"$(git worktree list 2>/dev/null | tail -n +2 | sed -n 's/.*\[\([^]]*\)\].*/\1/p')"})
                    _describe 'branch' branches
                    ;;
                config)
                    local -a config_cmds
                    config_cmds=(
                        'init:Create a default configuration file'
                        'show:Show effective configuration with sources'
                        'path:Print the config file path'
                    )
                    _describe 'config command' config_cmds
                    ;;
            esac
        fi
    }
    # Only register completion if compdef is available
    if (( $+functions[compdef] )); then
        compdef _wt_complete_zsh wt
    fi
fi
`)
			return nil
		},
	})

	registerCommand(&command{
		name:  "version",
		short: "Show version information",
		run: func(_ []string) error {
			if isJSONOutput() {
				return emitJSONSuccess("version", map[string]string{"version": version})
			}
			fmt.Printf("wt version %s\n", version)
			return nil
		},
	})
}
