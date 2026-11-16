// SPDX-License-Identifier: Apache-2.0
package rubylinecov

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/linecov"
)

// fakeSuite writes the collector's dump vocabulary without Ruby: four
// passing tests, each reaching one line of lib/a.rb; an isolated run
// (CORVINT_LINECOV_ONLY set) reports only its test, after a short pause.
// FAKE_RUBY replaces the Ruby version it reports.
const fakeSuite = `#!/bin/sh
out="$CORVINT_LINECOV_DIR/suite.jsonl"
only="$CORVINT_LINECOV_ONLY"
: > "$out"
printf '{"kind":"start","ruby":"%s","test":"load"}\n' "${FAKE_RUBY:-3.3.8}" >> "$out"
printf '%s\n' '{"kind":"load","lines":{"lib/a.rb":[1,5]}}' >> "$out"
n=0
for t in 1 2 3 4; do
  name="A#test_$t"
  if [ -n "$only" ] && [ "$only" != "$name" ]; then continue; fi
  n=$((n+1))
  if [ -n "$only" ]; then sleep 0.1; fi
  printf '{"kind":"test","seq":%d,"package":"test/a_test.rb","name":"%s","status":"passed","elapsed_ns":%d,"lines":{"lib/a.rb":[%d]},"methods":{}}\n' "$n" "$name" "$t" "$((t % 2 + 2))" >> "$out"
done
printf '%s\n' '{"kind":"framework","name":"minitest","version":"5"}' >> "$out"
printf '%s\n' '{"kind":"files","files":{"lib/a.rb":{"lines":[1,2,3,5],"methods":[]}}}' >> "$out"
printf '%s\n' '{"kind":"done"}' >> "$out"
`

func fakeProject(t *testing.T) (root, head string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake suite is a POSIX shell script")
	}
	root = t.TempDir()
	for p, content := range map[string]string{
		"suite.sh":       fakeSuite,
		"lib/a.rb":       "class A\n  def x\n    1\n  end\n  X = 1\nend\n",
		"test/a_test.rb": "# fixture\n",
	} {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "test@example.invalid")
	git(t, root, "config", "user.name", "test")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture")
	return root, git(t, root, "rev-parse", "HEAD")
}

func indexBytes(t *testing.T, root, head string, run *Run) []byte {
	t.Helper()
	oids, err := linecov.OIDs(context.Background(), root, head)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := Index(".", head, run, oids)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := ix.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// LPCV-V0-7609: a parallel collection interrupted at its budget and resumed
// from its state directory merges to the same index bytes as a serial run;
// a state bound to another collection is refused.
func TestIsolatedParallelResume(t *testing.T) {
	root, head := fakeProject(t)
	base := Options{Root: root, Argv: []string{"sh", "suite.sh"}, Timeout: time.Minute}
	serial := base
	serial.Work = t.TempDir()
	run, err := Collect(context.Background(), serial)
	if err != nil {
		t.Fatal(err)
	}
	want := indexBytes(t, root, head, run)
	if len(run.records) != 5 { // four tests and the load bucket
		t.Fatalf("records %d", len(run.records))
	}

	state := filepath.Join(t.TempDir(), "state")
	par := base
	par.State, par.Parallel, par.Budget = state, 2, time.Millisecond
	_, err = Collect(context.Background(), par)
	var incomplete *IncompleteError
	if !errors.As(err, &incomplete) || incomplete.Done >= incomplete.Total || incomplete.Total != 4 {
		t.Fatalf("budget did not interrupt: %v", err)
	}
	ordinary, err := os.ReadFile(filepath.Join(state, "dumps", "suite.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	par.Budget = 0
	par.Parallel = 3
	resumed, err := Collect(context.Background(), par)
	if err != nil {
		t.Fatal(err)
	}
	if got := indexBytes(t, root, head, resumed); !bytes.Equal(got, want) {
		t.Fatalf("resumed parallel index differs from the serial index")
	}
	again, err := os.ReadFile(filepath.Join(state, "dumps", "suite.jsonl"))
	if err != nil || !bytes.Equal(again, ordinary) {
		t.Fatalf("resume reran the retained ordinary-order run: %v", err)
	}

	other := par
	other.Argv = []string{"sh", "suite.sh", "extra"}
	if _, err := Collect(context.Background(), other); err == nil || !strings.Contains(err.Error(), "another collection") {
		t.Fatalf("mismatched state accepted: %v", err)
	}
	unbound := par
	unbound.State = t.TempDir()
	if err := os.WriteFile(filepath.Join(unbound.State, "stray"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Collect(context.Background(), unbound); err == nil || !strings.Contains(err.Error(), "no binding") {
		t.Fatalf("unbound non-empty state accepted: %v", err)
	}
}

// LPCV-V0-7609: launches never exceed the worker count, retained results are
// not relaunched, a failed run is retained as its error, and a cancelled run
// retains nothing.
func TestIsolatedRetention(t *testing.T) {
	work := t.TempDir()
	outcomes := []Outcome{{Name: "a", Status: "passed"}, {Name: "b", Status: "pending"}, {Name: "c", Status: "failed"}, {Name: "d", Status: "passed"}, {Name: "e", Status: "passed"}}
	var running, peak, launches atomic.Int32
	launch := func(i int, _ Outcome) error {
		launches.Add(1)
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		running.Add(-1)
		dir := filepath.Join(work, fmt.Sprintf("isolated-%d", i))
		_, err := retained(context.Background(), dir, true, "result.json", func() (*Run, error) { return nil, errors.New("boom") })
		if err == nil || err.Error() != "boom" {
			t.Errorf("retained error %v", err)
		}
		return nil
	}
	if err := runIsolated(context.Background(), outcomes, Options{Work: work, Parallel: 2}, launch); err != nil {
		t.Fatal(err)
	}
	if launches.Load() != 4 || peak.Load() > 2 {
		t.Fatalf("launches %d peak %d", launches.Load(), peak.Load())
	}
	if err := runIsolated(context.Background(), outcomes, Options{Work: work, Parallel: 2}, launch); err != nil || launches.Load() != 4 {
		t.Fatalf("retained runs relaunched: %v %d", err, launches.Load())
	}
	_, err := retained(context.Background(), filepath.Join(work, "isolated-0"), true, "result.json", func() (*Run, error) {
		t.Fatal("retained error rerun")
		return nil, nil
	})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("retained error not returned: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := filepath.Join(work, "cancelled")
	if _, err := retained(ctx, dir, true, "result.json", func() (*Run, error) { return nil, ctx.Err() }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "result.json")); !os.IsNotExist(err) {
		t.Fatalf("cancelled run retained: %v", err)
	}
}

// LPCV-V0-7609: the same byte identity with the real collector on the
// Minitest fixture: a serial run, then a parallel run resumed from a copy of
// its retained ordinary-order run.
func TestLiveCollectResume(t *testing.T) {
	requireRuby(t, "minitest")
	root := t.TempDir()
	head := project(t, root, "minitest")
	argv := []string{"ruby", "-Itest", "test/calc_test.rb"}
	first := filepath.Join(t.TempDir(), "state")
	run, err := Collect(context.Background(), Options{Root: root, Argv: argv, Timeout: 2 * time.Minute, State: first})
	if err != nil {
		t.Fatal(err)
	}
	want := indexBytes(t, root, head, run)
	second := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state.json", "ordinary.json", "corvint_linecov.rb"} {
		data, err := os.ReadFile(filepath.Join(first, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(second, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.CopyFS(filepath.Join(second, "dumps"), os.DirFS(filepath.Join(first, "dumps"))); err != nil {
		t.Fatal(err)
	}
	resumed, err := Collect(context.Background(), Options{Root: root, Argv: argv, Timeout: 2 * time.Minute, State: second, Parallel: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got := indexBytes(t, root, head, resumed); !bytes.Equal(got, want) {
		t.Fatal("parallel resumed index differs from the serial index")
	}
}

// LPCV-V0-7609: merging the same shapes again adds nothing, so thousands of
// isolated runs do not multiply the index's blocks.
func TestMergeShapesOnce(t *testing.T) {
	into := map[string]fileShape{}
	one := map[string]fileShape{"lib/a.rb": {Lines: []int32{1, 2, 5}, Methods: []span{{2, 2, 4, 5}}}}
	two := map[string]fileShape{"lib/a.rb": {Lines: []int32{5, 7}, Methods: []span{{2, 2, 4, 5}, {6, 2, 8, 5}}}}
	for i := 0; i < 3; i++ {
		mergeShapes(into, one)
		mergeShapes(into, two)
	}
	got := into["lib/a.rb"]
	if fmt.Sprint(got.Lines) != "[1 2 5 7]" || len(got.Methods) != 2 {
		t.Fatalf("merged shape %+v", got)
	}
}

// LPCV-V0-7609: a state directory belongs to one environment and one
// runner. A chunk resumed under another gem environment, or whose isolated
// runs report another runner than the retained ordinary-order run, is
// refused instead of merged.
func TestResumeBindsEnvironment(t *testing.T) {
	root, _ := fakeProject(t)
	begin := func(t *testing.T, declared ...string) Options {
		t.Helper()
		o := Options{Root: root, Argv: []string{"sh", "suite.sh"}, Timeout: time.Minute, Parallel: 2, Budget: time.Millisecond, Env: declared}
		o.State = filepath.Join(t.TempDir(), "state")
		var incomplete *IncompleteError
		if _, err := Collect(context.Background(), o); !errors.As(err, &incomplete) || incomplete.Done >= incomplete.Total {
			t.Fatalf("the first chunk was not interrupted: %v", err)
		}
		o.Budget = 0
		return o
	}
	t.Run("gem environment", func(t *testing.T) {
		t.Setenv("GEM_PATH", "/gems/a")
		o := begin(t)
		t.Setenv("GEM_PATH", "/gems/b")
		if _, err := Collect(context.Background(), o); err == nil || !strings.Contains(err.Error(), "GEM_PATH") {
			t.Fatalf("a chunk under another GEM_PATH resumed: %v", err)
		}
		t.Setenv("GEM_PATH", "/gems/a")
		if _, err := Collect(context.Background(), o); err != nil {
			t.Fatalf("the original environment did not resume: %v", err)
		}
	})
	t.Run("declared variable", func(t *testing.T) {
		t.Setenv("NO_CONNECTION", "1")
		t.Setenv("UNDECLARED", "a")
		o := begin(t, "NO_CONNECTION")
		t.Setenv("NO_CONNECTION", "")
		if _, err := Collect(context.Background(), o); err == nil || !strings.Contains(err.Error(), "NO_CONNECTION") {
			t.Fatalf("a chunk under another NO_CONNECTION resumed: %v", err)
		}
		undeclared := o
		undeclared.Env = nil
		t.Setenv("NO_CONNECTION", "1")
		if _, err := Collect(context.Background(), undeclared); err == nil || !strings.Contains(err.Error(), "NO_CONNECTION") {
			t.Fatalf("a chunk that no longer declares NO_CONNECTION resumed: %v", err)
		}
		t.Setenv("UNDECLARED", "b")
		if _, err := Collect(context.Background(), o); err != nil {
			t.Fatalf("an undeclared variable refused the resume: %v", err)
		}
	})
	t.Run("runner identity", func(t *testing.T) {
		o := begin(t)
		t.Setenv("FAKE_RUBY", "3.4.1")
		if _, err := Collect(context.Background(), o); err == nil || !strings.Contains(err.Error(), "runner") {
			t.Fatalf("isolated runs under another Ruby merged: %v", err)
		}
		// The foreign run was not retained: the collection's own runner resumes.
		t.Setenv("FAKE_RUBY", "3.3.8")
		if _, err := Collect(context.Background(), o); err != nil {
			t.Fatalf("the original runner did not resume: %v", err)
		}
	})
	bound := boundEnvironment([]string{"PATH=/bin", "GEM_HOME=/g", "BUNDLE_GEMFILE=/f", "RUBYOPT=-w", "HOME=/h", "CORVINT_LINECOV_DIR=/x"}, []string{"NO_CONNECTION"})
	var names []string
	for name, value := range bound {
		names = append(names, name+"="+value[:5])
	}
	sort.Strings(names)
	if got := strings.Join(names, " "); got != "BUNDLE_GEMFILE=sha25 GEM_HOME=sha25 NO_CONNECTION=unset PATH=sha25 RUBYLIB=unset RUBYOPT=sha25" {
		t.Fatalf("bound environment %s", got)
	}
}
