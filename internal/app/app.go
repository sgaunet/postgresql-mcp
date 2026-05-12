package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/sylvain/postgresql-mcp/internal/logger"
	"golang.org/x/sync/singleflight"
)

// Constants for default values.
const (
	DefaultSchema  = "public"
	maxQueryLogLen = 100
)

// truncateQuery safely truncates a query string for logging purposes,
// avoiding logging of potentially sensitive full query text.
func truncateQuery(query string, maxLen int) string {
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}

// ListTablesOptions represents options for listing tables.
type ListTablesOptions struct {
	Schema      string `json:"schema,omitempty"`
	IncludeSize bool   `json:"include_size,omitempty"`
}

// ExecuteQueryOptions represents options for executing queries.
type ExecuteQueryOptions struct {
	Query string `json:"query"`
	Args  []any  `json:"args,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

// App represents the main application structure.
//
// reconnectGroup dedupes concurrent reconnect attempts so that N handler
// goroutines observing the same connection failure trigger only one
// underlying Connect call (issue #83).
type App struct {
	client         PostgreSQLClient
	logger         *slog.Logger
	reconnectGroup singleflight.Group
}

// New creates a new App instance with the provided PostgreSQLClient.
// This constructor accepts a client implementation for dependency injection,
// making it easy to inject mocks or alternative implementations for testing.
func New(client PostgreSQLClient) *App {
	return &App{
		client: client,
		logger: logger.NewLogger("info"),
	}
}

// NewDefault creates a new App instance with a default PostgreSQLClient.
// Use Connect() method or connect_database tool to establish connection.
// This is a convenience constructor for production use.
func NewDefault() (*App, error) {
	client := NewPostgreSQLClient()
	app := &App{
		client: client,
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
func (a *App) Connect(ctx context.Context, connectionString string) error {
	if connectionString == "" {
		return ErrNoConnectionString
	}

	// Close existing connection if any
	if a.client != nil {
		if err := a.client.Ping(ctx); err == nil {
			// Connection exists and is active, close it first
			if closeErr := a.client.Close(); closeErr != nil {
				a.logger.Warn("Failed to close existing connection", "error", closeErr)
			}
		}
	}

	a.logger.Debug("Connecting to PostgreSQL database")

	if err := a.client.Connect(ctx, connectionString); err != nil {
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
func (a *App) ListDatabases(ctx context.Context) ([]*DatabaseInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	a.logger.Debug("Listing databases")

	databases, err := a.client.ListDatabases(ctx)
	if err != nil {
		a.logger.Error("Failed to list databases", "error", err)
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	a.logger.Debug("Successfully listed databases", "count", len(databases))
	return databases, nil
}

// ListSchemas returns a list of schemas in the current database.
func (a *App) ListSchemas(ctx context.Context) ([]*SchemaInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	a.logger.Debug("Listing schemas")

	schemas, err := a.client.ListSchemas(ctx)
	if err != nil {
		a.logger.Error("Failed to list schemas", "error", err)
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}

	a.logger.Debug("Successfully listed schemas", "count", len(schemas))
	return schemas, nil
}

// ListTables returns a list of tables in the specified schema.
func (a *App) ListTables(ctx context.Context, opts *ListTablesOptions) ([]*TableInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}

	schema := DefaultSchema
	if opts != nil && opts.Schema != "" {
		schema = opts.Schema
	}

	a.logger.Debug("Listing tables", "schema", schema)

	var tables []*TableInfo
	var err error

	// Use optimized query when stats are requested to avoid N+1 query pattern
	if opts != nil && opts.IncludeSize {
		tables, err = a.client.ListTablesWithStats(ctx, schema)
		if err != nil {
			a.logger.Error("Failed to list tables with stats", "error", err, "schema", schema)
			return nil, fmt.Errorf("failed to list tables with stats: %w", err)
		}
	} else {
		tables, err = a.client.ListTables(ctx, schema)
		if err != nil {
			a.logger.Error("Failed to list tables", "error", err, "schema", schema)
			return nil, fmt.Errorf("failed to list tables: %w", err)
		}
	}

	a.logger.Debug("Successfully listed tables", "count", len(tables), "schema", schema)
	return tables, nil
}

// DescribeTable returns detailed information about a table's structure.
func (a *App) DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Describing table", "schema", schema, "table", table)

	columns, err := a.client.DescribeTable(ctx, schema, table)
	if err != nil {
		a.logger.Error("Failed to describe table", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}

	a.logger.Debug("Successfully described table", "column_count", len(columns), "schema", schema, "table", table)
	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (a *App) GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Getting table stats", "schema", schema, "table", table)

	stats, err := a.client.GetTableStats(ctx, schema, table)
	if err != nil {
		a.logger.Error("Failed to get table stats", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}

	a.logger.Debug("Successfully retrieved table stats", "schema", schema, "table", table)
	return stats, nil
}

// ListIndexes returns a list of indexes for the specified table.
func (a *App) ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	if table == "" {
		return nil, ErrTableRequired
	}

	if schema == "" {
		schema = DefaultSchema
	}

	a.logger.Debug("Listing indexes", "schema", schema, "table", table)

	indexes, err := a.client.ListIndexes(ctx, schema, table)
	if err != nil {
		a.logger.Error("Failed to list indexes", "error", err, "schema", schema, "table", table)
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}

	a.logger.Debug("Successfully listed indexes", "count", len(indexes), "schema", schema, "table", table)
	return indexes, nil
}

// ExecuteQuery executes a read-only query and returns the results.
func (a *App) ExecuteQuery(ctx context.Context, opts *ExecuteQueryOptions) (*QueryResult, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if opts == nil || opts.Query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Executing query", "query", opts.Query)

	result, err := a.client.ExecuteQuery(ctx, opts.Query, opts.Args...)
	if err != nil {
		if errors.Is(err, ErrResultTooLarge) {
			a.logSecurityEvent("result_too_large", opts.Query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrQueryTooLong) {
			a.logSecurityEvent("query_too_long", opts.Query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrInvalidQuery) {
			a.logSecurityEvent("invalid_query", opts.Query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrMultiStatementQuery) {
			a.logSecurityEvent("multi_statement_query", opts.Query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
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
func (a *App) GetCurrentDatabase(ctx context.Context) (string, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}

	dbName, err := a.client.GetCurrentDatabase(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}
	return dbName, nil
}

// ExplainQuery returns the execution plan for a query.
func (a *App) ExplainQuery(ctx context.Context, query string, args ...any) (*QueryResult, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}

	if query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Explaining query", "query", query)

	result, err := a.client.ExplainQuery(ctx, query, args...)
	if err != nil {
		if errors.Is(err, ErrResultTooLarge) {
			a.logSecurityEvent("result_too_large", query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrQueryTooLong) {
			a.logSecurityEvent("query_too_long", query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrInvalidQuery) {
			a.logSecurityEvent("invalid_query", query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		if errors.Is(err, ErrMultiStatementQuery) {
			a.logSecurityEvent("multi_statement_query", query, err)
			return nil, fmt.Errorf("query rejected: %w", err)
		}
		a.logger.Error("Failed to explain query", "error", err, "query", query)
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}

	a.logger.Debug("Successfully explained query")
	return result, nil
}

// ValidateConnection checks if the database connection is valid (for backward compatibility).
func (a *App) ValidateConnection(ctx context.Context) error {
	return a.ensureConnection(ctx)
}

// tryConnect attempts to connect using environment variables as a fallback mechanism.
// Returns ErrNoConnectionString if no environment variables are set.
func (a *App) tryConnect(ctx context.Context) error {
	// Try environment variables as fallback
	connectionString := os.Getenv("POSTGRES_URL")
	if connectionString == "" {
		connectionString = os.Getenv("DATABASE_URL")
	}

	if connectionString == "" {
		return ErrNoConnectionString
	}

	return a.Connect(ctx, connectionString)
}

// ensureConnection checks if the database connection is alive and attempts
// automatic reconnection if the connection has been lost.
//
// This method is called before every database operation to provide transparent
// connection recovery. If the connection ping fails, it attempts to reconnect
// using the original connection parameters from POSTGRES_URL or DATABASE_URL
// environment variables.
//
// Reconnection behavior:
//   - Concurrent reconnect attempts are deduped via singleflight, so N handlers
//     observing the same failure trigger only one underlying Connect (issue #83).
//   - Uses a background context so reconnection is not cancelled by request timeout.
//   - Followers honor their own request ctx and may abort while the leader runs.
//   - Logs reconnection attempts at Debug level and results at Info/Error level.
//   - Returns ErrConnectionRequired if the client is nil or reconnection fails.
//
// Operations may experience a slight delay during reconnection. For environments
// where connection stability is critical, configure shorter pool lifetimes via
// POSTGRES_MCP_CONN_MAX_LIFETIME and POSTGRES_MCP_CONN_MAX_IDLE_TIME.
func (a *App) ensureConnection(ctx context.Context) error {
	if a.client == nil {
		return ErrConnectionRequired
	}

	// Fast path: current connection is healthy.
	if err := a.client.Ping(ctx); err == nil {
		return nil
	}

	// Slow path: dedupe concurrent reconnect attempts.
	ch := a.reconnectGroup.DoChan("reconnect", a.doReconnect)
	select {
	case res := <-ch:
		if res.Err != nil {
			return ErrConnectionRequired
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for reconnect: %w", ctx.Err())
	}
}

// reconnectResult is the singleflight payload for doReconnect. It carries no
// data; only the error is meaningful. A typed value is returned (rather than
// nil) so the singleflight callback satisfies linters that forbid (nil, nil).
type reconnectResult struct{}

// doReconnect is the singleflight leader callback for ensureConnection.
// Only one goroutine runs this at a time per key; concurrent callers share
// its result.
func (a *App) doReconnect() (any, error) {
	// Reconnection is infrastructure work and must not be cancelled by any
	// individual request context.
	reconnectCtx := context.Background()

	// Re-check inside the leader: another reconnect (e.g. manual
	// connect_database) may have already succeeded between our outer Ping
	// failure and acquiring leadership.
	if err := a.client.Ping(reconnectCtx); err == nil {
		return reconnectResult{}, nil
	}

	a.logger.Debug("Database connection lost, attempting to reconnect")
	if err := a.tryConnect(reconnectCtx); err != nil {
		a.logger.Error("Failed to reconnect to database", "error", err)
		return reconnectResult{}, err
	}
	a.logger.Info("Successfully reconnected to database")
	return reconnectResult{}, nil
}

// logSecurityEvent logs a security-relevant event (e.g., rejected query)
// with structured fields for monitoring and incident response.
func (a *App) logSecurityEvent(event string, query string, reason error) {
	a.logger.Warn("Security: query rejected",
		"event", event,
		"reason", reason.Error(),
		"query_preview", truncateQuery(query, maxQueryLogLen),
		"query_length", len(query),
	)
}
