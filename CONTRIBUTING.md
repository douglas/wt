# Contributing to wt

## Development

Prerequisites: Go 1.26+, [just](https://github.com/casey/just) task runner.

```sh
just build                    # build to bin/wt
just install-user             # install to ~/bin
eval "$(just dev-shellenv)"   # dev shell (runs from source)
```

## Testing

```sh
just test          # unit tests (-race -short)
just e2e           # e2e tests (builds binary, runs YAML scenarios)
just test-all      # both unit + e2e
go test -run TestFoo ./...    # run a single test
```

## Code Quality

```sh
golangci-lint run ./...
go vet ./...
go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

The project uses golangci-lint with errcheck, gocritic, gofmt, gosimple,
govet, ineffassign, misspell, revive, staticcheck, and others. See
`.golangci.yml` for the full configuration.

## Architecture

Single Go package (`main`), built on cobra. Key source files:

- `main.go` -- entry point
- `commands.go` -- all CLI commands
- `git.go` -- git helpers and result cache
- `pr.go` -- PR/MR integration
- `migrate.go` -- worktree migration logic
- `config.go` -- configuration loading
- `paths.go` -- path resolution
- `hooks.go` -- git hook management
- `init.go` -- shell integration setup
- `output.go` -- terminal output helpers

Tests use a mock git runner defined in `mock_test.go`.

## Pull Requests

- Run `golangci-lint run ./...` and `just test` before submitting.
- Add tests for new functionality.
- One logical change per commit.
