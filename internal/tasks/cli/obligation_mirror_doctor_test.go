//go:build darwin || linux

package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/dispatch"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestTOLV0124_UnblockObservedAndDoctorReportsParked: an item leaving
// BLOCKED raises the observation's unblock count, which releases a parked
// key (TOL-V0-124); the doctor reports a parked key whose ticket still has
// open obligation items as PARKED_WITH_OPEN_ITEMS (TOL-V0-125).
func TestTOLV0124_UnblockObservedAndDoctorReportsParked(t *testing.T) {
	r, id, _ := obligationRepo(t)
	seedObligations(t, r.Root, id)
	unblocks := func() int64 {
		t.Helper()
		tickets, err := cli.ObserveTickets(r.Root)
		if err != nil {
			t.Fatal(err)
		}
		for _, tk := range tickets {
			if tk.ID == id && tk.Obligations != nil {
				return tk.Obligations.Unblocks
			}
		}
		t.Fatalf("ticket %s not observed with its ledger", id)
		return 0
	}
	if x := mirrorSet(t, r.Root, id, "block", `[{"id":"AC-1","state":"BLOCKED","core":null,"reason":"waits on fixture"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("block: %s", x.stdout)
	}
	if n := unblocks(); n != 0 {
		t.Fatalf("unblocks while blocked = %d", n)
	}
	if x := mirrorSet(t, r.Root, id, "unblock", `[{"id":"AC-1","state":"OPEN","core":null,"reason":"fixture landed"}]`); x.res.Outcome != wire.OutcomeOK {
		t.Fatalf("unblock: %s", x.stdout)
	}
	if n := unblocks(); n != 1 {
		t.Fatalf("unblocks after leaving BLOCKED = %d", n)
	}

	prog := filepath.Join(t.TempDir(), "prog")
	if err := os.MkdirAll(prog, 0o700); err != nil {
		t.Fatal(err)
	}
	l := dispatch.Ledger{Profile: dispatch.StateProfile, Program: "prog", Workers: []*dispatch.Worker{},
		Backoff: map[string]*dispatch.BackoffState{id: {Parked: true, Fingerprint: "f"}, "ticket:other:q:x": {Parked: true, Fingerprint: "g"}}}
	raw, err := json.Marshal(&l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(prog, "state.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := writeDoctorConfig(t, `{"profile":"taskman-doctor-config/0","dispatchers":[{"programDir":"`+prog+`"}]}`)
	_, item := doctorOK(t, r.Root, "--config", cfg)
	got := findingsOf(item, "PARKED_WITH_OPEN_ITEMS")
	if len(got) != 1 || field(got[0], "who").Str != id || !strings.Contains(field(got[0], "detail").Str, "9 open obligation item") {
		t.Fatalf("parked finding: %s", wire.Encode(item))
	}
}
