//go:build darwin || linux

package dispatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// jitterRecorded is an identity another process recorded; jitterCases are
// the three cross-process reads every site class checks (PSR-V0-227): btime
// one second off, another start tick, and btime two seconds off.
const jitterRecorded = "linux:1000:77"

var jitterCases = []struct {
	name, fresh string
	want        supervisor.IdentityMatch
}{
	{"jitter tolerated", "linux:1001:77", supervisor.IdentitySame},
	{"start ticks mismatch", "linux:1001:78", supervisor.IdentityDifferent},
	{"more than 1 s", "linux:1002:77", supervisor.IdentityUnknown},
}

func fakeProcessIdentity(t *testing.T, fresh string) {
	t.Helper()
	prevTree, prevReader, prevDetached := processIdentity, readerProcessIdentity, detachedProcessIdentity
	t.Cleanup(func() {
		processIdentity, readerProcessIdentity, detachedProcessIdentity = prevTree, prevReader, prevDetached
	})
	read := func(int) (string, error) { return fresh, nil }
	processIdentity, readerProcessIdentity, detachedProcessIdentity = read, read, read
}

// TestPSRV0227_WorkerTreeToleratesJitter: tree membership, leader liveness,
// detached-supervisor exemption and the may-run check accept a recorded
// identity re-read with btime one second off, keeping the recorded value;
// another start tick is another process; a btime further off keeps the
// recorded tree with an uncertainty error, and the tree may still run.
func TestPSRV0227_WorkerTreeToleratesJitter(t *testing.T) {
	const leader, sup = 4000001, 4000002
	for _, c := range jitterCases {
		t.Run(c.name, func(t *testing.T) {
			fakeProcessIdentity(t, c.fresh)
			same := c.want == supervisor.IdentitySame
			procs := map[int]proc{leader: {pid: leader, ppid: 1, pgid: leader}, sup: {pid: sup, ppid: 1, pgid: sup}}
			w := &Worker{PID: leader, LeaderIdentity: jitterRecorded, Members: []Proc{{PID: leader, Identity: jitterRecorded}}}
			err := refreshTree(w, procs, nil)
			if unknown := c.want == supervisor.IdentityUnknown; (err != nil) != unknown {
				t.Fatalf("refreshTree error = %v, want uncertainty %v", err, unknown)
			}
			kept := len(w.Members) == 1 && w.Members[0] == (Proc{PID: leader, Identity: jitterRecorded})
			if want := c.want != supervisor.IdentityDifferent; kept != want {
				t.Fatalf("members %+v, want the recorded member kept %v", w.Members, want)
			}
			// leaderAlive reads the refreshed record: kept with the
			// uncertainty error, the leader stays recorded as alive.
			if got := leaderAlive(w); got != kept {
				t.Fatalf("leaderAlive = %v", got)
			}
			ex, err := newExemption(procs, map[int]string{sup: jitterRecorded}, leader)
			if err != nil {
				t.Fatal(err)
			}
			if got := ex.sups[sup]; got != same {
				t.Fatalf("exempt supervisor = %v", got)
			}
			if got := treeMayRun(leader, jitterRecorded, nil, nil); got != (c.want != supervisor.IdentityDifferent) {
				t.Fatalf("treeMayRun = %v", got)
			}
		})
	}
	// A leader PID whose member identity matches but whose PID differs is
	// not the leader.
	if leaderAlive(&Worker{PID: leader, LeaderIdentity: jitterRecorded, Members: []Proc{{PID: sup, Identity: jitterRecorded}}}) {
		t.Fatal("leaderAlive matched another pid")
	}
}

// TestPSRV0227_RecordedStatesTolerateJitter: a detached run's supervisor and
// the program lock holder read with btime one second off are RUNNING;
// another start tick is lost or not running; a btime further off is
// UNKNOWN, never lost.
func TestPSRV0227_RecordedStatesTolerateJitter(t *testing.T) {
	for _, c := range jitterCases {
		t.Run(c.name, func(t *testing.T) {
			fakeProcessIdentity(t, c.fresh)
			run := &DetachedRun{RunID: "r1", AttemptID: "a1", Generation: "1", State: RunRunning, SupervisorPID: 4000003, SupervisorIdentity: jitterRecorded}
			v := DetachedView{RunID: "r1", AttemptID: "a1", Generation: "1", SupervisorPID: 4000003, SupervisorIdentity: jitterRecorded}
			want := map[supervisor.IdentityMatch]string{supervisor.IdentitySame: RunRunning, supervisor.IdentityDifferent: "SUPERVISOR_LOST", supervisor.IdentityUnknown: StateUnknown}[c.want]
			if got := RunState(v, run, nil); got != want {
				t.Fatalf("RunState = %s, want %s", got, want)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "lock"), []byte("4000003 "+jitterRecorded+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			wantOwner := map[supervisor.IdentityMatch]string{supervisor.IdentitySame: "RUNNING", supervisor.IdentityDifferent: "NOT_RUNNING", supervisor.IdentityUnknown: "UNKNOWN"}[c.want]
			if got, _ := OwnerProcess(dir); got != wantOwner {
				t.Fatalf("OwnerProcess = %s, want %s", got, wantOwner)
			}
		})
	}
}

// TestPSRV0227_ReaderLeaderToleratesJitter: a recorded reader leader read
// with btime one second off still runs and keeps the quarantine; another
// start tick lets recovery proceed; a btime further off keeps it as
// uncertain.
func TestPSRV0227_ReaderLeaderToleratesJitter(t *testing.T) {
	fakeHost(t, digestHex("m"), digestHex("b1"))
	_, dir, mark := quarantinedProgram(t, "jitter")
	if err := publishReaderLeader(dir, "jitter", mark.marker.Run, 4242, jitterRecorded); err != nil {
		t.Fatal(err)
	}
	for _, c := range jitterCases {
		t.Run(c.name, func(t *testing.T) {
			fakeProcessIdentity(t, c.fresh)
			err := readerLeaderLive(dir, "jitter")
			if c.want == supervisor.IdentityDifferent {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if wire.CodeOf(err) != wire.CodeQuiescenceUnproved {
				t.Fatalf("readerLeaderLive = %v, want QUIESCENCE_UNPROVED", err)
			}
		})
	}
}
