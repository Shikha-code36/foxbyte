// SPDX-License-Identifier: AGPL-3.0-or-later

package host

import (
	"strings"
	"testing"
)

// The console URL carries the token where the login page looks for it, and
// nowhere else.
func TestConsoleURL(t *testing.T) {
	got := ConsoleURL("", "")
	if got != "https://localhost:8080/login" {
		t.Errorf("ConsoleURL with no token = %q", got)
	}
	got = ConsoleURL("", "abc-123_XYZ")
	if got != "https://localhost:8080/login?setup=abc-123_XYZ" {
		t.Errorf("ConsoleURL with a token = %q", got)
	}
	// A token with characters that must survive a query string.
	if got := ConsoleURL("https://127.0.0.1:9443", "a+b/c=d"); !strings.Contains(got, "setup=a%2Bb%2Fc%3Dd") {
		t.Errorf("token not escaped: %q", got)
	}
	// A base that cannot be parsed is returned as it came, rather than becoming
	// some other URL.
	if got := ConsoleURL("://nonsense", "t"); got != "://nonsense" {
		t.Errorf("ConsoleURL on a bad base = %q", got)
	}
}

// Only the console's own http(s) URLs are ever handed to the platform opener: a
// browser will act on other schemes, and on Windows some of them run programs.
func TestSafeURL(t *testing.T) {
	for _, u := range []string{"https://localhost:8080/login", "http://127.0.0.1:8080/"} {
		if !safeURL(u) {
			t.Errorf("safeURL(%q) = false, want true", u)
		}
	}
	for _, u := range []string{
		"file:///etc/passwd",
		"javascript:alert(1)",
		"ms-msdt:/id",
		"https://",
		"",
		"not a url at all",
	} {
		if safeURL(u) {
			t.Errorf("safeURL(%q) = true, want false", u)
		}
	}
}

// `fox setup-token` prints a token on a line of its own, or a sentence when the
// install already has an account. Only the first is a token.
func TestLooksLikeSetupToken(t *testing.T) {
	if !looksLikeSetupToken("Ab3-_xYz9012345678901234") {
		t.Error("a real token was rejected")
	}
	if !looksLikeSetupToken("  Ab3-_xYz9012345678901234\n") {
		t.Error("surrounding whitespace should be trimmed")
	}
	for _, s := range []string{
		"",
		"short",
		"This install already has its first account, so there is no setup token.",
		"token with spaces in it aaaaaaaaaaaaa",
		"has.a.dot.which.is.not.base64url.aaaa",
	} {
		if looksLikeSetupToken(s) {
			t.Errorf("looksLikeSetupToken(%q) = true, want false", s)
		}
	}
}

// FOX_NO_BROWSER is the documented way to stop the console opening, so it must
// win over everything else.
func TestNoBrowserEnvWins(t *testing.T) {
	t.Setenv("FOX_NO_BROWSER", "1")
	if canOpenBrowser() {
		t.Error("FOX_NO_BROWSER=1 should stop the console opening")
	}
	t.Setenv("FOX_NO_BROWSER", "0")
	t.Setenv(envInGuest, "1")
	if canOpenBrowser() {
		t.Error("inside the engine VM there is no browser to open")
	}
}

// OpenURL must refuse a URL it would not open, without starting anything.
func TestOpenURLRefusesUnsafe(t *testing.T) {
	t.Setenv("FOX_NO_BROWSER", "")
	t.Setenv(envInGuest, "")
	if OpenURL("file:///etc/passwd") {
		t.Error("OpenURL opened a file:// URL")
	}
}
