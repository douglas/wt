package main

import (
	"fmt"
)

func init() {
	registerCommand(&command{
		name:  "pr",
		short: "Checkout GitHub PR in worktree (uses gh CLI)",
		long: `Checkout a GitHub Pull Request in a worktree.

Looks up the PR's actual branch name using the 'gh' CLI, then creates
a worktree with that branch name — just like 'wt checkout <branch>'.
For GitLab Merge Requests, use 'wt mr' instead.

Examples:
  wt pr                                        # Interactive PR selection
  wt pr 123                                    # GitHub PR number
  wt pr https://github.com/org/repo/pull/123   # GitHub PR URL`,
		usage: "[number|url]",
		run: func(args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg, got %d", len(args))
			}

			var input string

			// Interactive selection if no PR provided
			if len(args) == 0 {
				if isJSONOutput() {
					return fmt.Errorf("wt pr with --format json requires an explicit PR number or URL")
				}
				numbers, labels, err := getOpenPRs()
				if err != nil {
					return fmt.Errorf("failed to get PRs: %w (is 'gh' CLI installed?)", err)
				}
				if len(labels) == 0 {
					return fmt.Errorf("no open PRs found")
				}

				idx, _, err := selectItem("Select Pull Request", labels)
				if err != nil {
					return ErrCancelled
				}
				input = numbers[idx]
			} else {
				input = args[0]
			}

			return checkoutPROrMR("pr", input, RemoteGitHub)
		},
	})

	registerCommand(&command{
		name:  "mr",
		short: "Checkout GitLab MR in worktree (uses glab CLI)",
		long: `Checkout a GitLab Merge Request in a worktree.

Looks up the MR's actual branch name using the 'glab' CLI, then creates
a worktree with that branch name — just like 'wt checkout <branch>'.
For GitHub Pull Requests, use 'wt pr' instead.

Examples:
  wt mr                                        # Interactive MR selection
  wt mr 123                                    # GitLab MR number
  wt mr https://gitlab.com/org/repo/-/merge_requests/123  # GitLab MR URL`,
		usage: "[number|url]",
		run: func(args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("accepts at most 1 arg, got %d", len(args))
			}

			var input string

			// Interactive selection if no MR provided
			if len(args) == 0 {
				if isJSONOutput() {
					return fmt.Errorf("wt mr with --format json requires an explicit MR number or URL")
				}
				numbers, labels, err := getOpenMRs()
				if err != nil {
					return fmt.Errorf("failed to get MRs: %w (is 'glab' CLI installed?)", err)
				}
				if len(labels) == 0 {
					return fmt.Errorf("no open MRs found")
				}

				idx, _, err := selectItem("Select Merge Request", labels)
				if err != nil {
					return ErrCancelled
				}
				input = numbers[idx]
			} else {
				input = args[0]
			}

			return checkoutPROrMR("mr", input, RemoteGitLab)
		},
	})
}
