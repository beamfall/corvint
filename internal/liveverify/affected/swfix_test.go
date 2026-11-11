// SPDX-License-Identifier: Apache-2.0
package affected_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Beamfall/corvint/internal/gateselect"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
	"github.com/Beamfall/corvint/internal/liveverify/affected/swift"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

func swfixFiles(t *testing.T, files map[string]string) (string, fstest.MapFS) {
	t.Helper()
	root, fsys := t.TempDir(), fstest.MapFS{}
	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		fsys[name] = &fstest.MapFile{Data: []byte(body)}
	}
	return root, fsys
}

// Use the same structural selector and manifest builder as the test command,
// without executing a suite or changing the execution universe.
func swfixSession(t *testing.T, root, module string, graph *affected.Graph, plan affected.Plan, languages ...sessiontest.LanguageCommand) sessiontest.Manifest {
	t.Helper()
	body, err := json.Marshal(map[string]any{"plan": plan})
	if err != nil {
		t.Fatal(err)
	}
	var receipt gateselect.Receipt
	if err := json.Unmarshal(body, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Provider.Go.State = "RUNNABLE"
	input := sessiontest.Input{ShardCount: 1, RunnerProfile: sessiontest.ProfileLocal, GraphDigest: graph.Digest(),
		Declared: sessiontest.Declared{Languages: []sessiontest.LanguageCommand{{Language: "swift", Argv: []string{"swift", "test"}}}}}
	input.Declared.Languages = append(input.Declared.Languages, languages...)
	for _, id := range graph.UnitIDs() {
		if pkg, ok := strings.CutPrefix(id, "go:"); ok {
			input.Universe = append(input.Universe, pkg)
		}
	}
	for _, selected := range plan.Selected {
		input.Selected = append(input.Selected, selected.UnitID)
		if pkg, ok := strings.CutPrefix(selected.UnitID, "go:"); ok {
			receipt.Provider.Go.Packages = append(receipt.Provider.Go.Packages, pkg)
		}
	}
	for _, unknown := range plan.Unknown {
		input.Unknown = append(input.Unknown, sessiontest.Unknown{Reason: unknown.Reason, Detail: unknown.Detail})
	}
	lines := gateselect.SelectStructural(receipt, module, root)
	input.GoSelection = lines[len(lines)-1]
	manifest, err := sessiontest.Build(input)
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

// AFP-V0-9310/9312: a Swift fixture's unindexed package manifest is build
// metadata. Existing Swift targets below it must retain their suite, while
// the Go selection keeps the enclosing and literal readers and excludes solo.
// The layout and manifest reproduce the covlive fixture on mbapproved
// 12b66eb3907ea8da99e51216df9584cd84304bf6; both disk and immutable sources agree.
func TestUnindexedSwiftFixtureReaders_AFPV09310(t *testing.T) {
	const fixture = "host/testdata/live/swift/"
	files := map[string]string{
		"plain/plain_test.go": `package plain
import "testing"
func TestPlain(t *testing.T) {}
`,
		"host/testdata/live/swift/unmapped.swift": "public func missed() {}\n",
		"outside/Package.swift":                   "import PackageDescription\n",
		"orphan/testdata/Package.swift":           "import PackageDescription\n",
		"Sources/Unmapped.swift":                  "public func outside() {}\n",
		"go.mod":                                  "module example.test/m\n\ngo 1.27.3\n",
		"host/host.go":                            "package host\n",
		"host/host_test.go":                       "package host\nimport \"testing\"\nfunc TestHost(t *testing.T) {}\n",
		"reader/reader_test.go":                   "package reader\nimport (\"testing\"; \"os\")\nfunc TestRead(t *testing.T) { _, _ = os.ReadFile(\"../host/testdata/live/swift/Package.swift\") }\n",
		"solo/solo_test.go":                       "package solo\nimport \"testing\"\nfunc TestSolo(t *testing.T) {}\n",
		fixture + "Package.swift":                 "import PackageDescription\nlet package = Package(name: \"CovliveProbe\", products: [.library(name: \"Logic\", targets: [\"Logic\"])], targets: [.target(name: \"Logic\"), .testTarget(name: \"LogicTests\", dependencies: [\"Logic\"])])\n",
		fixture + "Sources/Logic/Logic.swift":     "public func value() -> Int { 1 }\n",
		fixture + "Tests/LogicTests/Probe.swift":  "import XCTest\n@testable import Logic\nfinal class Probe: XCTestCase { func testValue() { XCTAssertEqual(value(), 1) } }\n",
		"plain/testdata/plain.swift":              "fixture bytes\n",
	}
	root, fsys := swfixFiles(t, files)
	disk, err := affected.Build(context.Background(), root, golang.New(), swift.New())
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := affected.BuildFS(fsys, golang.New(), swift.New())
	if err != nil {
		t.Fatal(err)
	}
	if disk.Digest() != pinned.Digest() {
		t.Fatal("disk/immutable graph mismatch")
	}
	for _, graph := range []*affected.Graph{disk, pinned} {
		for _, dirty := range []string{"host/testdata/live/swift/unmapped.swift", "Sources/Unmapped.swift", "outside/Package.swift", "orphan/testdata/Package.swift"} {
			plan := affected.Select(graph, []string{dirty})
			m := swfixSession(t, root, "example.test/m", graph, plan)
			found := false
			for _, u := range plan.Unknown {
				found = found || u.Reason == affected.UnknownUnindexedSourcePath && u.Detail == dirty
			}
			if !found || m.Scope != sessiontest.ScopeFull {
				t.Fatalf("unmapped Swift source lost uncertainty: plan=%+v manifest=%+v", plan, m)
			}
		}

		for _, dirty := range []string{fixture + "Package.swift", "plain/testdata/plain.swift"} {
			plan := affected.Select(graph, []string{dirty})
			var pathUnknown bool
			for _, u := range plan.Unknown {
				if u.Detail == dirty {
					pathUnknown = true
					if u.Reason != affected.UnknownUnownedDirtyPath {
						t.Fatalf("fixture not data: %+v", u)
					}
				}
			}
			if plan.Scope != affected.ScopeUnknown || !pathUnknown {
				t.Fatalf("lost fixture uncertainty: %+v", plan)
			}
			// The fixture is a path no runner owns, so the plan runs every
			// test with the named reason, its readers and the declared Swift
			// suite included (RNC-V1-004, owner decision R32).
			manifest := swfixSession(t, root, "example.test/m", graph, plan)
			if manifest.Scope != sessiontest.ScopeFull || !strings.HasPrefix(manifest.Reason, "unowned-path: ") {
				t.Fatalf("unowned fixture narrowed: %+v", manifest)
			}
			var units []string
			for _, u := range manifest.Units {
				units = append(units, u.Unit)
			}
			want := []string{"go:example.test/m/plain", "declared:swift"}
			if strings.HasSuffix(dirty, "Package.swift") {
				want = []string{"go:example.test/m/host", "go:example.test/m/reader", "declared:swift"}
			}
			for _, unit := range want {
				if !slices.Contains(units, unit) {
					t.Fatalf("dirty %s: units %v, want them to include %v", dirty, units, want)
				}
			}
		}
	}
}

// AFP-V0-9311: omit a compiled Go source after observation while preserving
// its claimant and confined readers. Neither a read scope nor testdata
// placement proves the compiler's missing dependency edges irrelevant.
func TestUnindexedCompiledSourceStillWidens_AFPV09311(t *testing.T) {
	root, _ := swfixFiles(t, map[string]string{
		"go.mod":                           "module example.test/m\n\ngo 1.27.3\n",
		"host/host.go":                     "package host\n",
		"host/host_test.go":                "package host\n",
		"host/missed.go":                   "package host\n",
		"host/testdata/compiled/missed.go": "package compiled\n",
		"host/testdata/compiled/kept.go":   "package compiled\n",
	})
	graph, err := affected.Build(context.Background(), root, swfixMissedGo{})
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range []string{"host/missed.go", "host/testdata/compiled/missed.go"} {
		plan := affected.Select(graph, []string{dirty})
		found := false
		for _, u := range plan.Unknown {
			found = found || u.Reason == affected.UnknownUnindexedSourcePath && u.Detail == dirty
		}
		if !found {
			t.Fatalf("compiled source lost uncertainty: %+v", plan)
		}
		manifest := swfixSession(t, root, "example.test/m", graph, plan)
		if manifest.Scope != sessiontest.ScopeFull || !strings.Contains(manifest.Reason, affected.UnknownUnindexedSourcePath) {
			t.Fatalf("compiled source narrowed: %+v", manifest)
		}
	}
}

type swfixMissedGo struct{}

func (swfixMissedGo) Name() string       { return "go" }
func (swfixMissedGo) Owns(p string) bool { return strings.HasSuffix(p, ".go") }
func (swfixMissedGo) Units(context.Context, string) (affected.Result, error) {
	return affected.Result{Units: []affected.Unit{
		{ID: "go:example.test/m/host", Sources: []string{"host/host.go"}, Tests: []string{"host/host_test.go"}, ReadScoped: true, ReadScope: []string{"host/"}},
		{ID: "go:example.test/m/compiled", Sources: []string{"host/testdata/compiled/kept.go"}},
	}}, nil
}

// AFP-V0-9310/9311: the manifest exception cannot erase another claimant or
// an adapter's explicitly unobserved directory, even with confined Go readers.
func TestSwiftFixtureClaimBoundaries_AFPV09310(t *testing.T) {
	const dirty = "host/testdata/Package.swift"
	root, _ := swfixFiles(t, map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27.3\n",
		"host/host.go":      "package host\n",
		"host/host_test.go": "package host\n",
		dirty:               "import PackageDescription\n",
	})
	for _, tc := range []struct {
		name, reason string
		plugins      []affected.Language
	}{
		{"another source claimant", affected.UnknownUnindexedSourcePath, []affected.Language{swift.New(), swfixClaim{name: "other"}}},
		{"Swift directory unobserved", affected.UnknownLanguageFrontier, []affected.Language{swfixClaim{name: "swift", unobserved: []string{"host/testdata"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph, err := affected.Build(context.Background(), root, append([]affected.Language{golang.New()}, tc.plugins...)...)
			if err != nil {
				t.Fatal(err)
			}
			plan := affected.Select(graph, []string{dirty})
			for _, u := range plan.Unknown {
				if u.Reason == tc.reason && strings.HasSuffix(u.Detail, dirty) {
					return
				}
			}
			t.Fatalf("lost %s: %+v", tc.reason, plan)
		})
	}
}

type swfixClaim struct {
	name       string
	unobserved []string
}

func (p swfixClaim) Name() string     { return p.name }
func (swfixClaim) Owns(p string) bool { return strings.HasSuffix(p, ".swift") }
func (p swfixClaim) Units(context.Context, string) (affected.Result, error) {
	result := affected.Result{Unobserved: p.unobserved}
	if p.name == "other" {
		result.Units = []affected.Unit{{ID: "other:compiled", Sources: []string{"host/testdata/kept.other"}}}
	}
	return result, nil
}

// Reclassification must keep path-open readers in every other language, even
// when no literal resolves their reads. Unknown frontier forms stay open too.
func TestSwiftFixtureComputedReaders_AFPV09310(t *testing.T) {
	const fixture = "host/testdata/swift/"
	root, fsys := swfixFiles(t, map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27.3\n",
		"host/host_test.go": "package host\nimport \"testing\"\nfunc TestHost(t *testing.T) {}\n",
		fixture + "Package.swift": `import PackageDescription
let package = Package(name: "Probe", targets: [.testTarget(name: "ProbeTests")])`,
		fixture + "Tests/ProbeTests/Probe.swift": "import XCTest\nfinal class Probe: XCTestCase { func testValue() {} }\n",
		"spec/fixture_spec.rb": `RSpec.describe "fixture" do
 it "reads" do
  File.read(ENV.fetch("FIXTURE_PATH"))
 end
end
`,
	})
	for _, tc := range []struct {
		name, frontier string
		plugin         affected.Language
	}{
		{"ruby", ruby.FrontierImplicitTestDependency + ":spec/fixture_spec.rb", ruby.New()},
		{"future", "future:unclassified-read", swfixFrontier{name: "future", frontier: []string{"future:unclassified-read"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			disk, err := affected.Build(context.Background(), root, golang.New(), swift.New(), tc.plugin)
			if err != nil {
				t.Fatal(err)
			}
			pinned, err := affected.BuildFS(fsys, golang.New(), swift.New(), tc.plugin)
			if err != nil {
				t.Fatal(err)
			}
			if disk.Digest() != pinned.Digest() {
				t.Fatal("disk/immutable graph mismatch")
			}
			for _, graph := range []*affected.Graph{disk, pinned} {
				plan := affected.Select(graph, []string{fixture + "Package.swift"})
				found := false
				for _, u := range plan.Unknown {
					found = found || u.Reason == affected.UnknownLanguageFrontier && u.Detail == tc.frontier
				}
				if !found {
					t.Fatalf("computed reader lost its frontier: %+v", plan)
				}
				m := swfixSession(t, root, "example.test/m", graph, plan, sessiontest.LanguageCommand{Language: tc.name, Argv: []string{"check-suite"}})
				units := []string{}
				for _, u := range m.Units {
					units = append(units, u.Unit)
				}
				if !slices.Contains(units, "declared:"+tc.name) || !slices.Contains(units, "declared:swift") {
					t.Fatalf("computed reader or Swift suite omitted: %+v", m)
				}
			}
		})
	}
}

// A library-only Swift graph has no frontier that can carry its suite to the
// session builder. Unknown frontier forms do not certify that obligation.
func TestSwiftFixtureSuiteProof_AFPV09310(t *testing.T) {
	const fixture = "host/testdata/swift/"
	root, fsys := swfixFiles(t, map[string]string{
		"go.mod":            "module example.test/m\n\ngo 1.27.3\n",
		"host/host_test.go": "package host\nimport \"testing\"\nfunc TestHost(t *testing.T) {}\n",
		fixture + "Package.swift": `import PackageDescription
let package = Package(name: "Probe", targets: [.target(name: "Logic")])`,
		fixture + "Sources/Logic/Logic.swift": "public func value() -> Int { 1 }\n",
	})
	for _, tc := range []struct {
		name         string
		plugin       affected.Language
		wantFrontier []string
	}{
		{"empty frontier", swift.New(), nil},
		{"unknown frontier", swfixFrontier{name: "swift", frontier: []string{"swift:future-suite-form"}}, []string{"swift:future-suite-form"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			disk, err := affected.Build(context.Background(), root, golang.New(), tc.plugin)
			if err != nil {
				t.Fatal(err)
			}
			pinned, err := affected.BuildFS(fsys, golang.New(), tc.plugin)
			if err != nil {
				t.Fatal(err)
			}
			if disk.Digest() != pinned.Digest() {
				t.Fatal("disk/immutable graph mismatch")
			}
			for _, graph := range []*affected.Graph{disk, pinned} {
				plan := affected.Select(graph, []string{fixture + "Package.swift"})
				frontiers := []string{}
				retained := false
				for _, u := range plan.Unknown {
					if u.Reason == affected.UnknownLanguageFrontier && strings.HasPrefix(u.Detail, "swift:") {
						frontiers = append(frontiers, u.Detail)
					}
					retained = retained || u.Reason == affected.UnknownUnindexedSourcePath && u.Detail == fixture+"Package.swift"
				}
				if !slices.Equal(frontiers, tc.wantFrontier) {
					t.Fatalf("fixture frontier: %v, want %v", frontiers, tc.wantFrontier)
				}
				if !retained {
					t.Fatalf("unproven suite lost source claim: %+v", plan)
				}
				m := swfixSession(t, root, "example.test/m", graph, plan)
				found := false
				for _, u := range m.Units {
					found = found || u.Unit == "declared:swift"
				}
				if m.Scope != sessiontest.ScopeFull || !found {
					t.Fatalf("unproven suite omitted: %+v", m)
				}
			}
		})
	}
}

type swfixFrontier struct {
	name     string
	frontier []string
}

func (p swfixFrontier) Name() string { return p.name }
func (p swfixFrontier) Owns(path string) bool {
	return p.name == "swift" && strings.HasSuffix(path, ".swift")
}
func (p swfixFrontier) Units(context.Context, string) (affected.Result, error) {
	return p.result(), nil
}
func (p swfixFrontier) UnitsSource(*affected.Source) (affected.Result, error) { return p.result(), nil }
func (p swfixFrontier) result() affected.Result {
	source := "future/reader.txt"
	if p.name == "swift" {
		source = "host/testdata/swift/Sources/Logic/Logic.swift"
	}
	return affected.Result{Frontier: p.frontier, Units: []affected.Unit{{ID: p.name + ":library", Sources: []string{source}}}}
}
