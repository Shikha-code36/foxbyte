// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"strings"
	"testing"
	"time"
)

// `fox connect --dsn` and `fox connect dev` both have to work, so a flag must
// never be mistaken for a branch name.
func TestArgOrSkipsFlags(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{nil, "main"},
		{[]string{}, "main"},
		{[]string{"--dsn"}, "main"},
		{[]string{"dev"}, "dev"},
		{[]string{"--dsn", "dev"}, "dev"},
		{[]string{"dev", "--dsn"}, "dev"},
	}
	for _, c := range cases {
		if got := argOr(c.args, "main"); got != c.want {
			t.Errorf("argOr(%v) = %q, want %q", c.args, got, c.want)
		}
	}
}

// How long a branch took is a headline number on a first run, so it is printed at
// the precision a person reads rather than in nanoseconds.
func TestTookReadsLikeATime(t *testing.T) {
	got := took(time.Now().Add(-1900 * time.Millisecond))
	if !strings.HasPrefix(got, "1.9") {
		t.Errorf("took(1.9s ago) = %q, want tenths of a second", got)
	}
	if strings.Contains(got, "µ") || strings.Contains(got, "ns") {
		t.Errorf("took = %q, too precise to read", got)
	}
	if got := took(time.Now().Add(-90 * time.Second)); got != "1m30s" {
		t.Errorf("took(90s ago) = %q, want 1m30s", got)
	}
}
