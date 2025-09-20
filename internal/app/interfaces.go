package app

import (
	"database/sql"
	"errors"
)

// Error variables for static errors.
var (
	ErrConnectionRequired = errors.New(
		"database connection failed. Please check POSTGRES_URL or DATABASE_URL environment variable",
	)
	ErrSchemaRequired       = errors.New("schema name is required")
	ErrTableRequired        = errors.New("table name is required")
	ErrQueryRequired        = errors.New("query is required")
	ErrInvalidQuery         = errors.New("only SELECT and WITH queries are allowed")
	ErrNoConnectionString = errors.New(
		"no database connection string found in POSTGRES_URL or DATABASE_URL environment variables",
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
	Name       string   `json:"name"`
	Table      string   `json:"table"`
	Columns    []string `json:"columns"`
	IsUnique   bool     `json:"is_unique"`
	IsPrimary  bool     `json:"is_primary"`
	IndexType  string   `json:"index_type"`
	Size       string   `json:"size,omitempty"`
}

// QueryResult represents the result of a query execution.
type QueryResult struct {
	Columns []string        `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
	RowCount int            `json:"row_count"`
}

// ConnectionManager handles database connection operations.
type ConnectionManager interface {
	Connect(connectionString string) error
	Close() error
	Ping() error
	GetDB() *sql.DB
}

// DatabaseExplorer handles database-level operations.
type DatabaseExplorer interface {
	ListDatabases() ([]*DatabaseInfo, error)
	GetCurrentDatabase() (string, error)
	ListSchemas() ([]*SchemaInfo, error)
}

// TableExplorer handles table-level operations.
type TableExplorer interface {
	ListTables(schema string) ([]*TableInfo, error)
	DescribeTable(schema, table string) ([]*ColumnInfo, error)
	GetTableStats(schema, table string) (*TableInfo, error)
	ListIndexes(schema, table string) ([]*IndexInfo, error)
}

// QueryExecutor handles query operations.
type QueryExecutor interface {
	ExecuteQuery(query string, args ...interface{}) (*QueryResult, error)
	ExplainQuery(query string, args ...interface{}) (*QueryResult, error)
}

// PostgreSQLClient interface combines all database operations.
type PostgreSQLClient interface {
	ConnectionManager
	DatabaseExplorer
	TableExplorer
	QueryExecutor
}