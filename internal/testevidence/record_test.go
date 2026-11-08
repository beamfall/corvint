// SPDX-License-Identifier: Apache-2.0
package testevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fixture() Record {
	r := New("repo", "suite/test", strings.Repeat("a", 40))
	r.Runner = RunnerIdentity{"plugin", "1", strings.Repeat("b", 64), "runner-1", strings.Repeat("c", 64)}
	r.Execution = ExecutionIdentity{"linux", "amd64", "trace", "1", strings.Repeat("d", 64)}
	r.Sources = []Producer{{strings.Repeat("e", 64), strings.Repeat("f", 40), strings.Repeat("c", 64), "refs/heads/main", "run-1", strings.Repeat("1", 64)}}
	r.CollectionCount = 1
	r.Executed = Component[ExecutedFile]{State: Present}
	r.Regions = Component[RegionsFile]{State: Present}
	r.Loaded = Component[Blob]{State: Present}
	r.Reads = Component[Blob]{State: Present}
	r.Listings = Component[Path]{State: Present}
	r.Lookups = Component[Path]{State: Present}
	r.Writes = Component[Write]{State: Present}
	r.Execs = Component[Executable]{State: Present}
	r.External = Component[Contact]{State: Present}
	return r
}
func blob() Blob { return Blob{Path: Path{"repo", "src.go", "src.go"}, OID: strings.Repeat("2", 40)} }
func clone(r Record) Record {
	b, _ := json.Marshal(r)
	var out Record
	_ = json.Unmarshal(b, &out)
	return out
}
func next(r Record) Record { r = clone(r); r.Sources[0].Run = "run-2"; return r }
func requireError(t *testing.T, err error, reason string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), reason) {
		t.Fatalf("want %s, got %v", reason, err)
	}
}

// RSP-V0-006: every component state has a committed round-trip golden.
func TestRSPV0006RoundTripStates(t *testing.T) {
	for _, state := range []State{Present, Absent, Incomplete} {
		t.Run(string(state), func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join("testdata", string(state)+".json"))
			if err != nil {
				t.Fatal(err)
			}
			r, err := Decode(bytes.TrimSpace(b))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Encode(r)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, bytes.TrimSpace(b)) {
				t.Fatal("golden round trip differs")
			}
			if r.Executed.State != state || r.Regions.State != state || r.Loaded.State != state || r.Reads.State != state || r.Listings.State != state || r.Lookups.State != state || r.Writes.State != state || r.Execs.State != state || r.External.State != state {
				t.Fatal("component state lost")
			}
		})
	}
	r := fixture()
	b := blob()
	r.Executed.Items = []ExecutedFile{{b, []Range{{1, 1, 2, 2}}, []Range{{3, 1, 3, 2}}}}
	r.Regions.Items = []RegionsFile{{b, []Body{{"outer", "", Range{1, 1, 5, 2}}, {"inner", "outer", Range{2, 1, 3, 2}}}}}
	r.Loaded.Items = []Blob{b}
	r.Reads.Items = []Blob{b}
	r.Listings.Items = []Path{{"repo", ".", "."}}
	r.Lookups.Items = []Path{{"repo", "missing", "missing"}}
	r.Writes.Items = []Write{{b.Path, Append}}
	r.Execs.Items = []Executable{{b, strings.Repeat("3", 64), false}}
	r.External.Items = []Contact{{"loopback"}}
	encoded, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("payload round trip: %v", err)
	}
}

// RSP-V0-006: malformed, unknown and capped evidence is never an empty success.
func TestRSPV0006RejectMalformed(t *testing.T) {
	b, _ := Encode(fixture())
	for _, bad := range [][]byte{[]byte(`{}`), append(append([]byte(nil), b...), b...), bytes.Replace(b, []byte(`"state":"present"`), []byte(`"state":"unknown"`), 1), bytes.Replace(b, []byte(`"test":"suite/test"`), []byte(`"test":"suite/test","test":"other"`), 1), bytes.Replace(b, []byte(`"test":"suite/test"`), []byte(`"test":"suite/test","unknown":true`), 1), bytes.Repeat([]byte(" "), MaxRecordBytes+1), append([]byte{0xff}, b...)} {
		if _, err := Decode(bad); err == nil {
			t.Fatal("accepted malformed input")
		}
	}
	for _, mutate := range []func(*Record){
		func(r *Record) { r.Reads.State = Absent; r.Reads.Reason = "" },
		func(r *Record) {
			r.Reads = Component[Blob]{State: Present, Items: []Blob{{Path: Path{"repo", "../escape", "../escape"}, OID: blob().OID}}}
		},
		func(r *Record) { r.Executed.Items = []ExecutedFile{{blob(), []Range{{0, 1, 2, 1}}, nil}} },
		func(r *Record) {
			r.Regions.Items = []RegionsFile{{blob(), []Body{{"body", "body", Range{1, 1, 2, 1}}}}}
		},
		func(r *Record) { r.Writes.Items = []Write{{blob().Path, "unknown"}} },
		func(r *Record) { r.External.Items = []Contact{{"unknown"}} },
		func(r *Record) { r.CollectionCount = 2 },
	} {
		r := fixture()
		mutate(&r)
		if _, err := Encode(r); err == nil {
			t.Fatal("accepted malformed component")
		}
	}
}

// RSP-V0-005: native artifacts and core trace occupy separate input types.
func TestRSPV0005Assemble(t *testing.T) {
	r := fixture()
	native := Native{r.Executed, r.Regions, r.Loaded}
	trace := Trace{r.Reads, r.Listings, r.Lookups, r.Writes, r.Execs, r.External, r.Outside}
	trace.Reads.Items = []Blob{blob()}
	got, err := Assemble(r, native, trace)
	if err != nil || len(got.Reads.Items) != 1 {
		t.Fatalf("assemble: %v", err)
	}
	trace.Reads.State = "plugin-asserted"
	_, err = Assemble(r, native, trace)
	requireError(t, err, "unknown-state")
	// No filesystem/store capability is passed to a native producer.
	if reflect.TypeFor[Native]().NumField() != 3 {
		t.Fatal("native producer surface widened")
	}
}

// RSP-V0-007: only proven newly-created content may erase a read dependency.
func TestRSPV0007WriteProof(t *testing.T) {
	b := blob()
	base := WriteProof{Write: Write{b.Path, Create}, SameTree: true, BeforeRead: true, Succeeded: true}
	for _, kind := range []string{"exclusive", "lookup", "rename"} {
		p := base
		switch kind {
		case "exclusive":
			p.ExclusiveCreate = true
		case "lookup":
			p.OwnFailedLookup = true
		case "rename":
			p.Write.Kind = Replace
			p.RenamedTreeCreatedFile = true
		}
		if !DropRead(b, p) {
			t.Fatalf("kept proven %s", kind)
		}
	}
	for _, kind := range []WriteKind{Create, Replace, Append, Overwrite, Truncate} {
		p := base
		p.Write.Kind = kind
		if kind != Create && kind != Replace {
			p.ExclusiveCreate = true
			p.RenamedTreeCreatedFile = true
		}
		if DropRead(b, p) {
			t.Fatalf("dropped %s", kind)
		}
	}
	for _, mutate := range []func(*WriteProof){func(p *WriteProof) { p.SameTree = false }, func(p *WriteProof) { p.BeforeRead = false }, func(p *WriteProof) { p.Succeeded = false }, func(p *WriteProof) { p.Write.Path.Repository = "other" }, func(p *WriteProof) { p.Write.Path.Resolved = "other" }} {
		p := base
		p.ExclusiveCreate = true
		mutate(&p)
		if DropRead(b, p) {
			t.Fatal("dropped unproved read")
		}
	}
}

// RSP-V0-007: creating the target does not prove a tracked link is independent.
func TestRSPV0007WriteProofSymlink(t *testing.T) {
	read := blob()
	read.Path = Path{"repo", "link", "generated"}
	read.LexicalOID = strings.Repeat("3", 40)
	for _, kind := range []WriteKind{Create, Replace} {
		t.Run(string(kind), func(t *testing.T) {
			proof := WriteProof{
				Write:    Write{Path{"repo", "generated", "generated"}, kind},
				SameTree: true, BeforeRead: true, Succeeded: true,
				ExclusiveCreate: true, OwnFailedLookup: true, RenamedTreeCreatedFile: true,
			}
			if !validBlob(read) {
				t.Fatal("invalid symlink fixture")
			}
			if DropRead(read, proof) {
				t.Fatal("dropped lexical link dependency on generated content")
			}
			proof.Write.Path = read.Path
			if DropRead(read, proof) {
				t.Fatal("matching write alias does not prove link provenance")
			}
		})
	}
}

// RSP-V0-007: lexical and resolved link names are retained and folded together.
func TestRSPV0007Paths(t *testing.T) {
	root := t.TempDir()
	reads := Component[Blob]{State: Present}
	var counts OutsideCounts
	CaptureRead(&reads, &counts, []Repository{{"repo", root, true}}, filepath.Join(root, "LINK"), filepath.Join(root, "FILE"), blob().OID, strings.Repeat("4", 40), Other)
	if len(reads.Items) != 1 || reads.Items[0].Path.Lexical != "link" || reads.Items[0].Path.Resolved != "file" {
		t.Fatalf("link lost: %+v", reads)
	}
	CaptureRead(&reads, &counts, nil, filepath.Join(root, "outside"), filepath.Join(root, "outside"), "", "", Cache)
	if counts.Cache != 1 || len(reads.Items) != 1 {
		t.Fatal("outside path retained")
	}
	CaptureRead(&reads, &counts, nil, "relative", "relative", "", "", Other)
	if reads.State != Incomplete || reads.Reason != "read-path-unresolved" {
		t.Fatal("unresolved path accepted")
	}
}

// RSP-V0-007, RSP-V0-049: only known classes with both names outside every
// repository may be count-only. Repository links and unknown classes abstain.
func TestRSPV0007OutsideLinks(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	outside := filepath.Join(root, "outside", "config")
	for _, class := range []OutsideClass{Toolchain, Cache, Temporary, Other, "future-class"} {
		for _, link := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/link=%t", class, link), func(t *testing.T) {
				lexical := outside
				if link {
					lexical = filepath.Join(repo, "CONFIG")
				}
				reads := Component[Blob]{State: Present}
				var counts OutsideCounts
				CaptureRead(&reads, &counts, []Repository{{"repo", repo, true}}, lexical, outside, blob().OID, strings.Repeat("4", 40), class)
				wantCounts := OutsideCounts{}
				switch class {
				case Toolchain:
					wantCounts.Toolchain = 1
				case Cache:
					wantCounts.Cache = 1
				case Temporary:
					wantCounts.Temporary = 1
				default:
					wantCounts.Other = 1
				}
				if counts != wantCounts || len(reads.Items) != 0 {
					t.Fatalf("outside capture: %+v %+v", reads, counts)
				}
				wantState, wantReason := Present, ""
				if class != Toolchain && class != Cache && class != Temporary {
					wantState, wantReason = Incomplete, "undeclared-outside-read"
				}
				if link {
					wantState, wantReason = Incomplete, "cross-repository-link"
				}
				if reads.State != wantState || reads.Reason != wantReason {
					t.Fatalf("got %+v, want %s %q", reads, wantState, wantReason)
				}
			})
		}
	}
}

// RSP-V0-009: qualified markers or a test alone may isolate attribution.
func TestRSPV0009Attribution(t *testing.T) {
	r := fixture()
	r.Reads.Items = []Blob{blob()}
	for _, a := range []Attribution{{Tests: []string{"a"}}, {Tests: []string{"a", "b"}, MarkedTest: "a", MarkersQualified: true}} {
		got, err := Attribute(r, a)
		if err != nil || len(got) != 1 || got[0].Test != "a" || len(got[0].Reads.Items) != 1 {
			t.Fatalf("isolated: %+v %v", got, err)
		}
	}
	got, err := Attribute(r, Attribution{Tests: []string{"a", "b"}, MarkedTest: "a"})
	if err != nil || len(got) != 2 || len(got[1].Reads.Items) != 1 {
		t.Fatalf("shared tree narrowed: %+v %v", got, err)
	}
	_, err = Attribute(r, Attribution{Tests: []string{"a"}, MarkedTest: "other", MarkersQualified: true})
	requireError(t, err, "marker-outside-tree")
	r.Listings.Items = []Path{{"repo", "templates", "templates"}}
	r.Lookups.Items = []Path{{"repo", "optional", "optional"}}
	r.Execs.Items = []Executable{{blob(), strings.Repeat("3", 64), false}}
	r.Writes.Items = []Write{{blob().Path, Create}}
	r.External.Items = []Contact{{"loopback"}}
	loaded := Component[Blob]{State: Present, Items: []Blob{blob()}}
	reads := r.Reads
	for _, state := range []State{Present, Incomplete} {
		if state == Incomplete {
			r.Reads.State, r.Reads.Reason = Incomplete, "trace-cap"
		}
		got, err = Attribute(r, Attribution{Tests: []string{"a", "b"}, MarkedTest: "a", MarkersQualified: true, BuildPhase: true, BuildGraphModeled: true, BuildLoaded: loaded})
		if err != nil || len(got) != 2 {
			t.Fatalf("modeled build attribution: %+v %v", got, err)
		}
		for i, record := range got {
			want := r
			want.Test, want.Loaded = []string{"a", "b"}[i], loaded
			if !reflect.DeepEqual(record, want) {
				t.Fatalf("build dependency lost for %s: got %+v want %+v", want.Test, record, want)
			}
		}
	}
	r.Reads = reads
	a := Attribution{Tests: []string{"a", "b"}, MarkedTest: "a", MarkersQualified: true, BuildPhase: true, BuildGraphModeled: true, BuildLoaded: loaded}
	got, err = Attribute(r, a)
	if err != nil || len(got) != 2 {
		t.Fatalf("build attribution: %+v %v", got, err)
	}
	for i, build := range got {
		want := r
		want.Test, want.Loaded = a.Tests[i], a.BuildLoaded
		if !reflect.DeepEqual(build, want) {
			t.Fatalf("build dependencies changed: got %+v want %+v", build, want)
		}
		runtime := next(fixture())
		runtime.Test = build.Test
		combined, err := Union(build, runtime)
		if err != nil || !reflect.DeepEqual(combined.Reads, build.Reads) || !reflect.DeepEqual(combined.Listings, build.Listings) || !reflect.DeepEqual(combined.Lookups, build.Lookups) || !reflect.DeepEqual(combined.Execs, build.Execs) {
			t.Fatalf("build/runtime union lost completeness or dependencies: %+v %v", combined, err)
		}
	}
	// Unknown trace forms cannot become valid by modeling the build.
	r.Reads.State = "future-state"
	_, err = Attribute(r, a)
	requireError(t, err, "reads:unknown-state")
}

// RSP-V0-054: unmodeled build trace is retained on every test, ignoring markers.
func TestRSPV0054UnmodeledBuild(t *testing.T) {
	r := fixture()
	r.Reads.Items = []Blob{blob()}
	r.Listings.Items = []Path{{"repo", "templates", "templates"}}
	got, err := Attribute(r, Attribution{Tests: []string{"a", "b"}, MarkedTest: "a", MarkersQualified: true, BuildPhase: true})
	if err != nil || len(got) != 2 {
		t.Fatal(err)
	}
	for _, r := range got {
		if len(r.Reads.Items) != 1 || len(r.Listings.Items) != 1 {
			t.Fatal("dropped build dependency")
		}
	}
	_, err = Attribute(r, Attribution{BuildPhase: true})
	requireError(t, err, "tests-missing")
	r.Reads.State = Incomplete
	r.Reads.Reason = "trace-cap"
	got, err = Attribute(r, Attribution{Tests: []string{"a", "b"}, BuildPhase: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if r.Reads.State != Incomplete {
			t.Fatal("cap hidden")
		}
	}
}

// RSP-V0-049: declared siblings are keyed dependencies, outside others incomplete.
func TestRSPV0049SiblingRead(t *testing.T) {
	root := t.TempDir()
	sibling := filepath.Join(root, "sibling")
	reads := Component[Blob]{State: Present}
	var counts OutsideCounts
	CaptureRead(&reads, &counts, []Repository{{"main", filepath.Join(root, "main"), false}, {"sibling", sibling, false}}, filepath.Join(sibling, "data"), filepath.Join(sibling, "data"), blob().OID, "", Other)
	if reads.State != Present || len(reads.Items) != 1 || reads.Items[0].Path.Repository != "sibling" || reads.Items[0].Path.Resolved != "data" {
		t.Fatalf("sibling: %+v", reads)
	}
	r := fixture()
	r.Reads = reads
	snapshot := Snapshot{Blobs: map[PathKey]string{{"sibling", "data"}: blob().OID}, ChangedPins: map[string]bool{"sibling": true}}
	if got := r.Fresh(r.Runner, r.Execution, snapshot); got.Fresh || got.Reason != "sibling-pin-drift" {
		t.Fatalf("pin drift: %+v", got)
	}
	CaptureRead(&reads, &counts, nil, filepath.Join(root, "outside"), filepath.Join(root, "outside"), "", "", Other)
	if reads.State != Incomplete || reads.Reason != "undeclared-outside-read" || counts.Other != 1 || len(reads.Items) != 1 {
		t.Fatalf("outside read narrowed: %+v", reads)
	}
}

// RSP-V0-049: each path-bearing component alone invalidates a changed sibling pin.
func TestRSPV0049SiblingPinComponents(t *testing.T) {
	b := blob()
	b.Path.Repository = "sibling"
	digest := strings.Repeat("3", 64)
	for _, tc := range []struct {
		name string
		set  func(*Record)
	}{
		{"executed", func(r *Record) { r.Executed.Items = []ExecutedFile{{Blob: b}} }},
		{"regions", func(r *Record) { r.Regions.Items = []RegionsFile{{Blob: b}} }},
		{"loaded", func(r *Record) { r.Loaded.Items = []Blob{b} }},
		{"reads", func(r *Record) { r.Reads.Items = []Blob{b} }},
		{"listings", func(r *Record) { r.Listings.Items = []Path{b.Path} }},
		{"lookups", func(r *Record) { r.Lookups.Items = []Path{{"sibling", "optional", "optional"}} }},
		{"writes", func(r *Record) { r.Writes.Items = []Write{{b.Path, Create}} }},
		{"execs", func(r *Record) { r.Execs.Items = []Executable{{b, digest, true}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := fixture()
			tc.set(&r)
			s := Snapshot{Blobs: map[PathKey]string{{"sibling", "src.go"}: b.OID}, Executables: map[PathKey]string{{"sibling", "src.go"}: digest}, ChangedPins: map[string]bool{"unrelated": true}}
			if got := r.Fresh(r.Runner, r.Execution, s); !got.Fresh {
				t.Fatalf("unchanged sibling: %+v", got)
			}
			s.ChangedPins["sibling"] = true
			if got := r.Fresh(r.Runner, r.Execution, s); got.Fresh || got.Reason != "sibling-pin-drift" {
				t.Fatalf("changed sibling: %+v", got)
			}
		})
	}
	// The closed record validator must reject an unknown form before freshness.
	r := fixture()
	r.Lookups.State = "future-state"
	if got := r.Fresh(r.Runner, r.Execution, Snapshot{}); got.Fresh || got.Reason != "lookups:unknown-state" {
		t.Fatalf("unknown lookup state accepted: %+v", got)
	}
	r = fixture()
	r.Writes.Items = []Write{{b.Path, "future-write"}}
	if got := r.Fresh(r.Runner, r.Execution, Snapshot{}); got.Fresh || got.Reason != "writes:malformed-item" {
		t.Fatalf("unknown write form accepted: %+v", got)
	}
}

// RSP-V0-010: every dependency and both identities are checked for drift.
func TestRSPV0010Freshness(t *testing.T) {
	r := fixture()
	b := blob()
	r.Executed.Items = []ExecutedFile{{b, []Range{{1, 1, 2, 2}}, nil}}
	r.Loaded.Items = []Blob{b}
	r.Reads.Items = []Blob{b}
	r.Execs.Items = []Executable{{b, strings.Repeat("3", 64), true}}
	s := Snapshot{Blobs: map[PathKey]string{{"repo", "src.go"}: b.OID}, Executables: map[PathKey]string{{"repo", "src.go"}: strings.Repeat("3", 64)}}
	if got := r.Fresh(r.Runner, r.Execution, s); !got.Fresh {
		t.Fatal(got)
	}
	for _, component := range []string{"executed", "loaded", "reads", "execs"} {
		x := clone(r)
		switch component {
		case "executed":
			x.Executed.Items[0].Blob.OID = strings.Repeat("4", 40)
		case "loaded":
			x.Loaded.Items[0].OID = strings.Repeat("4", 40)
		case "reads":
			x.Reads.Items[0].OID = strings.Repeat("4", 40)
		case "execs":
			x.Execs.Items[0].Digest = strings.Repeat("4", 64)
		}
		if got := x.Fresh(x.Runner, x.Execution, s); got.Fresh || got.Reason == "" {
			t.Fatalf("fresh %s drift", component)
		}
	}
	runner := r.Runner
	runner.RunnerVersion = "2"
	if r.Fresh(runner, r.Execution, s).Fresh {
		t.Fatal("runner drift accepted")
	}
	execution := r.Execution
	execution.EnvironmentDigest = strings.Repeat("4", 64)
	if r.Fresh(r.Runner, execution, s).Fresh {
		t.Fatal("environment drift accepted")
	}
	if r.Fresh(r.Runner, r.Execution, Snapshot{}).Fresh {
		t.Fatal("unknown snapshot accepted")
	}
}

// RSP-V0-053: target is part of both the lookup key and freshness identity.
func TestRSPV0053TargetKey(t *testing.T) {
	r := fixture()
	k, err := r.TargetKey()
	if err != nil || k.OS != "linux" || k.Architecture != "amd64" {
		t.Fatal(k, err)
	}
	other := r
	other.Execution.OS = "windows"
	otherKey, _ := other.TargetKey()
	if k == otherKey || r.Fresh(r.Runner, other.Execution, Snapshot{}).Fresh {
		t.Fatal("cross-target reuse")
	}
	other.Execution.Architecture = ""
	_, err = other.TargetKey()
	requireError(t, err, "target-identity-missing")
}

// RSP-V0-055: replay is not a collection; disagreement preserves the union.
func TestRSPV0055Collections(t *testing.T) {
	a := fixture()
	a.Reads.Items = []Blob{blob()}
	b := next(a)
	if ok, reason := a.CollectionReady(false); ok || reason != "collection-count-insufficient" {
		t.Fatal(ok, reason)
	}
	if ok, _ := a.CollectionReady(true); !ok {
		t.Fatal("qualified single collection refused")
	}
	u, err := Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if ok, reason := u.CollectionReady(false); !ok || u.CollectionCount != 2 {
		t.Fatal(ok, reason)
	}
	replay, err := Union(a, a)
	if err != nil || replay.CollectionCount != 1 {
		t.Fatalf("replay counted: %+v %v", replay, err)
	}
	b.Reads.Items = append(b.Reads.Items, Blob{Path: Path{"repo", "data", "data"}, OID: blob().OID})
	b.Listings.Items = []Path{{"repo", "fixtures", "fixtures"}}
	u, err = Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if !u.Nondeterministic || len(u.Reads.Items) != 2 || len(u.Listings.Items) != 1 {
		t.Fatal("disagreement lost")
	}
	if ok, reason := u.CollectionReady(false); ok || reason != "nondeterministic" {
		t.Fatal(ok, reason)
	}
	b.Reads.State = Incomplete
	b.Reads.Reason = "cap"
	u, err = Union(a, b)
	if err != nil || u.Reads.State != Incomplete {
		t.Fatal("cap narrowed", err)
	}
	b = next(a)
	b.Reads.Items[0].OID = strings.Repeat("4", 40)
	_, err = Union(a, b)
	requireError(t, err, "blob-drift")
}

// RSP-V0-010, RSP-V0-055: executed ranges union; newly executed bytes cease being unexecuted.
func TestRSPV0010UnionRanges(t *testing.T) {
	a := fixture()
	a.Executed.Items = []ExecutedFile{{blob(), []Range{{1, 1, 1, 4}}, []Range{{1, 4, 1, 10}}}}
	b := next(a)
	b.Executed.Items[0].Executed = []Range{{1, 4, 1, 7}}
	u, err := Union(a, b)
	if err != nil {
		t.Fatal(err)
	}
	f := u.Executed.Items[0]
	if !reflect.DeepEqual(f.Executed, []Range{{1, 1, 1, 7}}) || !reflect.DeepEqual(f.Unexecuted, []Range{{1, 7, 1, 10}}) {
		t.Fatalf("bad union %+v", f)
	}
	b.Execution.Architecture = "arm64"
	_, err = Union(a, b)
	requireError(t, err, "identity-drift")
}

// RSP-V0-058: values are keyed and ordered; key rotation includes the empty env.
func TestRSPV0058EnvironmentDigest(t *testing.T) {
	key := bytes.Repeat([]byte{1}, 32)
	otherKey := bytes.Repeat([]byte{2}, 32)
	value := "low-entropy"
	env := map[string]*string{"FLAG": &value}
	d, err := EnvironmentDigest(key, env, nil)
	if err != nil || len(d) != 64 {
		t.Fatal(d, err)
	}
	again, _ := EnvironmentDigest(key, env, nil)
	if d != again {
		t.Fatal("unstable digest")
	}
	changed, _ := EnvironmentDigest(otherKey, env, nil)
	if d == changed {
		t.Fatal("key rotation ignored")
	}
	empty, _ := EnvironmentDigest(key, nil, nil)
	rotated, _ := EnvironmentDigest(otherKey, nil, nil)
	if empty == rotated {
		t.Fatal("empty env key rotation ignored")
	}
	value2 := "new"
	changed, _ = EnvironmentDigest(key, map[string]*string{"FLAG": &value2}, nil)
	if changed == d {
		t.Fatal("value drift ignored")
	}
	ignored, _ := EnvironmentDigest(key, map[string]*string{"IGNORED": nil}, []string{"IGNORED"})
	if ignored != empty {
		t.Fatal("core exclusion ignored")
	}
	_, err = EnvironmentDigest(nil, env, nil)
	requireError(t, err, "key-missing")
	_, err = EnvironmentDigest(key, map[string]*string{"FLAG": nil}, nil)
	requireError(t, err, "uncapturable")
}

// RSP-V0-007: matching uses case-fold equivalence, not only ASCII lowercasing.
func TestRSPV0007UnicodeFold(t *testing.T) {
	if foldPath("Σ/FILE") != foldPath("ς/file") {
		t.Fatal("case-fold equivalents differ")
	}
}

// RSP-V0-055: ordering changes are not nondeterminism, and digest replay cannot
// claim another collection even with changed content.
func TestRSPV0055OrderAndReplay(t *testing.T) {
	a := fixture()
	a.Reads.Items = []Blob{blob(), {Path: Path{"repo", "data", "data"}, OID: blob().OID}}
	b := next(a)
	b.Reads.Items[0], b.Reads.Items[1] = b.Reads.Items[1], b.Reads.Items[0]
	u, err := Union(a, b)
	if err != nil || u.Nondeterministic {
		t.Fatalf("ordering: %v", err)
	}
	b = clone(a)
	b.Sources[0].ContentDigest = strings.Repeat("3", 64)
	_, err = Union(a, b)
	requireError(t, err, "collection-conflict")
}

// RSP-V0-006, RSP-V0-007: JSON must not silently replace an invalid UTF-8
// filesystem path, test identity or reason with a different dependency key.
func TestRSPV0006RejectInvalidText(t *testing.T) {
	encoded, _ := Encode(fixture())
	malformed := bytes.Replace(encoded, []byte(`"suite/test"`), []byte(`"\ud800"`), 1)
	if _, err := Decode(malformed); err == nil {
		t.Fatal("unpaired surrogate accepted")
	}
	bad := string([]byte{'x', 0xff})
	for _, mutate := range []func(*Record){
		func(r *Record) { r.Test = bad },
		func(r *Record) { r.Repository = bad },
		func(r *Record) { r.Runner.Plugin = bad },
		func(r *Record) { r.Execution.TracingBackend = bad },
		func(r *Record) { r.Sources[0].Run = bad },
		func(r *Record) { r.Reads = Component[Blob]{State: Incomplete, Reason: bad} },
		func(r *Record) { b := blob(); b.Path.Resolved = bad; b.Path.Lexical = bad; r.Reads.Items = []Blob{b} },
		func(r *Record) { r.Regions.Items = []RegionsFile{{blob(), []Body{{bad, "", Range{1, 1, 2, 1}}}}} },
	} {
		r := fixture()
		mutate(&r)
		if _, err := Encode(r); err == nil {
			t.Fatal("lossy text accepted")
		}
	}
	r := fixture()
	r.Test = "test-Σ-�"
	data, err := Encode(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(data)
	if err != nil || got.Test != r.Test {
		t.Fatal("valid Unicode changed", err)
	}
}

func TestRSPV0049SiblingProbePin(t *testing.T) {
	for _, component := range []string{"lookup", "listing", "write"} {
		t.Run(component, func(t *testing.T) {
			r := fixture()
			p := Path{"sibling", "templates", "templates"}
			switch component {
			case "lookup":
				r.Lookups.Items = []Path{p}
			case "listing":
				r.Listings.Items = []Path{p}
			case "write":
				r.Writes.Items = []Write{{p, Append}}
			}
			for _, pins := range []map[string]bool{nil, {"sibling": false}, {"unrelated": true}} {
				if got := r.Fresh(r.Runner, r.Execution, Snapshot{ChangedPins: pins}); !got.Fresh {
					t.Fatalf("unchanged dependency: %+v", got)
				}
			}
			if got := r.Fresh(r.Runner, r.Execution, Snapshot{ChangedPins: map[string]bool{"sibling": true}}); got.Fresh || got.Reason != "sibling-pin-drift" {
				t.Fatalf("probe-only sibling pin drift: %+v", got)
			}
		})
	}
}
