package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/procgroup"
)

var fixtureBinary []byte
var oldFixtureBinary, newFixtureBinary []byte

func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "corvint-compat-test-build-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := func() int {
		defer os.RemoveAll(root)
		goPath := "/usr/local/go/bin/go"
		if p := os.Getenv("GOROOT"); p != "" {
			goPath = filepath.Join(p, "bin", "go")
		}
		if _, e := os.Stat(goPath); e != nil {
			goPath = "/opt/homebrew/bin/go"
		}
		// Resolve the build tool only; replay executables never use PATH lookup.
		if p, e := findGo(); e == nil {
			goPath = p
		}
		cwd, _ := os.Getwd()
		path := filepath.Join(root, "fixture")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{goPath, "build", "-trimpath", "-o", path, "."}, Dir: cwd, Env: os.Environ(), Timeout: 60 * time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1 << 20})
		if o.Err != nil || o.ExitStatus != 0 {
			fmt.Fprintf(os.Stderr, "fixture build failed: %v %s\n", o.Err, o.Stderr)
			return 1
		}
		var err error
		fixtureBinary, err = os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, variant := range []string{"old", "new"} {
			target := filepath.Join(root, variant)
			o := procgroup.Run(ctx, procgroup.Spec{Argv: []string{goPath, "build", "-trimpath", "-buildvcs=false", "-o", target, "./fixtures/" + variant}, Dir: cwd, Env: os.Environ(), Timeout: 60 * time.Second, ShutdownTimeout: time.Second, InputLimit: 1, OutputLimit: 1 << 20})
			if o.Err != nil || o.ExitStatus != 0 {
				fmt.Fprintf(os.Stderr, "variant build failed: %v %s\n", o.Err, o.Stderr)
				return 1
			}
			data, err := os.ReadFile(target)
			if err != nil {
				return 1
			}
			if variant == "old" {
				oldFixtureBinary = data
			} else {
				newFixtureBinary = data
			}
		}
		ownedVariantHashes = digest(oldFixtureBinary) + "," + digest(newFixtureBinary)
		return m.Run()
	}()
	os.Exit(code)
}
func fixtureManifest(t *testing.T) (string, preparedManifest) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "bundle")
	if err := writeFixtureBundle(dir, fixtureBinary); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "manifest.json")
	p, err := loadManifest(path, ownedRegistry(fixtureBinary))
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[0].Refusal != procgroup.RefusalNone {
		t.Fatalf("%s: %s", p.Tasks[0].Refusal, p.Tasks[0].Detail)
	}
	return path, p
}
func TestReplayExactEnvelopeAndPinnedStdin(t *testing.T) {
	_, p := fixtureManifest(t)
	p.Tasks[0].Task.Argv = []string{"--owned-fixture", "stdin"}
	p.Tasks[0].Input = []byte("hello\x00\n")
	r := replay(context.Background(), p)
	c := r.Cases[0]
	if c.Status != procgroup.StatusPass || c.Outcome != procgroup.OutcomeCompatible || c.RecordCount != 6 || c.Launches != 6 || c.UnobservedLaunches != 0 || c.Inconclusive != nil || c.AcceptedBy != "NOT_PRODUCED" {
		t.Fatalf("%+v", c)
	}
	for _, record := range c.Records {
		if record.StdoutSHA256 != digest(p.Tasks[0].Input) || record.StdoutBytes != 7 || record.Exit != 0 || record.WallMS <= 0 || record.GOOS == "" || record.GOARCH == "" {
			t.Fatalf("%+v", record)
		}
		b, _ := json.Marshal(record)
		var fields map[string]any
		_ = json.Unmarshal(b, &fields)
		if len(fields) != 14 {
			t.Fatalf("record fields changed: %s", b)
		}
	}
}
func TestReplayEnvironmentIsExactlyDeclared(t *testing.T) {
	t.Setenv("UNDECLARED_SENTINEL", "must-not-escape")
	_, p := fixtureManifest(t)
	p.Tasks[0].Task.Argv = []string{"--owned-fixture", "env"}
	c := replay(context.Background(), p).Cases[0]
	for _, r := range c.Records {
		if r.StdoutSHA256 != digest([]byte("LC_ALL=C\nTZ=UTC")) {
			t.Fatalf("environment changed: %+v", r)
		}
	}
}
func TestReplayPerStreamBounds(t *testing.T) {
	for _, tc := range []struct {
		name           string
		stdout, stderr int
		status         procgroup.Status
		streams        []string
	}{
		{"both-at-bound", outputLimit, outputLimit, procgroup.StatusPass, []string{}},
		{"stdout-over", outputLimit + 1, outputLimit, procgroup.StatusFail, []string{"stdout"}},
		{"stderr-over", outputLimit, outputLimit + 1, procgroup.StatusFail, []string{"stderr"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, p := fixtureManifest(t)
			p.Tasks[0].Task.Argv = []string{"--owned-fixture", "overflow", fmt.Sprint(tc.stdout), fmt.Sprint(tc.stderr)}
			r := replay(context.Background(), p)
			c := r.Cases[0]
			if c.Status != tc.status || !reflect.DeepEqual(c.OverflowStreams, tc.streams) {
				t.Fatalf("%+v", c)
			}
			if tc.status == procgroup.StatusFail && (c.Launches != 1 || c.RecordCount != 1 || c.CaseInconclusiveState != procgroup.InconclusiveResource || c.Outcome != procgroup.OutcomeWithheld) {
				t.Fatalf("overflow promoted: %+v", c)
			}
		})
	}
}
func TestOverflowAbortsOnlyItsCase(t *testing.T) {
	_, p := fixtureManifest(t)
	other := p.Tasks[0]
	other.Task.ID = "later"
	p.Tasks[0].Task.Argv = []string{"--owned-fixture", "overflow", fmt.Sprint(outputLimit + 1), "0"}
	p.Tasks = append(p.Tasks, other)
	r := replay(context.Background(), p)
	if r.Cases[0].Launches != 1 || r.Cases[1].Launches != 6 || r.Cases[1].Status != procgroup.StatusPass {
		t.Fatalf("%+v", r)
	}
}
func TestBudgetCancellationIsPerTaskEntry(t *testing.T) {
	oldBudget := requestBudget
	requestBudget = 3 * time.Second
	t.Cleanup(func() { requestBudget = oldBudget })
	_, p := fixtureManifest(t)
	other := p.Tasks[0]
	other.Task.ID = "later"
	p.Tasks[0].Task.Argv = []string{"--owned-fixture", "sleep", "30s"}
	p.Tasks = append(p.Tasks, other)
	start := time.Now()
	r := replay(context.Background(), p)
	if time.Since(start) > 8*time.Second {
		t.Fatal("task-entry deadline did not actively kill")
	}
	a, b := r.Cases[0], r.Cases[1]
	if a.Status != procgroup.StatusFail || a.Launches != 1 || a.RecordCount != 1 || a.CaseInconclusiveState != procgroup.InconclusiveTimeout {
		t.Fatalf("%+v", a)
	}
	// A shared budget would leave the second entry nothing to spend: the first
	// consumed the whole 3 s. Its own budget is what lets it launch and finish
	// work at all, so one completed launch is the discriminating evidence. How
	// many of its six launches fit inside 3 s is host speed rather than
	// behaviour under test, and TestOverflowAbortsOnlyItsCase already covers a
	// full six-launch second entry at the production budget.
	completed := 0
	for _, record := range b.Records {
		if record.InconclusiveState == nil {
			completed++
		}
	}
	if completed < 1 || b.RecordCount != b.Launches {
		t.Fatalf("second entry did not get a budget of its own: %+v", b)
	}
	if b.Status != procgroup.StatusPass && b.CaseInconclusiveState != procgroup.InconclusiveTimeout {
		t.Fatalf("second entry ended for a reason other than its own budget: %+v", b)
	}
}
func TestEffectsAndInstability(t *testing.T) {
	for _, tc := range []struct {
		name     string
		argv     []string
		declared []effect
		entries  []fileEntry
		status   procgroup.Status
		outcome  procgroup.Outcome
		effect   *observedEffect
	}{
		{"undeclared-create", []string{"--owned-fixture", "write", "a.txt", "created"}, nil, nil, procgroup.StatusFail, procgroup.OutcomeWithheld, &observedEffect{"a.txt", "create", "PRODUCED", "undeclared-observed"}},
		{"declared-create", []string{"--owned-fixture", "write", "a.txt", "created"}, []effect{{"a.txt", "create"}}, nil, procgroup.StatusPass, procgroup.OutcomeCompatible, &observedEffect{"a.txt", "create", "PRODUCED", "declared-observed"}},
		{"declared-absent", []string{"--owned-fixture", "stdout", "same"}, []effect{{"a.txt", "create"}}, nil, procgroup.StatusPass, procgroup.OutcomeCompatible, &observedEffect{"a.txt", "create", "NOT_PRODUCED", "declared-absent"}},
		{"undeclared-delete", []string{"--owned-fixture", "delete", "a.txt"}, nil, []fileEntry{{"a.txt", 0644, []byte("original"), false}}, procgroup.StatusFail, procgroup.OutcomeWithheld, &observedEffect{"a.txt", "delete", "PRODUCED", "undeclared-observed"}},
		{"declared-delete", []string{"--owned-fixture", "delete", "a.txt"}, []effect{{"a.txt", "delete"}}, []fileEntry{{"a.txt", 0644, []byte("original"), false}}, procgroup.StatusPass, procgroup.OutcomeCompatible, &observedEffect{"a.txt", "delete", "PRODUCED", "declared-observed"}},
		{"undeclared-mode", []string{"--owned-fixture", "mode", "a.txt"}, nil, []fileEntry{{"a.txt", 0644, []byte("original"), false}}, procgroup.StatusFail, procgroup.OutcomeWithheld, &observedEffect{"a.txt", "mode", "PRODUCED", "undeclared-observed"}},
		{"declared-mode", []string{"--owned-fixture", "mode", "a.txt"}, []effect{{"a.txt", "mode"}}, []fileEntry{{"a.txt", 0644, []byte("original"), false}}, procgroup.StatusPass, procgroup.OutcomeCompatible, &observedEffect{"a.txt", "mode", "PRODUCED", "declared-observed"}},
		{"unstable", []string{"--owned-fixture", "unstable"}, nil, nil, procgroup.StatusPass, procgroup.OutcomeInstability, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, p := fixtureManifest(t)
			pt := &p.Tasks[0]
			pt.Task.Argv = tc.argv
			pt.Task.ExpectedEffects = tc.declared
			pt.Snapshot, _ = canonicalSnapshot(tc.entries)
			c := replay(context.Background(), p).Cases[0]
			if c.Status != tc.status || c.Outcome != tc.outcome || c.RecordCount != 6 {
				t.Fatalf("%+v", c)
			}
			if tc.effect != nil && (len(c.Effects) != 1 || c.Effects[0] != *tc.effect) {
				t.Fatalf("effects=%+v", c.Effects)
			}
			if tc.outcome == procgroup.OutcomeInstability && c.CaseInconclusiveState != procgroup.InconclusiveInconsistent {
				t.Fatal(c)
			}
		})
	}
}
func TestByteComparisonNeverNormalizesOrVotes(t *testing.T) {
	done := procgroup.Observation{ExitStatus: 0, Stdout: []byte("{\"a\":1,\"b\":2}\n")}
	o := []procgroup.Observation{done, done, done, done, done, done}
	if compare(o) != procgroup.OutcomeCompatible {
		t.Fatal("equal rejected")
	}
	other := done
	other.Stdout = []byte("{\"b\":2,\"a\":1}\n")
	o[3], o[4], o[5] = other, other, other
	if compare(o) != procgroup.OutcomeDifferent {
		t.Fatal("JSON normalized")
	}
	o[3], o[4], o[5] = done, done, done
	o[1] = other
	if compare(o) != procgroup.OutcomeInstability {
		t.Fatal("majority voted")
	}
	other = done
	other.Stdout = bytes.TrimSuffix(done.Stdout, []byte("\n"))
	o = []procgroup.Observation{done, done, done, other, other, other}
	if compare(o) != procgroup.OutcomeDifferent {
		t.Fatal("newline normalized")
	}
}
func TestEffectUnionKeepsOneRepetitionAndLiteralSuffix(t *testing.T) {
	e := observedEffect{"foo!mode", "write", "PRODUCED", "undeclared-observed"}
	if got := unionEffects([]observedEffect{e}); len(got) != 1 || got[0] != e {
		t.Fatal(got)
	}
}

func TestDeclaredObservedEffectIsReported(t *testing.T) {
	before := map[string]fileState{"foo!mode": {Digest: "old", Mode: 0644}}
	after := map[string]fileState{"foo!mode": {Digest: "new", Mode: 0644}}
	want := observedEffect{"foo!mode", "write", "PRODUCED", "declared-observed"}
	if got := diffEffects(before, after, []effect{{"foo!mode", "write"}}); len(got) != 1 || got[0] != want {
		t.Fatal(got)
	}
}
func TestDescriptorChangesAreRefusedBeforeLaunch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(map[string]any)
		refusal procgroup.PrelaunchRefusal
	}{
		{"source-object", func(m map[string]any) { m["source"] = map[string]any{"path": "x", "sha256": strings.Repeat("0", 64)} }, procgroup.RefusalDescriptorInvalid},
		{"unregistered-executable", func(m map[string]any) {
			tasks(m)["old"].(map[string]any)["executable_sha256"] = strings.Repeat("0", 64)
		}, procgroup.RefusalFixtureNotRegistered},
		{"cwd-escape", func(m map[string]any) { tasks(m)["cwd"] = "../live" }, procgroup.RefusalDescriptorInvalid},
		{"environment-missing", func(m map[string]any) { tasks(m)["env"] = []any{} }, procgroup.RefusalDescriptorInvalid},
		{"effect-escape", func(m map[string]any) {
			tasks(m)["expected_effects"] = []any{map[string]any{"path": "../outside", "kind": "write"}}
		}, procgroup.RefusalDescriptorInvalid},
		{"arbitrary-argv", func(m map[string]any) { tasks(m)["argv"] = []any{"-manifest", "elsewhere"} }, procgroup.RefusalDescriptorInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, _ := fixtureManifest(t)
			var m map[string]any
			b, _ := os.ReadFile(path)
			_ = json.Unmarshal(b, &m)
			tc.mutate(m)
			b, _ = json.Marshal(m)
			_ = os.WriteFile(path, b, 0600)
			p, err := loadManifest(path, ownedRegistry(fixtureBinary))
			if err != nil {
				if tc.refusal != procgroup.RefusalDescriptorInvalid {
					t.Fatal(err)
				}
				return
			}
			if p.Tasks[0].Refusal != tc.refusal {
				t.Fatalf("%s: %s", p.Tasks[0].Refusal, p.Tasks[0].Detail)
			}
			c := replay(context.Background(), p).Cases[0]
			if c.Status != procgroup.StatusNotRun || c.Launches != 0 || len(c.Records) != 0 || c.UnobservedLaunches != 0 {
				t.Fatal(c)
			}
		})
	}
}
func tasks(m map[string]any) map[string]any { return m["tasks"].([]any)[0].(map[string]any) }

func TestOwnedVersionedFixturesReplayDifferentThroughPreflight(t *testing.T) {
	path, p := fixtureManifest(t)
	base := filepath.Dir(path)
	if err := os.WriteFile(filepath.Join(base, "old"), oldFixtureBinary, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "new"), newFixtureBinary, 0700); err != nil {
		t.Fatal(err)
	}
	task := &p.Manifest.Tasks[0]
	task.Old.Executable = "old"
	task.Old.ExecutableSHA256 = digest(oldFixtureBinary)
	task.New.Executable = "new"
	task.New.ExecutableSHA256 = digest(newFixtureBinary)
	data, _ := json.Marshal(p.Manifest)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := loadManifest(path, ownedRegistry(fixtureBinary))
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Tasks[0].Refusal != procgroup.RefusalNone {
		t.Fatal(prepared.Tasks[0].Detail)
	}
	c := replay(context.Background(), prepared).Cases[0]
	if c.Status != procgroup.StatusPass || c.Outcome != procgroup.OutcomeDifferent || c.RecordCount != 6 || c.Inconclusive != nil {
		t.Fatalf("%+v", c)
	}
	for _, r := range c.Records {
		if r.StdoutSHA256 != digest([]byte(r.Version+":exact\n")) {
			t.Fatalf("fixture version not actually replayed: %+v", r)
		}
	}
}

func TestUnexecutableOwnedBinaryIsStartFailureWithoutLaunch(t *testing.T) {
	path, _ := fixtureManifest(t)
	if err := os.Chmod(filepath.Join(filepath.Dir(path), "fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	p, err := loadManifest(path, ownedRegistry(fixtureBinary))
	if err != nil {
		t.Fatal(err)
	}
	c := replay(context.Background(), p).Cases[0]
	if c.Status != procgroup.StatusNotRun || c.Reason != procgroup.ReasonStartFailed || c.RecordCount != 0 || c.Launches != 0 || c.UnobservedLaunches != 0 {
		t.Fatalf("%+v", c)
	}
}
func TestTenSecondCeilingActivelyTerminates(t *testing.T) {
	_, p := fixtureManifest(t)
	pt := p.Tasks[0]
	pt.Task.Argv = []string{"--owned-fixture", "sleep", "30s"}
	start := time.Now()
	o, r, detail := repetition(context.Background(), p.SHA256, pt, pt.Old, "old", 1)
	elapsed := time.Since(start)
	if elapsed < 10*time.Second || elapsed > 11200*time.Millisecond {
		t.Fatalf("ceiling elapsed %s", elapsed)
	}
	if !o.TimedOut || !o.ExitObserved || !o.OwnedProcessGroupCleanup || r.InconclusiveState == nil || *r.InconclusiveState != procgroup.InconclusiveTimeout {
		t.Fatalf("%+v %s", o, detail)
	}
}

func TestUndeclaredEffectDoesNotEraseInstability(t *testing.T) {
	_, p := fixtureManifest(t)
	p.Tasks[0].Task.Argv = []string{"--owned-fixture", "unstable-write", "a.txt"}
	c := replay(context.Background(), p).Cases[0]
	if c.Status != procgroup.StatusFail || c.Outcome != procgroup.OutcomeWithheld || c.CaseInconclusiveState != procgroup.InconclusiveInconsistent || c.RecordCount != 6 || len(c.Effects) != 1 {
		t.Fatalf("%+v", c)
	}
}
