// SPDX-License-Identifier: AGPL-3.0-or-later

package branch

import (
	"slices"
	"strings"
	"testing"
)

// A long-lived container must come back by itself — after a crash, and after the
// machine or the engine VM reboots. Nothing was supervised before this.
func TestStandbyCarriesTheRestartPolicy(t *testing.T) {
	args := standbyRunArgs("/tmp/standby-data")
	if !hasRestartPolicy(args) {
		t.Errorf("the standby would not be restarted after a crash:\n%s", strings.Join(args, " "))
	}
}

// "unless-stopped", never "always": `fox stop` and suspending a branch stop a
// container on purpose, and Docker must leave it stopped.
func TestRestartPolicyRespectsAStopOnPurpose(t *testing.T) {
	if got := strings.Join(restartPolicy, " "); got != "--restart unless-stopped" {
		t.Errorf("restartPolicy = %q; \"always\" would restart a container the user stopped", got)
	}
}

// The policy has to come before the image name, or docker reads it as an
// argument to the container's own command rather than a flag of its own.
func TestRestartPolicyComesBeforeTheImage(t *testing.T) {
	args := standbyRunArgs("/tmp/standby-data")
	restart := slices.Index(args, "--restart")
	if restart < 0 {
		t.Fatal("no --restart in the standby's arguments")
	}
	// The container's own command is the bare word "postgres"; the image is the
	// argument before it. Anything after that point is the container's, not
	// docker's. (Matching "postgres" loosely would find the volume path, which
	// ends in /var/lib/postgresql/data.)
	cmd := slices.Index(args, "postgres")
	if cmd < 1 {
		t.Fatal("no `postgres` command in the standby's arguments")
	}
	if restart > cmd-1 {
		t.Errorf("--restart (%d) is not before the image (%d), so docker would pass it to the container:\n%s",
			restart, cmd-1, strings.Join(args, " "))
	}
}

func hasRestartPolicy(args []string) bool {
	i := slices.Index(args, "--restart")
	return i >= 0 && i+1 < len(args) && args[i+1] == "unless-stopped"
}
