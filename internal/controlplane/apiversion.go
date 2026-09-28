// SPDX-License-Identifier: AGPL-3.0-or-later

package controlplane

import (
	"net/http"
	"strings"
)

// A versioned path for the API (audit v2 G35).
//
// Every route was served at /api/… with no version in it, so the first change
// that could not be made backwards-compatibly would have had nowhere to go: the
// SDKs, the MCP server and other people's code are all written against those
// paths. /api/v1/… now reaches exactly the same handlers, and /api/… keeps
// working — it is what is published and what every client in the wild uses, so it
// stays, and nothing has to be rewritten to get a version.
//
// This is a prefix rewrite rather than a second set of routes, so there is one
// implementation of each endpoint and no chance of the two drifting. When a v2
// arrives, it will need real routes of its own; until then, saying "v1" is a
// promise about what these paths mean, not a second copy of them.

// APIVersion is the version /api/v1 serves.
const APIVersion = "v1"

const versionedPrefix = "/api/" + APIVersion + "/"

// versionAlias maps /api/v1/… onto /api/… before anything else looks at the path,
// so authorization, the Blackbox alias and the routes themselves see one shape.
func versionAlias(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, versioned := strings.CutPrefix(r.URL.Path, versionedPrefix)
		if !versioned {
			// Exactly /api/v1, with no trailing slash: answer where the API is rather
			// than 404, since it is the first thing someone tries by hand.
			if r.URL.Path == "/api/"+APIVersion {
				http.Redirect(w, r, "/api/"+APIVersion+"/status", http.StatusTemporaryRedirect)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/api/" + rest
		// Both forms, separately. Path is the decoded one and RawPath the original
		// spelling; building one from the other escapes a branch name twice, so a
		// name with a %2F in it arrived as %252F and matched nothing. The prefix
		// being removed has nothing escapable in it, so it comes off either form.
		if r.URL.RawPath != "" {
			if raw, ok := strings.CutPrefix(r.URL.RawPath, versionedPrefix); ok {
				r2.URL.RawPath = "/api/" + raw
			} else {
				r2.URL.RawPath = ""
			}
		}
		next.ServeHTTP(w, r2)
	})
}
