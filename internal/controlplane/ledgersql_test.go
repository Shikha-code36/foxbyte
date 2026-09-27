// SPDX-License-Identifier: AGPL-3.0-or-later

package controlplane

import (
	"net/url"
	"strings"
	"testing"
)

// The default columns are a contract: the SDKs and the console were built
// against them, and the contract tests assert them exactly. Extra columns are
// opt-in, so a client that asks for nothing gets what it always got.
func TestLedgerSQLDefaultColumns(t *testing.T) {
	sql := ledgerSQL(url.Values{})
	for _, unwanted := range []string{"session", "prev_hash", "row_hash"} {
		if strings.Contains(sql, unwanted) {
			t.Errorf("the default query selects %q, which clients were not built for:\n%s", unwanted, sql)
		}
	}
	for _, want := range []string{"at", "actor", "actor_kind", "tool", "branch", "command_tag", "object_identity", "statement", "status", "risk"} {
		if !strings.Contains(sql, want) {
			t.Errorf("the default query lost %q:\n%s", want, sql)
		}
	}
}

// with=chain is what lets the console show that an entry is linked to the one
// before it, and lets it offer to branch from before an entry — which needs the
// id the default response never carried.
func TestLedgerSQLWithChain(t *testing.T) {
	sql := ledgerSQL(url.Values{"with": {"chain"}})
	for _, want := range []string{", id", "prev_hash", "row_hash"} {
		if !strings.Contains(sql, want) {
			t.Errorf("with=chain did not add %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "session") {
		t.Errorf("with=chain should not add session:\n%s", sql)
	}
}

// Both together, in one request, which is what the Blackbox page asks for.
func TestLedgerSQLWithSessionAndChain(t *testing.T) {
	sql := ledgerSQL(url.Values{"with": {"session,chain"}})
	for _, want := range []string{"session", "id", "prev_hash", "row_hash"} {
		if !strings.Contains(sql, want) {
			t.Errorf("with=session,chain did not add %q:\n%s", want, sql)
		}
	}
	// Spacing around the names is how the columns reach the client; a malformed
	// list would fail at the database rather than here.
	if strings.Count(sql, "prev_hash") != 1 || strings.Count(sql, "session") != 1 {
		t.Errorf("a column was added twice:\n%s", sql)
	}
}

// An unknown value is ignored rather than reaching the database.
func TestLedgerSQLIgnoresUnknownWith(t *testing.T) {
	sql := ledgerSQL(url.Values{"with": {"session, chain , nonsense"}})
	if strings.Contains(sql, "nonsense") {
		t.Errorf("an unknown `with` value reached the query:\n%s", sql)
	}
	// Whitespace around the names is accepted, as a hand-written URL will have it.
	for _, want := range []string{"session", "prev_hash"} {
		if !strings.Contains(sql, want) {
			t.Errorf("padding stopped %q being added:\n%s", want, sql)
		}
	}
}
