package app

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/sylvain/postgresql-mcp/internal/logger"
)

// Constants for default values.
const (
	DefaultSchema = "public"
)

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

// New creates a new App instance without establishing a connection.
// Use Connect() method or connect_database tool to establish connection.
func New() (*App, error) {
	app := &App{
		client: NewPostgreSQLClient(),
		logger: logger.NewLogger("info"),
	}

	// Note: Connection is now explicit via Connect() or connect_database tool
	// Environment variables are still supported as fallback via tryConnect()

	return app, nil
}

// SetLogger sets the logger for the app.
func (a *App) SetLogger(logger *slog.Logger) {
	a.logger = logger
}

// Connect establishes a database connection with the provided connection string.
// If a connection already exists, it will be closed before establishing a new one.
func (a *App) Connect(connectionString string) error {
	if connectionString == "" {
		return ErrNoConnectionString
	}

	// Close existing connection if any
	if a.client != nil {
		if err := a.client.Ping(); err == nil {
			// Connection exists and is active, close it first
			if closeErr := a.client.Close(); closeErr != nil {
				a.logger.Warn("Failed to close existing connection", "error", closeErr)
			}
		}
	}

	a.logger.Debug("Connecting to PostgreSQL database")

	if err := a.client.Connect(connectionString); err != nil {
		a.logger.Error("Failed to connect to database", "error", err)
		return fmt.Errorf("failed to connect: %w", err)
	}

	a.logger.Info("Successfully connected to PostgreSQL database")
	return nil
}

// Disconnect closes the database connection.
func (a *App) Disconnect() error {
	if a.client != nil {
		if err := a.client.Close(); err != nil {
			return fmt.Errorf("failed to close database connection: %w", err)
		}
	}
	return nil
}

// ListDatabases returns a list of all databases.
func (a *App) ListDatabases() ([]*DatabaseInfo, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	a.logger.Debug("Listing databases")

	databases, err := a.client.ListDatabases()
	if err != nil {
		a.logger.Error("Failed to list databases", "error", err)
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	a.logger.Debug("Successfully listed databases", "count", len(databases))
	return databases, nil
}

// ListSchemas returns a list of schemas in the current database.
func (a *App) ListSchemas() ([]*SchemaInfo, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	a.logger.Debug("Listing schemas")

	schemas, err := a.client.ListSchemas()
	if err != nil {
		a.logger.Error("Failed to list schemas", "error", err)
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	a.logger.Debug("Successfully listed schemas", "count", len(schemas))
	return schemas, nil
}

// ListTables returns a list of tables in the specified schema.
func (a *App) ListTables(opts *ListTablesOptions) ([]*TableInfo, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	schema := DefaultSchema
	if opts != nil && opts.Schema != "" {
		schema = opts.Schema
	}

	a.logger.Debug("Listing tables", "schema", schema)

	tables, err := a.client.ListTables(schema)
	if err != nil {
		a.logger.Error("Failed to list tables", "error", err, "schema", schema)
		return nil, fmt.Errorf("failed to list tables: %w", err)
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
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Describing table", "schema", schema, "table", table)

	columns, err := a.client.DescribeTable(schema, table)
	if err != nil {
		a.logger.Error("Failed to describe table", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}

	a.logger.Debug("Successfully described table", "column_count", len(columns), "schema", schema, "table", table)
	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (a *App) GetTableStats(schema, table string) (*TableInfo, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Getting table stats", "schema", schema, "table", table)

	stats, err := a.client.GetTableStats(schema, table)
	if err != nil {
		a.logger.Error("Failed to get table stats", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	a.logger.Debug("Successfully retrieved table stats", "schema", schema, "table", table)
	return stats, nil
}

// ListIndexes returns a list of indexes for the specified table.
func (a *App) ListIndexes(schema, table string) ([]*IndexInfo, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Listing indexes", "schema", schema, "table", table)

	indexes, err := a.client.ListIndexes(schema, table)
	if err != nil {
		a.logger.Error("Failed to list indexes", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	a.logger.Debug("Successfully listed indexes", "count", len(indexes), "schema", schema, "table", table)
	return indexes, nil
}

// ExecuteQuery executes a read-only query and returns the results.
func (a *App) ExecuteQuery(opts *ExecuteQueryOptions) (*QueryResult, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	if opts == nil || opts.Query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Executing query", "query", opts.Query)

	result, err := a.client.ExecuteQuery(opts.Query, opts.Args...)
	if err != nil {
		a.logger.Error("Failed to execute query", "error", err, "query", opts.Query)
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	// Apply limit if specified
	if opts.Limit > 0 && len(result.Rows) > opts.Limit {
		result.Rows = result.Rows[:opts.Limit]
		result.RowCount = len(result.Rows)
	}

	a.logger.Debug("Successfully executed query", "row_count", result.RowCount)
	return result, nil
}

// GetCurrentDatabase returns the name of the current database.
func (a *App) GetCurrentDatabase() (string, error) {
	if err := a.ensureConnection(); err != nil {
		return "", err
	}

	dbName, err := a.client.GetCurrentDatabase()
	if err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}
	return dbName, nil
}

// ExplainQuery returns the execution plan for a query.
func (a *App) ExplainQuery(query string, args ...interface{}) (*QueryResult, error) {
	if err := a.ensureConnection(); err != nil {
		return nil, err
	}

	if query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Explaining query", "query", query)

	result, err := a.client.ExplainQuery(query, args...)
	if err != nil {
		a.logger.Error("Failed to explain query", "error", err, "query", query)
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}

	a.logger.Debug("Successfully explained query")
	return result, nil
}

// ValidateConnection checks if the database connection is valid (for backward compatibility).
func (a *App) ValidateConnection() error {
	return a.ensureConnection()
}

// tryConnect attempts to connect using environment variables as a fallback mechanism.
// Returns ErrNoConnectionString if no environment variables are set.
func (a *App) tryConnect() error {
	// Try environment variables as fallback
	connectionString := os.Getenv("POSTGRES_URL")
	if connectionString == "" {
		connectionString = os.Getenv("DATABASE_URL")
	}

	if connectionString == "" {
		return ErrNoConnectionString
	}

	return a.Connect(connectionString)
}

// ensureConnection checks if the database connection is valid and attempts to reconnect if needed.
func (a *App) ensureConnection() error {
	if a.client == nil {
		return ErrConnectionRequired
	}

	// Test current connection
	if err := a.client.Ping(); err != nil {
		a.logger.Debug("Database connection lost, attempting to reconnect", "error", err)

		// Attempt to reconnect
		if reconnectErr := a.tryConnect(); reconnectErr != nil {
			a.logger.Error("Failed to reconnect to database", "ping_error", err, "reconnect_error", reconnectErr)
			return ErrConnectionRequired
		}

		a.logger.Info("Successfully reconnected to database")
	}

	return nil
}
