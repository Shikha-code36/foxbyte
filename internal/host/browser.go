// SPDX-License-Identifier: AGPL-3.0-or-later

package host

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/thefoxbyte/foxbyte/internal/brand"
)

// Opening the console for the user.
//
// `fox start` printed a URL and left it there, so the first thing a new install
// asked of a person was to copy a link and then a setup token. Nothing here is
// clever: it runs the platform's own "open this" command, and it declines in
// every situation where a browser would be the wrong answer — inside the engine
// VM (no display), over SSH, when there is no terminal, and whenever
// FOX_NO_BROWSER is set.

// EnvNoBrowser switches off opening the console.
const EnvNoBrowser = "NO_BROWSER"

// OpenURL opens u in the user's browser, best effort. It reports whether it
// tried: a caller that prints "opening the console…" should only say so if this
// returns true.
func OpenURL(u string) bool {
	if !canOpenBrowser() || !safeURL(u) {
		return false
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u)
	case "windows":
		// rundll32 takes the URL as one argument and does not go through a shell,
		// so nothing in the string can become a command.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	// The browser outlives fox; its output is not ours to print.
	cmd.Stdout, cmd.Stderr = nil, nil
	return cmd.Start() == nil
}

// canOpenBrowser reports whether a browser is the right thing to reach for here.
func canOpenBrowser() bool {
	if truthy(brand.Getenv(EnvNoBrowser)) {
		return false
	}
	// Inside the engine VM there is no display, and the user's browser is on the
	// host — which opens the console itself after the forwarded start returns.
	if os.Getenv(envInGuest) != "" {
		return false
	}
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	// A headless Linux box has no session to open anything in. macOS and Windows
	// always do.
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	// Not a terminal: fox is being scripted, or its output is going to a log.
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// safeURL allows only the http(s) URLs this package builds. A browser will run
// some other schemes (file:, and worse on Windows), and the only URLs worth
// opening here are the console's.
func safeURL(u string) bool {
	p, err := url.Parse(u)
	if err != nil || (p.Scheme != "https" && p.Scheme != "http") || p.Host == "" {
		return false
	}
	return true
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no":
		return false
	}
	return true
}

// ConsoleURL is where the web console lives, with the one-time setup token in the
// query string when there is one. The login page reads it and fills the field in,
// so a first run is one click and a password rather than a copied token.
func ConsoleURL(base, setupToken string) string {
	if base == "" {
		base = "https://localhost:8080"
	}
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	u.Path = "/login"
	if setupToken != "" {
		u.RawQuery = url.Values{"setup": {setupToken}}.Encode()
	}
	return u.String()
}

// EnvConsoleURL overrides where the console is, for an install whose ports are
// not the defaults.
const EnvConsoleURL = "CONSOLE_URL"

func consoleBase() string {
	if v := strings.TrimSpace(brand.Getenv(EnvConsoleURL)); v != "" {
		return v
	}
	return "https://localhost:8080"
}

// setupTokenShape is what `fox setup-token` prints when there is one: 18 random
// bytes, base64url, on a line of its own. Anything else — the sentence it prints
// when an account already exists, or an error — is not a token.
var setupTokenShape = regexp.MustCompile(`^[A-Za-z0-9_-]{20,64}$`)

func looksLikeSetupToken(s string) bool {
	s = strings.TrimSpace(s)
	return !strings.ContainsAny(s, " \t\n") && setupTokenShape.MatchString(s)
}

// openConsoleAfterStart opens the console once the engine has started inside the
// VM. The token has to come from the engine: the state directory holding it is in
// the VM, not on this machine. Nothing here is fatal — a browser that will not
// open leaves the URL printed, exactly as before.
func openConsoleAfterStart() {
	if !canOpenBrowser() {
		return
	}
	tok, err := forwardCapture([]string{"setup-token"})
	if err != nil || !looksLikeSetupToken(tok) {
		tok = "" // an account already exists, or the engine could not say
	}
	u := ConsoleURL(consoleBase(), tok)
	if !OpenURL(u) {
		return
	}
	if tok != "" {
		fmt.Println("\nOpening the console, with the setup token filled in — choose an email and a password there.")
		return
	}
	fmt.Printf("\nOpening the console: %s\n", u)
}
