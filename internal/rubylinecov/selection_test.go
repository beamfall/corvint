// SPDX-License-Identifier: Apache-2.0

package rubylinecov

import (
	"crypto/sha1"
	"fmt"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
)

func selectionFixture(source string) (*Run, map[string]SelectionSource) {
	b := []byte(source)
	oid := fmt.Sprintf("%x", sha1.Sum(append([]byte(fmt.Sprintf("blob %d\x00", len(b))), b...)))
	return &Run{
		Runner: "ruby 3.3 minitest 5 coverage=lines,methods", isolated: true,
		Outcomes: []Outcome{{Package: "test/calc_test.rb", Name: "CalcTest#test_add", Status: "passed", Recorded: true}},
		records:  []record{{pkg: "test/calc_test.rb", name: "CalcTest#test_add", lines: map[string][]int32{"lib/calc.rb": {2}}}, {pkg: Load, name: Load, lines: map[string][]int32{"lib/calc.rb": {1}}}},
		files:    map[string]fileShape{"lib/calc.rb": {Lines: []int32{1, 2, 3}}},
	}, map[string]SelectionSource{"lib/calc.rb": {OID: oid, Bytes: b}}
}
func TestRSPV0005RubyCoverageLinesAndLoad(t *testing.T) {
	run, sources := selectionFixture("def add(n)\n n + 1\nend\n")
	evidence, err := SelectionEvidence(run, "repo", sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("%+v", evidence)
	}
	n := evidence[0].Native
	if n.Executed.State != testevidence.Present || len(n.Executed.Items) != 1 {
		t.Fatalf("%+v", n)
	}
	f := n.Executed.Items[0]
	if len(f.Executed) != 2 || f.Executed[0].Start != 1 || f.Executed[1].Start != 2 || len(f.Unexecuted) != 1 || f.Unexecuted[0].Start != 3 {
		t.Fatalf("%+v", f)
	}
	if f.Blob.OID != sources["lib/calc.rb"].OID || n.Loaded.State != testevidence.Absent {
		t.Fatalf("%+v", n)
	}
}
func TestRSPV0005RubyCoverageFailsClosed(t *testing.T) {
	for name, change := range map[string]func(*Run, map[string]SelectionSource){
		"nonisolated":    func(r *Run, _ map[string]SelectionSource) { r.isolated = false },
		"failed":         func(r *Run, _ map[string]SelectionSource) { r.Outcomes[0].Status = "failed" },
		"incomplete":     func(r *Run, _ map[string]SelectionSource) { r.Outcomes[0].Recorded = false },
		"missing-load":   func(r *Run, _ map[string]SelectionSource) { r.records = r.records[:1] },
		"missing-record": func(r *Run, _ map[string]SelectionSource) { r.records = r.records[1:] },
		"missing-shape":  func(r *Run, _ map[string]SelectionSource) { r.files = map[string]fileShape{} },
		"unknown-line":   func(r *Run, _ map[string]SelectionSource) { r.records[0].lines["lib/calc.rb"] = []int32{12} },
		"bad-shape-line": func(r *Run, _ map[string]SelectionSource) {
			r.files["lib/calc.rb"] = fileShape{Lines: []int32{-1, 1, 2, 100}}
		},
		"missing-source": func(_ *Run, s map[string]SelectionSource) { delete(s, "lib/calc.rb") },
		"blob-drift": func(_ *Run, s map[string]SelectionSource) {
			f := s["lib/calc.rb"]
			f.Bytes = []byte("different")
			s["lib/calc.rb"] = f
		},
	} {
		t.Run(name, func(t *testing.T) {
			r, s := selectionFixture("def add(n)\n n + 1\nend\n")
			change(r, s)
			got, err := SelectionEvidence(r, "repo", s)
			if err != nil {
				return
			}
			if len(got) != 1 || got[0].Native.Executed.State == testevidence.Present || got[0].Native.Executed.Reason == "" {
				t.Fatalf("%+v", got)
			}
		})
	}
	if _, err := SelectionEvidence(nil, "repo", nil); err == nil {
		t.Fatal("missing capture accepted")
	}
}
func TestRSPV0048RubyDialectRegionsAndCoreTrace(t *testing.T) {
	r, s := selectionFixture("def add(n)\n n + 1\nend\n")
	got, err := SelectionEvidence(r, "repo", s)
	if err != nil {
		t.Fatal(err)
	}
	n := got[0].Native
	if n.Regions.State != testevidence.Present || len(n.Regions.Items) != 1 || len(n.Regions.Items[0].Bodies) != 1 {
		t.Fatalf("%+v", n.Regions)
	}
	body := n.Regions.Items[0].Bodies[0]
	if body.Range.Start != 1 || body.Range.End != 4 {
		t.Fatalf("whole-file region: %+v", body)
	}
	h := testevidence.New("repo", got[0].Name, strings.Repeat("a", 40))
	digest := strings.Repeat("b", 64)
	h.Runner = testevidence.RunnerIdentity{Plugin: "ruby", PluginVersion: "fixture", ExecutableDigest: digest, RunnerVersion: "fixture", ConfigurationDigest: digest}
	h.Execution = testevidence.ExecutionIdentity{OS: "linux", Architecture: "amd64", TracingBackend: "fixture", TracingVersion: "1", EnvironmentDigest: digest}
	h.Sources = []testevidence.Producer{{CoreDigest: digest, VerifierRevision: h.Commit, ConfigurationDigest: digest, Ref: "refs/heads/main", Run: "fixture", ContentDigest: digest}}
	h.CollectionCount = 1
	blob := n.Executed.Items[0].Blob
	trace := testevidence.Trace{Reads: testevidence.Component[testevidence.Blob]{State: testevidence.Present, Items: []testevidence.Blob{blob}}, Listings: testevidence.Missing[testevidence.Path]("untraced"), Lookups: testevidence.Missing[testevidence.Path]("untraced"), Writes: testevidence.Missing[testevidence.Write]("untraced"), Execs: testevidence.Missing[testevidence.Executable]("untraced"), External: testevidence.Missing[testevidence.Contact]("untraced")}
	assembled, err := testevidence.Assemble(h, n, trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(assembled.Reads.Items) != 1 || assembled.Reads.Items[0] != blob || assembled.Listings.State != testevidence.Absent {
		t.Fatalf("trace replaced by native components: %+v", assembled)
	}
}
func TestRSPV0048RubyComputedReadsAndMalformedDialectWiden(t *testing.T) {
	for _, source := range []string{"def add(n)\n ENV.fetch(n)\nend\n", "def add(n)\n File.read(n)\nend\n", "def add(n)\n File.read(ENV.fetch('FIXTURE'))\nend\n"} {
		r, s := selectionFixture(source)
		got, err := SelectionEvidence(r, "repo", s)
		if err != nil {
			t.Fatal(err)
		}
		if got[0].Native.Executed.State != testevidence.Incomplete || got[0].Native.Executed.Reason != "ruby-computed-read-or-dispatch" {
			t.Fatalf("%+v", got)
		}
	}
	r, s := selectionFixture("def add(n)\n 'unterminated\nend\n")
	got, err := SelectionEvidence(r, "repo", s)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Native.Regions.State != testevidence.Absent || got[0].Native.Regions.Reason != "ruby-dialect-incomplete" {
		t.Fatalf("%+v", got)
	}
}
func TestRSPV0048RubyCaptureCapsAndDuplicateIdentity(t *testing.T) {
	r, s := selectionFixture("def add(n)\n n + 1\nend\n")
	r.Outcomes = append(r.Outcomes, r.Outcomes[0])
	if _, err := SelectionEvidence(r, "repo", s); err == nil {
		t.Fatal("duplicate accepted")
	}
	r.Outcomes = make([]Outcome, 4097)
	if _, err := SelectionEvidence(r, "repo", s); err == nil {
		t.Fatal("cap accepted")
	}
}
