package main

import "testing"

func BenchmarkParseRemoteURL_HTTPS(b *testing.B) {
	b.ReportAllocs()
	input := "https://github.com/timvw/wt.git"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseRemoteURL(input)
	}
}

func BenchmarkParseRemoteURL_SCP(b *testing.B) {
	b.ReportAllocs()
	input := "git@github.com:timvw/wt.git"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseRemoteURL(input)
	}
}

func BenchmarkRenderWorktreePath(b *testing.B) {
	b.ReportAllocs()

	oldPattern := worktreePattern
	oldSeparator := worktreeSeparator
	oldRoot := worktreeRoot
	worktreePattern = "{.worktreeRoot}/{.repo.Name}/{.branch}"
	worktreeSeparator = "-"
	worktreeRoot = "/tmp/wt-bench"
	defer func() {
		worktreePattern = oldPattern
		worktreeSeparator = oldSeparator
		worktreeRoot = oldRoot
	}()

	info := repoInfo{
		Main:  "/home/user/repos/wt",
		Host:  "github.com",
		Owner: "timvw",
		Name:  "wt",
	}
	branch := "feature/my-branch"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderWorktreePath(info, branch)
	}
}

func BenchmarkResolveWorktreePattern(b *testing.B) {
	b.ReportAllocs()

	oldStrategy := worktreeStrategy
	oldPattern := worktreePattern
	worktreeStrategy = "global"
	worktreePattern = ""
	defer func() {
		worktreeStrategy = oldStrategy
		worktreePattern = oldPattern
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolveWorktreePattern()
	}
}

func BenchmarkGetPRNumber_URL(b *testing.B) {
	b.ReportAllocs()
	input := "https://github.com/timvw/wt/pull/42"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getPRNumber(input)
	}
}

func BenchmarkGetPRNumber_Number(b *testing.B) {
	b.ReportAllocs()
	input := "42"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getPRNumber(input)
	}
}

func BenchmarkParsePROutput(b *testing.B) {
	b.ReportAllocs()
	input := "1\tFix login bug\n2\tAdd dark mode\n3\tUpdate dependencies\n4\tRefactor auth module\n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsePROutput(input)
	}
}

func BenchmarkParseMROutput(b *testing.B) {
	b.ReportAllocs()
	input := "!10  open  Fix login bug  (fix-login) \u2190 (main)\n!11  open  Add dark mode  (dark-mode) \u2190 (main)\n!12  open  Update deps  (update-deps) \u2190 (develop)\n"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseMROutput(input)
	}
}
