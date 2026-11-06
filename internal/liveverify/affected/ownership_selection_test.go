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
	after := Select(g.WithOwnershipChanges(nil, nil), dirty)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("no membership change altered plan bytes")
	}
}

// AZU-V0-9901: fixture closure removes only the unowned-path cascade. It never
// removes a reached unit's frontier, a foreign unit below that fixture, a
// resolution input, or an unrelated unowned path in the same dirty set.
func TestOwnershipFixtureClosureBounds_AZU_V0_9901(t *testing.T) {
	fixture := "pkg/testdata/new.json"
	for _, tc := range []struct {
		name  string
		extra []Unit
		input string
		other string
		want  bool
	}{
		{name: "closed", want: false},
		{name: "reached frontier", extra: []Unit{{ID: "y:reader", Sources: []string{"reader.y"}, Tests: []string{"reader_test.y"}, PathTokens: []string{fixture}, Frontier: []string{pathOpenY}}}, want: true},
		{name: "foreign fixture unit", extra: []Unit{{ID: "y:fixture", Sources: []string{"pkg/testdata/source.y"}}}, want: true},
		{name: "resolution input", input: fixture, want: true},
		{name: "other unowned", other: "other/data.json", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			y := reaching{fake: fake{name: "y", units: tc.extra, frontier: []string{pathOpenY}}, inputs: func(path string) bool { return tc.input != "" && path == tc.input }}
			g, err := Build(context.Background(), t.TempDir(), fake{name: "go", units: []Unit{{ID: "go:pkg", Sources: []string{"pkg/pkg.go"}, Tests: []string{"pkg/pkg_test.go"}}, {ID: "go:gate", Tests: []string{OwnershipAuditTestPath}, ReadScoped: true}}}, y)
			if err != nil {
				t.Fatal(err)
			}
			dirty := []string{fixture}
			if tc.other != "" {
				dirty = append(dirty, tc.other)
			}
			p := Select(g.WithOwnershipChanges(dirty, dirty), dirty)
			has := slices.ContainsFunc(p.Unknown, func(u Unknown) bool { return u.Reason == UnknownLanguageFrontier && u.Detail == pathOpenY })
			if has != tc.want {
				t.Fatalf("frontier present=%v want=%v: %+v", has, tc.want, p.Unknown)
			}
		})
	}
}
