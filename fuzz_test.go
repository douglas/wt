package main

import (
	"testing"
)

func FuzzParseRemoteURL(f *testing.F) {
	f.Add("https://github.com/user/repo.git")
	f.Add("https://github.com/user/repo")
	f.Add("git@github.com:user/repo.git")
	f.Add("git@gitlab.com:group/subgroup/repo.git")
	f.Add("https://gitlab.com/group/subgroup/repo.git")
	f.Add("ssh://git@github.com/user/repo.git")
	f.Add("")
	f.Add("not-a-url")
	f.Add("https://")
	f.Add("git@:")

	f.Fuzz(func(_ *testing.T, input string) {
		// Must not panic on any input.
		parseRemoteURL(input)
	})
}

func FuzzGetPRNumber(f *testing.F) {
	f.Add("123")
	f.Add("1")
	f.Add("99999")
	f.Add("https://github.com/user/repo/pull/42")
	f.Add("https://github.com/org/project/pull/1234")
	f.Add("https://gitlab.com/group/project/-/merge_requests/56")
	f.Add("")
	f.Add("abc")
	f.Add("https://example.com/not-a-pr")

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic on any input.
		result, err := getPRNumber(input)
		if err == nil && result == "" {
			t.Error("getPRNumber returned no error but empty result")
		}
	})
}

func FuzzParsePROutput(f *testing.F) {
	f.Add("123\tFix bug\n456\tAdd feature")
	f.Add("1\tFirst PR")
	f.Add("100\tRefactor core\n200\tUpdate docs\n300\tBump deps")
	f.Add("")
	f.Add("\n\n\n")
	f.Add("no-tab-here")
	f.Add("123\t")
	f.Add("\t")

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic on any input.
		numbers, labels := parsePROutput(input)
		if len(numbers) != len(labels) {
			t.Errorf("parsePROutput returned mismatched slices: %d numbers, %d labels",
				len(numbers), len(labels))
		}
	})
}

func FuzzParseMROutput(f *testing.F) {
	f.Add("!123  open  Fix bug  (branch) \u2190 (main)")
	f.Add("!456  open  Add feature  (feat-branch) \u2190 (main)")
	f.Add("!1  open  Initial MR  (dev) \u2190 (main)\n!2  open  Second MR  (fix) \u2190 (main)")
	f.Add("")
	f.Add("not a merge request line")
	f.Add("!abc  open  Bad number  (branch) \u2190 (main)")
	f.Add("!\n!\n!")

	f.Fuzz(func(t *testing.T, input string) {
		// Must not panic on any input.
		numbers, labels := parseMROutput(input)
		if len(numbers) != len(labels) {
			t.Errorf("parseMROutput returned mismatched slices: %d numbers, %d labels",
				len(numbers), len(labels))
		}
	})
}
