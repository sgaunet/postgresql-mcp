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
	ErrInvalidQuery       = errors.New("only SELECT and WITH queries are allowed")
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

// ConnectionManager handles database connection operations.
type ConnectionManager interface {
	Connect(ctx context.Context, connectionString string) error
	Close() error
	Ping(ctx context.Context) error
	GetDB() *sql.DB
}

// DatabaseExplorer handles database-level operations.
type DatabaseExplorer interface {
	ListDatabases(ctx context.Context) ([]*DatabaseInfo, error)
	GetCurrentDatabase(ctx context.Context) (string, error)
	ListSchemas(ctx context.Context) ([]*SchemaInfo, error)
}

// TableExplorer handles table-level operations.
type TableExplorer interface {
	ListTables(ctx context.Context, schema string) ([]*TableInfo, error)
	ListTablesWithStats(ctx context.Context, schema string) ([]*TableInfo, error)
	DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error)
	GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error)
	ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error)
}

// QueryExecutor handles query operations.
type QueryExecutor interface {
	ExecuteQuery(ctx context.Context, query string, args ...any) (*QueryResult, error)
	ExplainQuery(ctx context.Context, query string, args ...any) (*QueryResult, error)
}

// PostgreSQLClient interface combines all database operations.
type PostgreSQLClient interface {
	ConnectionManager
	DatabaseExplorer
	TableExplorer
	QueryExecutor
}
