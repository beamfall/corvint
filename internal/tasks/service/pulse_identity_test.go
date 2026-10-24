package service

import (
	"os"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestPSRV0227_PulseIdentityToleratesJitter (V1-1091): the service pulse
// identity was written by the running service; a read one second off in
// btime is the same process and keeps the pulse's state, while another
// start tick or a btime further off is STALE, as before.
func TestPSRV0227_PulseIdentityToleratesJitter(t *testing.T) {
	s := newServiceHome(t)
	root := s.root(t)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	raw, err := EncodePulse(Pulse{Program: "site", ManifestSha256: wire.Sum([]byte("manifest")), Identity: "linux:1000:77", State: "RUNNING", PID: 4000001, At: s.h.now().Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(root, pulseFile, raw); err != nil {
		t.Fatal(err)
	}
	prev := pulseProcessIdentity
	t.Cleanup(func() { pulseProcessIdentity = prev })
	for _, c := range []struct{ name, fresh, want string }{
		{"exact", "linux:1000:77", "RUNNING"},
		{"jitter tolerated", "linux:1001:77", "RUNNING"},
		{"start ticks mismatch", "linux:1001:78", "STALE"},
		{"more than 1 s", "linux:1002:77", "STALE"},
		{"gone", "", "STALE"},
	} {
		pulseProcessIdentity = func(int) (string, error) { return c.fresh, nil }
		if got, _ := s.h.pulseState(root, ""); got != c.want {
			t.Fatalf("%s: pulse state %s, want %s", c.name, got, c.want)
		}
	}
}
