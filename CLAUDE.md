# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PostgreSQL MCP (Model Context Protocol) server that exposes 9 read-only PostgreSQL tools over stdio. Built with Go, uses `mcp-go` for the protocol and `lib/pq` as the PostgreSQL driver.

## Build & Development Commands

This project uses [Task](https://taskfile.dev/) as its task runner:

```bash
task build              # Build binary → ./postgresql-mcp
task test               # All tests (requires Docker for integration tests)
task test-unit          # Unit tests only (no Docker)
task test-integration   # Integration tests only (Docker required)
task coverage           # Generate coverage report
task lint               # Run golangci-lint
task snapshot           # GoReleaser snapshot build
```

To run a single test: `go test -v -run "TestName" ./...`

Integration tests use testcontainers (real PostgreSQL in Docker). Skip them with `SKIP_INTEGRATION_TESTS=true`.

## Architecture

```
main.go                    → MCP server setup, tool registration, request handlers
internal/app/
  app.go                   → App struct: orchestrates client calls for each tool
  client.go                → PostgreSQLClientImpl: raw SQL queries, query validation, row processing
  interfaces.go            → PostgreSQLClient interface (composed of 4 sub-interfaces), data types, error vars
  logger.go                → Logging helpers
internal/logger/logger.go  → slog wrapper
```

**Key flow:** `main.go` registers tools on the MCP server → each tool handler extracts args, calls `App` methods → `App` delegates to `PostgreSQLClient` interface → `PostgreSQLClientImpl` executes SQL.

**Dependency injection:** `App` accepts a `PostgreSQLClient` interface via `New(client)`. Production uses `NewDefault()`. Tests use `MockPostgreSQLClient` (testify/mock).

**Security model:** Query validation in `client.go` restricts execution to SELECT/WITH statements only. All queries use parameterized arguments.

## Testing Patterns

- Unit tests mock the `PostgreSQLClient` interface with testify/mock
- Integration tests (`integration_test.go`) spin up real PostgreSQL via testcontainers
- Root-level test files test the MCP tool handlers; `internal/app/*_test.go` test the app/client layer

## Linter Configuration

golangci-lint with cyclomatic complexity limit of 15, function length limit of 80 lines / 50 statements. See `.golangci.yml` for disabled linters.
