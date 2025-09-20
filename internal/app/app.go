package app

import (
	"errors"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/sylvain/postgresql-mcp/internal/logger"
)

// Constants for default values.
const (
	defaultSchema = "public"
)

// Error variables for static errors.
var (
	ErrConnectionRequired      = errors.New("database connection is required")
	ErrConnectionStringRequired = errors.New("POSTGRES_URL or connection parameters are required")
	ErrSchemaRequired         = errors.New("schema name is required")
	ErrTableRequired          = errors.New("table name is required")
	ErrQueryRequired          = errors.New("query is required")
	ErrInvalidQuery           = errors.New("only SELECT and WITH queries are allowed")
)

// ConnectOptions represents database connection options.
type ConnectOptions struct {
	ConnectionString string `json:"connection_string,omitempty"`
	Host             string `json:"host,omitempty"`
	Port             int    `json:"port,omitempty"`
	Database         string `json:"database,omitempty"`
	Username         string `json:"username,omitempty"`
	Password         string `json:"password,omitempty"`
	SSLMode          string `json:"ssl_mode,omitempty"`
}

// ListTablesOptions represents options for listing tables.
type ListTablesOptions struct {
	Schema      string `json:"schema,omitempty"`
	IncludeSize bool   `json:"include_size,omitempty"`
}

// ExecuteQueryOptions represents options for executing queries.
type ExecuteQueryOptions struct {
	Query string        `json:"query"`
	Args  []interface{} `json:"args,omitempty"`
	Limit int           `json:"limit,omitempty"`
}

// App represents the main application structure.
type App struct {
	client PostgreSQLClient
	logger *slog.Logger
}

// New creates a new App instance.
func New() (*App, error) {
	return &App{
		client: NewPostgreSQLClient(),
		logger: logger.NewLogger("info"),
	}, nil
}

// SetLogger sets the logger for the app.
func (a *App) SetLogger(logger *slog.Logger) {
	a.logger = logger
}

// Connect establishes a connection to the PostgreSQL database.
func (a *App) Connect(opts *ConnectOptions) error {
	if opts == nil {
		return ErrConnectionStringRequired
	}

	connectionString := opts.ConnectionString

	// If no connection string provided, try to build one from individual parameters
	if connectionString == "" {
		connectionString = a.buildConnectionString(opts)
	}

	// If still no connection string, try environment variables
	if connectionString == "" {
		connectionString = os.Getenv("POSTGRES_URL")
		if connectionString == "" {
			connectionString = os.Getenv("DATABASE_URL")
		}
	}

	if connectionString == "" {
		return ErrConnectionStringRequired
	}

	a.logger.Debug("Connecting to PostgreSQL database")

	if err := a.client.Connect(connectionString); err != nil {
		a.logger.Error("Failed to connect to database", "error", err)
		return err
	}

	a.logger.Info("Successfully connected to PostgreSQL database")
	return nil
}

// buildConnectionString builds a connection string from individual parameters.
func (a *App) buildConnectionString(opts *ConnectOptions) string {
	if opts.Host == "" {
		return ""
	}

	var parts []string

	parts = append(parts, "host="+opts.Host)

	if opts.Port > 0 {
		parts = append(parts, "port="+strconv.Itoa(opts.Port))
	}

	if opts.Database != "" {
		parts = append(parts, "dbname="+opts.Database)
	}

	if opts.Username != "" {
		parts = append(parts, "user="+opts.Username)
	}

	if opts.Password != "" {
		parts = append(parts, "password="+opts.Password)
	}

	if opts.SSLMode != "" {
		parts = append(parts, "sslmode="+opts.SSLMode)
	} else {
		parts = append(parts, "sslmode=prefer")
	}

	return strings.Join(parts, " ")
}

// Disconnect closes the database connection.
func (a *App) Disconnect() error {
	if a.client != nil {
		return a.client.Close()
	}
	return nil
}

// ValidateConnection checks if the database connection is valid.
func (a *App) ValidateConnection() error {
	if a.client == nil {
		return ErrConnectionRequired
	}
	return a.client.Ping()
}

// ListDatabases returns a list of all databases.
func (a *App) ListDatabases() ([]*DatabaseInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	a.logger.Debug("Listing databases")

	databases, err := a.client.ListDatabases()
	if err != nil {
		a.logger.Error("Failed to list databases", "error", err)
		return nil, err
	}

	a.logger.Debug("Successfully listed databases", "count", len(databases))
	return databases, nil
}

// GetCurrentDatabase returns the name of the current database.
func (a *App) GetCurrentDatabase() (string, error) {
	if err := a.ValidateConnection(); err != nil {
		return "", err
	}

	return a.client.GetCurrentDatabase()
}

// ListSchemas returns a list of schemas in the current database.
func (a *App) ListSchemas() ([]*SchemaInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	a.logger.Debug("Listing schemas")

	schemas, err := a.client.ListSchemas()
	if err != nil {
		a.logger.Error("Failed to list schemas", "error", err)
		return nil, err
	}

	a.logger.Debug("Successfully listed schemas", "count", len(schemas))
	return schemas, nil
}

// ListTables returns a list of tables in the specified schema.
func (a *App) ListTables(opts *ListTablesOptions) ([]*TableInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	schema := defaultSchema
	if opts != nil && opts.Schema != "" {
		schema = opts.Schema
	}

	a.logger.Debug("Listing tables", "schema", schema)

	tables, err := a.client.ListTables(schema)
	if err != nil {
		a.logger.Error("Failed to list tables", "error", err, "schema", schema)
		return nil, err
	}

	// Get additional stats if requested
	if opts != nil && opts.IncludeSize {
		for _, table := range tables {
			stats, err := a.client.GetTableStats(table.Schema, table.Name)
			if err != nil {
				a.logger.Warn("Failed to get table stats", "error", err, "table", table.Name)
				continue
			}
			table.RowCount = stats.RowCount
			table.Size = stats.Size
		}
	}

	a.logger.Debug("Successfully listed tables", "count", len(tables), "schema", schema)
	return tables, nil
}

// DescribeTable returns detailed information about a table's structure.
func (a *App) DescribeTable(schema, table string) ([]*ColumnInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = defaultSchema
	}

	a.logger.Debug("Describing table", "schema", schema, "table", table)

	columns, err := a.client.DescribeTable(schema, table)
	if err != nil {
		a.logger.Error("Failed to describe table", "error", err, "schema", schema, "table", table)
		return nil, err
	}

	a.logger.Debug("Successfully described table", "column_count", len(columns), "schema", schema, "table", table)
	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (a *App) GetTableStats(schema, table string) (*TableInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = defaultSchema
	}

	a.logger.Debug("Getting table stats", "schema", schema, "table", table)

	stats, err := a.client.GetTableStats(schema, table)
	if err != nil {
		a.logger.Error("Failed to get table stats", "error", err, "schema", schema, "table", table)
		return nil, err
	}

	a.logger.Debug("Successfully retrieved table stats", "schema", schema, "table", table)
	return stats, nil
}

// ListIndexes returns a list of indexes for the specified table.
func (a *App) ListIndexes(schema, table string) ([]*IndexInfo, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = defaultSchema
	}

	a.logger.Debug("Listing indexes", "schema", schema, "table", table)

	indexes, err := a.client.ListIndexes(schema, table)
	if err != nil {
		a.logger.Error("Failed to list indexes", "error", err, "schema", schema, "table", table)
		return nil, err
	}

	a.logger.Debug("Successfully listed indexes", "count", len(indexes), "schema", schema, "table", table)
	return indexes, nil
}

// ExecuteQuery executes a read-only query and returns the results.
func (a *App) ExecuteQuery(opts *ExecuteQueryOptions) (*QueryResult, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	if opts == nil || opts.Query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Executing query", "query", opts.Query)

	result, err := a.client.ExecuteQuery(opts.Query, opts.Args...)
	if err != nil {
		a.logger.Error("Failed to execute query", "error", err, "query", opts.Query)
		return nil, err
	}

	// Apply limit if specified
	if opts.Limit > 0 && len(result.Rows) > opts.Limit {
		result.Rows = result.Rows[:opts.Limit]
		result.RowCount = len(result.Rows)
	}

	a.logger.Debug("Successfully executed query", "row_count", result.RowCount)
	return result, nil
}

// ExplainQuery returns the execution plan for a query.
func (a *App) ExplainQuery(query string, args ...interface{}) (*QueryResult, error) {
	if err := a.ValidateConnection(); err != nil {
		return nil, err
	}

	if query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Explaining query", "query", query)

	result, err := a.client.ExplainQuery(query, args...)
	if err != nil {
		a.logger.Error("Failed to explain query", "error", err, "query", query)
		return nil, err
	}

	a.logger.Debug("Successfully explained query")
	return result, nil
}