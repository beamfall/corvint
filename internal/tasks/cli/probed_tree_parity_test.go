package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// withProbedTree runs body with withStore decoding the probe's tree (on) or
// reading it a third time through LoadExpecting (off).
func withProbedTree(on bool, body func()) {
	old := reuseProbedTree
	reuseProbedTree = on
	defer func() { reuseProbedTree = old }()
	body()
}

func runBytes(cwd string, args ...string) []byte {
	var out bytes.Buffer
	Run(Env{Cwd: cwd, Args: args, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &out})
	return out.Bytes()
}

var probedTreeReads = [][]string{
	{"queue", "status"}, {"receipt", "audit"}, {"ticket", "list"}, {"ticket", "show", "A"},
	{"ticket", "show", "Z"}, {"ticket", "blockers", "B"}, {"ticket", "search", "--text", "A"},
	{"ticket", "export"}, {"roadmap"}, {"critical-path", "B"}, {"gate", "list"}, {"policy", "show"},
	{"plan", "preview"},
}

// TM-V0-008 / CAL-V0-061: decoding the tree the first probe hashed gives
// byte-identical envelopes to the third LoadExpecting pass it replaces, on a
// clean store and on stores whose intent tree or journal is refused.
func TestTMV0008_ProbedTreeReadParity(t *testing.T) {
	for _, kind := range []string{"clean", "diverged-ticket", "malformed-ticket", "unexpected-entry", "queue-mismatch", "receipt-pending"} {
		t.Run(kind, func(t *testing.T) {
			r := fixture.TempRepo(t)
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r)
			b := fixture.Ticket("B")
			b.Dependencies = []ticket.Dependency{fixture.Dep("A")}
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
			fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/B.json": b.Encode()})
			switch kind {
			case "diverged-ticket":
				a := fixture.Ticket("A")
				a.Title = "edited outside the journal"
				fixture.Write(t, filepath.Join(r.IntentDir, intent.TicketsDir, "A.json"), a.Encode())
			case "malformed-ticket":
				fixture.Write(t, filepath.Join(r.IntentDir, intent.TicketsDir, "A.json"), []byte("{}\n"))
			case "unexpected-entry":
				fixture.Write(t, filepath.Join(r.IntentDir, "stray.json"), []byte("{}\n"))
			case "queue-mismatch":
				q := fixture.QueueValue()
				q.Obj.Set("queueId", wire.String("01J00000000000000000000000"))
				fixture.Write(t, filepath.Join(r.IntentDir, intent.QueueFile), wire.EncodeFile(q))
			case "receipt-pending":
				fixture.PlantReceipt(t, r, 4)
			}
			for _, args := range probedTreeReads {
				var on, off []byte
				withProbedTree(false, func() { off = runBytes(r.Root, args...) })
				withProbedTree(true, func() { on = runBytes(r.Root, args...) })
				if res, err := wire.DecodeResult(on); err == nil {
					t.Logf("%v: %s %v", args, res.Outcome, res.Codes)
				}
				if !bytes.Equal(on, off) {
					t.Fatalf("%v differs:\n on  %s\n off %s", args, on, off)
				}
				if kind == "clean" && args[0] == "queue" {
					if res, err := wire.DecodeResult(on); err != nil || res.Outcome != wire.OutcomeOK {
						t.Fatalf("clean %v: %v %s", args, err, on)
					}
				}
			}
		})
	}
}

// TM-V0-008: a commit between the body and the second probe still discards
// the attempt and re-reads, with the probed tree as with the third pass.
func TestTMV0008_ProbedTreeMovedSnapshotParity(t *testing.T) {
	for _, on := range []bool{false, true} {
		withProbedTree(on, func() {
			r := fixture.TempRepo(t)
			fixture.WriteState(t, r)
			fixture.WriteIntent(t, r)
			calls := 0
			res := ticketShow(Env{Cwd: r.Root, afterRead: func() {
				calls++
				if calls == 1 {
					fixture.CommitPosts(t, r, "MUTATION", "", map[string][]byte{"intent/tickets/A.json": fixture.Ticket("A").Encode()})
				}
			}}, []string{"A"}, true)
			if calls != 2 || res.Outcome != wire.OutcomeOK || len(res.Items) != 1 || len(res.Warnings) != 0 {
				t.Fatalf("probed=%v calls=%d result=%+v", on, calls, res)
			}
		})
	}
}
