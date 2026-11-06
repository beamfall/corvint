// SPDX-License-Identifier: Apache-2.0
package affected

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// AZU-V0-9900 / AZU-V0-9901: the gate runs even with no content consumer.
// Selecting it does not prove an arbitrary unowned path safe, and a content
// modification without a membership input must leave the old plan unchanged.
func TestOwnershipSelectionAndUnknowns_AZU_V0_9900(t *testing.T) {
	gate := Unit{ID: "go:gate", Tests: []string{OwnershipAuditTestPath}, ReadScoped: true}
	g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{gate}}, fake{name: "y", frontier: []string{pathOpenY}})
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"data/new.json"}
	before := Select(g, dirty)
	p := Select(g.WithOwnershipChanges(dirty, dirty), dirty)
	if !slices.ContainsFunc(p.Selected, func(s Selection) bool {
		return s.UnitID == gate.ID && s.Witness.Kind == WitnessOwnershipAudit && strings.Contains(strings.Join(s.Argv, " "), OwnershipAuditTest)
	}) {
		t.Fatalf("gate not named: %+v", p.Selected)
	}
	if !slices.ContainsFunc(p.Unknown, func(u Unknown) bool {
		return u.Reason == UnknownUnownedDirtyPath && strings.Contains(u.Hint, RuntimeReadersPath)
	}) {
		t.Fatalf("no fix hint: %+v", p.Unknown)
	}
	if !slices.ContainsFunc(p.Unknown, func(u Unknown) bool { return u.Reason == UnknownLanguageFrontier && u.Detail == pathOpenY }) {
		t.Fatalf("unbounded real consumer lost: %+v", p.Unknown)
	}
	after := Select(g.WithOwnershipChanges(nil, dirty), dirty)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("no membership change altered plan bytes")
	}
}

// AZU-V0-9901 is limited by the frozen Core contract in repositories without
// the ownership ratchet. Observing committed paths must preserve those bytes.
func TestOwnershipGuidanceRequiresRatchet_AZU_V0_9901(t *testing.T) {
	g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}}})
	if err != nil {
		t.Fatal(err)
	}
	dirty := []string{"data/new.json"}
	before := Select(g, dirty)
	after := Select(g.WithOwnershipChanges(dirty, dirty), dirty)
	if !slices.ContainsFunc(after.Unknown, func(u Unknown) bool {
		return u.Reason == UnknownUnownedDirtyPath && u.Detail == dirty[0]
	}) {
		t.Fatalf("unowned path lost: %+v", after.Unknown)
	}
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatalf("repository without ratchet changed frozen plan bytes:\n%s\n%s", a, b)
	}
}

// AZU-V0-9901: fixture closure removes only the unowned-path cascade. It never
// removes a reached unit's frontier, a foreign unit below that fixture, a
// resolution input, or an unrelated unowned path in the same dirty set.
func TestOwnershipFixtureClosureBounds_AZU_V0_9901(t *testing.T) {
	fixture := "pkg/testdata/new.json"
	// Exercise actual fixture-closed reasons. Dynamic loads are path-open
	// and must survive even when their sites are outside testdata.
	const frontier = "y:runner-unknown:reader_test.y"
	for _, tc := range []struct {
		name  string
		extra []Unit
		input string
		other string
		want  bool
	}{
		{name: "closed", want: false},
		{name: "reached frontier", extra: []Unit{{ID: "y:reader", Sources: []string{"reader.y"}, Tests: []string{"reader_test.y"}, PathTokens: []string{fixture}, Frontier: []string{frontier}}}, want: true},
		{name: "foreign fixture unit", extra: []Unit{{ID: "y:fixture", Sources: []string{"pkg/testdata/source.y"}}}, want: true},
		{name: "resolution input", input: fixture, want: true},
		{name: "other unowned", other: "other/data.json", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := reaching{fake: fake{name: "y", units: tc.extra, frontier: []string{frontier}}, fixtureClosed: []string{"y:runner-unknown"}, inputs: func(path string) bool { return tc.input != "" && path == tc.input }}
			g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}, {ID: "go:gate", Tests: []string{OwnershipAuditTestPath}, ReadScoped: true}}}, y)
			if err != nil {
				t.Fatal(err)
			}
			dirty := []string{fixture}
			if tc.other != "" {
				dirty = append(dirty, tc.other)
			}
			p := Select(g.WithOwnershipChanges(dirty, dirty), dirty)
			has := slices.ContainsFunc(p.Unknown, func(u Unknown) bool { return u.Reason == UnknownLanguageFrontier && u.Detail == frontier })
			if has != tc.want {
				t.Fatalf("frontier present=%v want=%v: %+v", has, tc.want, p.Unknown)
			}
		})
	}
}

// AZU-V0-9901 / AFP-V0-511: each plugin and reason supplies its own closure
// proof. A selected Go host supplies none for dynamic or unclassified readers.
func TestOwnershipFixtureFrontierClassification_AZU_V0_9901(t *testing.T) {
	fixture := "pkg/testdata/new.json"
	for _, plugin := range []Language{bystander(), fake{name: "z", frontier: []string{"z:unclassified:reader.z"}}} {
		t.Run(plugin.Name(), func(t *testing.T) {
			g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
				{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}},
				{ID: "go:gate", Tests: []string{OwnershipAuditTestPath}, ReadScoped: true},
			}}, plugin)
			if err != nil {
				t.Fatal(err)
			}
			p := Select(g.WithOwnershipChanges([]string{fixture}, []string{fixture}), []string{fixture})
			want := []string{pathOpenY, boundY}
			if plugin.Name() == "z" {
				want = []string{"z:unclassified:reader.z"}
			}
			if got := languageFrontier(p); !slices.Equal(got, want) {
				t.Fatalf("frontiers=%q want=%q", got, want)
			}
		})
	}
}

// AZU-V0-9901: a terminal reader of a separate Go edit must not turn a
// newly tracked, hosted fixture into participation in every language reason.
// The usual per-reason proof still retains dynamic/bounded reads, resolution
// inputs, foreign fixture sites, independently reached frontiers and any
// other unowned path. Content-only plans keep their existing bytes.
func TestOwnershipFixtureTerminalReaders_AZU_V0_9901(t *testing.T) {
	const fixture = "pkg/testdata/new.json"
	const closed = "y:runner-unknown:reader_test.y"
	const source = "y:import-unresolved:reader.y"
	const dynamic = "y:dynamic-load:reader.y"
	for _, tc := range []struct {
		name                                                   string
		membership, resolution, foreign, own, unbounded, other bool
		testless                                               bool
		wantClosed, wantSource                                 bool
	}{
		{name: "terminal", membership: true},
		{name: "content only", wantClosed: true, wantSource: true},
		{name: "resolution input", membership: true, resolution: true, wantClosed: true, wantSource: true},
		{name: "foreign fixture", membership: true, foreign: true, wantClosed: true},
		{name: "own source", membership: true, own: true, wantClosed: true, wantSource: true},
		{name: "unbounded reader", membership: true, unbounded: true, wantClosed: true, wantSource: true},
		{name: "other unowned", membership: true, other: true, wantClosed: true, wantSource: true},
		{name: "testless host", membership: true, testless: true, wantClosed: true, wantSource: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := reaching{fake: fake{name: "y", frontier: []string{closed, source, dynamic}, units: []Unit{{ID: "y:reader", Sources: []string{"reader.y"}, Tests: []string{"reader_test.y"}, PathTokens: []string{"pkg/pkg.go"}, Frontier: []string{dynamic}}}}, sourceBound: []string{"y:import-unresolved"}, fixtureClosed: []string{"y:runner-unknown"}}
			if tc.foreign {
				y.units = append(y.units, Unit{ID: "y:fixture", Sources: []string{"pkg/testdata/source.y"}})
			}
			y.inputs = func(path string) bool { return tc.resolution && path == fixture }
			var plugin Language = y
			if tc.unbounded {
				plugin = reading{y}
			}
			host := Unit{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}
			if tc.testless {
				host.Tests = nil
			}
			g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{
				host,
				{ID: "go:gate", Tests: []string{OwnershipAuditTestPath}, ReadScoped: true},
			}}, plugin)
			if err != nil {
				t.Fatal(err)
			}
			dirty := []string{fixture, "pkg/pkg.go"}
			if tc.own {
				dirty = append(dirty, "reader.y")
			}
			if tc.other {
				dirty = append(dirty, "other/data.json")
			}
			if tc.membership {
				g = g.WithOwnershipChanges([]string{fixture}, []string{fixture})
			}
			p := Select(g, dirty)
			frontier := languageFrontier(p)
			if slices.Contains(frontier, closed) != tc.wantClosed || slices.Contains(frontier, source) != tc.wantSource || !slices.Contains(frontier, dynamic) {
				t.Fatalf("frontier=%q; closed=%v source=%v; dynamic must remain", frontier, tc.wantClosed, tc.wantSource)
			}
			if tc.membership && !tc.own && !tc.unbounded && !tc.other && !tc.testless && !tc.resolution {
				if !slices.ContainsFunc(p.Unknown, func(u Unknown) bool {
					return u.Reason == UnknownUnownedDirtyPath && u.Detail == fixture
				}) {
					t.Fatal("fixture must remain explicitly unowned")
				}
				found := false
				for _, exclusion := range p.FrontierExcluded {
					if exclusion.Language != "y" {
						continue
					}
					found = true
					if exclusion.Reason == FrontierExclusionReason || !strings.Contains(exclusion.Reason, "go:pkg") || !strings.Contains(exclusion.Reason, "SourceBoundFrontiers ("+source+")") {
						t.Errorf("missing fixture-specific exclusion proof: %+v", exclusion)
					}
					if !tc.foreign && !strings.Contains(exclusion.Reason, "FixtureClosedFrontiers ("+closed+")") {
						t.Errorf("missing fixture closure classification: %+v", exclusion)
					}
				}
				if !found {
					t.Fatal("missing frontier exclusion certificate")
				}
			}
		})
	}
}
