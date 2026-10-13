package cli_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestCALV0222_HeartbeatSettlesHeldPendingReceipt reproduces issue 860: a
// receipt is linked in and its posts published, but head.json is never
// renamed to name it (a writer killed at §5.2 crash point C2), and no other
// writer runs. Reads keep reporting REDO_PENDING. Each lease command used to
// refuse the same REDO_PENDING at its pre-write probe, so consecutive
// heartbeats all failed while nothing settled the receipt. Now the first
// heartbeat settles the pending receipt under the store lock and records its
// own signal, round after round.
func TestCALV0222_HeartbeatSettlesHeldPendingReceipt(t *testing.T) {
	root, repo, _, attempt, generation := lockWaitStore(t)
	headPath := filepath.Join(repo.StateDir, "head.json")
	headSeq := func() int64 {
		t.Helper()
		x := atm(t, root, nil, "receipt", "audit")
		if x.res.Outcome != wire.OutcomeOK {
			t.Fatalf("receipt audit: %s", x.stdout)
		}
		n, err := wire.ParseCount("headSeq", field(x.res.Items[0], "headSeq").Str)
		if err != nil {
			t.Fatal(err)
		}
		return n.Int()
	}
	lease := func(verb, request string) run {
		args := []string{"attempt", "heartbeat"}
		if verb == "renew" {
			args = []string{"renew", "--lease-minutes", "30"}
		}
		return atm(t, root, nil, append(args, "--attempt", attempt, "--generation", generation, "--request-id", request)...)
	}
	// The pending receipt is a heartbeat, then a renew: any lease receipt.
	for i, verb := range []string{"attempt heartbeat", "renew", "attempt heartbeat"} {
		held, err := os.ReadFile(headPath)
		if err != nil {
			t.Fatal(err)
		}
		seq := headSeq()
		if x := lease(verb, fmt.Sprintf("pending-%d", i)); x.code != 0 {
			t.Fatalf("round %d %s: %s", i, verb, x.stdout)
		}
		fixture.Write(t, headPath, held)
		if _, err := cli.ObserveTickets(root); wire.CodeOf(err) != wire.CodeRedoPending {
			t.Fatalf("round %d: the rewound head does not hold a pending receipt: %v", i, err)
		}
		// A second read still sees it: nothing settles a pending receipt on read.
		if _, err := cli.ObserveTickets(root); wire.CodeOf(err) != wire.CodeRedoPending {
			t.Fatalf("round %d: a read settled the pending receipt: %v", i, err)
		}
		x := lease("attempt heartbeat", fmt.Sprintf("beat-%d", i))
		if x.code != 0 || x.res.Outcome != wire.OutcomeOK || hasCode(x.res, wire.CodeRedoPending) {
			t.Fatalf("round %d: heartbeat over a held pending receipt: %s", i, x.stdout)
		}
		if !bytes.Contains(x.stdout, []byte("a receipt left pending by an interrupted run was completed")) {
			t.Fatalf("round %d: the heartbeat does not report the redo: %s", i, x.stdout)
		}
		if field(x.res.Items[0], "replayed").Kind != wire.KindBool || field(x.res.Items[0], "replayed").Bool {
			t.Fatalf("round %d: the heartbeat replayed instead of recording a signal: %s", i, x.stdout)
		}
		// The pending receipt and the heartbeat's own receipt both committed.
		if got := headSeq(); got != seq+2 {
			t.Fatalf("round %d: head seq %d, want %d", i, got, seq+2)
		}
		if _, err := cli.ObserveTickets(root); err != nil {
			t.Fatalf("round %d: read after the settling heartbeat: %v", i, err)
		}
	}
	show := atm(t, root, nil, "attempt", "show", attempt)
	if show.code != 0 || field(show.res.Items[0], "lastHeartbeatAt").Kind != wire.KindString || field(show.res.Items[0], "holderStatus").Str != "FRESH_HOLDER" {
		t.Fatalf("attempt after settling heartbeats: %s", show.stdout)
	}
}

// TestCALV0223_HeartbeatHelpNamesRedoAndLiveness: the verbose help of
// `attempt heartbeat` states that a pending receipt does not refuse it and
// that a refused heartbeat records nothing, so liveness is not extended.
func TestCALV0223_HeartbeatHelpNamesRedoAndLiveness(t *testing.T) {
	root, _, _, _, _ := lockWaitStore(t)
	x := atm(t, root, nil, "attempt", "heartbeat", "--help", "--verbose")
	note := field(x.res.Items[0], "note").Str
	for _, want := range []string{"REDO_PENDING", "settles", "records nothing", "heartbeatTTLSeconds"} {
		if !strings.Contains(note, want) {
			t.Fatalf("heartbeat note lacks %q: %s", want, note)
		}
	}
}
