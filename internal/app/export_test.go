package app

// This file is compiled only under `go test`. It re-exports the unexported
// symbols that the black-box tests in package app_test legitimately need,
// without widening the production API surface. Per issue #97, it is the single
// permitted place where test code reaches into package internals.

import (
	"context"
	"log/slog"
	"time"
)

// DefaultQueryTimeout exposes the unexported defaultQueryTimeout for tests.
const DefaultQueryTimeout = defaultQueryTimeout

// ValidateQuery exposes validateQuery for black-box tests.
func ValidateQuery(query string) error { return validateQuery(query) }

// StripComments exposes stripComments for black-box tests.
func StripComments(query string) string { return stripComments(query) }

// ContainsSemicolonOutsideLiterals exposes containsSemicolonOutsideLiterals for black-box tests.
func ContainsSemicolonOutsideLiterals(query string) bool {
	return containsSemicolonOutsideLiterals(query)
}

// InjectReadOnlyOption exposes injectReadOnlyOption for black-box tests.
func InjectReadOnlyOption(connStr string) string { return injectReadOnlyOption(connStr) }

// InjectStatementTimeout exposes injectStatementTimeout for black-box tests.
func InjectStatementTimeout(connStr string, d time.Duration) string {
	return injectStatementTimeout(connStr, d)
}

// EnvIntOrDefault exposes envIntOrDefault for black-box tests.
func EnvIntOrDefault(key string, defaultVal int) int { return envIntOrDefault(key, defaultVal) }

// ClampMaxIdle exposes clampMaxIdle for black-box tests.
func ClampMaxIdle(maxOpen, maxIdle int) (int, bool) { return clampMaxIdle(maxOpen, maxIdle) }

// PoolConfig exposes poolConfig for black-box tests.
func PoolConfig() (int, int, time.Duration, time.Duration) { return poolConfig() }

// MaxResultRows exposes maxResultRows for black-box tests.
func MaxResultRows() int { return maxResultRows() }

// TruncateQuery exposes truncateQuery for black-box tests.
func TruncateQuery(query string, maxLen int) string { return truncateQuery(query, maxLen) }

// EnsureConnection exposes the unexported ensureConnection method for black-box tests.
func (a *App) EnsureConnection(ctx context.Context) error { return a.ensureConnection(ctx) }

// ConnectIfNeeded exposes the reconnect-path connect(replaceExisting=false) for
// black-box tests of the no-op-when-already-connected behavior (finding H3).
func (a *App) ConnectIfNeeded(ctx context.Context, connStr string) error {
	return a.connect(ctx, connStr, false)
}

// Client exposes the unexported client field for black-box tests.
func (a *App) Client() PostgreSQLClient { return a.client }

// Logger exposes the unexported logger field for black-box tests.
func (a *App) Logger() *slog.Logger { return a.logger.Load() }
