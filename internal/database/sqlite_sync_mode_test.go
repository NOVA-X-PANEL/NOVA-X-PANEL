package database

import "testing"

// The `synchronous` pragma means different things in WAL and rollback-journal
// modes, so its default is derived from the journal mode. These cases pin that
// pairing: WAL gets NORMAL (SQLite's documented recommendation, and the panel
// commits on timers so FULL was an fsync per commit for no correctness gain),
// while a rollback journal keeps FULL because NORMAL there drops a sync that
// guards the journal itself.
func TestSQLiteSynchronousDefaultsFollowJournalMode(t *testing.T) {
	t.Setenv("XUI_DB_SYNCHRONOUS", "")

	cases := []struct {
		journal string
		want    string
	}{
		{"WAL", "NORMAL"},
		{"wal", "NORMAL"},
		{"DELETE", "FULL"},
		{"delete", "FULL"},
		{"", "FULL"},
		{"PERSIST", "FULL"},
	}

	for _, tc := range cases {
		t.Run(tc.journal, func(t *testing.T) {
			if got := sqliteSynchronous(tc.journal); got != tc.want {
				t.Errorf("sqliteSynchronous(%q) = %q, want %q", tc.journal, got, tc.want)
			}
		})
	}
}

// An explicit setting always wins, in both directions: an operator who wants
// FULL back in WAL mode can still ask for it, and one who accepts the risk of
// NORMAL with a rollback journal can ask for that too.
func TestSQLiteSynchronousExplicitOverrideWins(t *testing.T) {
	cases := []struct {
		env     string
		journal string
		want    string
	}{
		{"FULL", "WAL", "FULL"},
		{"full", "WAL", "FULL"},
		{"NORMAL", "DELETE", "NORMAL"},
		{"OFF", "WAL", "OFF"},
		{"EXTRA", "WAL", "EXTRA"},
		{" NORMAL ", "DELETE", "NORMAL"},
	}

	for _, tc := range cases {
		t.Run(tc.env+"/"+tc.journal, func(t *testing.T) {
			t.Setenv("XUI_DB_SYNCHRONOUS", tc.env)
			if got := sqliteSynchronous(tc.journal); got != tc.want {
				t.Errorf("with XUI_DB_SYNCHRONOUS=%q, journal=%q: got %q, want %q",
					tc.env, tc.journal, got, tc.want)
			}
		})
	}
}

// A value the code does not recognise must not be forwarded to SQLite as-is:
// an unknown pragma value is a syntax error there. It falls back to the journal
// mode's default instead.
func TestSQLiteSynchronousUnknownValueFallsBack(t *testing.T) {
	for _, bad := range []string{"FAST", "0", "yes", "NORMALISH"} {
		t.Run(bad, func(t *testing.T) {
			t.Setenv("XUI_DB_SYNCHRONOUS", bad)
			if got := sqliteSynchronous("WAL"); got != "NORMAL" {
				t.Errorf("unknown value %q should fall back to the WAL default, got %q", bad, got)
			}
			if got := sqliteSynchronous("DELETE"); got != "FULL" {
				t.Errorf("unknown value %q should fall back to the DELETE default, got %q", bad, got)
			}
		})
	}
}

// WAL is still the default journal mode; the pairing above depends on it.
func TestSQLiteJournalModeDefaultsToWAL(t *testing.T) {
	t.Setenv("XUI_DB_JOURNAL_MODE", "")
	if got := sqliteJournalMode(); got != "WAL" {
		t.Errorf("sqliteJournalMode() = %q, want WAL", got)
	}
	t.Setenv("XUI_DB_JOURNAL_MODE", "delete")
	if got := sqliteJournalMode(); got != "DELETE" {
		t.Errorf("explicit DELETE should be honoured, got %q", got)
	}
}
