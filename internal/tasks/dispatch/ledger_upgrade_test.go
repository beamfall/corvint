//go:build darwin || linux

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/supervisor"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// version2Ledger launches n real workers under this build, closes the
// dispatcher, and rewrites its ledger as the version 2 ledger the build
// before the CAL-V0-185 stall counts would have left: the same members less
// stall. It returns the state path and the recorded workers.
func version2Ledger(t *testing.T, n int, script string) (*Config, *fakeQueue, string, []Worker) {
	t.Helper()
	c := testConfig(t, script)
	c.GlobalCap = n
	q := &fakeQueue{}
	for i := 1; i <= n; i++ {
		q.obs.Tickets = append(q.obs.Tickets, ticket("t"+string(rune('0'+i)), "P1", uint64(i)))
	}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.Workers) != n {
		t.Fatalf("launched %d workers, want %d", len(d.ledger.Workers), n)
	}
	var workers []Worker
	for _, w := range d.ledger.Workers {
		workers = append(workers, *w)
		pid := w.PID
		t.Cleanup(func() { syscall.Kill(-pid, syscall.SIGKILL) })
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(ProgramDir(c, "prog"), "state.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		t.Fatal(err)
	}
	delete(members, "stall")
	members["profile"] = json.RawMessage(`"` + drainedState2Profile + `"`)
	if raw, err = json.Marshal(members); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return c, q, path, workers
}

// stopTree ends a worker's whole process group and waits for its leader.
func stopTree(t *testing.T, w Worker) {
	t.Helper()
	_ = syscall.Kill(-w.PID, syscall.SIGKILL)
	for _, m := range w.Members {
		if !gone(m.PID) {
			t.Fatalf("worker %s process %d survived SIGKILL", w.ID, m.PID)
		}
	}
	if !gone(w.PID) {
		t.Fatalf("worker %s leader survived SIGKILL", w.ID)
	}
}

// TestCALV0186_Version2LedgerWithGoneWorkersMigrates: a version 2 ledger
// whose every recorded worker is gone is adopted as version 3 with the
// workers kept as recorded, and the first tick reaps them as it reaps any
// adopted worker (adopted, then finished) and saves version 3 with none.
func TestCALV0186_Version2LedgerWithGoneWorkersMigrates(t *testing.T) {
	c, q, path, workers := version2Ledger(t, 2, `sleep 300`)
	for _, w := range workers {
		stopTree(t, w)
	}
	l, err := LoadLedger(ProgramDir(c, "prog"), "prog")
	if err != nil || l.Profile != StateProfile || len(l.Workers) != 2 || len(l.Workers[0].Members) == 0 {
		t.Fatalf("LoadLedger %+v %v", l, err)
	}
	c.Backoff.CooldownSeconds = 3600 // nothing relaunches after the reap
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	finished := 0
	for _, e := range eventsOf(t, d, "finished") {
		if e.Worker == workers[0].ID || e.Worker == workers[1].ID {
			finished++
		}
	}
	if got := kinds(t, d); !has(got, "adopted") || finished != 2 || len(d.ledger.Workers) != 0 {
		t.Fatalf("reap: events %v finished %d workers %d", got, finished, len(d.ledger.Workers))
	}
	raw, err := os.ReadFile(path)
	var saved struct {
		Profile string    `json:"profile"`
		Workers []*Worker `json:"workers"`
	}
	if err != nil || json.Unmarshal(raw, &saved) != nil || saved.Profile != StateProfile || len(saved.Workers) != 0 {
		t.Fatalf("saved ledger is not version 3 without the reaped workers: %v %+v", err, saved)
	}
}

// TestCALV0186_Version2LedgerWithLiveOrUnprovenWorkerRefuses: a version 2
// ledger with one worker still running, one whose record lacks a process
// identity, or one whose identity cannot be read refuses
// UNSUPPORTED_VERSION, unchanged and without touching the live worker; the
// refusal names the worker, the build that wrote the ledger and the
// clearing command. A version 1 ledger with a worker names them too.
func TestCALV0186_Version2LedgerWithLiveOrUnprovenWorkerRefuses(t *testing.T) {
	c, q, path, workers := version2Ledger(t, 2, `sleep 300`)
	command := "`corvint-tasks dispatch --program prog --config CONFIG --once`"
	refuses := func(name string, raw []byte, profile, want string) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
		_, loadErr := LoadLedger(ProgramDir(c, "prog"), "prog")
		_, openErr := Open("prog", c, q, io.Discard)
		for _, err := range []error{loadErr, openErr} {
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			if wire.CodeOf(err) != wire.CodeUnsupportedVersion || !strings.Contains(msg, "a "+profile+" ledger "+want) ||
				!strings.Contains(msg, "a corvint-tasks build whose `version` formats list "+profile) ||
				!strings.Contains(msg, command) || !strings.Contains(msg, "cap to 0") {
				t.Errorf("%s: %v", name, err)
			}
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, raw) {
			t.Errorf("%s: refused ledger rewritten", name)
		}
	}
	stopTree(t, workers[0])
	live, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	refuses("one live worker", live, drainedState2Profile, `records worker "`+workers[1].ID+`" (pid `+strconv.Itoa(workers[1].PID)+`) that is still running`)
	if id, _ := identityOf(workers[1].PID); id != workers[1].LeaderIdentity {
		t.Fatal("the refusal touched the live worker")
	}
	stopTree(t, workers[1])
	noIdentity := bytes.Replace(live, []byte(`"leaderIdentity":"`+workers[0].LeaderIdentity+`"`), []byte(`"leaderIdentity":""`), 1)
	version1 := []byte(`{"profile":"` + drainedStateProfile + `","program":"prog","launchSeq":1,"eventSeq":1,"workers":[{"id":"w1"}],"backoff":{}}`)
	for _, b := range [][]byte{noIdentity, version1} {
		if bytes.Equal(b, live) {
			t.Fatal("fixture not reached")
		}
	}
	refuses("no identity", noIdentity, drainedState2Profile, `records worker "`+workers[0].ID+`" (pid `+strconv.Itoa(workers[0].PID)+`) whose end cannot be proven: the record lacks a process identity`)
	refuses("version 1 worker", version1, drainedStateProfile, `records a worker`)
	processIdentity = func(int) (string, error) { return "", errors.New("injected identity failure") }
	refuses("unreadable identity", live, drainedState2Profile, `records worker "`+workers[0].ID+`" (pid `+strconv.Itoa(workers[0].PID)+`) whose end cannot be proven: injected identity failure`)
	processIdentity = supervisor.ProcessIdentity
	// The same ledger with every worker now gone is the control: it adopts.
	if err := os.WriteFile(path, live, 0o600); err != nil {
		t.Fatal(err)
	}
	if l, err := LoadLedger(ProgramDir(c, "prog"), "prog"); err != nil || len(l.Workers) != 2 {
		t.Fatalf("control: %v", err)
	}
}

// TestCALV0186_Version2LedgerWithOrphanedProcessRefuses: a worker whose
// leader has exited is not proven gone while a process it left in its
// group or session still runs; the old build would stop that process
// first (ORPHANED), so the ledger refuses.
func TestCALV0186_Version2LedgerWithOrphanedProcessRefuses(t *testing.T) {
	c, _, _, workers := version2Ledger(t, 1, `sleep 300 & echo "$!" > "$CORVINT_DISPATCH_WORKER.child"; wait`)
	w := workers[0]
	var child int
	for i := 0; i < 100 && child == 0; i++ {
		raw, _ := os.ReadFile(filepath.Join(c.WorkRoot, w.ID+".child"))
		child = atoi(strings.TrimSpace(string(raw)))
		time.Sleep(20 * time.Millisecond)
	}
	if child == 0 {
		t.Fatal("child pid not written")
	}
	t.Cleanup(func() { syscall.Kill(child, syscall.SIGKILL) })
	_ = syscall.Kill(w.PID, syscall.SIGKILL)
	if !gone(w.PID) {
		t.Fatal("leader survived SIGKILL")
	}
	_, err := LoadLedger(ProgramDir(c, "prog"), "prog")
	if wire.CodeOf(err) != wire.CodeUnsupportedVersion || !strings.Contains(err.Error(), `records worker "`+w.ID+`" (pid `+strconv.Itoa(w.PID)+`) that is still running`) {
		t.Fatalf("orphaned tree: %v", err)
	}
}
