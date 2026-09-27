// SPDX-License-Identifier: AGPL-3.0-or-later

package branch

import (
	"bytes"
	"strings"
	"testing"
)

// The connection string a person is shown must be the one that works: the
// gateway's host and port, the branch as the database name, and sslmode=require.
func TestGatewayDSN(t *testing.T) {
	got := GatewayDSN("dev", "")
	for _, want := range []string{"postgresql://", "dbadmin:", KeyPlaceholder, "localhost:6432", "/dev", "sslmode=require"} {
		if !strings.Contains(got, want) {
			t.Errorf("GatewayDSN(\"dev\", \"\") = %q, missing %q", got, want)
		}
	}
	// The placeholder is shown as a person would type it, not percent-escaped.
	if strings.Contains(got, "%3C") || strings.Contains(got, "%3E") {
		t.Errorf("the placeholder is escaped and unreadable: %q", got)
	}
	// A real key is escaped, because a key may hold characters a URL reserves.
	withKey := GatewayDSN("dev", "key_abc/def")
	if strings.Contains(withKey, "key_abc/def") {
		t.Errorf("a key with a '/' must be escaped in the DSN: %q", withKey)
	}
	if strings.Contains(withKey, KeyPlaceholder) {
		t.Errorf("a real key should replace the placeholder: %q", withKey)
	}
}

// FOX_GATEWAY_HOSTPORT moves the gateway; a printed DSN has to follow it, or the
// line we tell people to paste points at nothing.
func TestGatewayDSNFollowsTheGateway(t *testing.T) {
	t.Setenv(EnvGatewayHostPort, "db.example.com:7000")
	if got := GatewayDSN("main", ""); !strings.Contains(got, "db.example.com:7000") {
		t.Errorf("GatewayDSN ignored %s: %q", EnvGatewayHostPort, got)
	}
}

// The hint printed after `fox branch create` has to name the branch, how to get
// the password, and how to open a shell — the three things a new user asks next.
func TestPrintConnectHint(t *testing.T) {
	var b bytes.Buffer
	PrintConnectHint(&b, "feature-x")
	out := b.String()
	for _, want := range []string{"feature-x", "apikey create", "connect feature-x", "sslmode=require"} {
		if !strings.Contains(out, want) {
			t.Errorf("the connect hint is missing %q:\n%s", want, out)
		}
	}
}

// Connect refuses a name the engine would not use before it touches docker.
func TestConnectChecksTheName(t *testing.T) {
	if err := Connect("../../etc", false); err == nil {
		t.Error("Connect accepted a name that is not a branch")
	}
}
