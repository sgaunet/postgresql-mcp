// Package app implements the PostgreSQL MCP tool handlers and client abstraction.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/sylvain/postgresql-mcp/internal/logger"
	"golang.org/x/sync/singleflight"
)

// Constants for default values.
const (
	DefaultSchema  = "public"
	maxQueryLogLen = 100
)

// truncateQuery safely truncates a query string for logging purposes,
// avoiding logging of potentially sensitive full query text. maxLen is a
// parameter so unit tests can exercise the cut-off behavior at small
// lengths; production callers always use maxQueryLogLen via LogSafeQuery.
//
//nolint:unparam // see comment above re: test ergonomics
func truncateQuery(query string, maxLen int) string {
	if len(query) <= maxLen {
		return query
	}
	return query[:maxLen] + "..."
}

// LogSafeQuery returns a log-safe (truncated) representation of a query so
// callers outside this package can avoid emitting full query text — which
// may carry PII or credentials — to debug/error logs (issue #85).
func LogSafeQuery(query string) string {
	return truncateQuery(query, maxQueryLogLen)
}

// applyLimit wraps a user query in a subquery with an outer LIMIT so that
// PostgreSQL itself caps the result set instead of streaming maxResultRows
// to the server only to discard them in memory (issue #91).
//
// limit must be > 0. The inner query is trimmed of trailing whitespace and
// any trailing semicolons that slipped past validation; validateQuery
// already rejects multi-statement queries, so this is a defensive trim.
func applyLimit(query string, limit int) string {
	inner := strings.TrimRight(query, "; \t\r\n")
	// SELECT * is required: the user query's projection is opaque here and
	// must be preserved verbatim. limit is an int caller-supplied integer
	// (bounded by the MCP tool schema) and cannot inject SQL.
	//nolint:unqueryvet // wrapper preserves user-defined projection
	return fmt.Sprintf("SELECT * FROM (%s) AS _postgres_mcp_limit_sub LIMIT %d", inner, limit)
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
//
// connStr holds the connection string from the most recent successful
// Connect call. ensureConnection uses it for reconnects so that an
// explicit connect_database session is not silently overridden by
// POSTGRES_URL / DATABASE_URL on the next ping failure (issue #87).
type App struct {
	client         PostgreSQLClient
	logger         *slog.Logger
	reconnectGroup singleflight.Group

	connStrMu sync.RWMutex
	connStr   string
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

	// Remember the string that established this session so that automatic
	// reconnects target the same database (issue #87).
	a.connStrMu.Lock()
	a.connStr = connectionString
	a.connStrMu.Unlock()

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

	a.logger.Debug("Executing query", "query", truncateQuery(opts.Query, maxQueryLogLen), "limit", opts.Limit)

	query := opts.Query
	if opts.Limit > 0 {
		query = applyLimit(query, opts.Limit)
	}

	result, err := a.client.ExecuteQuery(ctx, query, opts.Args...)
	if err != nil {
		if ok, rejErr := a.rejectQuery(opts.Query, err); ok {
			return nil, rejErr
		}
		a.logger.Error("Failed to execute query", "error", err, "query", truncateQuery(opts.Query, maxQueryLogLen))
		return nil, fmt.Errorf("failed to execute query: %w", err)
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

// ExplainQuery returns the execution plan for a query. When analyze is false
// (the default for the explain_query MCP tool) the plan is non-executing
// (EXPLAIN FORMAT JSON); when true it runs EXPLAIN (ANALYZE, BUFFERS, …)
// which executes the query — bounded by ctx — and may carry the same cost
// as the underlying SELECT. See issue #89.
func (a *App) ExplainQuery(ctx context.Context, query string, analyze bool, args ...any) (*QueryResult, error) {
	if err := a.ensureConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}

	if query == "" {
		return nil, ErrQueryRequired
	}

	a.logger.Debug("Explaining query", "query", truncateQuery(query, maxQueryLogLen), "analyze", analyze)

	result, err := a.client.ExplainQuery(ctx, query, analyze, args...)
	if err != nil {
		if ok, rejErr := a.rejectQuery(query, err); ok {
			return nil, rejErr
		}
		a.logger.Error("Failed to explain query", "error", err, "query", truncateQuery(query, maxQueryLogLen))
		return nil, fmt.Errorf("failed to explain query: %w", err)
	}

	a.logger.Debug("Successfully explained query")
	return result, nil
}

// ValidateConnection checks that the database connection is valid. Unlike the
// per-request ensureConnection (which no longer pings — issue #93), this is an
// explicit, non-hot-path liveness probe: it ensures a pool exists (bootstrapping
// one if necessary) and then actively pings it.
func (a *App) ValidateConnection(ctx context.Context) error {
	if err := a.ensureConnection(ctx); err != nil {
		return err
	}
	if err := a.client.Ping(ctx); err != nil {
		return fmt.Errorf("connection validation failed: %w", err)
	}
	return nil
}

// tryConnect picks the connection string for an (initial or recovery) Connect.
//
// Sticky-session: if a prior Connect succeeded, reuse the same connection
// string so that auto-reconnect targets the same database the caller
// explicitly chose via connect_database (issue #87). POSTGRES_URL /
// DATABASE_URL are consulted only when no prior session exists, i.e. for
// initial bootstrap.
//
// Returns ErrNoConnectionString if there is no stored string and no env var
// fallback.
func (a *App) tryConnect(ctx context.Context) error {
	a.connStrMu.RLock()
	stored := a.connStr
	a.connStrMu.RUnlock()

	if stored != "" {
		return a.Connect(ctx, stored)
	}

	// Initial bootstrap only: env-var fallback.
	connectionString := os.Getenv("POSTGRES_URL")
	if connectionString == "" {
		connectionString = os.Getenv("DATABASE_URL")
	}
	if connectionString == "" {
		return ErrNoConnectionString
	}
	return a.Connect(ctx, connectionString)
}

// ensureConnection makes sure a usable connection pool exists before a database
// operation, without adding a network round-trip to the request.
//
// It does NOT proactively ping (issue #93): once a *sql.DB pool exists,
// database/sql validates pooled connections itself and transparently opens a
// fresh connection when a cached one has gone bad (driver.ErrBadConn), so a
// per-request ping merely doubled latency and pool pressure. The pool's presence
// is detected with a cheap, lock-free HasConnection() load (no RTT).
//
// The only case that needs work here is when no pool exists yet — initial
// lazy bootstrap, or recovery after Disconnect. Establishing it is deduped via
// singleflight so that N handlers racing on a cold start trigger only one
// underlying Connect (issue #83); the connection string is chosen by tryConnect
// (sticky session from the last Connect, else POSTGRES_URL/DATABASE_URL — #87).
//
// Behavior:
//   - Uses a background context for the connect so it is not cancelled by an
//     individual request timeout.
//   - Followers honor their own request ctx and may abort while the leader runs.
//   - Returns ErrConnectionRequired if the client is nil or bootstrap fails.
func (a *App) ensureConnection(ctx context.Context) error {
	if a.client == nil {
		return ErrConnectionRequired
	}

	// Fast path: a pool already exists — database/sql handles connection
	// validation and recycling, so there is nothing to do (no ping, no RTT).
	if a.client.HasConnection() {
		return nil
	}

	// Slow path: no pool yet. Dedupe concurrent bootstrap attempts.
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
	// Establishing a connection is infrastructure work and must not be
	// cancelled by any individual request context.
	reconnectCtx := context.Background()

	// Re-check inside the leader: another goroutine (e.g. a manual
	// connect_database) may have established the pool between our outer
	// HasConnection() check and acquiring leadership.
	if a.client.HasConnection() {
		return reconnectResult{}, nil
	}

	a.logger.Debug("No database connection, attempting to establish one")
	if err := a.tryConnect(reconnectCtx); err != nil {
		a.logger.Error("Failed to establish database connection", "error", err)
		return reconnectResult{}, err
	}
	a.logger.Info("Successfully established database connection")
	return reconnectResult{}, nil
}

// rejectQuery maps a query-rejection sentinel error to a logged security event
// and a "query rejected" wrapped error. ok is false when err is not a rejection
// sentinel, signalling the caller to fall through to its generic error handling.
func (a *App) rejectQuery(query string, err error) (bool, error) {
	var event string
	switch {
	case errors.Is(err, ErrResultTooLarge):
		event = "result_too_large"
	case errors.Is(err, ErrQueryTooLong):
		event = "query_too_long"
	case errors.Is(err, ErrInvalidQuery):
		event = "invalid_query"
	case errors.Is(err, ErrMultiStatementQuery):
		event = "multi_statement_query"
	default:
		return false, nil
	}
	a.logSecurityEvent(event, query, err)
	return true, fmt.Errorf("query rejected: %w", err)
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
