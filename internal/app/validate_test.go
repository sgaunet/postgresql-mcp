package app_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/sylvain/postgresql-mcp/internal/app"
)

// TestValidateConnectionString exercises the pgconn-backed validator across
// both URL and keyword/value DSN forms. The DSN cases are the regression guard
// for review finding H2: a keyword/value string slipped past the old
// net/url-based check unvalidated.
func TestValidateConnectionString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		connStr string
		wantErr bool
	}{
		// URL form — valid
		{"url no sslmode", "postgres://u:p@localhost:5432/db", false},
		{"url valid sslmode require", "postgres://u:p@localhost:5432/db?sslmode=require", false},
		{"url valid sslmode disable", "postgres://u:p@localhost:5432/db?sslmode=disable", false},
		{"url verify-full", "postgres://u:p@localhost:5432/db?sslmode=verify-full", false},
		// URL form — invalid
		{"url invalid sslmode", "postgres://u:p@localhost:5432/db?sslmode=banana", true},
		{"url uppercase sslmode rejected", "postgres://u:p@localhost:5432/db?sslmode=DISABLE", true},
		// DSN keyword/value form — valid (finding H2 path)
		{"dsn valid sslmode disable", "host=localhost port=5432 dbname=db user=u sslmode=disable", false},
		{"dsn no sslmode", "host=localhost dbname=db user=u", false},
		// DSN keyword/value form — invalid (finding H2 path)
		{"dsn invalid sslmode", "host=localhost dbname=db user=u sslmode=banana", true},
		// Malformed / empty
		{"empty", "", true},
		{"garbage keyword/value", "this-is-not-a-valid-dsn", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := app.ValidateConnectionString(tc.connStr)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, app.ErrInvalidConnectionParameters)
				// The raw connection string must never leak into the error.
				if tc.connStr != "" {
					assert.NotContains(t, err.Error(), tc.connStr)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
