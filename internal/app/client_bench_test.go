package app

import (
	"strings"
	"testing"
)

// Representative queries for the validation/parsing hot path. The "large"
// variants make the cost of whole-query allocations (rune buffers, ToUpper
// copies) visible relative to the small constant-token prefix check.

func benchSmallQuery() string {
	return `SELECT u.id, u.name, u.email
	        FROM users u
	        JOIN orders o ON o.user_id = u.id
	        WHERE u.created_at > '2020-01-01' AND u.name = 'O''Brien'
	        ORDER BY u.id`
}

// benchLargeQuery builds a ~64KB SELECT with mixed string literals and
// comments so the byte-walk vs. rune-conversion difference is measurable.
func benchLargeQuery() string {
	var b strings.Builder
	b.WriteString("SELECT id FROM events WHERE\n")
	for range 1000 {
		b.WriteString("  label = 'value with /* not a comment */ text' OR -- a line comment\n")
	}
	b.WriteString("  id > 0")
	return b.String()
}

func BenchmarkValidateQuery(b *testing.B) {
	q := benchSmallQuery()
	b.ReportAllocs()
	for b.Loop() {
		if err := validateQuery(q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateQueryLarge(b *testing.B) {
	q := benchLargeQuery()
	b.ReportAllocs()
	for b.Loop() {
		if err := validateQuery(q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStripComments(b *testing.B) {
	q := benchLargeQuery()
	b.ReportAllocs()
	for b.Loop() {
		_ = stripComments(q)
	}
}

func BenchmarkContainsSemicolonOutsideLiterals(b *testing.B) {
	q := benchLargeQuery()
	b.ReportAllocs()
	for b.Loop() {
		_ = containsSemicolonOutsideLiterals(q)
	}
}
