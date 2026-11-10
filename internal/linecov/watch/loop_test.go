// SPDX-License-Identifier: Apache-2.0
package watch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// indexedModule commits a one-package module whose F body is a known block
// and hand-builds an index in which TestPass reached it and TestFail did not.
func indexedModule(t *testing.T) (string, *linecov.Index) {
	t.Helper()
	root := module(t)
	write(t, root, "p/p.go", "package p\nfunc F() int {\n return 1\n}\n")
	base := commit(t, root, "body")
	ctx := context.Background()
	oids, err := linecov.OIDs(ctx, root, base)
	if err != nil {
		t.Fatal(err)
	}
	id, _, err := linecov.RunnerID(ctx, root, nil, linecov.DefaultTestTimeout, "")
	if err != nil {
		t.Fatal(err)
	}
	// A hand-built index needs an in-order record, else its package is
	// unordered and runs whole (GLS-V0-008), and records observed under an
	// environment, else it binds none (GLS-V0-017).
	ix, err := linecov.Build("example.com/m", base, id, oids, []linecov.Record{
		{Package: "p", Name: "TestPass", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 1\n")},
		{Package: "p", Name: "TestFail", EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")},
		{Package: "p", Name: linecov.InOrder, Union: true, EnvObserved: true, Profile: []byte("mode: atomic\nexample.com/m/p/p.go:2.14,4.2 1 0\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	return root, ix
}

func events(t *testing.T, stream string) []Event {
	t.Helper()
	var out []Event
	for _, l := range strings.Split(strings.TrimSpace(stream), "\n") {
		if l == "" {
			continue
		}
		var e Event
		if err := json.Unmarshal([]byte(l), &e); err != nil {
			t.Fatalf("%s: %v", l, err)
		}
		if e.Scope != "user" || e.GateEligible {
			t.Fatalf("unsafe event %+v", e)
		}
		out = append(out, e)
	}
	return out
}

// LPCV-V0-1085: a dry run answers which tests a working-tree edit can affect
// and records the replan latency; the runner is never called and no result
// event is written.
func TestRunDryRunAnswersWithoutRunning(t *testing.T) {
	root, ix := indexedModule(t)
	write(t, root, "p/p.go", "package p\nfunc F() int {\n return 2\n}\n")
	var stream bytes.Buffer
	once := Once()
	err := Run(context.Background(), Options{Root: root, Index: ix, Static: []string{"p"}, Saves: once.C, Stopped: once.Err, Out: &stream})
	if err != nil {
		t.Fatal(err)
	}
	got := events(t, stream.String())
	if len(got) != 1 || got[0].Kind != "plan" || got[0].Commit != ix.Commit {
		t.Fatalf("events %+v", got)
	}
	if p := got[0].Plan; p == nil || len(p.Run) != 1 || p.Run[0].Run != "^(TestPass)$" || p.Run[0].Package != "./p" {
		t.Fatalf("plan %+v", got[0].Plan)
	}
	if got[0].ReplanMillis <= 0 {
		t.Fatalf("replan latency not recorded: %+v", got[0])
	}
}

// LPCV-V0-1087: an event source that fails ends the loop with a terminal
// error event naming the cause and a wrapped ErrSourceStopped; a deliberate
// stop ends it with neither.
func TestRunReportsStoppedSourceExplicitly(t *testing.T) {
	root, ix := indexedModule(t)
	t.Run("failure", func(t *testing.T) {
		ch := make(chan struct{})
		s := &Source{C: ch}
		s.stop(errors.New("inotify watches cannot be restored"))
		close(ch)
		var stream bytes.Buffer
		ran := false
		err := Run(context.Background(), Options{Root: root, Index: ix, Static: []string{"p"}, Saves: s.C, Stopped: s.Err, Out: &stream, Run: func(context.Context, Plan) error { ran = true; return nil }})
		if !errors.Is(err, ErrSourceStopped) || !strings.Contains(err.Error(), "inotify watches cannot be restored") {
			t.Fatalf("err %v", err)
		}
		got := events(t, stream.String())
		if ran || len(got) != 1 || got[0].Kind != "error" || !strings.Contains(got[0].Message, "stale") || !strings.Contains(got[0].Message, "inotify watches cannot be restored") {
			t.Fatalf("ran=%t events %+v", ran, got)
		}
	})
	t.Run("deliberate", func(t *testing.T) {
		ch := make(chan struct{})
		s := &Source{C: ch}
		s.stop(nil)
		close(ch)
		var stream bytes.Buffer
		if err := Run(context.Background(), Options{Root: root, Index: ix, Static: []string{"p"}, Saves: s.C, Stopped: s.Err, Out: &stream}); err != nil || stream.Len() != 0 {
			t.Fatalf("err %v stream %q", err, stream.String())
		}
	})
	t.Run("context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var stream bytes.Buffer
		if err := Run(ctx, Options{Root: root, Index: ix, Saves: make(chan struct{}), Out: &stream}); err != nil || stream.Len() != 0 {
			t.Fatalf("err %v stream %q", err, stream.String())
		}
	})
}

// LPCV-V0-7607: saves replan dirty contents against the recorded commit and
// stream both failures and repairs as user-only observations.
func TestRunStreamsFailureThenRepair(t *testing.T) {
	root, ix := indexedModule(t)
	write(t, root, "p/p.go", "package p\nfunc F() int {\n return 2\n}\n")
	saves := make(chan struct{}, 2)
	saves <- struct{}{}
	saves <- struct{}{}
	close(saves)
	var stream bytes.Buffer
	calls := 0
	err := Run(context.Background(), Options{Root: root, Index: ix, Static: []string{"p"}, Saves: saves, Out: &stream, Run: func(_ context.Context, p Plan) error {
		calls++
		if len(p.Run) != 1 || p.Run[0].Run != "^(TestPass)$" {
			t.Fatalf("plan %+v", p)
		}
		if calls == 1 {
			write(t, root, "p/p.go", "package p\nfunc F() int {\n return 3\n}\n")
			return os.ErrInvalid
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := events(t, stream.String())
	if calls != 2 || len(got) != 4 || got[1].Kind != "result" || got[1].ExitCode != 1 || got[3].Kind != "result" || got[3].ExitCode != 0 {
		t.Fatalf("calls %d events %+v", calls, got)
	}
}

// A non-Go index without a non-Go planner is refused before any save is
// consumed, so a Go-only caller never silently plans nothing.
func TestRunRefusesNonGoWithoutPlanner(t *testing.T) {
	root, ix := indexedModule(t)
	ix.Language = "ruby"
	err := Run(context.Background(), Options{Root: root, Index: ix, Saves: Once().C, Out: io.Discard})
	if err == nil || !strings.Contains(err.Error(), "ruby") {
		t.Fatalf("err %v", err)
	}
}

// Once fires exactly once and stops deliberately; Lines fires per line and
// reports a reader failure through Err.
func TestOnceAndLinesSources(t *testing.T) {
	once := Once()
	if _, ok := <-once.C; !ok {
		t.Fatal("Once did not fire")
	}
	if _, ok := <-once.C; ok || once.Err() != nil {
		t.Fatalf("Once fired twice or failed: %v", once.Err())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lines := Lines(ctx, strings.NewReader("save\nsave\n"))
	for i := 0; i < 2; i++ {
		awaitSave(t, ctx, lines)
	}
	if _, ok := <-lines.C; ok || lines.Err() != nil {
		t.Fatalf("Lines after EOF: open=%t err=%v", ok, lines.Err())
	}
	failing := Lines(ctx, io.MultiReader(strings.NewReader("save\n"), &failingReader{}))
	awaitSave(t, ctx, failing)
	if _, ok := <-failing.C; ok || failing.Err() == nil {
		t.Fatal("a reader failure stopped Lines silently")
	}
}

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) { return 0, errors.New("editor pipe broke") }

// LPCV-V0-1086: package discovery stops with the watch; a cancelled context
// ends the `go list` it runs instead of waiting for it.
func TestPackagesStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Packages(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatalf("Packages under a cancelled context: %v", err)
	}
}
