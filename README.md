# PostgreSQL MCP Server

[![GitHub release](https://img.shields.io/github/release/sgaunet/postgresql-mcp.svg)](https://github.com/sgaunet/postgresql-mcp/releases/latest)
![GitHub Downloads](https://img.shields.io/github/downloads/sgaunet/postgresql-mcp/total)
![Coverage](https://raw.githubusercontent.com/wiki/sgaunet/postgresql-mcp/coverage-badge.svg)
[![coverage](https://github.com/sgaunet/postgresql-mcp/actions/workflows/coverage.yml/badge.svg)](https://github.com/sgaunet/postgresql-mcp/actions/workflows/coverage.yml)
[![Snapshot Build](https://github.com/sgaunet/postgresql-mcp/actions/workflows/snapshot.yml/badge.svg)](https://github.com/sgaunet/postgresql-mcp/actions/workflows/snapshot.yml)
[![Release Build](https://github.com/sgaunet/postgresql-mcp/actions/workflows/release.yml/badge.svg)](https://github.com/sgaunet/postgresql-mcp/actions/workflows/release.yml)
[![License](https://img.shields.io/github/license/sgaunet/postgresql-mcp.svg)](LICENSE)

A Model Context Protocol (MCP) server that provides PostgreSQL integration tools for Claude Code.

## Features

- **List Databases**: List all databases on the PostgreSQL server
- **List Schemas**: List all schemas in the current database
- **List Tables**: List tables in a specific schema with optional metadata (size, row count)
- **Describe Table**: Get detailed table structure (columns, types, constraints, defaults)
- **Execute Query**: Execute read-only SQL queries (SELECT and WITH statements only)
- **List Indexes**: List indexes for a specific table with usage statistics
- **Explain Query**: Get execution plans for SQL queries to analyze performance
- **Get Table Stats**: Get detailed statistics for tables (row count, size, etc.)
- Security-first design with read-only operations by default
- Compatible with Claude Code's MCP architecture

## Prerequisites

- Go 1.25 or later
- Docker (required for running integration tests)
- Access to PostgreSQL databases

## Installation

### Option 1: Install with Homebrew (Recommended for macOS/Linux)

```bash
# Add the tap and install
brew tap sgaunet/homebrew-tools
brew install sgaunet/tools/postgresql-mcp
```

### Option 2: Download from GitHub Releases

1. **Download the latest release:**

   Visit the [releases page](https://github.com/sgaunet/postgresql-mcp/releases/latest) and download the appropriate binary for your platform:

   - **macOS**: `postgresql-mcp_VERSION_darwin_amd64` (Intel) or `postgresql-mcp_VERSION_darwin_arm64` (Apple Silicon)
   - **Linux**: `postgresql-mcp_VERSION_linux_amd64` (x86_64) or `postgresql-mcp_VERSION_linux_arm64` (ARM64)
   - **Windows**: `postgresql-mcp_VERSION_windows_amd64.exe`

2. **Make it executable (macOS/Linux):**
   ```bash
   chmod +x postgresql-mcp_*
   ```

3. **Move to a location in your PATH:**
   ```bash
   # Example for macOS/Linux
   sudo mv postgresql-mcp_* /usr/local/bin/postgresql-mcp
   ```

### Option 3: Build from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/sgaunet/postgresql-mcp.git
   cd postgresql-mcp
   ```

2. **Build the project:**
   ```bash
   task build
   ```

   Or manually:
   ```bash
   go build -o postgresql-mcp
   ```

3. **Install to your PATH:**
   ```bash
   sudo mv postgresql-mcp /usr/local/bin/
   ```

## Installation for a project

A PostgreSQL connection is specific to the project you are working on, so declare the MCP server at the project level rather than globally.

### Option 1: Add it with the `claude` CLI (Recommended)

Run this from the root of the project that needs database access:

```bash
claude mcp add -s project \
  --env "POSTGRES_URL=postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
  --transport stdio postgresql postgresql-mcp
```

Breakdown of the command:

| Part | Meaning |
|------|---------|
| `-s project` | Writes the server to `.mcp.json` in the current directory (project scope) instead of your user-wide config |
| `--env "POSTGRES_URL=..."` | Sets the connection URL for the server process (see [Configuration](#configuration) for all supported variables) |
| `--transport stdio` | The server talks MCP over stdio |
| `postgresql` | The name the server is registered under in Claude Code |
| `postgresql-mcp` | The command to run — must be on your `PATH` (use an absolute path otherwise) |

Then verify the server is registered and reachable:

```bash
claude mcp list          # show configured servers
claude mcp get postgresql # show this server's configuration
```

To remove it again:

```bash
claude mcp remove -s project postgresql
```

### Option 2: Write `.mcp.json` by hand

At the root of your project, create a file named `.mcp.json` with the following content:

```json
{
  "mcpServers": {
    "postgresql": {
      "type": "stdio",
      "command": "postgresql-mcp",
      "args": [],
      "env": {
        "POSTGRES_URL": "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
      }
    }
  }
}
```

This is the same file the `claude mcp add -s project` command writes for you.

**Note:** `.mcp.json` contains your database credentials. Add it to your `.gitignore` if you don't want to commit it, or omit `--env`/the `env` block and export `POSTGRES_URL` in your shell instead.

## Configuration

The PostgreSQL MCP server can be configured via environment variables.

### Connection

- `POSTGRES_URL`: PostgreSQL connection URL (format: `postgres://user:password@host:port/dbname?sslmode=prefer`)
- `DATABASE_URL`: Alternative to `POSTGRES_URL` if `POSTGRES_URL` is not set

**Example:**
```bash
export POSTGRES_URL="postgres://user:password@localhost:5432/mydb?sslmode=prefer"
# or
export DATABASE_URL="postgres://user:password@localhost:5432/mydb?sslmode=prefer"
```

**Note:** Connection environment variables are optional. Use the `connect_database` tool for explicit connection management. The server will attempt to reconnect automatically when a tool is requested.

### Tuning

| Variable | Description | Default |
|----------|-------------|---------|
| `POSTGRES_MCP_MAX_OPEN_CONNS` | Maximum open database connections (pgxpool `MaxConns`) | `10` |
| `POSTGRES_MCP_MAX_IDLE_CONNS` | Minimum connections kept warm (pgxpool `MinConns`); clamped to the open limit | `5` |
| `POSTGRES_MCP_CONN_MAX_LIFETIME` | Connection max lifetime in seconds | `3600` |
| `POSTGRES_MCP_CONN_MAX_IDLE_TIME` | Connection max idle time in seconds | `600` |
| `POSTGRES_MCP_MAX_RESULT_ROWS` | Maximum rows returned per query | `10000` |

## Connection Management

The server automatically manages database connections with health checks and transparent reconnection:

1. Before every tool operation, the server pings the database to verify the connection is alive.
2. If the ping fails (e.g., database restart, network interruption), the server logs a warning and attempts **one** automatic reconnection using the original connection parameters.
3. If reconnection succeeds, the operation proceeds normally (with a slight delay).
4. If reconnection fails, the operation returns an error asking the user to reconnect via `connect_database`.

Only one reconnection attempt is made per operation — there is no retry loop or backoff. For environments with frequent connection drops, consider tuning `POSTGRES_MCP_CONN_MAX_LIFETIME` and `POSTGRES_MCP_CONN_MAX_IDLE_TIME` to recycle connections proactively.

## Available Tools

The PostgreSQL MCP server provides 9 database tools for interacting with PostgreSQL databases. For detailed information about each tool, including parameters, return values, and examples, see the [Tools Documentation](docs/tools.md).

## Security

This MCP server is designed with security as a priority:

- **Read-only by default**: Only SELECT and WITH queries are permitted
- **Parameterized queries**: Protection against SQL injection
- **Connection validation**: Ensures valid database connections before operations
- **Error handling**: Comprehensive error handling with detailed logging

## Usage with Claude Code

1. **Register the MCP server in the project that needs it** — see [Installation for a project](#installation-for-a-project):
   ```bash
   claude mcp add -s project \
     --env "POSTGRES_URL=postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
     --transport stdio postgresql postgresql-mcp
   ```

2. **Or set up your database connection via environment variables** instead of `--env`:
   ```bash
   export POSTGRES_URL="postgres://user:pass@localhost:5432/mydb"
   ```

3. **Use the tools in your conversations:**
   ```
   List all tables in the public schema
   Describe the users table
   Execute query: SELECT * FROM users LIMIT 10
   ```

## Documentation

- [Tools Documentation](docs/tools.md) - Detailed reference for all available tools with parameters and examples
- [Architecture](docs/ARCHITECTURE.md) - Design patterns, layer responsibilities, and system architecture

## Development

### Building
```bash
go build -o postgresql-mcp
```

### Testing

#### Unit Tests
```bash
# Run unit tests only (no Docker required)
SKIP_INTEGRATION_TESTS=true go test ./...
```

#### Integration Tests
```bash
# Run all tests including integration tests (requires Docker)
go test ./...

# Run only integration tests
go test -run "TestIntegration" ./...
```

**Note:** Integration tests use [testcontainers](https://golang.testcontainers.org/) to automatically spin up PostgreSQL instances in Docker containers. This ensures tests are isolated, reproducible, and don't require manual PostgreSQL setup.

### Dependencies
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [pgx/v5](https://github.com/jackc/pgx) - PostgreSQL driver and connection pool (pgxpool)
- [testcontainers-go](https://github.com/testcontainers/testcontainers-go) - Integration testing with Docker containers

## Troubleshooting

### Connection Issues
- Verify PostgreSQL is running and accessible
- Check the `POSTGRES_URL` or `DATABASE_URL` environment variable is correctly set
- Ensure the connection string format is correct: `postgres://user:password@host:port/dbname?sslmode=prefer`
- Verify database credentials and permissions
- Check firewall and network connectivity

### Permission Issues
- Ensure the database user has appropriate read permissions
- Verify the user can connect to the specified database
- Check if the user has access to the schemas and tables you're trying to query

### Query Errors
- Remember that only SELECT and WITH statements are allowed
- Ensure proper SQL syntax
- Check that referenced tables and columns exist
- Verify you have read permissions on the objects being queried

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Submit a pull request

## License

This project is licensed under MIT license.