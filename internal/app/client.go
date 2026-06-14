package app

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	readOnlyOption = "-c default_transaction_read_only=on"

	// commentTokenLen is the length of SQL comment tokens (/* */ --).
	commentTokenLen = 2

	// MaxQueryLength is the maximum allowed query size in bytes (1MB).
	// Queries exceeding this limit are rejected before any processing
	// to prevent memory exhaustion and DoS attacks.
	MaxQueryLength = 1 << 20

	// Connection pool defaults for the MCP server use case.
	defaultMaxOpenConns    = 10
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = time.Hour
	defaultConnMaxIdleTime = 10 * time.Minute

	// defaultMaxResultRows is the maximum number of rows returned by a query.
	// Configurable via POSTGRES_MCP_MAX_RESULT_ROWS environment variable.
	defaultMaxResultRows = 10000

	// rowCapHint caps the up-front capacity reserved for a result set so a large
	// (env-configurable) maxResultRows cannot force a huge allocation for queries
	// that return only a handful of rows, while still avoiding regrowth for
	// typical result sizes.
	rowCapHint = 256

	// maxCountFallbackTables caps the COUNT(*) fallback in ListTablesWithStats
	// so a schema with hundreds of fresh tables cannot fan out into unbounded
	// sequential round-trips (issue #90).
	maxCountFallbackTables = 20

	// countFallbackTimeout caps each COUNT(*) fallback call so a single billion-row
	// table cannot tie up a pool connection for minutes (issue #90).
	countFallbackTimeout = 5 * time.Second

	// defaultQueryTimeout bounds every tool handler's context and is also
	// pushed into the connection options as statement_timeout so PostgreSQL
	// cancels a runaway query even if the client context is unbounded
	// (issue #89). Overridable via POSTGRES_MCP_QUERY_TIMEOUT.
	defaultQueryTimeout = 30 * time.Second
)

// QueryTimeout resolves the query timeout from POSTGRES_MCP_QUERY_TIMEOUT,
// defaulting to defaultQueryTimeout when unset, blank, or unparseable.
// The value accepts Go duration syntax ("45s", "2m", "500ms"); bare integers
// are treated as seconds for ergonomic shell use. Non-positive values fall
// back to the default so a misconfiguration cannot silently disable the
// timeout (issue #89).
func QueryTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("POSTGRES_MCP_QUERY_TIMEOUT"))
	if raw == "" {
		return defaultQueryTimeout
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return defaultQueryTimeout
}

// injectOption appends a single "-c key=value"-style server option to the
// PostgreSQL connection string's options= payload, handling both URL-style
// (postgres://...) and keyword-value style DSNs. Multiple options can be
// added by calling this function repeatedly; each call appends to whatever
// options payload already exists.
func injectOption(connStr, opt string) string {
	connStr = strings.TrimSpace(connStr)
	if connStr == "" {
		return connStr
	}

	// URL-style connection string
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		u, err := url.Parse(connStr)
		if err != nil {
			return connStr
		}
		q := u.Query()
		existing := q.Get("options")
		if existing != "" {
			q.Set("options", existing+" "+opt)
		} else {
			q.Set("options", opt)
		}
		u.RawQuery = q.Encode()
		return u.String()
	}

	// Keyword-value style connection string.
	optionsIdx := strings.Index(connStr, "options=")
	if optionsIdx == -1 {
		return connStr + " options='" + opt + "'"
	}
	valStart := optionsIdx + len("options=")

	// Quoted form: inject the new option before the closing quote.
	if valStart < len(connStr) && connStr[valStart] == '\'' {
		closeRel := strings.Index(connStr[valStart+1:], "'")
		if closeRel == -1 {
			// Malformed (unterminated quote): leave alone rather than corrupt the DSN.
			return connStr
		}
		closeIdx := valStart + 1 + closeRel
		return connStr[:closeIdx] + " " + opt + connStr[closeIdx:]
	}

	// Unquoted form (issue #84): value runs to next whitespace or EOS.
	// Rewrite as the quoted form so a multi-token options payload stays a
	// single value; previously this branch silently returned the DSN
	// unchanged, skipping the option-injection guarantee.
	valEnd := len(connStr)
	if rel := strings.IndexAny(connStr[valStart:], " \t\n"); rel != -1 {
		valEnd = valStart + rel
	}
	existing := connStr[valStart:valEnd]
	return connStr[:optionsIdx] + "options='" + existing + " " + opt + "'" + connStr[valEnd:]
}

// injectReadOnlyOption appends default_transaction_read_only=on to the connection
// string so every pool connection is read-only at the PostgreSQL session level.
func injectReadOnlyOption(connStr string) string {
	return injectOption(connStr, readOnlyOption)
}

// injectStatementTimeout appends a server-side statement_timeout (milliseconds)
// to the connection options so a runaway query is killed by PostgreSQL itself
// even when the caller's context is unbounded (issue #89). A non-positive
// duration is a no-op.
func injectStatementTimeout(connStr string, d time.Duration) string {
	if d <= 0 {
		return connStr
	}
	return injectOption(connStr, fmt.Sprintf("-c statement_timeout=%d", d.Milliseconds()))
}

// PostgreSQLClientImpl implements the PostgreSQLClient interface.
//
// db is held as atomic.Pointer so that concurrent handler goroutines can
// load the current *pgxpool.Pool without locking, while Connect can atomically
// swap in a freshly opened pool during reconnection (issue #83).
type PostgreSQLClientImpl struct {
	db atomic.Pointer[pgxpool.Pool]
}

// NewPostgreSQLClient creates a new PostgreSQL client.
func NewPostgreSQLClient() *PostgreSQLClientImpl {
	return &PostgreSQLClientImpl{}
}

// envIntOrDefault returns the integer value of an environment variable,
// or the default value if the variable is not set or cannot be parsed.
func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

// clampMaxIdle bounds the idle connection count to the open connection count.
// database/sql silently caps idle connections to the open limit, so a
// misconfiguration like maxIdle=20, maxOpen=10 would leave intended capacity
// unusable with no signal. Clamping here lets the caller surface the problem
// (issue #104). The returned bool reports whether clamping occurred.
func clampMaxIdle(maxOpen, maxIdle int) (int, bool) {
	if maxIdle > maxOpen {
		return maxOpen, true
	}
	return maxIdle, false
}

// poolConfig returns connection pool settings from environment variables,
// falling back to sensible defaults for the MCP server use case. When the
// configured idle limit exceeds the open limit it is clamped to the open limit
// and a warning is logged, since database/sql would otherwise cap it silently
// (issue #104).
func poolConfig() (int, int, time.Duration, time.Duration) {
	maxOpen := envIntOrDefault("POSTGRES_MCP_MAX_OPEN_CONNS", defaultMaxOpenConns)
	maxIdle := envIntOrDefault("POSTGRES_MCP_MAX_IDLE_CONNS", defaultMaxIdleConns)
	if clamped, capped := clampMaxIdle(maxOpen, maxIdle); capped {
		slog.Warn(
			"POSTGRES_MCP_MAX_IDLE_CONNS exceeds POSTGRES_MCP_MAX_OPEN_CONNS; "+
				"clamping idle connections to the open limit",
			"requested_max_idle", maxIdle,
			"max_open", maxOpen,
			"effective_max_idle", clamped,
		)
		maxIdle = clamped
	}
	lifetimeSec := envIntOrDefault("POSTGRES_MCP_CONN_MAX_LIFETIME", int(defaultConnMaxLifetime.Seconds()))
	idleTimeSec := envIntOrDefault("POSTGRES_MCP_CONN_MAX_IDLE_TIME", int(defaultConnMaxIdleTime.Seconds()))
	return maxOpen, maxIdle, time.Duration(lifetimeSec) * time.Second, time.Duration(idleTimeSec) * time.Second
}

// maxResultRows returns the maximum number of rows allowed in a query result,
// configurable via POSTGRES_MCP_MAX_RESULT_ROWS environment variable.
func maxResultRows() int {
	return envIntOrDefault("POSTGRES_MCP_MAX_RESULT_ROWS", defaultMaxResultRows)
}

// Connect establishes a connection to the PostgreSQL database.
// The connection is configured as read-only at the PostgreSQL session level
// to provide defense-in-depth against SQL injection attacks.
// Pool settings can be overridden via environment variables:
//   - POSTGRES_MCP_MAX_OPEN_CONNS (pgxpool MaxConns, default: 10)
//   - POSTGRES_MCP_MAX_IDLE_CONNS (pgxpool MinConns — connections kept warm;
//     default: 5, clamped to max open conns)
//   - POSTGRES_MCP_CONN_MAX_LIFETIME (seconds, default: 3600)
//   - POSTGRES_MCP_CONN_MAX_IDLE_TIME (seconds, default: 600)
func (c *PostgreSQLClientImpl) Connect(ctx context.Context, connectionString string) error {
	hardenedConnStr := injectStatementTimeout(injectReadOnlyOption(connectionString), QueryTimeout())
	cfg, err := pgxpool.ParseConfig(hardenedConnStr)
	if err != nil {
		return fmt.Errorf("failed to parse database connection config: %w", err)
	}
	applyPoolConfig(cfg)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Atomic swap; close any previous pool that handler goroutines may still
	// hold a reference to. pgxpool.Pool.Close waits for in-flight queries
	// acquired from that pool to finish, so concurrent readers degrade
	// gracefully.
	if old := c.db.Swap(pool); old != nil {
		old.Close()
	}
	return nil
}

// applyPoolConfig maps the environment-derived pool settings onto a pgxpool
// config. POSTGRES_MCP_MAX_OPEN_CONNS becomes MaxConns and
// POSTGRES_MCP_MAX_IDLE_CONNS becomes MinConns (connections pgxpool keeps warm);
// clampMaxIdle in poolConfig guarantees MinConns <= MaxConns, which pgxpool
// requires. The int->int32 conversions are safe: the values come from
// envIntOrDefault, which only accepts small positive integers.
func applyPoolConfig(cfg *pgxpool.Config) {
	maxOpen, maxIdle, maxLifetime, maxIdleTime := poolConfig()
	//nolint:gosec // pool sizes come from envIntOrDefault as small positive operator-set values, not attacker input.
	cfg.MaxConns = int32(maxOpen)
	//nolint:gosec // pool sizes come from envIntOrDefault as small positive operator-set values, not attacker input.
	cfg.MinConns = int32(maxIdle)
	cfg.MaxConnLifetime = maxLifetime
	cfg.MaxConnIdleTime = maxIdleTime
}

// Close closes the database connection.
func (c *PostgreSQLClientImpl) Close() error {
	pool := c.db.Swap(nil)
	if pool == nil {
		return nil
	}
	pool.Close()
	return nil
}

// Ping checks if the database connection is alive.
func (c *PostgreSQLClientImpl) Ping(ctx context.Context) error {
	pool := c.db.Load()
	if pool == nil {
		return ErrNoDatabaseConnection
	}
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

// Pool returns the underlying pgxpool connection pool.
func (c *PostgreSQLClientImpl) Pool() *pgxpool.Pool {
	return c.db.Load()
}

// HasConnection reports whether a connection pool has been established, using a
// cheap, lock-free atomic load (no network round-trip).
func (c *PostgreSQLClientImpl) HasConnection() bool {
	return c.db.Load() != nil
}

// ListDatabases returns a list of all databases on the server.
func (c *PostgreSQLClientImpl) ListDatabases(ctx context.Context) ([]*DatabaseInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	query := `
		SELECT datname, pg_catalog.pg_get_userbyid(datdba) as owner, pg_encoding_to_char(encoding) as encoding
		FROM pg_database
		WHERE datistemplate = false
		ORDER BY datname`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}
	defer rows.Close()

	var databases []*DatabaseInfo
	for rows.Next() {
		var info DatabaseInfo
		if err := rows.Scan(&info.Name, &info.Owner, &info.Encoding); err != nil {
			return nil, fmt.Errorf("failed to scan database row: %w", err)
		}
		databases = append(databases, &info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate database rows: %w", err)
	}
	return databases, nil
}

// GetCurrentDatabase returns the name of the current database.
func (c *PostgreSQLClientImpl) GetCurrentDatabase(ctx context.Context) (string, error) {
	pool := c.db.Load()
	if pool == nil {
		return "", ErrNoDatabaseConnection
	}

	var dbName string
	err := pool.QueryRow(ctx, "SELECT current_database()").Scan(&dbName)
	if err != nil {
		return "", fmt.Errorf("failed to get current database: %w", err)
	}

	return dbName, nil
}

// ListSchemas returns a list of schemas in the current database.
func (c *PostgreSQLClientImpl) ListSchemas(ctx context.Context) ([]*SchemaInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	query := `
		SELECT schema_name, schema_owner
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
		ORDER BY schema_name`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var schemas []*SchemaInfo
	for rows.Next() {
		var schema SchemaInfo
		if err := rows.Scan(&schema.Name, &schema.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		schemas = append(schemas, &schema)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate schema rows: %w", err)
	}
	return schemas, nil
}

// ListTables returns a list of tables in the specified schema.
func (c *PostgreSQLClientImpl) ListTables(ctx context.Context, schema string) ([]*TableInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			schemaname,
			tablename,
			'table' as type,
			tableowner as owner
		FROM pg_tables
		WHERE schemaname = $1
		UNION ALL
		SELECT
			schemaname,
			viewname as tablename,
			'view' as type,
			viewowner as owner
		FROM pg_views
		WHERE schemaname = $1
		ORDER BY tablename`

	rows, err := pool.Query(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []*TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type, &table.Owner); err != nil {
			return nil, fmt.Errorf("failed to scan table row: %w", err)
		}
		tables = append(tables, &table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate table rows: %w", err)
	}
	return tables, nil
}

// ListTablesWithStats returns a list of tables with size and row count statistics in a single query.
// Row count prefers pg_stat_user_tables (n_tup_ins - n_tup_del); when that is 0 — e.g., fresh tables —
// it falls back to pg_class.reltuples in the same SELECT, so the result remains O(1) round-trips
// regardless of how many tables show empty statistics.
func (c *PostgreSQLClientImpl) ListTablesWithStats(ctx context.Context, schema string) ([]*TableInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	// Single optimized query that joins tables with statistics. Row count uses
	// n_tup_ins - n_tup_del (more accurate than n_live_tup after recent writes);
	// when that is 0 — pg_stat not yet populated for fresh tables — fall back to
	// pg_class.reltuples in the same round-trip. Avoids per-table COUNT(*) which
	// is O(rows) and previously fanned out as N+1 over the result set (issue #90).
	// reltuples is -1 on PG14+ until first ANALYZE, so clamp with GREATEST.
	query := `
		WITH table_list AS (
			SELECT
				schemaname,
				tablename,
				'table' as type,
				tableowner as owner
			FROM pg_tables
			WHERE schemaname = $1
			UNION ALL
			SELECT
				schemaname,
				viewname as tablename,
				'view' as type,
				viewowner as owner
			FROM pg_views
			WHERE schemaname = $1
		)
		SELECT
			t.schemaname,
			t.tablename,
			t.type,
			t.owner,
			CASE
				WHEN COALESCE(s.n_tup_ins - s.n_tup_del, 0) > 0
					THEN s.n_tup_ins - s.n_tup_del
				WHEN t.type = 'table'
					THEN GREATEST(COALESCE(c.reltuples, 0)::bigint, 0)
				ELSE 0
			END as row_count,
			pg_size_pretty(COALESCE(pg_total_relation_size(quote_ident(t.schemaname) || '.' || quote_ident(t.tablename)), 0)) as size
		FROM table_list t
		LEFT JOIN pg_stat_user_tables s
			ON t.schemaname = s.schemaname AND t.tablename = s.relname
		LEFT JOIN pg_namespace n
			ON n.nspname = t.schemaname
		LEFT JOIN pg_class c
			ON c.relname = t.tablename AND c.relnamespace = n.oid AND c.relkind IN ('r', 'p')
		ORDER BY t.tablename`

	rows, err := pool.Query(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables with stats: %w", err)
	}
	defer rows.Close()

	var tables []*TableInfo
	for rows.Next() {
		var table TableInfo
		if err := rows.Scan(&table.Schema, &table.Name, &table.Type, &table.Owner, &table.RowCount, &table.Size); err != nil {
			return nil, fmt.Errorf("failed to scan table row with stats: %w", err)
		}
		tables = append(tables, &table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate table rows with stats: %w", err)
	}

	c.refineZeroRowCounts(ctx, tables)
	return tables, nil
}

// DescribeTable returns detailed column information for a table.
func (c *PostgreSQLClientImpl) DescribeTable(ctx context.Context, schema, table string) ([]*ColumnInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			column_name,
			data_type,
			is_nullable = 'YES' as is_nullable,
			COALESCE(column_default, '') as default_value
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`

	rows, err := pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to describe table: %w", err)
	}
	defer rows.Close()

	var columns []*ColumnInfo
	for rows.Next() {
		var column ColumnInfo
		if err := rows.Scan(&column.Name, &column.DataType, &column.IsNullable, &column.DefaultValue); err != nil {
			return nil, fmt.Errorf("failed to scan column row: %w", err)
		}
		columns = append(columns, &column)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate column rows: %w", err)
	}

	// Check if table exists (if no columns found, table doesn't exist)
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %s.%s: %w", schema, table, ErrTableNotFound)
	}

	return columns, nil
}

// GetTableStats returns statistics for a specific table.
func (c *PostgreSQLClientImpl) GetTableStats(ctx context.Context, schema, table string) (*TableInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	tableInfo := &TableInfo{
		Schema: schema,
		Name:   table,
	}

	// Single round-trip: prefer n_tup_ins - n_tup_del when present, otherwise
	// fall back to pg_class.reltuples. reltuples is -1 on PG14+ until first
	// ANALYZE, so clamp with GREATEST.
	estimateQuery := `
		SELECT
			CASE
				WHEN COALESCE(s.n_tup_ins - s.n_tup_del, 0) > 0
					THEN s.n_tup_ins - s.n_tup_del
				ELSE GREATEST(COALESCE(c.reltuples, 0)::bigint, 0)
			END as row_count
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_stat_user_tables s
			ON s.schemaname = n.nspname AND s.relname = c.relname
		WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('r', 'p')`

	var rowCount *int64
	err := pool.QueryRow(ctx, estimateQuery, schema, table).Scan(&rowCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("table %s.%s: %w", schema, table, ErrTableNotFound)
		}
		return nil, fmt.Errorf("failed to get table stats: %w", err)
	}
	if rowCount != nil {
		tableInfo.RowCount = *rowCount
	}

	// If the estimate is 0, fall back to an exact COUNT(*) for freshly written
	// tables that pg_stat has not observed and that no ANALYZE has touched.
	// The fallback is bounded by countFallbackTimeout so a single billion-row
	// table cannot tie up a pool connection for minutes (issue #90).
	if tableInfo.RowCount == 0 {
		countQuery := "SELECT COUNT(*) FROM " + pgx.Identifier{schema, table}.Sanitize()
		countCtx, cancel := context.WithTimeout(ctx, countFallbackTimeout)
		defer cancel()
		var actualCount int64
		if err := pool.QueryRow(countCtx, countQuery).Scan(&actualCount); err == nil {
			tableInfo.RowCount = actualCount
		}
	}

	return tableInfo, nil
}

// ListIndexes returns a list of indexes for the specified table.
func (c *PostgreSQLClientImpl) ListIndexes(ctx context.Context, schema, table string) ([]*IndexInfo, error) {
	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	if schema == "" {
		schema = DefaultSchema
	}

	query := `
		SELECT
			i.relname as index_name,
			t.relname as table_name,
			array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as columns,
			ix.indisunique as is_unique,
			ix.indisprimary as is_primary,
			am.amname as index_type
		FROM pg_class t
		JOIN pg_index ix ON t.oid = ix.indrelid
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_am am ON i.relam = am.oid
		JOIN pg_namespace n ON t.relnamespace = n.oid
		JOIN pg_attribute a ON a.attrelid = t.oid
		WHERE n.nspname = $1 AND t.relname = $2 AND a.attnum = ANY(ix.indkey)
		GROUP BY i.relname, t.relname, ix.indisunique, ix.indisprimary, am.amname
		ORDER BY i.relname`

	rows, err := pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer rows.Close()

	var indexes []*IndexInfo
	for rows.Next() {
		var index IndexInfo
		// pgx scans a PostgreSQL text[] directly into a []string.
		if err := rows.Scan(
			&index.Name, &index.Table, &index.Columns,
			&index.IsUnique, &index.IsPrimary, &index.IndexType,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}

		indexes = append(indexes, &index)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate index rows: %w", err)
	}
	return indexes, nil
}

// copySingleQuotedLiteral copies a single-quoted string literal from query[start]
// (which must be a single quote) into result, returning the new index after the
// closing quote. Handles escaped quotes (”). It walks bytes rather than runes:
// every SQL token involved is ASCII, and UTF-8 continuation bytes (>= 0x80) never
// collide with it, so multibyte content is copied through verbatim.
func copySingleQuotedLiteral(query string, start int, result *strings.Builder) int {
	result.WriteByte(query[start]) // opening quote
	i := start + 1
	for i < len(query) {
		idx := strings.IndexByte(query[i:], '\'')
		if idx < 0 {
			result.WriteString(query[i:]) // unterminated literal: copy remainder
			return len(query)
		}
		j := i + idx
		result.WriteString(query[i : j+1]) // copy through the closing quote
		i = j + 1
		if i < len(query) && query[i] == '\'' {
			result.WriteByte('\'') // escaped quote: consume the second and continue
			i++
			continue
		}
		return i
	}
	return i
}

// copyDoubleQuotedIdentifier copies a double-quoted identifier from query[start]
// (which must be a double quote) into result, returning the new index after the
// closing quote. Matches the original behavior of stopping at the first closing
// quote (PostgreSQL "" un-escaping is intentionally not handled here).
func copyDoubleQuotedIdentifier(query string, start int, result *strings.Builder) int {
	result.WriteByte(query[start]) // opening quote
	i := start + 1
	idx := strings.IndexByte(query[i:], '"')
	if idx < 0 {
		result.WriteString(query[i:]) // unterminated identifier: copy remainder
		return len(query)
	}
	j := i + idx
	result.WriteString(query[i : j+1]) // copy through the closing quote
	return j + 1
}

// skipBlockComment skips a block comment starting at query[start] (which must be '/')
// with nesting support. Returns the new index after the closing */.
func skipBlockComment(query string, start int) int {
	depth := 1
	i := start + commentTokenLen
	for i < len(query) && depth > 0 {
		switch {
		case i+1 < len(query) && query[i] == '/' && query[i+1] == '*':
			depth++
			i += commentTokenLen
		case i+1 < len(query) && query[i] == '*' && query[i+1] == '/':
			depth--
			i += commentTokenLen
		default:
			i++
		}
	}
	return i
}

// skipLineComment skips a line comment starting at query[start] (which must be '-').
// Returns the new index at the newline character (or end of query).
func skipLineComment(query string, start int) int {
	idx := strings.IndexByte(query[start+commentTokenLen:], '\n')
	if idx < 0 {
		return len(query)
	}
	return start + commentTokenLen + idx
}

// stripComments removes SQL comments from a query while preserving
// content inside single-quoted string literals and double-quoted identifiers.
// Block comments (/* */, including nested) and line comments (--) are replaced with spaces.
// It walks bytes directly (no []rune conversion): all SQL tokens handled are ASCII,
// and multibyte UTF-8 sequences are copied through verbatim.
func stripComments(query string) string {
	var result strings.Builder
	result.Grow(len(query))
	i := 0

	for i < len(query) {
		switch {
		case query[i] == '\'':
			i = copySingleQuotedLiteral(query, i, &result)
		case query[i] == '"':
			i = copyDoubleQuotedIdentifier(query, i, &result)
		case i+1 < len(query) && query[i] == '/' && query[i+1] == '*':
			i = skipBlockComment(query, i)
			result.WriteByte(' ')
		case i+1 < len(query) && query[i] == '-' && query[i+1] == '-':
			i = skipLineComment(query, i)
			result.WriteByte(' ')
		default:
			result.WriteByte(query[i])
			i++
		}
	}
	return result.String()
}

// containsSemicolonOutsideLiterals checks if the query contains a semicolon
// that is not inside a single-quoted string literal or double-quoted identifier.
// It walks bytes directly; all tokens handled are ASCII. An unterminated literal
// swallows the remainder of the query, so any trailing semicolon is treated as
// being inside the literal (returns false), matching the original behavior.
func containsSemicolonOutsideLiterals(query string) bool {
	i := 0
	for i < len(query) {
		switch query[i] {
		case '\'':
			i++
			for i < len(query) {
				idx := strings.IndexByte(query[i:], '\'')
				if idx < 0 {
					return false // unterminated literal
				}
				i += idx + 1
				if i < len(query) && query[i] == '\'' {
					i++ // escaped quote: skip the second and keep scanning the literal
					continue
				}
				break
			}
		case '"':
			i++
			idx := strings.IndexByte(query[i:], '"')
			if idx < 0 {
				return false // unterminated identifier
			}
			i += idx + 1
		case ';':
			return true
		default:
			i++
		}
	}
	return false
}

// hasPrefixFold reports whether s begins with prefix, ignoring ASCII case,
// without allocating an upper/lower-cased copy of s.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// validateQuery checks if the query is allowed (SELECT or WITH only)
// and rejects multi-statement queries.
// Comments are stripped before validation to prevent comment-based injection.
func validateQuery(query string) error {
	if len(query) > MaxQueryLength {
		return ErrQueryTooLong
	}
	stripped := stripComments(query)
	trimmed := strings.TrimSpace(stripped)
	if !hasPrefixFold(trimmed, "SELECT") && !hasPrefixFold(trimmed, "WITH") {
		return ErrInvalidQuery
	}
	if containsSemicolonOutsideLiterals(stripped) {
		return ErrMultiStatementQuery
	}
	return nil
}

// processRows reads query result rows into native Go values. maxRows limits the
// number of rows returned to prevent memory exhaustion. Column names come from
// the row description; cell values come from pgx's native decoding (Values),
// normalized for stable JSON serialization.
func processRows(rows pgx.Rows, maxRows int) ([]string, [][]any, error) {
	fields := rows.FieldDescriptions()
	columns := make([]string, len(fields))
	for i := range fields {
		columns[i] = fields[i].Name
	}

	// Preallocate the result capped so a large configurable maxRows can't force a
	// huge up-front allocation. rows.Values allocates a fresh slice per row, so
	// each appended row is safe to retain.
	result := make([][]any, 0, min(maxRows, rowCapHint))
	for rows.Next() {
		if len(result) >= maxRows {
			return nil, nil, fmt.Errorf("result set exceeded %d rows: %w", maxRows, ErrResultTooLarge)
		}

		values, err := rows.Values()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to read row values: %w", err)
		}
		normalizeRowValues(values)
		result = append(result, values)
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("failed to iterate query rows: %w", err)
	}
	return columns, result, nil
}

// normalizeRowValues rewrites natively-decoded pgx cell values into stable,
// JSON-friendly forms. int/float/bool/string/time.Time already serialize
// cleanly and pass through untouched. []byte (bytea and unmapped text) becomes a
// string, preserving the pre-pgx behavior. uuid decodes to a [16]byte, which is
// rendered as a canonical UUID string. Remaining decoded types that implement
// driver.Valuer (e.g. pgtype.Numeric) are reduced to a primitive so a result
// never carries an opaque struct into the JSON tool response.
func normalizeRowValues(values []any) {
	for i, v := range values {
		switch t := v.(type) {
		case []byte:
			values[i] = string(t)
		case [16]byte:
			values[i] = encodeUUID(t)
		case driver.Valuer:
			values[i] = valuerToPrimitive(t)
		}
	}
}

// valuerToPrimitive reduces a driver.Valuer to a JSON-friendly primitive,
// falling back to its formatted form when the value cannot be resolved.
func valuerToPrimitive(v driver.Valuer) any {
	dv, err := v.Value()
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	if b, ok := dv.([]byte); ok {
		return string(b)
	}
	return dv
}

// encodeUUID renders the 16 raw bytes of a PostgreSQL uuid as the canonical
// 8-4-4-4-12 lowercase hexadecimal string.
func encodeUUID(b [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ExecuteQuery executes a SELECT query and returns the results.
func (c *PostgreSQLClientImpl) ExecuteQuery(ctx context.Context, query string, args ...any) (*QueryResult, error) {
	if err := validateQuery(query); err != nil {
		return nil, err
	}

	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, result, err := processRows(rows, maxResultRows())
	if err != nil {
		return nil, err
	}
	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, nil
}

// ExplainQuery returns the execution plan for a query. When analyze is false
// the plan is non-executing — EXPLAIN (FORMAT JSON) — which is the safe
// default for an LLM-driven tool surface that may submit heavy queries
// (issue #89). When analyze is true the query is executed via
// EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON); the caller's context bounds the
// run, complementing the server-side statement_timeout.
func (c *PostgreSQLClientImpl) ExplainQuery(ctx context.Context, query string, analyze bool, args ...any) (*QueryResult, error) {
	if err := validateQuery(query); err != nil {
		return nil, err
	}

	pool := c.db.Load()
	if pool == nil {
		return nil, ErrNoDatabaseConnection
	}

	prefix := "EXPLAIN (FORMAT JSON) "
	if analyze {
		prefix = "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "
	}
	// query is validated by validateQuery above (SELECT/WITH only), so the
	// EXPLAIN-prefixed string carries no untrusted statement.
	explainQuery := prefix + query

	rows, err := pool.Query(ctx, explainQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute explain query: %w", err)
	}
	defer rows.Close()

	columns, result, err := processRows(rows, maxResultRows())
	if err != nil {
		return nil, err
	}
	return &QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}, nil
}

// refineZeroRowCounts issues a bounded number of COUNT(*) probes — each with a
// per-call timeout — for tables that still report 0 rows after the reltuples
// fallback. This handles freshly written tables that pg_stat has not yet
// observed and that no ANALYZE has touched, without re-introducing the
// unbounded N+1 / long-running COUNT(*) pattern of issue #90.
func (c *PostgreSQLClientImpl) refineZeroRowCounts(ctx context.Context, tables []*TableInfo) {
	pool := c.db.Load()
	if pool == nil {
		return
	}
	probed := 0
	for _, table := range tables {
		if probed >= maxCountFallbackTables {
			return
		}
		if table.RowCount != 0 || table.Type != "table" {
			continue
		}

		// pgx.Identifier.Sanitize escapes both schema and table to defend against
		// SQL injection via malicious identifiers.
		countQuery := "SELECT COUNT(*) FROM " + pgx.Identifier{table.Schema, table.Name}.Sanitize()

		countCtx, cancel := context.WithTimeout(ctx, countFallbackTimeout)
		var actualCount int64
		if err := pool.QueryRow(countCtx, countQuery).Scan(&actualCount); err == nil {
			table.RowCount = actualCount
		}
		cancel()
		probed++
	}
}
