# PostgreSQL MCP Server

A Model Context Protocol (MCP) server that provides PostgreSQL integration tools for Claude Code.

## Features

- **Connect Database**: Connect to PostgreSQL databases using connection strings or individual parameters
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

- Go 1.21 or later
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

## Configuration

The PostgreSQL MCP server can be configured using environment variables or connection parameters passed to the `connect_database` tool.

### Environment Variables

- `POSTGRES_URL`: PostgreSQL connection URL (format: `postgres://user:password@host:port/dbname?sslmode=prefer`)
- `DATABASE_URL`: Alternative to `POSTGRES_URL`

### Connection Parameters

When using the `connect_database` tool, you can provide either:

1. **Connection String:**
   ```json
   {
     "connection_string": "postgres://user:password@localhost:5432/mydb?sslmode=prefer"
   }
   ```

2. **Individual Parameters:**
   ```json
   {
     "host": "localhost",
     "port": 5432,
     "database": "mydb",
     "username": "user",
     "password": "password",
     "ssl_mode": "prefer"
   }
   ```

## Available Tools

### `connect_database`
Connect to a PostgreSQL database using connection parameters.

**Parameters:**
- `connection_string` (string, optional): Complete PostgreSQL connection URL
- `host` (string, optional): Database host (default: localhost)
- `port` (number, optional): Database port (default: 5432)
- `database` (string, optional): Database name
- `username` (string, optional): Database username
- `password` (string, optional): Database password
- `ssl_mode` (string, optional): SSL mode: disable, require, verify-ca, verify-full (default: prefer)

### `list_databases`
List all databases on the PostgreSQL server.

**Returns:** Array of database objects with name, owner, and encoding information.

### `list_schemas`
List all schemas in the current database.

**Returns:** Array of schema objects with name and owner information.

### `list_tables`
List tables in a specific schema.

**Parameters:**
- `schema` (string, optional): Schema name to list tables from (default: public)
- `include_size` (boolean, optional): Include table size and row count information (default: false)

**Returns:** Array of table objects with schema, name, type, owner, and optional size/row count.

### `describe_table`
Get detailed information about a table's structure.

**Parameters:**
- `table` (string, required): Table name to describe
- `schema` (string, optional): Schema name (default: public)

**Returns:** Array of column objects with name, data type, nullable flag, and default values.

### `execute_query`
Execute a read-only SQL query.

**Parameters:**
- `query` (string, required): SQL query to execute (SELECT or WITH statements only)
- `limit` (number, optional): Maximum number of rows to return

**Returns:** Query result object with columns, rows, and row count.

**Note:** Only SELECT and WITH statements are allowed for security reasons.

### `list_indexes`
List indexes for a specific table.

**Parameters:**
- `table` (string, required): Table name to list indexes for
- `schema` (string, optional): Schema name (default: public)

**Returns:** Array of index objects with name, columns, type, and usage information.

### `explain_query`
Get the execution plan for a SQL query to analyze performance.

**Parameters:**
- `query` (string, required): SQL query to explain (SELECT or WITH statements only)

**Returns:** Query execution plan with performance metrics and optimization information.

### `get_table_stats`
Get detailed statistics for a specific table.

**Parameters:**
- `table` (string, required): Table name to get statistics for
- `schema` (string, optional): Schema name (default: public)

**Returns:** Table statistics object with row count, size, and other metadata.

## Security

This MCP server is designed with security as a priority:

- **Read-only by default**: Only SELECT and WITH queries are permitted
- **Parameterized queries**: Protection against SQL injection
- **Connection validation**: Ensures valid database connections before operations
- **Error handling**: Comprehensive error handling with detailed logging

## Usage with Claude Code

1. **Configure the MCP server in your Claude Code settings.**

2. **Use the tools in your conversations:**
   ```
   Connect to database: postgres://user:pass@localhost:5432/mydb
   List all tables in the public schema
   Describe the users table
   Execute query: SELECT * FROM users LIMIT 10
   ```

## Examples

### Connecting to a Database
```json
{
  "tool": "connect_database",
  "parameters": {
    "host": "localhost",
    "port": 5432,
    "database": "myapp",
    "username": "myuser",
    "password": "mypassword",
    "ssl_mode": "prefer"
  }
}
```

### Listing Tables with Metadata
```json
{
  "tool": "list_tables",
  "parameters": {
    "schema": "public",
    "include_size": true
  }
}
```

### Describing a Table
```json
{
  "tool": "describe_table",
  "parameters": {
    "table": "users",
    "schema": "public"
  }
}
```

### Executing a Query
```json
{
  "tool": "execute_query",
  "parameters": {
    "query": "SELECT id, name, email FROM users WHERE active = true",
    "limit": 50
  }
}
```

### Listing Table Indexes
```json
{
  "tool": "list_indexes",
  "parameters": {
    "table": "users",
    "schema": "public"
  }
}
```

### Explaining a Query
```json
{
  "tool": "explain_query",
  "parameters": {
    "query": "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id WHERE u.active = true"
  }
}
```

### Getting Table Statistics
```json
{
  "tool": "get_table_stats",
  "parameters": {
    "table": "users",
    "schema": "public"
  }
}
```

## Development

### Building
```bash
go build -o postgresql-mcp
```

### Testing
```bash
go test ./...
```

### Dependencies
- [mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [lib/pq](https://github.com/lib/pq) - PostgreSQL driver

## Troubleshooting

### Connection Issues
- Verify PostgreSQL is running and accessible
- Check connection parameters (host, port, database, credentials)
- Ensure SSL mode is appropriate for your setup
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