//go:build linux

package authority

import (
	"strings"
	"testing"
)

func TestTMV0010_AS10_FixtureLinuxMountObservation(t *testing.T) {
	for _, raw := range []string{"", "mnt_id:\n", "mnt_id: 0\n", "mnt_id: x\n", "mnt_id: 1\nmnt_id: 1\n", "mnt_id: 1 extra\n", strings.Repeat("x", 4097)} {
		if _, err := fixtureLinuxMountID([]byte(raw)); err == nil {
			t.Fatalf("ambiguous mount %q accepted", raw)
		}
	}
	got, err := fixtureLinuxMountID([]byte("pos:\t0\nflags:\t0100000\nmnt_id:\t27\nino:\t42\n"))
	if err != nil || got != "27" {
		t.Fatalf("mount id %q %v", got, err)
	}
}
