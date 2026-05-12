package app

import (
	"context"
	"database/sql"
	"errors"
)

// Error variables for static errors.
var (
	ErrConnectionRequired = errors.New(
		"database connection failed. Please connect to a database using the connect_database tool",
	)
	ErrSchemaRequired     = errors.New("schema name is required")
	ErrTableRequired      = errors.New("table name is required")
	ErrQueryRequired      = errors.New("query is required")
	ErrInvalidQuery        = errors.New("only SELECT and WITH queries are allowed")
	ErrMultiStatementQuery = errors.New("multi-statement queries are not allowed")
	ErrQueryTooLong        = errors.New("query exceeds maximum allowed length")
	ErrResultTooLarge      = errors.New("result set exceeds maximum allowed rows")
	ErrNoConnectionString = errors.New(
		"no database connection string provided. " +
			"Either call connect_database tool or set POSTGRES_URL/DATABASE_URL environment variable",
	)
	ErrNoDatabaseConnection = errors.New("no database connection")
	ErrTableNotFound        = errors.New("table does not exist")
	ErrMarshalFailed        = errors.New("failed to marshal data to JSON")
)

// DatabaseInfo represents basic database metadata.
type DatabaseInfo struct {
	Name     string `json:"name"`
	Owner    string `json:"owner"`
	Encoding string `json:"encoding"`
	Size     string `json:"size,omitempty"`
}

// SchemaInfo represents schema metadata.
type SchemaInfo struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

// TableInfo represents table metadata.
type TableInfo struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Type        string `json:"type"` // table, view, materialized view
	RowCount    int64  `json:"row_count,omitempty"`
	Size        string `json:"size,omitempty"`
	Owner       string `json:"owner"`
	Description string `json:"description,omitempty"`
}

// ColumnInfo represents column metadata.
type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
}

// IndexInfo represents index metadata.
type IndexInfo struct {
	Name      string   `json:"name"`
	Table     string   `json:"table"`
	Columns   []string `json:"columns"`
	IsUnique  bool     `json:"is_unique"`
	IsPrimary bool     `json:"is_primary"`
	IndexType string   `json:"index_type"`
	Size      string   `json:"size,omitempty"`
}

// QueryResult represents the result of a query execution.
type QueryResult struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int      `json:"row_count"`
}

// ConnectionManager handles database connection lifecycle.
type ConnectionManager interface {
	// Connect establishes a connection to a PostgreSQL database.
	// The connection is configured as read-only with pool settings from environment variables.
	Connect(ctx context.Context, connectionString string) error
	// Close closes the database connection and releases resources.
	Close() error
	// Ping verifies the database connection is alive.
	Ping(ctx context.Context) error
	// GetDB returns the underlying *sql.DB for advanced usage or testing.
	GetDB() *sql.DB
}

// DatabaseExplorer handles database and schema discovery.
type DatabaseExplorer interface {
	// ListDatabases returns all non-template databases on the server.
	ListDatabases(ctx context.Context) ([]*DatabaseInfo, error)
	// GetCurrentDatabase returns the name of the currently connected database.
	GetCurrentDatabase(ctx context.Context) (string, error)
	// ListSchemas returns all user-created schemas (excludes system schemas).
	ListSchemas(ctx context.Context) ([]*SchemaInfo, error)
}

// TableExplorer handles table metadata and statistics retrieval.
type TableExplorer interface {
	// ListTables returns tables and views in the given schema.
	ListTables(ctx context.Context, schema string) ([]*TableInfo, error)
	// ListTablesWithStats returns tables with size and row count in a single optimized query.
	ListTablesWithStats(ctx context.Context, schema string) ([]*TableInfo, error)
	// DescribeTable returns column metadata (name, type, nullable, default) for a table.
	DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error)
	// GetTableStats returns row count statistics for a table, using pg_stat estimates
	// and falling back to pg_class.reltuples for tables not yet covered by pg_stat.
	GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error)
	// ListIndexes returns all indexes on a table with column, uniqueness, and type info.
	ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error)
}

// QueryExecutor handles read-only query execution and analysis.
type QueryExecutor interface {
	// ExecuteQuery runs a validated SELECT/WITH query and returns the result set.
	// Queries are validated for safety (no mutations, no multi-statement, size limits).
	ExecuteQuery(ctx context.Context, query string, args ...any) (*QueryResult, error)
	// ExplainQuery returns the EXPLAIN ANALYZE execution plan for a query as JSON.
	ExplainQuery(ctx context.Context, query string, args ...any) (*QueryResult, error)
}

// PostgreSQLClient combines all database operations into a single read-only interface.
// All query execution is restricted to SELECT and WITH statements.
// Implementations must enforce read-only access at both the validation and connection level.
type PostgreSQLClient interface {
	ConnectionManager
	DatabaseExplorer
	TableExplorer
	QueryExecutor
}
