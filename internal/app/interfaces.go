package app

import (
	"database/sql"
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

// PostgreSQLClient interface for database operations.
type PostgreSQLClient interface {
	// Connection management
	Connect(connectionString string) error
	Close() error
	Ping() error

	// Database operations
	ListDatabases() ([]*DatabaseInfo, error)
	GetCurrentDatabase() (string, error)

	// Schema operations
	ListSchemas() ([]*SchemaInfo, error)

	// Table operations
	ListTables(schema string) ([]*TableInfo, error)
	DescribeTable(schema, table string) ([]*ColumnInfo, error)
	GetTableStats(schema, table string) (*TableInfo, error)

	// Index operations
	ListIndexes(schema, table string) ([]*IndexInfo, error)

	// Query operations
	ExecuteQuery(query string, args ...interface{}) (*QueryResult, error)
	ExplainQuery(query string, args ...interface{}) (*QueryResult, error)

	// Utility methods
	GetDB() *sql.DB
}