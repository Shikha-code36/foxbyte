// SPDX-License-Identifier: AGPL-3.0-or-later

package controlplane

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// /api/v1/… has to reach the same handler as /api/…, and the handler must see the
// unversioned path: every route, the authorization middleware and the Blackbox
// alias are written against that shape.
func TestVersionAliasRewritesToTheUnversionedPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/api/v1/status", "/api/status"},
		{"/api/v1/branches", "/api/branches"},
		{"/api/v1/branches/dev/ledger/verify", "/api/branches/dev/ledger/verify"},
		{"/api/v1/requests/3/approve", "/api/requests/3/approve"},
		// Not versioned: left exactly as it came.
		{"/api/status", "/api/status"},
		{"/api/branches/v1/ledger", "/api/branches/v1/ledger"},
		// A branch that happens to be called v1 is not a version.
		{"/api/branches/v1", "/api/branches/v1"},
	}
	for _, c := range cases {
		var seen string
		h := versionAlias(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.URL.Path }))
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", c.in, nil))
		if seen != c.want {
			t.Errorf("%s reached the handler as %q, want %q", c.in, seen, c.want)
		}
	}
}

// A branch name with something that needs escaping must survive the rewrite: the
// name is read back out of the path by the routes.
func TestVersionAliasKeepsAnEscapedName(t *testing.T) {
	var seen string
	h := versionAlias(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.URL.EscapedPath() }))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/branches/a%2Fb/ledger", nil))
	if seen != "/api/branches/a%2Fb/ledger" {
		t.Errorf("the escaped path became %q", seen)
	}
}

// /api/v1 on its own is what someone types by hand; it should say where to go
// rather than 404.
func TestBareVersionRedirects(t *testing.T) {
	w := httptest.NewRecorder()
	versionAlias(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the bare version should not reach the routes")
	})).ServeHTTP(w, httptest.NewRequest("GET", "/api/v1", nil))
	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("GET /api/v1 = %d, want a redirect", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/api/v1/status" {
		t.Errorf("redirected to %q", got)
	}
}
