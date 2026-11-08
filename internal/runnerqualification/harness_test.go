// SPDX-License-Identifier: Apache-2.0

package runnerqualification

import (
	"context"
	"errors"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/langplugin"
	s "github.com/Beamfall/corvint/internal/runnerselection"
	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func fixture() Request {
	h := strings.Repeat("a", 64)
	commit := strings.Repeat("1", 40)
	d := Declaration{Runner: "fixture", Corpus: Corpus{"fixture-repository", commit}, Model: "traced", Classes: []s.Class{s.BodyEdit}, Mutation: []langplugin.SubPlugin{{Name: "fixture-mutation", Paths: []string{"fixture/provider"}}}, Identity: Identity{Runner: testevidence.RunnerIdentity{Plugin: "fixture-plugin", PluginVersion: "1", ExecutableDigest: h, RunnerVersion: "1", ConfigurationDigest: h}, Execution: testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "fixture-trace", TracingVersion: "1", EnvironmentDigest: h}, SelectorVersion: "1", RecordFormat: testevidence.Format, NonSemanticDigest: h, VerifierRevision: commit}}
	return Request{Declaration: d, Checkout: "/operator/corpus", Samples: TrialFloor, Seed: 591, Budget: time.Minute, Merges: []Merge{{Commit: strings.Repeat("3", 40), Base: strings.Repeat("2", 40), Head: strings.Repeat("3", 40), Classes: []s.Class{s.BodyEdit}}}}
}

type fixtureDriver struct {
	change     func(Experiment, *Pair)
	population func(s.Class) Population
	verifyErr  error
	captures   []RetainedCapture
}

func (d *fixtureDriver) Verify(context.Context, Declaration, string, []Merge) error {
	return d.verifyErr
}
func (d *fixtureDriver) Population(_ context.Context, _ Declaration, _ string, c s.Class) (Population, error) {
	if d.population != nil {
		return d.population(c), nil
	}
	return Population{Complete: true, Mutants: []Mutant{{ID: "one", Operator: "negate-condition", Piece: "fixture-mutation", Class: c, Edits: []Edit{{"subject.source", []byte("true"), []byte("false")}}}}}, nil
}
func observation(d Declaration, commit string) Observation {
	return Observation{Inventory: testrunner.RunnerInventory{Key: testrunner.InventoryKey{Checkout: commit, Runner: d.Runner, ConfigDigest: d.Identity.Runner.ConfigurationDigest, OS: d.Identity.Execution.OS, Arch: d.Identity.Execution.Architecture}, Complete: true, Tests: []testrunner.InventoryTest{{ID: "a", Path: "a.test"}, {ID: "b", Path: "b.test"}}}, Results: []testrunner.RunnerResult{{ID: "a", Outcome: "passed"}, {ID: "b", Outcome: "passed"}}, Plan: testrunner.RunSet{Tests: []string{"a"}}, TimingsObserved: true, InventoryWall: time.Millisecond, CollectionWall: 2 * time.Millisecond}
}
func (d *fixtureDriver) Capture(_ context.Context, decl Declaration, _ string, e Experiment) (Pair, error) {
	p := Pair{Traced: observation(decl, e.Commit), Untraced: observation(decl, e.Commit)}
	if e.Mutant != nil || e.Commit == strings.Repeat("3", 40) {
		p.Traced.Results[0].Outcome = "failed"
		p.Untraced.Results[0].Outcome = "failed"
	}
	if e.Mutant != nil && e.Mutant.Class == s.RunnerConfig || e.Commit == strings.Repeat("3", 40) {
		p.Traced.Plan.Full = true
		p.Untraced.Plan.Full = true
	}
	if d.change != nil {
		d.change(e, &p)
	}
	d.captures = append(d.captures, RetainedCapture{e, p})
	return p, nil
}
func mustPass(t *testing.T, r Record) {
	t.Helper()
	if !r.Complete || len(r.Reasons) > 0 || len(r.Classes) == 0 {
		t.Fatalf("incomplete: %+v", r)
	}
	for _, c := range r.Classes {
		if !c.Passed {
			t.Fatalf("unqualified: %+v", c)
		}
	}
}
func mustBlock(t *testing.T, r Record, reason string) {
	t.Helper()
	if strings.Contains(strings.Join(r.Reasons, ","), reason) {
		return
	}
	for _, c := range r.Classes {
		if !c.Passed && strings.Contains(strings.Join(c.Reasons, ","), reason) {
			return
		}
	}
	t.Fatalf("did not block for %q: %+v", reason, r)
}

func TestRSPV0033PinnedDeclarationAndMutationPiece(t *testing.T) {
	req := fixture()
	d := &fixtureDriver{}
	r := Run(context.Background(), req, d)
	mustPass(t, r)
	if !reflect.DeepEqual(r.Declaration, req.Declaration) {
		t.Fatal("lost pinned declaration")
	}
	if r.Classes[0].PopulationSize != 1 {
		t.Fatal("population lost")
	}
}
func TestRSPV0033MissingPinsAndMutationPiece(t *testing.T) {
	for _, f := range []func(*Request){func(r *Request) { r.Declaration.Corpus.Commit = "HEAD" }, func(r *Request) { r.Declaration.Mutation = nil }, func(r *Request) { r.Declaration.Model = "unknown" }} {
		req := fixture()
		f(&req)
		r := Run(context.Background(), req, &fixtureDriver{})
		if len(r.Reasons) == 0 && len(r.Classes) > 0 && r.Classes[0].Passed {
			t.Fatalf("invalid declaration passed: %+v", r)
		}
	}
	mustBlock(t, Run(context.Background(), fixture(), &fixtureDriver{verifyErr: errors.New("wrong corpus")}), "corpus-verification")
}
func TestRSPV0033LanguageNeutralOperators(t *testing.T) {
	cases := []struct {
		class   s.Class
		targets []FileTarget
	}{
		{s.DataEdit, []FileTarget{{Path: "data", Bytes: []byte("ab"), Class: s.DataEdit, Kind: "data"}}},
		{s.Add, []FileTarget{{Path: "dir/new", Class: s.Add, Kind: "listed-add", DestinationAbsent: true}, {Path: "missing", Class: s.Add, Kind: "lookup-add", DestinationAbsent: true}}},
		{s.DeleteRename, []FileTarget{{Path: "old", Bytes: []byte{}, Class: s.DeleteRename, Kind: "delete"}, {Path: "old", Bytes: []byte("x"), Class: s.DeleteRename, Kind: "rename", Destination: "new", DestinationAbsent: true}}},
		{s.ExecTarget, []FileTarget{{Path: "script", Bytes: []byte("x"), Class: s.ExecTarget, Kind: "exec"}}},
		{s.RunnerConfig, []FileTarget{{Path: "config", Bytes: []byte("x"), Class: s.RunnerConfig, Kind: "configuration"}}},
	}
	for _, c := range cases {
		p := FilePopulation(c.targets)
		if err := ValidatePopulation(fixture().Declaration, c.class, p); err != nil {
			t.Fatalf("%s: %v", c.class, err)
		}
		if !reflect.DeepEqual(p, FilePopulation(c.targets)) {
			t.Fatal("nondeterministic population")
		}
	}
}
func TestRSPV0033MalformedAndCappedPopulation(t *testing.T) {
	for _, target := range []FileTarget{{Path: "../escape", Class: s.DataEdit, Kind: "data", Bytes: []byte("x")}, {Path: "x", Class: s.Add, Kind: "lookup-add"}, {Path: "x", Destination: "../out", Class: s.DeleteRename, Kind: "rename", Bytes: []byte("x"), DestinationAbsent: true}, {Path: "x", Class: s.DataEdit, Kind: "data", Bytes: make([]byte, 50000)}} {
		if FilePopulation([]FileTarget{target}).Complete {
			t.Fatal("malformed or capped population complete")
		}
	}
	p := FilePopulation([]FileTarget{{Path: "x", Class: s.Add, Kind: "listed-add", DestinationAbsent: true}})
	if ValidatePopulation(fixture().Declaration, s.Add, p) == nil {
		t.Fatal("missing lookup control operator accepted")
	}
}

func TestRSPV0034ReplayAndPerClassFaults(t *testing.T) {
	for _, class := range []s.Class{s.BodyEdit, s.DeclarationEdit, s.DataEdit, s.Add, s.DeleteRename, s.ExecTarget, s.RunnerConfig, s.E2EServer} {
		t.Run(string(class), func(t *testing.T) {
			req := fixture()
			req.Declaration.Classes = []s.Class{class}
			req.Merges[0].Classes = []s.Class{class}
			d := &fixtureDriver{population: func(c s.Class) Population {
				switch c {
				case s.DataEdit:
					return FilePopulation([]FileTarget{{Path: "subject", Bytes: []byte("a"), Class: c, Kind: "data"}})
				case s.Add:
					return FilePopulation([]FileTarget{{Path: "listed/new", Class: c, Kind: "listed-add", DestinationAbsent: true}, {Path: "lookup", Class: c, Kind: "lookup-add", DestinationAbsent: true}})
				case s.DeleteRename:
					return FilePopulation([]FileTarget{{Path: "subject", Bytes: []byte("a"), Class: c, Kind: "delete"}, {Path: "subject", Bytes: []byte("a"), Class: c, Kind: "rename", Destination: "new", DestinationAbsent: true}})
				case s.ExecTarget:
					return FilePopulation([]FileTarget{{Path: "subject", Bytes: []byte("a"), Class: c, Kind: "exec"}})
				case s.RunnerConfig:
					return FilePopulation([]FileTarget{{Path: "subject", Bytes: []byte("a"), Class: c, Kind: "configuration"}})
				default:
					return Population{Complete: true, Mutants: []Mutant{{ID: "language", Operator: "language-operator", Piece: "fixture-mutation", Class: c, Edits: []Edit{{"subject", []byte("a"), []byte("b")}}}}}
				}
			}}
			mustPass(t, Run(context.Background(), req, d))
		})
	}
}
func TestRSPV0034SeededMissBlocksClass(t *testing.T) {
	d := &fixtureDriver{change: func(e Experiment, p *Pair) {
		if e.Mutant != nil {
			p.Traced.Plan.Tests = []string{"b"}
		}
	}}
	r := Run(context.Background(), fixture(), d)
	mustBlock(t, r, "sampled-mutant-miss")
	if len(r.Classes[0].Misses) != TrialFloor || r.Classes[0].Upper95 != 1 {
		t.Fatal("misses or confidence limit lost")
	}
}
func TestRSPV0034ReplayControlParityAndIncompleteFailures(t *testing.T) {
	cases := []struct {
		name, reason string
		change       func(Experiment, *Pair)
	}{
		{"replay", "replay-miss", func(e Experiment, p *Pair) {
			if e.Commit == strings.Repeat("3", 40) {
				p.Traced.Plan = testrunner.RunSet{Tests: []string{"b"}}
			}
		}},
		{"control", "negative-control", func(e Experiment, p *Pair) {
			if e.Control != "" {
				p.Traced.Results[0].Outcome = "failed"
				p.Untraced.Results[0].Outcome = "failed"
			}
		}},
		{"parity", "tracing-outcome-mismatch", func(e Experiment, p *Pair) {
			if e.Mutant != nil {
				p.Untraced.Results[0].Outcome = "passed"
			}
		}},
		{"result", "full-results-incomplete", func(e Experiment, p *Pair) {
			if e.Mutant != nil {
				p.Traced.Results = p.Traced.Results[:1]
			}
		}},
		{"inventory", "inventory-incomplete", func(e Experiment, p *Pair) {
			if e.Mutant != nil {
				p.Traced.Inventory.Complete = false
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mustBlock(t, Run(context.Background(), fixture(), &fixtureDriver{change: c.change}), c.reason)
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mustBlock(t, Run(ctx, fixture(), &fixtureDriver{}), "cancellation")
}

func TestRSPV0035TrialsBoundsAndRecordedDraws(t *testing.T) {
	req := fixture()
	a := Run(context.Background(), req, &fixtureDriver{})
	b := Run(context.Background(), req, &fixtureDriver{})
	mustPass(t, a)
	c := a.Classes[0]
	if c.Trials != 300 || c.Sampled != 300 || c.Unkilled != 0 || !reflect.DeepEqual(c.Draws, b.Classes[0].Draws) || c.Seed != req.Seed {
		t.Fatal("sample accounting lost")
	}
	if math.Abs(c.Upper95-0.00993608194445771) > 1e-14 {
		t.Fatalf("zero-miss upper=%g", c.Upper95)
	}
	// With one miss in two trials, CDF(1;2,p)=1-p²=0.05.
	if math.Abs(Upper95(1, 2)-math.Sqrt(0.95)) > 1e-13 {
		t.Fatal("nonzero-miss bound incorrect")
	}
}
func TestRSPV0035FloorAndUnkilledNeverQualify(t *testing.T) {
	req := fixture()
	req.Samples = 299
	mustBlock(t, Run(context.Background(), req, &fixtureDriver{}), "trial-floor:299/300")
	req.Samples = 300
	d := &fixtureDriver{change: func(e Experiment, p *Pair) {
		if e.Mutant != nil {
			p.Traced.Results[0].Outcome = "passed"
			p.Untraced.Results[0].Outcome = "passed"
		}
	}}
	r := Run(context.Background(), req, d)
	mustBlock(t, r, "trial-floor:0/300")
	if r.Classes[0].Unkilled != 300 || r.Classes[0].Upper95 != 1 {
		t.Fatal("unkilled trials or zero probability claimed")
	}
	for _, v := range [][2]int{{0, 0}, {-1, 5}, {6, 5}, {0, MaxSamples + 1}} {
		if Upper95(v[0], v[1]) != 1 {
			t.Fatal("invalid statistics narrowed")
		}
	}
}

func TestRSPV0035UniformDrawUsesWholePopulation(t *testing.T) {
	population := func(c s.Class) Population {
		p := Population{Complete: true}
		for _, id := range []string{"c", "a", "b"} {
			p.Mutants = append(p.Mutants, Mutant{ID: id, Operator: "condition", Piece: "fixture-mutation", Class: c, Edits: []Edit{{"subject", []byte("a"), []byte("b")}}})
		}
		return p
	}
	req := fixture()
	a := Run(context.Background(), req, &fixtureDriver{population: population})
	mustPass(t, a)
	req.Seed++
	b := Run(context.Background(), req, &fixtureDriver{population: population})
	mustPass(t, b)
	if slices.Equal(a.Classes[0].Draws, b.Classes[0].Draws) {
		t.Fatal("seed ignored")
	}
	for _, id := range []string{"a", "b", "c"} {
		if !slices.Contains(a.Classes[0].Draws, id) {
			t.Fatal("population member omitted")
		}
	}
	if a.Classes[0].PopulationDigest != b.Classes[0].PopulationDigest {
		t.Fatal("population digest depends on draw")
	}
}

func TestRSPV0034ConfigurationMustBeFull(t *testing.T) {
	req := fixture()
	req.Declaration.Classes = []s.Class{s.RunnerConfig}
	req.Merges[0].Classes = []s.Class{s.RunnerConfig}
	d := &fixtureDriver{population: func(c s.Class) Population {
		return FilePopulation([]FileTarget{{Path: "config", Bytes: []byte("a"), Class: c, Kind: "configuration"}})
	}, change: func(e Experiment, p *Pair) {
		if e.Mutant != nil {
			p.Traced.Plan.Full = false
		}
	}}
	mustBlock(t, Run(context.Background(), req, d), "configuration-not-full")
}

func TestRSPV0056CollectingRecordSelectsWithoutWaiting(t *testing.T) {
	req := fixture()
	d := req.Declaration
	r := Run(context.Background(), req, &fixtureDriver{})
	qualified := PlanFor(d, []Admission{{Record: r, Admitted: true}}, time.Minute, true)
	head := req.Merges[0].Head
	inventory := observation(d, head).Inventory
	piece := testrunner.RunnerPiece{Present: true}
	runner := s.Runner{Name: d.Runner, Identity: d.Identity.Runner, Execution: d.Identity.Execution, Pieces: testrunner.RunnerPieces{Inventory: piece, TestEvidence: piece, Qualification: piece}, Inventory: inventory, Qualification: qualified.Qualification, IndexBase: d.Corpus.Commit, IndexVerified: true}
	in := s.Input{Base: d.Corpus.Commit, Head: head, Repository: d.Corpus.Repository, Repositories: map[string]bool{d.Corpus.Repository: true}, DiffComplete: true, DirectoriesComplete: true, Snapshot: testevidence.Snapshot{}, Runners: []s.Runner{runner}}
	plan := s.Select(in)
	if len(plan.Runners) != 1 || len(plan.Runners[0].Selected) != len(inventory.Tests) || len(plan.Runners[0].Excluded) != 0 {
		t.Fatalf("missing or collecting evidence excluded a test: %+v", plan)
	}
	for _, selected := range plan.Runners[0].Selected {
		if !slices.Contains(selected.Rules, "record-missing") {
			t.Fatalf("missing record reason lost: %+v", selected)
		}
	}
}

func TestRSPV0036ExactAdmittedRecordAndJob(t *testing.T) {
	req := fixture()
	r := Run(context.Background(), req, &fixtureDriver{})
	p := PlanFor(req.Declaration, []Admission{{Record: r, Admitted: true, Sequence: 1}}, time.Minute, true)
	if p.Mode != "selected" || p.Status != "qualified" || !p.Qualification.Passed || len(p.Jobs) != 0 {
		t.Fatalf("exact record refused: %+v", p)
	}
	p = PlanFor(req.Declaration, nil, time.Minute, true)
	if p.Mode != "full" || p.Status != "requalifying" || len(p.Jobs) != 1 || p.Jobs[0].Budget != time.Minute {
		t.Fatalf("no bounded job: %+v", p)
	}
}
func TestRSPV0036EveryIdentityChangeInvalidates(t *testing.T) {
	req := fixture()
	r := Run(context.Background(), req, &fixtureDriver{})
	changes := map[string]func(*Identity){
		"plugin-digest": func(i *Identity) { i.Runner.ExecutableDigest = strings.Repeat("b", 64) }, "runner": func(i *Identity) { i.Runner.RunnerVersion = "2" }, "backend": func(i *Identity) { i.Execution.TracingBackend = "other" }, "trace-version": func(i *Identity) { i.Execution.TracingVersion = "2" }, "selector": func(i *Identity) { i.SelectorVersion = "2" }, "format": func(i *Identity) { i.RecordFormat = "future/2" }, "configuration": func(i *Identity) { i.Runner.ConfigurationDigest = strings.Repeat("b", 64) }, "environment": func(i *Identity) { i.Execution.EnvironmentDigest = strings.Repeat("b", 64) }, "nonsemantic-list": func(i *Identity) { i.NonSemanticDigest = strings.Repeat("b", 64) }, "verifier": func(i *Identity) { i.VerifierRevision = strings.Repeat("b", 40) }, "os": func(i *Identity) { i.Execution.OS = "other" }, "arch": func(i *Identity) { i.Execution.Architecture = "other" }, "plugin-version": func(i *Identity) { i.Runner.PluginVersion = "2" },
	}
	for name, f := range changes {
		t.Run(name, func(t *testing.T) {
			d := req.Declaration
			f(&d.Identity)
			p := PlanFor(d, []Admission{{Record: r, Admitted: true, Sequence: 1}}, time.Minute, true)
			if p.Mode != "full" || p.Status != "requalifying" || len(p.Jobs) != 1 || p.Jobs[0].Identity != d.Identity {
				t.Fatalf("identity retained or job lost: %+v", p)
			}
		})
	}
	for _, records := range [][]Admission{{{Record: r, Admitted: false, Sequence: 1}}, {{Record: r, Admitted: true, Sequence: 1}, {Record: Record{Declaration: req.Declaration}, Admitted: true, Sequence: 2}}} {
		if PlanFor(req.Declaration, records, time.Minute, true).Mode != "full" {
			t.Fatal("unadmitted or older passing record used")
		}
	}
	if len(PlanFor(req.Declaration, nil, 0, true).Jobs) != 0 {
		t.Fatal("invented CI ceiling")
	}
}

func TestRSPV0056MeasuredCollectionAndInventoryCosts(t *testing.T) {
	r := Run(context.Background(), fixture(), &fixtureDriver{})
	mustPass(t, r)
	if len(r.Timings) != 2*(TrialFloor+4) {
		t.Fatalf("missing timings: %d", len(r.Timings))
	}
	for _, cost := range r.Timings {
		if cost.InventoryWall != time.Millisecond || cost.CollectionWall != 2*time.Millisecond {
			t.Fatal("cost changed")
		}
	}
}
func TestRSPV0056UnobservedCostStaysUnqualified(t *testing.T) {
	for _, change := range []func(Experiment, *Pair){func(_ Experiment, p *Pair) { p.Traced.TimingsObserved = false }, func(_ Experiment, p *Pair) { p.Traced.CollectionWall = -1 }} {
		mustBlock(t, Run(context.Background(), fixture(), &fixtureDriver{change: change}), "time-unobserved")
	}
}

func TestRSPV0034RetainedReplayBindsEveryDraw(t *testing.T) {
	req := fixture()
	d := &fixtureDriver{}
	mustPass(t, Run(context.Background(), req, d))
	pop, _ := d.Population(context.Background(), req.Declaration, req.Checkout, s.BodyEdit)
	r := Replay{Request: req, Populations: []ClassPopulation{{s.BodyEdit, pop}}, Captures: d.captures}
	verify := func(context.Context, Declaration, string, []Merge) error { return nil }
	mustPass(t, ReplayRetained(context.Background(), r, verify))
	r.Captures = r.Captures[:len(r.Captures)-1]
	mustBlock(t, ReplayRetained(context.Background(), r, verify), "retained-capture-missing")
	r.Captures = d.captures
	r.Captures[4].Experiment.Commit = strings.Repeat("9", 40)
	mustBlock(t, ReplayRetained(context.Background(), r, verify), "retained-experiment-mismatch")
}
