# Tool API Reference

This document describes all 9 tools available in the PostgreSQL MCP server, including parameters, response formats, and error conditions.

## Overview

| Tool | Description |
|------|-------------|
| [connect_database](#connect_database) | Connect to a PostgreSQL database |
| [list_databases](#list_databases) | List all databases on the server |
| [list_schemas](#list_schemas) | List schemas in the current database |
| [list_tables](#list_tables) | List tables in a schema with optional metadata |
| [describe_table](#describe_table) | Get detailed table structure |
| [execute_query](#execute_query) | Execute read-only SQL queries |
| [list_indexes](#list_indexes) | List indexes for a table |
| [explain_query](#explain_query) | Get execution plan for a query |
| [get_table_stats](#get_table_stats) | Get table statistics |

---

## connect_database

Connect to a PostgreSQL database using a connection URL or individual parameters.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `connection_url` | string | No | Full PostgreSQL connection URL. If provided, individual parameters are ignored. |
| `host` | string | No | Database host (default: `localhost`) |
| `port` | number | No | Database port (default: `5432`) |
| `user` | string | No | Database user |
| `password` | string | No | Database password |
| `database` | string | No | Database name |
| `sslmode` | string | No | SSL mode: `disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full` (default: `prefer`) |

### Response

```json
{
  "status": "connected",
  "database": "mydb",
  "message": "Successfully connected to database: mydb"
}
```

### Errors

| Error | Description |
|-------|-------------|
| Host is required | `host` is empty when using individual parameters |
| User is required | `user` is empty when using individual parameters |
| Database is required | `database` is empty when using individual parameters |
| Connection failure | Could not connect to the database |

---

## list_databases

List all databases on the PostgreSQL server.

### Parameters

None.

### Response

```json
[
  {
    "name": "mydb",
    "owner": "postgres",
    "encoding": "UTF8"
  }
]
```

### Errors

| Error | Description |
|-------|-------------|
| `database connection failed` | No active database connection. Use `connect_database` first. |

---

## list_schemas

List all schemas in the current database (excludes `information_schema`, `pg_catalog`, `pg_toast`).

### Parameters

None.

### Response

```json
[
  {
    "name": "public",
    "owner": "postgres"
  }
]
```

### Errors

| Error | Description |
|-------|-------------|
| `database connection failed` | No active database connection. Use `connect_database` first. |

---

## list_tables

List tables and views in a specific schema.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `schema` | string | No | Schema name (default: `public`) |
| `include_size` | boolean | No | Include table size and row count (default: `false`) |

### Response

```json
[
  {
    "schema": "public",
    "name": "users",
    "type": "table",
    "owner": "postgres",
    "row_count": 1500,
    "size": "256 kB"
  }
]
```

`row_count` and `size` are only included when `include_size` is `true`.

### Errors

| Error | Description |
|-------|-------------|
| `database connection failed` | No active database connection. Use `connect_database` first. |

---

## describe_table

Get detailed column information for a table.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `table` | string | **Yes** | Table name to describe |
| `schema` | string | No | Schema name (default: `public`) |

### Response

```json
[
  {
    "name": "id",
    "data_type": "integer",
    "is_nullable": false,
    "default_value": "nextval('users_id_seq'::regclass)"
  },
  {
    "name": "email",
    "data_type": "character varying",
    "is_nullable": true,
    "default_value": ""
  }
]
```

### Errors

| Error | Description |
|-------|-------------|
| `table name is required` | `table` parameter is missing or empty |
| `table does not exist` | The specified table was not found |
| `database connection failed` | No active database connection |

---

## execute_query

Execute a read-only SQL query. Only `SELECT` and `WITH` statements are allowed.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | **Yes** | SQL query (SELECT or WITH only) |
| `limit` | number | No | Maximum rows to return (applied after fetch) |

### Response

```json
{
  "columns": ["id", "name", "email"],
  "rows": [
    [1, "Alice", "alice@example.com"],
    [2, "Bob", "bob@example.com"]
  ],
  "row_count": 2
}
```

### Errors

| Error | Description |
|-------|-------------|
| `query is required` | `query` parameter is missing or empty |
| `only SELECT and WITH queries are allowed` | Query starts with a disallowed statement (INSERT, UPDATE, DELETE, etc.) |
| `multi-statement queries are not allowed` | Query contains semicolons outside string literals |
| `query exceeds maximum allowed length` | Query exceeds 1MB |
| `result set exceeds maximum allowed rows` | Result exceeds row limit (default: 10,000) |
| `database connection failed` | No active database connection |

---

## list_indexes

List indexes for a specific table.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `table` | string | **Yes** | Table name to list indexes for |
| `schema` | string | No | Schema name (default: `public`) |

### Response

```json
[
  {
    "name": "users_pkey",
    "table": "users",
    "columns": ["id"],
    "is_unique": true,
    "is_primary": true,
    "index_type": "btree"
  },
  {
    "name": "idx_users_email",
    "table": "users",
    "columns": ["email"],
    "is_unique": true,
    "is_primary": false,
    "index_type": "btree"
  }
]
```

### Errors

| Error | Description |
|-------|-------------|
| `table name is required` | `table` parameter is missing or empty |
| `database connection failed` | No active database connection |

---

## explain_query

Get the execution plan for a SQL query using `EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON)`.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | **Yes** | SQL query to explain (SELECT or WITH only) |

### Response

```json
{
  "columns": ["QUERY PLAN"],
  "rows": [
    ["[{\"Plan\": {\"Node Type\": \"Seq Scan\", \"Relation Name\": \"users\", ...}}]"]
  ],
  "row_count": 1
}
```

The execution plan is returned as a JSON string inside the rows. Parse the first row's first column to access the full plan.

### Errors

| Error | Description |
|-------|-------------|
| `query is required` | `query` parameter is missing or empty |
| `only SELECT and WITH queries are allowed` | Query starts with a disallowed statement |
| `multi-statement queries are not allowed` | Query contains semicolons outside string literals |
| `query exceeds maximum allowed length` | Query exceeds 1MB |
| `database connection failed` | No active database connection |

---

## get_table_stats

Get row count statistics for a specific table.

### Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `table` | string | **Yes** | Table name to get statistics for |
| `schema` | string | No | Schema name (default: `public`) |

### Response

```json
{
  "schema": "public",
  "name": "users",
  "row_count": 15000
}
```

Row count uses `pg_stat_user_tables` estimates when available, falling back to `COUNT(*)` for newly created tables.

### Errors

| Error | Description |
|-------|-------------|
| `table name is required` | `table` parameter is missing or empty |
| `database connection failed` | No active database connection |

---

## Error Reference

All error messages that can be returned by the tools:

| Error Message | Affected Tools |
|---------------|----------------|
| `database connection failed. Please connect to a database using the connect_database tool` | All tools except `connect_database` |
| `table name is required` | `describe_table`, `list_indexes`, `get_table_stats` |
| `query is required` | `execute_query`, `explain_query` |
| `only SELECT and WITH queries are allowed` | `execute_query`, `explain_query` |
| `multi-statement queries are not allowed` | `execute_query`, `explain_query` |
| `query exceeds maximum allowed length` | `execute_query`, `explain_query` |
| `result set exceeds maximum allowed rows` | `execute_query`, `explain_query` |
| `table does not exist` | `describe_table` |

---

## Security and Limits

The server enforces several security measures:

- **Read-only queries**: Only `SELECT` and `WITH` statements are allowed. All other SQL statements are rejected before execution.
- **Read-only connections**: Database connections use `default_transaction_read_only=on` at the PostgreSQL session level as defense-in-depth.
- **Comment stripping**: SQL comments are stripped before validation to prevent comment-based injection.
- **Multi-statement prevention**: Semicolons outside string literals are rejected to prevent chained statement injection.
- **Query size limit**: Queries exceeding 1MB are rejected.
- **Result size limit**: Result sets exceeding 10,000 rows (configurable) are rejected during fetch to prevent memory exhaustion.
- **Identifier escaping**: Schema and table names use `pq.QuoteIdentifier()` for safe escaping.

### Configurable Limits

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `POSTGRES_MCP_MAX_RESULT_ROWS` | Maximum rows returned per query | `10000` |
| `POSTGRES_MCP_MAX_OPEN_CONNS` | Maximum open database connections | `10` |
| `POSTGRES_MCP_MAX_IDLE_CONNS` | Maximum idle database connections | `5` |
| `POSTGRES_MCP_CONN_MAX_LIFETIME` | Connection max lifetime (seconds) | `3600` |
| `POSTGRES_MCP_CONN_MAX_IDLE_TIME` | Connection max idle time (seconds) | `600` |
