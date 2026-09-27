// SPDX-License-Identifier: AGPL-3.0-or-later

package branch

import (
	"regexp"
	"strings"
	"testing"
)

// What the sample data has to contain, because the first ten minutes are built on
// it: three tables, a foreign key between two of them, and five rows each.
func TestDemoSQLShape(t *testing.T) {
	sql := DemoSQL()
	if sql == "" {
		t.Fatal("demo.sql was not embedded")
	}
	for _, table := range []string{"users", "projects", "events"} {
		if !strings.Contains(sql, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("demo.sql does not create %q", table)
		}
	}
	if !strings.Contains(sql, "REFERENCES projects(id)") || !strings.Contains(sql, "REFERENCES users(id)") {
		t.Error("demo.sql has no foreign keys, which is the point of having three tables")
	}
	// Five rows each, counted as the value tuples of the three inserts.
	for _, table := range []string{"users", "projects", "events"} {
		re := regexp.MustCompile(`(?s)INSERT INTO ` + table + ` \([^)]*\) VALUES(.*?);`)
		m := re.FindStringSubmatch(sql)
		if m == nil {
			t.Errorf("no INSERT for %q", table)
			continue
		}
		if n := strings.Count(m[1], "\n\t("); n != 5 {
			t.Errorf("%q gets %d rows, want 5", table, n)
		}
	}
}

// Re-running the seed must change nothing: it runs on every first start, and a
// user may run it by hand on a branch that already has it.
func TestDemoSQLIsIdempotent(t *testing.T) {
	sql := DemoSQL()
	if strings.Count(sql, "CREATE TABLE ") != strings.Count(sql, "CREATE TABLE IF NOT EXISTS ") {
		t.Error("a CREATE TABLE without IF NOT EXISTS would fail on a second run")
	}
	if n := strings.Count(sql, "ON CONFLICT (id) DO NOTHING"); n != 3 {
		t.Errorf("%d inserts guard against a second run, want 3", n)
	}
}

// The serial sequences have to be moved past the seeded ids, or the first insert
// a user makes collides with the sample rows.
func TestDemoSQLAdvancesTheSequences(t *testing.T) {
	sql := DemoSQL()
	for _, table := range []string{"users", "projects", "events"} {
		if !strings.Contains(sql, "pg_get_serial_sequence('"+table+"'") &&
			!strings.Contains(sql, `pg_get_serial_sequence('`+table+`',`) {
			t.Errorf("no setval for %q's sequence", table)
		}
	}
}

// FOX_NO_DEMO is how the integration suites and anyone wanting an empty database
// switch the sample data off.
func TestDemoOff(t *testing.T) {
	for _, v := range []string{"1", "yes", "true", "on"} {
		t.Setenv("FOX_NO_DEMO", v)
		if !DemoOff() {
			t.Errorf("FOX_NO_DEMO=%q should switch the sample data off", v)
		}
	}
	for _, v := range []string{"", "0", "false", "no"} {
		t.Setenv("FOX_NO_DEMO", v)
		if DemoOff() {
			t.Errorf("FOX_NO_DEMO=%q should leave the sample data on", v)
		}
	}
}

// The seed refuses a name the engine would not use, before it reaches docker.
func TestSeedDemoChecksTheName(t *testing.T) {
	if err := SeedDemo("../etc/passwd"); err == nil {
		t.Error("SeedDemo accepted a name that is not a branch")
	}
	if err := DropDemo("no spaces allowed"); err == nil {
		t.Error("DropDemo accepted a name that is not a branch")
	}
}
