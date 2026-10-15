package mysql

import "testing"

// TestSQLStringLiteralEscapesSingleQuotes (audit P5-8): values interpolated
// into single-quoted SQL string literals escape embedded quotes by doubling
// so names like we'ird cannot break out of the literal.
func TestSQLStringLiteralEscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	if got, want := sqlStringLiteral("we'ird"), "'we''ird'"; got != want {
		t.Errorf("we'ird: want %v, got %v", want, got)
	}
	if got, want := sqlStringLiteral("plain"), "'plain'"; got != want {
		t.Errorf("plain: want %v, got %v", want, got)
	}
	if got, want := sqlStringLiteral("x') OR 1=1 --"), "'x'') OR 1=1 --'"; got != want {
		t.Errorf("breakout: want %v, got %v", want, got)
	}
	if got, want := escapeSQLStringLiteral("it's"), "it''s"; got != want {
		t.Errorf("escape only: want %v, got %v", want, got)
	}
}
