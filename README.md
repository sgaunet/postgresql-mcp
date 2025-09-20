# PostgreSQL MCP Server

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

### Build from Source

1. **Clone the repository:**
   ```bash
   git clone https://github.com/sylvain/postgresql-mcp.git
   cd postgresql-mcp
   ```

2. **Build the server:**
   ```bash
   go build -o postgresql-mcp
   ```

3. **Test the installation:**
   ```bash
   ./postgresql-mcp -v
   ```

## Installation for a project

Add the MCP server in the configuration of the project. At the root of your project, create a file named `.mcap.json' with the following content:

```json
{
  "mcpServers": {
    "postgres": {
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

Don't forget to add the .mcp.json file in your .gitignore file if you don't want to commit it. It usually make sense to declare the MCP server for postgresl at the project level, as the database connection is project specific.

## Configuration

The PostgreSQL MCP server requires database connection information to be provided via environment variables.

### Environment Variables

- `POSTGRES_URL` (required): PostgreSQL connection URL (format: `postgres://user:password@host:port/dbname?sslmode=prefer`)
- `DATABASE_URL` (alternative): Alternative to `POSTGRES_URL` if `POSTGRES_URL` is not set

**Example:**
```bash
export POSTGRES_URL="postgres://user:password@localhost:5432/mydb?sslmode=prefer"
# or
export DATABASE_URL="postgres://user:password@localhost:5432/mydb?sslmode=prefer"
```

**Note:** The server will attempt to connect to the database on startup. If the connection fails, it will log a warning and retry when the first tool is requested.

## Available Tools

The PostgreSQL MCP server provides 8 database tools for interacting with PostgreSQL databases. For detailed information about each tool, including parameters, return values, and examples, see the [Tools Documentation](docs/tools.md).

## Security

This MCP server is designed with security as a priority:

- **Read-only by default**: Only SELECT and WITH queries are permitted
- **Parameterized queries**: Protection against SQL injection
- **Connection validation**: Ensures valid database connections before operations
- **Error handling**: Comprehensive error handling with detailed logging

## Usage with Claude Code

1. **Configure the MCP server in your Claude Code settings.**

2. **Set up your database connection via environment variables:**
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
- [lib/pq](https://github.com/lib/pq) - PostgreSQL driver
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