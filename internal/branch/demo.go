// SPDX-License-Identifier: AGPL-3.0-or-later

package branch

import (
	"embed"
	"fmt"
	"os"
	"strings"

	"github.com/thefoxbyte/foxbyte/internal/brand"
)

// Sample data on a first start. Until now a new install opened the console on an
// empty database: no tables to query, no Blackbox entries to look at, and the
// first ten minutes spent inventing a schema before anything the product does
// could be seen. demo.sql gives main three related tables with five rows each,
// and because the CREATE TABLEs go through the Blackbox like any other change,
// the record is populated the moment the page loads.
//
// Seeded once, guarded by a marker file rather than by looking for the tables: a
// user who drops them meant to, and a second start should not put them back.

//go:embed demo.sql
var demoFS embed.FS

// demoMarker records that this install has been seeded. In the state directory,
// beside the other per-install files.
func demoMarker() string { return brand.StatePath("demo-seeded") }

// DemoOff reports whether sample data is switched off (FOX_NO_DEMO). The
// integration suites set it: they count Blackbox entries and list tables, and
// data that appears on its own would change what they see.
func DemoOff() bool {
	switch strings.ToLower(strings.TrimSpace(brand.Getenv("NO_DEMO"))) {
	case "", "0", "false", "no":
		return false
	}
	return true
}

// DemoSQL is the sample schema and rows, for callers that want to show it.
func DemoSQL() string {
	b, _ := demoFS.ReadFile("demo.sql")
	return string(b)
}

// SeedDemo applies the sample data to a branch. Idempotent: every statement is
// IF NOT EXISTS or ON CONFLICT, so running it twice changes nothing.
func SeedDemo(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if !Exists(name) {
		return fmt.Errorf("no branch %q", name)
	}
	// One -c, not one per statement: psql runs it as a single implicit
	// transaction, so a failure part way leaves no half-seeded schema.
	if err := SQL(name, DemoSQL()); err != nil {
		return fmt.Errorf("seeding the sample data into %q: %w", name, err)
	}
	return nil
}

// ensureDemoData seeds main the first time the stack comes up, unless sample data
// is switched off. Failure is a warning: an install that cannot seed is still a
// working install, and saying so beats stopping a start over five rows.
func ensureDemoData() {
	if DemoOff() {
		return
	}
	if _, err := os.Stat(demoMarker()); err == nil {
		return
	}
	if err := SeedDemo("main"); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return
	}
	// Written after the fact, so a failed seed is retried on the next start.
	if err := os.WriteFile(demoMarker(), []byte("sample data seeded into main\n"), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: recording that the sample data was seeded: %v\n", err)
	}
	fmt.Printf("Sample data in main: users, projects, events (five rows each). `%s demo drop` removes it.\n", brand.CLI)
}

// DropDemo removes the sample tables from a branch. The guardrail refuses DROP
// TABLE by default, so this goes through the same override an admin would use —
// it is recorded in the Blackbox either way.
func DropDemo(name string) error {
	if err := checkName(name); err != nil {
		return err
	}
	if err := SQL(name, "SET bb.allow_destructive = 'on'; DROP TABLE IF EXISTS events, projects, users;"); err != nil {
		return fmt.Errorf("removing the sample data from %q: %w", name, err)
	}
	if name == "main" {
		_ = os.Remove(demoMarker())
	}
	return nil
}
