# Architecture

This document explains the design patterns and system architecture of the PostgreSQL MCP server.

## Overview

The server follows a three-layer architecture:

```
┌─────────────────┐
│   MCP Client    │  (Claude Code)
└────────┬────────┘
         │ JSON-RPC 2.0 over stdio
┌────────▼────────┐
│   MCP Server    │  main.go — tool registration, request handlers
└────────┬────────┘
         │ Go function calls
┌────────▼────────┐
│   App Layer     │  internal/app/app.go — business logic, logging, error wrapping
└────────┬────────┘
         │ PostgreSQLClient interface
┌────────▼────────┐
│   Client Layer  │  internal/app/client.go — SQL queries, validation, row processing
└────────┬────────┘
         │ lib/pq driver
┌────────▼────────┐
│   PostgreSQL    │
└─────────────────┘
```

## Layer Responsibilities

### MCP Server Layer (`main.go`)

- Registers 9 tools on the MCP server using `mcp-go`
- Extracts and validates arguments from `CallToolRequest`
- Delegates to App layer methods
- Formats responses as JSON `CallToolResult`
- Handles command-line flags (`-h`, `-v`)

### App Layer (`internal/app/app.go`)

- Orchestrates client calls for each operation
- Manages connection lifecycle (`ensureConnection`, auto-reconnect)
- Applies business rules (query limits, default schema)
- Logs operations at Debug/Info/Error levels
- Wraps errors with operation context
- Routes security errors to audit logging

### Client Layer (`internal/app/client.go`)

- Executes raw SQL queries against PostgreSQL
- Validates queries (SELECT/WITH only, no semicolons, comment stripping, length limit)
- Processes result rows with type conversion
- Manages connection pool configuration
- Enforces read-only mode at the PostgreSQL session level

### Interface Layer (`internal/app/interfaces.go`)

- Defines `PostgreSQLClient` interface composed of 4 sub-interfaces:
  - `ConnectionManager` — Connect, Close, Ping, GetDB
  - `DatabaseExplorer` — ListDatabases, GetCurrentDatabase, ListSchemas
  - `TableExplorer` — ListTables, ListTablesWithStats, DescribeTable, GetTableStats, ListIndexes
  - `QueryExecutor` — ExecuteQuery, ExplainQuery
- Defines all data types (DatabaseInfo, TableInfo, ColumnInfo, IndexInfo, QueryResult)
- Defines all error variables

## Tool Registration Pattern

Each tool follows a consistent setup pattern:

```go
func setupXxxTool(s *server.MCPServer, appInstance *app.App, debugLogger *slog.Logger) {
    tool := mcp.NewTool("tool_name",
        mcp.WithDescription("..."),
        mcp.WithString("param", mcp.Required(), mcp.Description("...")),
    )
    s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        args := request.GetArguments()
        // 1. Extract and validate parameters
        // 2. Call appInstance method
        // 3. Marshal result to JSON
        // 4. Return mcp.NewToolResultText(jsonData)
    })
}
```

All setup functions are called from `registerAllTools()` in `main()`.

### TableToolConfig Abstraction

Three table-based tools (describe_table, list_indexes, get_table_stats) share identical boilerplate: table/schema parameter extraction, JSON marshaling, error handling. `TableToolConfig` eliminates this duplication:

```go
type TableToolConfig struct {
    Name        string
    Description string
    TableDesc   string
    Operation   func(ctx context.Context, appInstance *app.App, schema, table string) (any, error)
    SuccessMsg  func(result any, schema, table string) (string, []any)
    ErrorMsg    string
}
```

`setupTableTool()` accepts a config and generates the full tool registration. Use it for any new tool that takes table + optional schema parameters.

## Interface Design

`PostgreSQLClient` is an interface (not a concrete type) for two reasons:

1. **Testing**: Unit tests inject `MockPostgreSQLClient` (testify/mock) to test App logic without a database
2. **Separation of concerns**: App layer depends on behavior, not implementation

Production uses `PostgreSQLClientImpl` via `NewDefault()`. Tests use `New(mockClient)`.

## Error Handling

Errors are handled differently at each layer:

| Layer | Strategy |
|-------|----------|
| **Client** | Returns typed errors (`ErrInvalidQuery`, `ErrMultiStatementQuery`, etc.) or wrapped database errors |
| **App** | Wraps all errors with operation context (`fmt.Errorf("failed to list tables: %w", err)`). Routes security errors (`ErrInvalidQuery`, `ErrMultiStatementQuery`, `ErrQueryTooLong`, `ErrResultTooLarge`) to `logSecurityEvent()` |
| **MCP Server** | Converts errors to `mcp.NewToolResultError(err.Error())` for the client |

Security events are logged at Warn level with structured fields (event type, truncated query preview, query length) for monitoring.

## Security Model

Defense-in-depth with multiple layers:

1. **Query validation** (`validateQuery`): Rejects non-SELECT/WITH queries, strips comments first to prevent comment-based injection, detects semicolons outside literals to block multi-statement attacks
2. **Read-only transactions**: Connection string injected with `default_transaction_read_only=on` so PostgreSQL itself rejects any mutation
3. **Identifier escaping**: `pq.QuoteIdentifier()` for all dynamic schema/table names
4. **Query size limit**: 1MB max (`MaxQueryLength`), checked before any processing
5. **Result size limit**: 10,000 rows max (`defaultMaxResultRows`), enforced during row iteration

## Connection Management

- **Pool configuration**: MaxOpenConns=10, MaxIdleConns=5, ConnMaxLifetime=1h, ConnMaxIdleTime=10m (all configurable via `POSTGRES_MCP_*` env vars)
- **Auto-reconnection**: `ensureConnection()` pings before every operation; on failure, attempts one reconnection using background context
- **Read-only injection**: `injectReadOnlyOption()` appends `default_transaction_read_only=on` to connection strings (handles both URL and keyword-value formats)

## Testing Strategy

### Unit Tests (no Docker required)

- Mock `PostgreSQLClient` interface with testify/mock
- Test App layer logic, error handling, security audit routing
- Test client-layer helpers directly (validateQuery, stripComments, etc.)
- Run with: `task test-unit` or `SKIP_INTEGRATION_TESTS=true go test ./...`

### Integration Tests (Docker required)

- Use testcontainers to spin up real PostgreSQL 17
- Test end-to-end: connection, queries, read-only enforcement, pool configuration
- Run with: `task test` or `go test ./...`
- Skip with: `SKIP_INTEGRATION_TESTS=true`

### Test file locations

| File | Scope |
|------|-------|
| `internal/app/client_test.go` | Client layer: validation, helpers, connection |
| `internal/app/client_mocked_test.go` | Client layer with mocked DB |
| `internal/app/app_test.go` | App layer with mocked client |
| `main_test.go`, `main_*_test.go` | MCP tool handlers, CLI flags |
| `integration_test.go` | End-to-end with real PostgreSQL |

## Adding a New Tool

1. **Add App method** in `internal/app/app.go`:
   ```go
   func (a *App) NewOperation(ctx context.Context, ...) (ResultType, error) {
       if err := a.ensureConnection(ctx); err != nil {
           return nil, fmt.Errorf("failed to new operation: %w", err)
       }
       // delegate to a.client
   }
   ```

2. **Add Client method** in `internal/app/client.go` if new SQL is needed, and add it to the appropriate sub-interface in `interfaces.go`.

3. **Register the tool** in `main.go`:
   - For table+schema tools: use `setupTableTool()` with a `TableToolConfig`
   - For other tools: create a `setupNewTool()` function following the existing pattern

4. **Add to `registerAllTools()`** in `main.go`.

5. **Add tests**: unit test in `app_test.go` (with mock), optionally integration test in `integration_test.go`.

6. **Update docs**: add the tool to `docs/tools.md`.
