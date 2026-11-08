// SPDX-License-Identifier: Apache-2.0
package runnerselection

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/testevidence"
	"github.com/Beamfall/corvint/internal/testrunner"
)

func fixture() Input {
	r := testevidence.New("repo", "suite/test", strings.Repeat("a", 40))
	r.Runner = testevidence.RunnerIdentity{Plugin: "fixture-plugin", PluginVersion: "1", ExecutableDigest: strings.Repeat("b", 64), RunnerVersion: "1", ConfigurationDigest: strings.Repeat("c", 64)}
	r.Execution = testevidence.ExecutionIdentity{OS: "target-os", Architecture: "target-arch", TracingBackend: "trace", TracingVersion: "1", EnvironmentDigest: strings.Repeat("d", 64)}
	r.Sources = []testevidence.Producer{{CoreDigest: strings.Repeat("e", 64), VerifierRevision: strings.Repeat("f", 40), ConfigurationDigest: strings.Repeat("c", 64), Ref: "refs/heads/main", Run: "run-1", ContentDigest: strings.Repeat("1", 64)}}
	r.CollectionCount = 1
	r.Executed = testevidence.Component[testevidence.ExecutedFile]{State: testevidence.Present}
	r.Regions = testevidence.Component[testevidence.RegionsFile]{State: testevidence.Present}
	r.Loaded = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Present}
	r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Present}
	r.Writes = testevidence.Component[testevidence.Write]{State: testevidence.Present}
	r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Present}
	r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Present}
	head := strings.Repeat("9", 40)
	key := testrunner.InventoryKey{Checkout: head, Runner: "fixture-runner", ConfigDigest: r.Runner.ConfigurationDigest, OS: r.Execution.OS, Arch: r.Execution.Architecture}
	pieces := testrunner.RunnerPieces{Inventory: testrunner.RunnerPiece{Present: true}, TestEvidence: testrunner.RunnerPiece{Present: true}, Qualification: testrunner.RunnerPiece{Present: true}}
	runner := Runner{Name: key.Runner, Identity: r.Runner, Execution: r.Execution, Pieces: pieces,
		Inventory:     testrunner.RunnerInventory{Key: key, Complete: true, Tests: []testrunner.InventoryTest{{ID: r.Test, Path: "suite/tests"}}},
		Qualification: Qualification{Admitted: true, Passed: true, SamplingEnabled: true, SingleCollectionQualified: true, Model: "traced", Classes: []Class{BodyEdit, DeclarationEdit, DataEdit, Add, DeleteRename, ExecTarget, RunnerConfig, E2EServer}, Runner: r.Runner, Execution: r.Execution},
		Records:       []Evidence{{Record: r, Admitted: true}}, IndexBase: r.Commit, IndexVerified: true}
	return Input{Base: r.Commit, Head: head, Repository: r.Repository, Repositories: map[string]bool{"repo": true},
		Changes: []Change{{Repository: "repo", Path: "payload", Kind: "modify", Decoded: true}}, DiffComplete: true,
		BaseDirectories: map[testevidence.PathKey]bool{{Repository: "repo", Path: "."}: true}, DirectoriesComplete: true,
		Snapshot: testevidence.Snapshot{Blobs: map[testevidence.PathKey]string{}, Executables: map[testevidence.PathKey]string{}}, Runners: []Runner{runner}}
}
func record(in *Input) *testevidence.Record { return &in.Runners[0].Records[0].Record }
func blob(in *Input, p string) testevidence.Blob {
	b := testevidence.Blob{Path: testevidence.Path{Repository: "repo", Lexical: p, Resolved: p}, OID: strings.Repeat("2", 40)}
	in.Snapshot.Blobs[testevidence.PathKey{Repository: "repo", Path: p}] = b.OID
	return b
}
func span(a, b int32) testevidence.Range {
	return testevidence.Range{Start: a, StartColumn: 1, End: b, EndColumn: 20}
}
func source(in *Input) {
	b := blob(in, "source")
	record(in).Executed.Items = []testevidence.ExecutedFile{{Blob: b, Executed: []testevidence.Range{span(2, 4)}, Unexecuted: []testevidence.Range{span(5, 9)}}}
	record(in).Regions.Items = []testevidence.RegionsFile{{Blob: b, Bodies: []testevidence.Body{{ID: "body", Range: span(1, 10)}}}}
	in.Changes[0].Path = "source"
	in.Changes[0].Hunks = []Hunk{{Start: 8, End: 8}}
}
func selected(t *testing.T, in Input, rule string) RunnerPlan {
	t.Helper()
	p := Select(in).Runners[0]
	if len(p.Selected) != 1 || len(p.Excluded) != 0 || !slices.Contains(p.Selected[0].Rules, rule) {
		t.Fatalf("want selected by %s: %+v", rule, p)
	}
	return p
}
func excluded(t *testing.T, in Input) RunnerPlan {
	t.Helper()
	p := Select(in).Runners[0]
	if p.Mode != "selected" || len(p.Selected) != 0 || len(p.Excluded) != 1 {
		t.Fatalf("want exclusion: %+v", p)
	}
	return p
}
func full(t *testing.T, in Input, reason string) RunnerPlan {
	t.Helper()
	p := Select(in).Runners[0]
	if p.Mode != "full" || len(p.Excluded) != 0 || !slices.Contains(p.Reasons, reason) {
		t.Fatalf("want FULL %s: %+v", reason, p)
	}
	return p
}

func golden(t *testing.T, name string, in Input) {
	t.Helper()
	p := Select(in)
	if p.SelectionNS == nil || *p.SelectionNS < 0 {
		t.Fatal("selection timing missing")
	}
	p.SelectionNS = nil
	got, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
		t.Fatalf("golden %s differs:\n%s", name, got)
	}
}

func TestRSPV0011PureAndDetached(t *testing.T) {
	in := fixture()
	before, _ := json.Marshal(in.Runners)
	p := Select(in)
	p.Runners[0].Excluded[0].Changes[0].Path = "mutated"
	after, _ := json.Marshal(in.Runners)
	if !bytes.Equal(before, after) || in.Changes[0].Path != "payload" {
		t.Fatal("selector mutated inputs")
	}
	excluded(t, in)
}
func TestRSPV0011UnknownInputFull(t *testing.T) {
	in := fixture()
	in.DiffComplete = false
	in.DiffReason = "capped"
	full(t, in, "diff-incomplete:capped")
}

func TestRSPV0012RuleGoldens(t *testing.T) {
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		t.Run(name, func(t *testing.T) {
			in := fixture()
			switch name {
			case "a":
				in.Runners[0].ConfigurationPaths = []testevidence.PathKey{{Repository: "repo", Path: "payload"}}
			case "b":
				source(&in)
				in.Changes[0].Hunks = []Hunk{{Start: 3, End: 3}}
			case "c":
				record(&in).Loaded.Items = []testevidence.Blob{blob(&in, "payload")}
			case "d":
				record(&in).Reads.Items = []testevidence.Blob{blob(&in, "payload")}
			case "e":
				in.Changes[0].Kind = "add"
				record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: ".", Resolved: "."}}
			case "f":
				in.Changes[0].Kind = "delete"
				record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: ".", Resolved: "."}}
			case "g":
				record(&in).Loaded.Items = []testevidence.Blob{blob(&in, "payload")}
				in.Changes[0].ModeChanged = true
			}
			selected(t, in, "rule:"+name)
			golden(t, "rule-"+name, in)
		})
	}
}
func TestRSPV0012NonintersectionAndUnion(t *testing.T) {
	in := fixture()
	source(&in)
	excluded(t, in)
	record(&in).Reads.Items = []testevidence.Blob{blob(&in, "source")}
	p := selected(t, in, "rule:d")
	if slices.Contains(p.Selected[0].Rules, "rule:c") {
		t.Fatal("body nonintersection must suppress only c")
	}
	in.Changes[0].Hunks = []Hunk{{Start: 3, End: 3}}
	p = selected(t, in, "rule:b")
	if !slices.Contains(p.Selected[0].Rules, "rule:d") {
		t.Fatal("first matching rule hid data read")
	}
}
func TestRSPV0012InsertionAndUncoveredLines(t *testing.T) {
	in := fixture()
	source(&in)
	in.Changes[0].Hunks = []Hunk{{Start: 4, End: 3}}
	selected(t, in, "rule:b")
	in.Changes[0].Hunks = []Hunk{{Start: 7, End: 6}}
	excluded(t, in)
	record(&in).Executed.Items[0].Unexecuted = nil
	selected(t, in, "rule:b")
	in.Changes[0].Hunks = []Hunk{{Start: 10, End: 9}}
	selected(t, in, "rule:c")
}
func TestRSPV0012AncestorsAndRenameRepresentations(t *testing.T) {
	for _, p := range []string{"fixtures/group/only.txt", "fixtures/group/deeper/only.txt"} {
		for _, kind := range []string{"delete", "rename", "delete-add"} {
			t.Run(p+"/"+kind, func(t *testing.T) {
				in := fixture()
				record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: "fixtures", Resolved: "fixtures"}}
				in.Changes = []Change{{Repository: "repo", Path: p, Kind: "delete", Decoded: true}}
				if kind == "rename" {
					in.Changes[0].Kind = "rename"
					in.Changes[0].OldPath = p
					in.Changes[0].Path = "elsewhere/only.txt"
				}
				if kind == "delete-add" {
					in.Changes = append(in.Changes, Change{Repository: "repo", Path: "elsewhere/only.txt", Kind: "add", Decoded: true})
				}
				selected(t, in, "rule:f")
			})
		}
	}
	// A surviving sibling must not cancel ancestor selection.
	in := fixture()
	record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: "fixtures", Resolved: "fixtures"}}
	in.Changes[0] = Change{Repository: "repo", Path: "fixtures/group/only.txt", Kind: "delete", Decoded: true}
	blob(&in, "fixtures/group/stays.txt")
	selected(t, in, "rule:f")
	in.Changes[0].Path = "unrelated/only.txt"
	excluded(t, in)
	in.Changes[0].Decoded = false
	selected(t, in, "ancestry-unproven")
	in.Changes[0].Decoded = true
	record(&in).Listings.Items[0].Repository = "unresolved"
	selected(t, in, "ancestry-unproven")
	in = fixture()
	in.Changes[0] = Change{Repository: "repo", Path: "a/b/c", Kind: "delete", Decoded: true}
	record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: ".", Resolved: "."}}
	selected(t, in, "rule:f")
}

// The fixture constructs the tracked head tree without leftover empty
// directories. An immediate-parent-only selector misses the failing test.
func TestRSPV0012LastTrackedDescendantRegression(t *testing.T) {
	for _, removed := range []string{"fixtures/group/only.txt", "fixtures/group/deeper/only.txt"} {
		for _, rename := range []bool{false, true} {
			for _, survives := range []bool{false, true} {
				root := t.TempDir()
				files := []string{"fixtures/keep.txt"}
				if survives {
					files = append(files, "fixtures/group/stays.txt")
				}
				if rename {
					files = append(files, "elsewhere/only.txt")
				}
				for _, p := range files {
					dst := filepath.Join(root, filepath.FromSlash(p))
					if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(dst, []byte("tracked"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				entries, err := os.ReadDir(filepath.Join(root, "fixtures"))
				if err != nil {
					t.Fatal(err)
				}
				hasGroup := false
				for _, e := range entries {
					hasGroup = hasGroup || e.Name() == "group"
				}
				if hasGroup != survives {
					t.Fatal("tracked head fixture retained an empty directory")
				}
				in := fixture()
				in.Changes[0] = Change{Repository: "repo", Path: removed, Kind: "delete", Decoded: true}
				if rename {
					in.Changes[0].Kind = "rename"
					in.Changes[0].OldPath = removed
					in.Changes[0].Path = "elsewhere/only.txt"
				}
				record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: "fixtures", Resolved: "fixtures"}}
				selected(t, in, "rule:f")
			}
		}
	}
}

func TestRSPV0012MalformedAncestryCannotExclude(t *testing.T) {
	for _, p := range []string{"../escape", "/absolute", "opaque\\token", "a/../b", "a//b", "", string([]byte{0xff})} {
		in := fixture()
		in.Changes[0].Kind = "delete"
		in.Changes[0].Path = p
		selected(t, in, "ancestry-unproven")
	}
	in := fixture()
	in.Changes[0].Repository = "unknown"
	selected(t, in, "ancestry-unproven")
	in = fixture()
	in.Changes[0].Hunks = []Hunk{{Start: 2147483647, End: 2147483646}}
	selected(t, in, "ancestry-unproven")
}

func TestRSPV0012AddLookupAndLink(t *testing.T) {
	in := fixture()
	in.Changes[0] = Change{Repository: "repo", Path: "fixtures/new/file", Kind: "add", Decoded: true}
	in.BaseDirectories[testevidence.PathKey{Repository: "repo", Path: "fixtures"}] = true
	record(&in).Lookups.Items = []testevidence.Path{{Repository: "repo", Lexical: "fixtures/new", Resolved: "fixtures/new"}}
	selected(t, in, "rule:e")
	record(&in).Lookups.Items = nil
	excluded(t, in)
	in.DirectoriesComplete = false
	selected(t, in, "ancestry-unproven")
	in = fixture()
	b := blob(&in, "payload")
	b.Path.Lexical = "link"
	b.LexicalOID = strings.Repeat("3", 40)
	in.Snapshot.Blobs[testevidence.PathKey{Repository: "repo", Path: "link"}] = b.LexicalOID
	record(&in).Reads.Items = []testevidence.Blob{b}
	in.Changes[0].Path = "link"
	in.Changes[0].LinkChanged = true
	selected(t, in, "rule:d")
	selected(t, in, "rule:g")
	in = fixture()
	b = blob(&in, "link/child")
	record(&in).Reads.Items = []testevidence.Blob{b}
	in.Changes[0].Path = "link"
	in.Changes[0].LinkChanged = true
	selected(t, in, "rule:d")
	in.Changes[0].Path = "unrelated"
	excluded(t, in)
	in = fixture()
	record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: "link/directory", Resolved: "target/directory"}}
	in.Changes[0].Path = "link"
	in.Changes[0].LinkChanged = true
	selected(t, in, "rule:g")
}

func setMissing(r *testevidence.Record, name string, state testevidence.State) {
	// Reflection keeps every component in the same malformed-input matrix.
	v := reflect.ValueOf(r).Elem().FieldByName(strings.ToUpper(name[:1]) + name[1:])
	v.FieldByName("State").SetString(string(state))
	v.FieldByName("Reason").SetString("not-collected")
}
func TestRSPV0013PresentComponents(t *testing.T) { excluded(t, fixture()) }
func TestRSPV0013MissingComponentGoldens(t *testing.T) {
	for _, name := range components() {
		for _, state := range []testevidence.State{testevidence.Absent, testevidence.Incomplete} {
			t.Run(name+"/"+string(state), func(t *testing.T) {
				in := fixture()
				setMissing(record(&in), name, state)
				selected(t, in, "component-missing:"+name)
				golden(t, "missing-"+name, in)
			})
		}
	}
}
func TestRSPV0013ExternalAlwaysSelects(t *testing.T) {
	in := fixture()
	record(&in).External.Items = []testevidence.Contact{{Kind: "network"}}
	selected(t, in, "external-observed")
	in.Changes = nil
	selected(t, in, "external-observed")
	record(&in).External.Items = nil
	record(&in).External.State = testevidence.Incomplete
	record(&in).External.Reason = "cap"
	selected(t, in, "component-missing:external")
}

func TestRSPV0014QualifiedClasses(t *testing.T) {
	for _, class := range []Class{BodyEdit, DeclarationEdit, DataEdit, Add, DeleteRename, ExecTarget, RunnerConfig, E2EServer} {
		t.Run(string(class), func(t *testing.T) {
			in := fixture()
			switch class {
			case BodyEdit:
				source(&in)
			case DeclarationEdit:
				source(&in)
				in.Changes[0].Hunks = []Hunk{{Start: 12, End: 12}}
			case Add:
				in.Changes[0].Kind = "add"
			case DeleteRename:
				in.Changes[0].Kind = "delete"
			case ExecTarget:
				b := blob(&in, "payload")
				d := strings.Repeat("4", 64)
				record(&in).Execs.Items = []testevidence.Executable{{Blob: b, Digest: d}}
				in.Snapshot.Executables[testevidence.PathKey{Repository: "repo", Path: "payload"}] = d
			case RunnerConfig:
				in.Runners[0].ConfigurationPaths = []testevidence.PathKey{{Repository: "repo", Path: "payload"}}
			case E2EServer:
				in.Runners[0].ServerPaths = []testevidence.PathKey{{Repository: "repo", Path: "payload"}}
			}
			p := Select(in).Runners[0]
			if p.Changes[0].Class != class || slices.Contains(p.Reasons, "class-unqualified:"+string(class)) {
				t.Fatalf("class not qualified: %+v", p)
			}
			in.Runners[0].Qualification.Classes = nil
			full(t, in, "class-unqualified:"+string(class))
		})
	}
}
func TestRSPV0014UnqualifiedClassFull(t *testing.T) {
	in := fixture()
	in.Runners[0].Qualification.Classes = []Class{BodyEdit}
	full(t, in, "class-unqualified:data-edit")
}

func TestRSPV0015QualifiedRunner(t *testing.T) { excluded(t, fixture()) }
func TestRSPV0015FullReasons(t *testing.T) {
	cases := []struct {
		name, reason string
		change       func(*Input)
	}{
		{"piece", "piece-unavailable:inventory:missing", func(in *Input) { in.Runners[0].Pieces.Inventory = testrunner.RunnerPiece{Reason: "missing"} }},
		{"qualification", "runner-not-qualified", func(in *Input) { in.Runners[0].Qualification.Passed = false }},
		{"admission", "qualification-not-admitted", func(in *Input) { in.Runners[0].Qualification.Admitted = false }},
		{"evidence", "evidence-not-admitted", func(in *Input) { in.Runners[0].Records[0].Admitted = false }},
		{"plugin", "plugin-failed:crash", func(in *Input) { in.Runners[0].PluginFailure = "crash" }},
		{"inventory", "inventory-incomplete:truncated", func(in *Input) {
			in.Runners[0].Inventory.Complete = false
			in.Runners[0].Inventory.Reason = "truncated"
		}},
		{"divergence", "inventory-divergent", func(in *Input) { in.Runners[0].InventoryDivergent = true }},
		{"index", "index-missing-stale-or-unverifiable", func(in *Input) { in.Runners[0].IndexBase = "" }},
		{"config", "configuration-changed", func(in *Input) { in.Runners[0].ConfigurationChanged = true }},
		{"execution", "execution-configuration-unqualified", func(in *Input) { in.Runners[0].Execution.EnvironmentDigest = strings.Repeat("0", 64) }},
		{"disabled", "confirmed-miss-disabled", func(in *Input) { in.Runners[0].Disabled = true }},
		{"sampling", "sampling-disabled", func(in *Input) { in.Runners[0].Qualification.SamplingEnabled = false }},
		{"model", "execution-model-unsupported", func(in *Input) { in.Runners[0].Qualification.Model = "unknown" }},
		{"cap", "input-cap-exceeded", func(in *Input) { in.Runners[0].Mandates = make([]Mandate, MaxInputs+1) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { in := fixture(); c.change(&in); full(t, in, c.reason) })
	}
}
func TestRSPV0015MissingStaleMalformedRecord(t *testing.T) {
	in := fixture()
	in.Runners[0].Records = nil
	selected(t, in, "record-missing")
	in = fixture()
	record(&in).Format = "bad"
	selected(t, in, "record-stale:unsupported-record-format")
	in = fixture()
	source(&in)
	in.Snapshot.Blobs = nil
	selected(t, in, "record-stale:blob-drift-or-missing")
	in = fixture()
	record(&in).Test = "another-test"
	selected(t, in, "record-missing")
}

func TestRSPV0016AuditAndCounts(t *testing.T) {
	in := fixture()
	source(&in)
	p := excluded(t, in)
	e := p.Excluded[0]
	if len(e.RecordDigest) != 64 || !reflect.DeepEqual(e.Components, components()) || e.Invalidation != invalidation || len(e.Changes) != 1 || e.Changes[0].Class != BodyEdit {
		t.Fatalf("incomplete certificate: %+v", e)
	}
	if p.Counts != (Counts{Inventoried: 1, Excluded: 1, InventoryKnown: true}) {
		t.Fatalf("counts %+v", p.Counts)
	}
	in.Runners[0].Mandates = []Mandate{{Test: "suite/test", Reason: "must-run"}, {Test: "suite/test", Reason: "declared-owner"}}
	p = selected(t, in, "project-mandated")
	if !reflect.DeepEqual(p.Selected[0].Mandates, []string{"declared-owner", "must-run"}) || p.Counts.Fallback != 1 {
		t.Fatalf("mandates/counts %+v", p)
	}
}
func TestRSPV0016UnknownInventoryNotZeroTests(t *testing.T) {
	in := fixture()
	in.Runners[0].Inventory.Complete = false
	p := Select(in).Runners[0]
	if p.Mode != "full" || p.Counts.InventoryKnown {
		t.Fatalf("unknown inventory advertised as known: %+v", p)
	}
	in = fixture()
	in.Runners[0].Mandates = []Mandate{{Test: "missing", Reason: "must-run"}}
	full(t, in, "mandate-invalid:missing")
}
func TestRSPV0017ShadowClaim(t *testing.T) {
	p := Select(fixture())
	if !p.Shadow || p.GoverningPlan != "static-unqualified" || p.Soundness != shadowClaim {
		t.Fatalf("wrong shadow claim %+v", p)
	}
}
func TestRSPV0017FullNeverClaimsSoundness(t *testing.T) {
	in := fixture()
	in.Runners[0].Qualification.Passed = false
	p := Select(in)
	if !strings.Contains(p.Soundness, "no soundness claim") || p.Runners[0].Mode != "full" {
		t.Fatal("shadow claimed enforcement")
	}
}
func TestRSPV0040NoEnforcementSwitch(t *testing.T) {
	p := Select(fixture())
	if !p.Shadow || len(p.Runners[0].Excluded) != 1 {
		t.Fatal("qualified input must remain shadow")
	}
}
func TestRSPV0040UnqualifiedKeepsGoverningPlan(t *testing.T) {
	in := fixture()
	in.Runners[0].Qualification.Passed = false
	p := Select(in)
	if p.GoverningPlan != "static-unqualified" || !p.Shadow || p.Runners[0].Mode != "full" {
		t.Fatal("unqualified input changed governing mode")
	}
}

// RSP-V0-012(g): getdents observes an entry's type without reading the entry.
func TestRSPV0012EntryTypeListing(t *testing.T) {
	for _, resolved := range []bool{false, true} {
		for _, link := range []bool{false, true} {
			in := fixture()
			in.Changes[0] = Change{Repository: "repo", Path: "fixtures/item", Kind: "modify", Decoded: true, ModeChanged: !link, LinkChanged: link}
			p := testevidence.Path{Repository: "repo", Lexical: "fixtures", Resolved: "elsewhere"}
			if resolved {
				p.Lexical, p.Resolved = p.Resolved, p.Lexical
			}
			record(&in).Listings.Items = []testevidence.Path{p}
			selected(t, in, "rule:g")
			in.Changes[0].Path = "unrelated/item"
			excluded(t, in)
		}
	}
	// The closed decoded-change allowlist cannot silently admit future forms.
	in := fixture()
	in.Changes[0] = Change{Repository: "repo", Path: "fixtures/item", Kind: "future-entry-kind", Decoded: true}
	record(&in).Listings.Items = []testevidence.Path{{Repository: "repo", Lexical: "fixtures", Resolved: "fixtures"}}
	selected(t, in, "ancestry-unproven")
}
