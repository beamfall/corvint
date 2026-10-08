//go:build darwin || linux

package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func progressDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestCALV0064_CommandGrammarAndRoleSeparation(t *testing.T) {
	for _, raw := range []string{
		`{"t":"work"}`, `{ "t": { "progress":"commit-A", "state":"work" } }`,
		`{"t":{"state":"work","progress":"\ud83d\ude00"}}`,
	} {
		if _, err := decodeCommandStates([]byte(raw)); err != nil {
			t.Fatalf("compatible JSON %q: %v", raw, err)
		}
	}
	for _, raw := range []string{
		`{"t":"work","t":"review"}`, `{"t":{"state":"work","state":"review"}}`,
		`{"t":{"state":"work","progress":"a","progress":"b"}}`,
		`{"t":{"state":"work","extra":""}}`, `{"t":{"progress":"a"}}`,
		`{"t":null}`, `{"t":{"state":null}}`, `{"t":{"state":"work","progress":3}}`,
		`{"t":{"state":"work","progress":"\ud800"}}`,
		`{"t":{"state":"work","progress":"\udc00"}}`,
		`{"t":{"state":"work","progress":"\ud800\u1234"}}`,
		`{"t":{"state":"work","progress":"\n"}}`, `{} {}`, `[]`,
		`{"t":{"state":"work","progress":"` + strings.Repeat("x", 129) + `"}}`,
		string([]byte{'{', '"', 't', '"', ':', '"', 0xff, '"', '}'}),
	} {
		if _, err := decodeCommandStates([]byte(raw)); err == nil {
			t.Errorf("malformed JSON accepted: %q", raw)
		}
	}
	c := testConfig(t, "exit 0")
	source := filepath.Join(c.WorkRoot, "states.json")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
	write := func(raw string) {
		t.Helper()
		if err := os.WriteFile(source, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ts := []Ticket{ticket("t", "P1", 1)}
	write(`{"t":{"state":"work","progress":"review"}}`)
	if a := ReadStates(context.Background(), c, ts); len(a) != 0 {
		t.Fatal(a)
	}
	c.Roles[0].Match.States = []string{"review"}
	if got := Roster(c, &Observation{Tickets: ts}, nil, nil); len(got) != 0 {
		t.Fatal("progress token matched a role")
	}
	c.Roles[0].Match.States = []string{"work"}
	if got := Roster(c, &Observation{Tickets: ts}, nil, nil); len(got) != 1 {
		t.Fatal(got)
	}
	write(`{"t":{"state":"work","progress":"local"},"ticket:a:q:t":{"state":"","progress":"full"}}`)
	if a := ReadStates(context.Background(), c, ts); len(a) != 0 || ts[0].State != StateNone || ts[0].ProgressToken != "full" {
		t.Fatalf("full-ID precedence: %+v %v", ts, a)
	}
}

func progressDispatcher(t *testing.T) (*Dispatcher, *fakeQueue, string) {
	t.Helper()
	c := testConfig(t, "exit 0")
	c.Roles = nil // no worker launch in these accounting fixtures
	source := filepath.Join(c.WorkRoot, "states.json")
	c.WorkState = &WorkState{Kind: "command", Argv: []string{"/bin/cat", source}}
	q := &fakeQueue{obs: Observation{Tickets: []Ticket{ticket("t", "P1", 1)}}}
	d, err := Open("prog", c, q, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(d.dir, 0700); _ = d.Close() })
	return d, q, source
}

func writeProgress(t *testing.T, path, token string) {
	t.Helper()
	v := map[string]any{"t": map[string]string{"state": "work", "progress": token}}
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func progressObservation(token string) *Observation {
	ts := ticket("t", "P1", 1)
	ts.State, ts.ProgressToken = "work", token
	return &Observation{Tickets: []Ticket{ts}}
}

func parkProgress(t *testing.T, d *Dispatcher) {
	t.Helper()
	key := "ticket:a:q:t"
	h := d.ledger.Progress[key]
	base := baseFingerprint(progressObservation(""), key)
	d.ledger.Backoff[key] = &BackoffState{NoProgress: 3, Parked: true,
		BaseFingerprint: base, ProgressDigest: h.Current, Fingerprint: progressFingerprint(base, h.Current)}
	if err := d.ledger.save(d.dir); err != nil {
		t.Fatal(err)
	}
}

func TestCALV0064_ChangedFileReplayAndRestart(t *testing.T) {
	d, _, source := progressDispatcher(t)
	key := "ticket:a:q:t"
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	parkProgress(t, d)
	writeProgress(t, source, "B")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Backoff[key] != nil {
		t.Fatal("new file token did not unpark")
	}
	// Normal Tick return must not let a captured old deferred receiver undo
	// the admission, even before Close.
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || l.Progress[key].Current != progressDigest("B") || l.Backoff[key] != nil {
		t.Fatalf("normal-return readback: %+v %v", l, err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open("prog", d.Config, d.Queue, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d.lock = restarted.lock
	d.ledger = restarted.ledger
	parkProgress(t, d)
	for _, token := range []string{"B", "A", ""} {
		writeProgress(t, source, token)
		if err := d.Tick(context.Background()); err != nil {
			t.Fatal(err)
		}
		if d.ledger.Backoff[key] == nil || d.ledger.Progress[key].Current != progressDigest("B") || len(d.ledger.Progress[key].Seen) != 2 {
			t.Fatalf("replay/missing %q manufactured progress", token)
		}
	}
	if err := os.WriteFile(source, []byte(`{"unknown":{"state":"work","progress":"new"},"t":"work"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.Progress) != 1 {
		t.Fatal("unknown ticket consumed history")
	}
	if err := os.WriteFile(source, []byte(`{"t":{"state":"work","progress":null}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Progress[key].Current != progressDigest("B") || d.ledger.Backoff[key] == nil {
		t.Fatal("malformed reader changed history or parking")
	}
}

func TestCALV0064_CheckedSaveFailureDoesNotGrantOrEscapeThroughClose(t *testing.T) {
	d, _, source := progressDispatcher(t)
	key := "ticket:a:q:t"
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	parkProgress(t, d)
	request := filepath.Join(d.dir, "requests", "unpark.json")
	if err := os.WriteFile(request, []byte(`{"unpark":"ticket:a:q:t"}`), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeProgress(t, source, "B")
	// Publish and retire the reader before injecting the checked ledger failure.
	// A directory failure before Tick would instead refuse its reader marker.
	saveCalls := 0
	d.progressSave = func(staged *Ledger, dir string) error {
		saveCalls++
		if err := os.Chmod(dir, 0500); err != nil {
			t.Fatal(err)
		}
		probe, err := os.CreateTemp(dir, ".should-fail-*")
		if err == nil {
			probe.Close()
			os.Remove(probe.Name())
			t.Fatal("fixture did not refuse writes")
		}
		return staged.save(dir)
	}
	t.Cleanup(func() { _ = os.Chmod(d.dir, 0700) })
	if err := d.Tick(context.Background()); err == nil || !strings.Contains(err.Error(), "admission failed") {
		t.Fatalf("failed admission: %v", err)
	}
	if saveCalls != 1 || d.reader != nil || d.readerErr != nil {
		t.Fatalf("checked save not reached after reader release: calls=%d held=%v", saveCalls, d.readerErr)
	}
	after, err := os.ReadFile(filepath.Join(d.dir, "state.json"))
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("failed admission changed committed bytes")
	}
	if d.ledger.Progress[key].Current != progressDigest("A") || d.ledger.Backoff[key] == nil {
		t.Fatal("failed staging was published")
	}
	if _, err := os.Stat(request); err != nil {
		t.Fatal("failed admission consumed operator request")
	}
	if err := os.Chmod(d.dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := d.Close(); err != nil {
		t.Fatal(err)
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || l.Progress[key].Current != progressDigest("A") {
		t.Fatalf("Close persisted failed staging: %v", err)
	}
	d.lock, err = os.OpenFile(filepath.Join(d.dir, "lock"), os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
}

type progressWriter func([]byte) (int, error)

func (f progressWriter) Write(p []byte) (int, error) { return f(p) }

func TestCALV0064_LaterSaveFailureCannotReviveGrantedParking(t *testing.T) {
	d, _, source := progressDispatcher(t)
	key := "ticket:a:q:t"
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	parkProgress(t, d)
	d.Out = progressWriter(func(p []byte) (int, error) {
		if strings.Contains(string(p), "unparked") {
			if err := os.Chmod(d.dir, 0500); err != nil {
				t.Fatal(err)
			}
		}
		return len(p), nil
	})
	writeProgress(t, source, "B")
	// CAL-V0-194: the tick's own final save fails and says so.
	if err := d.Tick(context.Background()); !errors.Is(err, ErrLedgerUnsaved) {
		t.Fatalf("later save failure not reported: %v", err)
	}
	if err := d.Close(); err == nil {
		t.Fatal("later save failure not established")
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || l.Progress[key].Current != progressDigest("B") || l.Backoff[key] != nil {
		t.Fatalf("granted state revived after later save failure: %v", err)
	}
	if err := os.Chmod(d.dir, 0700); err != nil {
		t.Fatal(err)
	}
	restarted, err := Open("prog", d.Config, d.Queue, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	d.lock, d.ledger, d.Out = restarted.lock, restarted.ledger, io.Discard
	parkProgress(t, d)
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Backoff[key] == nil {
		t.Fatal("same committed token granted again after restart")
	}
}

func TestCALV0064_ActivePendingUnknownAndSeedAccounting(t *testing.T) {
	d, _, _ := progressDispatcher(t)
	key := "ticket:a:q:t"
	base := baseFingerprint(progressObservation(""), key)
	w := &Worker{ID: "ended", Role: "impl", Key: key, Ticket: key, Fingerprint: base, Started: time.Now()}
	d.ledger.Workers = []*Worker{w}
	d.ledger.Backoff[key] = &BackoffState{NoProgress: 2, Fingerprint: base}
	g, p, _, err := d.admitProgress(context.Background(), progressObservation("A"), nil)
	if err != nil || len(g)+len(p) != 0 {
		t.Fatal(err)
	}
	w = d.ledger.Workers[0]
	if w.ProgressDigest != progressDigest("A") {
		t.Fatal("first seed not retained as launch baseline")
	}
	if _, _, _, err := d.admitProgress(context.Background(), progressObservation("B"), nil); err != nil {
		t.Fatal(err)
	}
	unknown := progressObservation("B")
	unknown.Tickets[0].State = StateUnknown
	g, p, _, err = d.admitProgress(context.Background(), unknown, []*Worker{w})
	if err != nil || !p[w.ID] || len(g) != 0 {
		t.Fatal("UNKNOWN discarded pending credit")
	}
	d.finish(unknown, []*Worker{w}, g, p)
	if len(d.ledger.Workers) != 1 || d.ledger.Backoff[key].NoProgress != 2 {
		t.Fatal("UNKNOWN accounted worker")
	}
	g, p, _, err = d.admitProgress(context.Background(), progressObservation("A"), []*Worker{w})
	if err != nil || !g[w.ID] || len(p) != 0 {
		t.Fatal("healthy stale observation did not use pending accepted credit", err)
	}
	d.finish(progressObservation("A"), []*Worker{w}, g, p)
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || len(l.Workers) != 0 || l.Backoff[key] != nil || l.Progress[key].Current != progressDigest("B") {
		t.Fatal("pending grant removal/backoff not committed together", err)
	}
}

func TestCALV0064_FirstSeedIsNotProgressAndCancellationIsNotAdmission(t *testing.T) {
	d, _, source := progressDispatcher(t)
	key := "ticket:a:q:t"
	base := baseFingerprint(progressObservation(""), key)
	d.ledger.Backoff[key] = &BackoffState{NoProgress: 3, Parked: true, Fingerprint: base}
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Backoff[key] == nil || !d.ledger.Backoff[key].Parked {
		t.Fatal("first seed manufactured progress")
	}
	before, _ := json.Marshal(d.ledger)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, _, err := d.admitProgress(ctx, progressObservation("B"), nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	after, _ := json.Marshal(d.ledger)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("canceled admission changed history")
	}
}

func TestCALV0064_FirstSeedEndedWorkerAndLaterFailure(t *testing.T) {
	for _, advance := range []bool{false, true} {
		t.Run(fmt.Sprint(advance), func(t *testing.T) {
			d, _, source := progressDispatcher(t)
			key := "ticket:a:q:t"
			base := baseFingerprint(progressObservation(""), key)
			d.ledger.Workers = []*Worker{{ID: "ended", Key: key, Ticket: key, Fingerprint: base, Started: time.Now()}}
			writeProgress(t, source, "A")
			if advance {
				if _, _, _, err := d.admitProgress(context.Background(), progressObservation("A"), nil); err != nil {
					t.Fatal(err)
				}
				writeProgress(t, source, "B")
				d.Out = progressWriter(func(p []byte) (int, error) {
					if strings.Contains(string(p), "finished on") {
						if err := os.Chmod(d.dir, 0500); err != nil {
							t.Fatal(err)
						}
					}
					return len(p), nil
				})
			}
			// CAL-V0-194: with advance the tick's own final save fails and says so.
			if err := d.Tick(context.Background()); advance && !errors.Is(err, ErrLedgerUnsaved) || !advance && err != nil {
				t.Fatalf("tick: %v", err)
			}
			if advance {
				if err := d.Close(); err == nil {
					t.Fatal("later save did not fail")
				}
				l, err := LoadLedger(d.dir, d.Program)
				if err != nil || len(l.Workers) != 0 || l.Backoff[key] != nil || l.Progress[key].Current != progressDigest("B") {
					t.Fatalf("ended worker grant revived after later save failure: %v", err)
				}
				if err := os.Chmod(d.dir, 0700); err != nil {
					t.Fatal(err)
				}
				restarted, err := Open("prog", d.Config, d.Queue, io.Discard)
				if err != nil {
					t.Fatal(err)
				}
				d.lock, d.ledger, d.Out = restarted.lock, restarted.ledger, io.Discard
			} else if len(d.ledger.Workers) != 0 || d.ledger.Backoff[key] == nil || d.ledger.Backoff[key].NoProgress != 1 {
				t.Fatal("first seed counted as an ended worker's progress")
			}
		})
	}
}

type canceledProgressQueue struct {
	*fakeQueue
	reads  int
	cancel context.CancelFunc
}

func (q *canceledProgressQueue) Observe(ctx context.Context) (*Observation, error) {
	q.reads++
	if q.reads == 2 {
		q.cancel()
	}
	return q.fakeQueue.Observe(ctx)
}

func TestCALV0064_CanceledReobservationCannotAdmitEarlierToken(t *testing.T) {
	d, q, source := progressDispatcher(t)
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	key := "ticket:a:q:t"
	base := baseFingerprint(progressObservation(""), key)
	d.ledger.Workers = []*Worker{{ID: "ended", Key: key, Ticket: key, BaseFingerprint: base,
		ProgressDigest: progressDigest("A"), Fingerprint: progressFingerprint(base, progressDigest("A")), Started: time.Now()}}
	parkProgress(t, d)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.Queue = &canceledProgressQueue{fakeQueue: q, cancel: cancel}
	writeProgress(t, source, "B")
	if err := d.Tick(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if d.ledger.Progress[key].Current != progressDigest("A") || len(d.ledger.Workers) != 1 || d.ledger.Backoff[key] == nil {
		t.Fatal("earlier successful read granted progress despite canceled final observation")
	}
}

func TestCALV0064_PostCommitCancellationPreservesFactsAndStopsEffects(t *testing.T) {
	d, _, source := progressDispatcher(t)
	key := "ticket:a:q:t"
	writeProgress(t, source, "A")
	if err := d.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	parkProgress(t, d)
	request := filepath.Join(d.dir, "requests", "unpark.json")
	if err := os.WriteFile(request, []byte(`{"unpark":"ticket:a:q:t"}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := d.ledger.EventSeq
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.progressSave = func(l *Ledger, dir string) error {
		err := l.save(dir) // actual atomic save, not a synthetic successful receipt
		if err == nil {
			cancel()
		}
		return err
	}
	writeProgress(t, source, "B")
	if err := d.Tick(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || l.Progress[key].Current != progressDigest("B") || l.Backoff[key] != nil {
		t.Fatal("post-commit cancellation rolled back or overwrote completed facts", err)
	}
	if d.ledger.EventSeq != before {
		t.Fatal("post-commit cancellation emitted downstream decisions")
	}
	if _, err := os.Stat(request); err != nil {
		t.Fatal("post-commit cancellation consumed operator request")
	}
}

func TestCALV0064_CapacitySortedAllocationAndStrictLoad(t *testing.T) {
	d, _, _ := progressDispatcher(t)
	d.ledger.Progress = map[string]*ProgressHistory{}
	for n := 0; n < 32; n++ {
		h := &ProgressHistory{}
		for j := 0; j < 256; j++ {
			if n == 31 && j == 255 {
				break
			}
			h.Seen = append(h.Seen, progressDigest(fmt.Sprintf("%d:%d", n, j)))
		}
		sort.Strings(h.Seen)
		h.Current = h.Seen[0]
		d.ledger.Progress[fmt.Sprintf("ticket:a:q:old%02d", n)] = h
	}
	a, b := ticket("a", "P1", 1), ticket("b", "P1", 1)
	a.State, b.State, a.ProgressToken, b.ProgressToken = "work", "work", "a", "b"
	if _, _, _, err := d.admitProgress(context.Background(), &Observation{Tickets: []Ticket{b, a}}, nil); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Progress[a.ID] == nil || d.ledger.Progress[b.ID] != nil {
		t.Fatal("capacity allocation depended on input order")
	}
	full := ticket("old00", "P1", 1)
	full.State, full.ProgressToken = "work", "new-over-key-capacity"
	current := d.ledger.Progress[full.ID].Current
	if _, _, _, err := d.admitProgress(context.Background(), &Observation{Tickets: []Ticket{full}}, nil); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Progress[full.ID].Current != current || len(d.ledger.Progress[full.ID].Seen) != 256 {
		t.Fatal("per-key exhaustion evicted or advanced history")
	}
	l, err := LoadLedger(d.dir, d.Program)
	if err != nil || len(l.Progress) != 33 {
		t.Fatal(err)
	}
	valid, _ := json.Marshal(l)
	for _, mutate := range []func(*Ledger){
		func(l *Ledger) { l.Progress[a.ID].Seen = append(l.Progress[a.ID].Seen, l.Progress[a.ID].Current) },
		func(l *Ledger) { l.Progress[a.ID].Current = strings.Repeat("f", 64) },
		func(l *Ledger) { l.Progress["local"] = l.Progress[a.ID] },
		func(l *Ledger) {
			l.Workers = []*Worker{{Key: a.ID, Fingerprint: baseFingerprint(&Observation{Tickets: []Ticket{a}}, a.ID)}}
		},
		func(l *Ledger) { l.Progress[a.ID].Seen[0] = "invalid" },
	} {
		var bad Ledger
		if err := json.Unmarshal(valid, &bad); err != nil {
			t.Fatal(err)
		}
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		path := filepath.Join(d.dir, "state.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(d.dir, d.Program); err == nil {
			t.Fatal("malformed progress history accepted")
		}
		after, _ := os.ReadFile(path)
		if !reflect.DeepEqual(raw, after) {
			t.Fatal("refused ledger rewritten")
		}
	}
	if err := os.WriteFile(filepath.Join(d.dir, "state.json"), append(valid, []byte(" {}")...), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(d.dir, d.Program); err == nil {
		t.Fatal("trailing token ledger JSON accepted")
	}
	duplicate := strings.Replace(string(valid), `"current":`, `"current":"`+strings.Repeat("a", 64)+`","current":`, 1)
	if err := os.WriteFile(filepath.Join(d.dir, "state.json"), []byte(duplicate), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadLedger(d.dir, d.Program); err == nil {
		t.Fatal("duplicate token ledger JSON accepted")
	}
	for _, raw := range []string{
		`{"profile":"` + StateProfile + `","program":"prog","workers":[],"backoff":{},"progress":null}`,
		strings.TrimSuffix(string(valid), "}") + `,"progress":null}`,
	} {
		path := filepath.Join(d.dir, "state.json")
		if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadLedger(d.dir, d.Program); err == nil {
			t.Fatal("null progress member discarded history")
		}
		after, _ := os.ReadFile(path)
		if string(after) != raw {
			t.Fatal("refused null progress ledger rewritten")
		}
	}
}

func TestCALV0064_NoTokenPreservesLegacyLedgerAndFingerprint(t *testing.T) {
	l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{}}
	raw, _ := json.Marshal(l)
	for _, field := range []string{"progress", "baseFingerprint", "progressDigest"} {
		if strings.Contains(string(raw), field) {
			t.Fatal("legacy ledger gained optional token fields")
		}
	}
	obs := progressObservation("")
	if Fingerprint(obs, obs.Tickets[0].ID) != baseFingerprint(obs, obs.Tickets[0].ID) {
		t.Fatal("legacy fingerprint changed")
	}
}

func TestCALV0064_LedgerCanonicalFieldsAndCaseSensitiveKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	a, b := progressDigest("A"), progressDigest("B")
	seen := []string{a, b}
	sort.Strings(seen)
	key := "ticket:a:q:Case"
	base := strings.Repeat("a", 64)
	l := &Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{},
		Progress: map[string]*ProgressHistory{key: {Current: b, Seen: seen}, "ticket:a:q:case": {Current: a, Seen: []string{a}}},
		Seen:     &Seen{Tickets: map[string]string{"Current": "upper", "current": "lower"}, Claims: map[string]string{}, Lanes: map[string]string{}}}
	raw, _ := json.Marshal(l)
	legacy, _ := json.Marshal(&Ledger{Profile: StateProfile, Program: "prog", Workers: []*Worker{}, Backoff: map[string]*BackoffState{}})
	withAccounting := *l
	withAccounting.Workers = []*Worker{{ID: "worker", Key: key, BaseFingerprint: base, ProgressDigest: b, Fingerprint: progressFingerprint(base, b), Members: []Proc{{PID: 100, Identity: "identity"}}}}
	withAccounting.Backoff = map[string]*BackoffState{key: {BaseFingerprint: base, ProgressDigest: b, Fingerprint: progressFingerprint(base, b)}}
	accounting, _ := json.Marshal(&withAccounting)
	backoffOnly := *l
	backoffOnly.Backoff = withAccounting.Backoff
	backoff, _ := json.Marshal(&backoffOnly)
	for name, malformed := range map[string]string{
		"canonical-then-alias-null": strings.TrimSuffix(string(raw), "}") + `,"Progress":null}`,
		"uppercase-only-null":       strings.TrimSuffix(string(legacy), "}") + `,"Progress":null}`,
		"uppercase-only-history":    strings.Replace(string(raw), `"progress":`, `"Progress":`, 1),
		"unicode-folded-progress":   strings.Replace(string(raw), `"progress":`, `"progreſſ":`, 1),
		"uppercase-only-trailing":   strings.TrimSuffix(string(legacy), "}") + `,"Progress":null} {}`,
		"uppercase-only-duplicate":  strings.TrimSuffix(string(legacy), "}") + `,"Progress":{},"Progress":null}`,
		"history-current-alias":     strings.Replace(string(raw), `"current":"`+b+`"`, `"current":"`+b+`","Current":"`+a+`"`, 1),
		"history-uppercase-current": strings.Replace(string(raw), `"current":"`+b+`"`, `"Current":"`+b+`"`, 1),
		"history-seen-alias":        strings.Replace(string(raw), `"seen":[`, `"Seen":[`, 1),
		"worker-alias":              strings.Replace(string(accounting), `"workers":`, `"Workers":`, 1),
		"worker-baseline-alias":     strings.Replace(string(accounting), `"baseFingerprint":`, `"BaseFingerprint":`, 1),
		"backoff-digest-alias":      strings.Replace(string(backoff), `"progressDigest":`, `"ProgressDigest":`, 1),
		"backoff-field-alias":       strings.Replace(string(backoff), `"noProgress":`, `"NoProgress":`, 1),
		"proc-identity-alias":       strings.Replace(string(accounting), `"identity":`, `"Identity":`, 1),
		"seen-struct-alias":         strings.Replace(string(raw), `"tickets":`, `"Tickets":`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(malformed), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadLedger(dir, "prog"); err == nil {
				t.Fatal("case-aliased ledger accepted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != malformed {
				t.Fatal("refusal changed persisted history")
			}
		})
	}
	for _, valid := range [][]byte{raw, accounting, legacy, []byte(strings.Replace(string(legacy), `"profile":`, `"Profile":`, 1))} {
		if err := os.WriteFile(path, valid, 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := LoadLedger(dir, "prog")
		if err != nil {
			t.Fatal(err)
		}
		if loaded.Progress != nil && (len(loaded.Progress) != 2 || loaded.Progress[key].Current != b || len(loaded.Progress[key].Seen) != 2 || loaded.Seen.Tickets["Current"] != "upper" || loaded.Seen.Tickets["current"] != "lower") {
			t.Fatal("dynamic keys were folded or restart history changed")
		}
		after, _ := os.ReadFile(path)
		if string(after) != string(valid) {
			t.Fatal("valid load rewrote persisted bytes")
		}
	}
}

func TestCALV0064_KeyBoundaryAndOperatorUnparkRetainLifetimeBudget(t *testing.T) {
	d, _, _ := progressDispatcher(t)
	key := "ticket:a:q:t"
	h := &ProgressHistory{}
	for i := 0; i < 255; i++ {
		h.Seen = append(h.Seen, progressDigest(fmt.Sprint(i)))
	}
	sort.Strings(h.Seen)
	h.Current = h.Seen[0]
	d.ledger.Progress = map[string]*ProgressHistory{key: h}
	obs := progressObservation("slot-256")
	if _, _, _, err := d.admitProgress(context.Background(), obs, nil); err != nil {
		t.Fatal(err)
	}
	if len(d.ledger.Progress[key].Seen) != 256 || d.ledger.Progress[key].Current != progressDigest("slot-256") {
		t.Fatal("255-to-256 admission failed")
	}
	current := d.ledger.Progress[key].Current
	obs = progressObservation("slot-257")
	if _, _, _, err := d.admitProgress(context.Background(), obs, nil); err != nil {
		t.Fatal(err)
	}
	if d.ledger.Progress[key].Current != current || len(d.ledger.Progress[key].Seen) != 256 {
		t.Fatal("key capacity advanced")
	}
	parkProgress(t, d)
	request := filepath.Join(d.dir, "requests", "unpark.json")
	if err := os.WriteFile(request, []byte(`{"unpark":"ticket:a:q:t"}`), 0600); err != nil {
		t.Fatal(err)
	}
	d.unpark(obs)
	if d.ledger.Backoff[key] != nil || d.ledger.Progress[key].Current != current || len(d.ledger.Progress[key].Seen) != 256 {
		t.Fatal("operator unpark cleared replay protection or capacity")
	}
}
