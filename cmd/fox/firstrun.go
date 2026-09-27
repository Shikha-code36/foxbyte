// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/thefoxbyte/foxbyte/internal/branch"
	"github.com/thefoxbyte/foxbyte/internal/brand"
)

// The first ten minutes: sample data, and commands that tell a new user where
// they are. `fox connect` and the timing on `fox branch create` live in main.go's
// switch; the helpers they need are here.

// argOr returns the first positional argument, or def when there is none. Flags
// are skipped, so `fox connect --dsn` still means main.
func argOr(args []string, def string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return def
}

// took formats how long something took, at the precision a person reads: tenths
// of a second up to a minute, then whole seconds.
func took(start time.Time) string {
	d := time.Since(start)
	if d < time.Minute {
		return d.Round(100 * time.Millisecond).String()
	}
	return d.Round(time.Second).String()
}

// demoCmd is `fox demo`: the sample data, on demand. A first start seeds main
// (internal/branch/demo.go); this is how to put it on another branch, take it
// away, or read what it is before running it.
func demoCmd(args []string) {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "seed":
		name := argOr(args[1:], "main")
		must(branch.SeedDemo(name))
		fmt.Printf("Sample data in %q: users, projects, events (five rows each).\n", name)
		fmt.Printf("Every CREATE TABLE is in the Blackbox — see it with `%s blackbox %s`.\n", brand.CLI, name)
	case "drop":
		name := argOr(args[1:], "main")
		must(branch.DropDemo(name))
		fmt.Printf("Sample data removed from %q (recorded in the Blackbox).\n", name)
	case "sql":
		fmt.Print(branch.DemoSQL())
	default:
		fmt.Printf("usage: %[1]s demo seed [branch]   Put the sample tables on a branch\n"+
			"       %[1]s demo drop [branch]   Remove them\n"+
			"       %[1]s demo sql             Print the SQL without running it\n\n"+
			"A first start seeds main. Set %sNO_DEMO=1 to start empty.\n", brand.CLI, brand.EnvPrefix)
		os.Exit(2)
	}
}
