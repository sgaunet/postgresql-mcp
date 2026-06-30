package app

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// ValidateConnectionString verifies that connStr is a connection string the
// driver can parse, using pgx's own parser (pgconn.ParseConfig) so validation
// never diverges from what Connect will actually accept later.
//
// Using the driver's parser — rather than a hand-rolled net/url check — means
// it transparently covers both URL form (postgres://...) and libpq
// keyword/value DSN form (host=... sslmode=...), rejects unknown sslmode values
// early, and inherits pgx's exact case-sensitivity and duplicate-parameter
// semantics. The raw string is never echoed in the returned error so embedded
// credentials cannot leak into logs.
func ValidateConnectionString(connStr string) error {
	if connStr == "" {
		return ErrInvalidConnectionParameters
	}
	if _, err := pgconn.ParseConfig(connStr); err != nil {
		return fmt.Errorf("%w: unparseable connection string (check URL/DSN syntax and sslmode)",
			ErrInvalidConnectionParameters)
	}
	return nil
}
