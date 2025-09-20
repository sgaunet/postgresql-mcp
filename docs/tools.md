# Available Tools

This document describes all the tools available in the PostgreSQL MCP server.

## `list_databases`
List all databases on the PostgreSQL server.

**Returns:** Array of database objects with name, owner, and encoding information.

## `list_schemas`
List all schemas in the current database.

**Returns:** Array of schema objects with name and owner information.

## `list_tables`
List tables in a specific schema.

**Parameters:**
- `schema` (string, optional): Schema name to list tables from (default: public)
- `include_size` (boolean, optional): Include table size and row count information (default: false)

**Returns:** Array of table objects with schema, name, type, owner, and optional size/row count.

## `describe_table`
Get detailed information about a table's structure.

**Parameters:**
- `table` (string, required): Table name to describe
- `schema` (string, optional): Schema name (default: public)

**Returns:** Array of column objects with name, data type, nullable flag, and default values.

## `execute_query`
Execute a read-only SQL query.

**Parameters:**
- `query` (string, required): SQL query to execute (SELECT or WITH statements only)
- `limit` (number, optional): Maximum number of rows to return

**Returns:** Query result object with columns, rows, and row count.

**Note:** Only SELECT and WITH statements are allowed for security reasons.

## `list_indexes`
List indexes for a specific table.

**Parameters:**
- `table` (string, required): Table name to list indexes for
- `schema` (string, optional): Schema name (default: public)

**Returns:** Array of index objects with name, columns, type, and usage information.

## `explain_query`
Get the execution plan for a SQL query to analyze performance.

**Parameters:**
- `query` (string, required): SQL query to explain (SELECT or WITH statements only)

**Returns:** Query execution plan with performance metrics and optimization information.

## `get_table_stats`
Get detailed statistics for a specific table.

**Parameters:**
- `table` (string, required): Table name to get statistics for
- `schema` (string, optional): Schema name (default: public)

**Returns:** Table statistics object with row count, size, and other metadata.

## Examples

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