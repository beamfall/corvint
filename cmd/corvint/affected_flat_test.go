// SPDX-License-Identifier: Apache-2.0
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// affectedFlatDocument decodes the members AFP-V0-9987 adds and the plan
// members they restate.
type affectedFlatDocument struct {
	Plan struct {
		Selected []struct {
			UnitID string `json:"unitId"`
		} `json:"selected"`
	} `json:"plan"`
	SelectedFlat []affectedFlatSelection `json:"selectedFlat"`
	Summary      struct {
		Selected           int  `json:"selected"`
		UnresolvedPackages *int `json:"unresolvedPackages"`
	} `json:"summary"`
}

// AFP-V0-9987: the compact plan lists its selection flat, with one runnable
// handle and the witness per unit, and counts the unresolved Go packages.
func TestAffectedSelectedFlatAndUnresolvedCount_AFPV09987(t *testing.T) {
	t.Parallel()
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"docs/b.md"})
	compact, err := compactAffected(affectedReceipt{Plan: plan})
	if err != nil {
		t.Fatal(err)
	}
	want := []affectedFlatSelection{
		{UnitID: "go:any", Test: "any", Witness: affected.WitnessUnboundedReader, DirtyPath: "docs/b.md", Reason: "any dirty path, such as docs/b.md; unbounded reads of go:any: any/any_test.go calls os.Getwd"},
		{UnitID: "go:b", Test: "b", Witness: affected.WitnessPathLiteralReader, DirtyPath: "docs/b.md", Reason: "dirty path docs/b.md"},
	}
	slices.SortFunc(want, func(a, b affectedFlatSelection) int {
		return slices.IndexFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == a.UnitID }) -
			slices.IndexFunc(plan.Selected, func(s affected.Selection) bool { return s.UnitID == b.UnitID })
	})
	if !reflect.DeepEqual(compact.SelectedFlat, want) {
		t.Fatalf("selectedFlat = %+v\nwant %+v", compact.SelectedFlat, want)
	}
	if compact.Summary.UnresolvedPackages != 1 {
		t.Fatalf("unresolvedPackages = %d, want 1 (go:any)", compact.Summary.UnresolvedPackages)
	}
	body, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{`"selectedFlat":[{"unitId":`, `"unresolvedPackages":1`, `"test":"any"`, `"witness":"UNBOUNDED_READER"`, `"dirtyPath":"docs/b.md"`, `"reason":"`} {
		if !bytes.Contains(body, []byte(member)) {
			t.Errorf("document lacks %s: %s", member, body)
		}
	}

	// The runnable handle and the reason for every other shape of unit.
	for _, test := range []struct {
		selection affected.Selection
		test      string
	}{
		{affected.Selection{UnitID: "shell:script/a_test.sh", Tests: []string{"script/a_test.sh"}}, "script/a_test.sh"},
		{affected.Selection{UnitID: "typescript:web", Tests: []string{"web/a.test.ts", "web/b.test.ts"}}, "typescript:web"},
		{affected.Selection{UnitID: "driver:make:Makefile#check", Kind: affected.SelectionKindDriverTarget}, "driver:make:Makefile#check"},
	} {
		if got := affectedFlatTest(test.selection); got != test.test {
			t.Errorf("test of %s = %q, want %q", test.selection.UnitID, got, test.test)
		}
	}
	reason := affectedFlatReason(affectedCompactWitness{Kind: affected.WitnessDeclaredReadScope, DirtyPath: "d/x", ViaLength: 3, Glob: "d/*", ReadScope: &affected.ReadScopeCause{Declaration: ".corvint/test-read-scopes.json", Entry: "d/"}})
	if reason != "dirty path d/x; glob d/*; read scope .corvint/test-read-scopes.json entry d/; via a 3-unit chain" {
		t.Errorf("reason = %q", reason)
	}
	// Only go: units count as unresolved packages.
	if got := affectedUnresolvedPackages([]affected.Selection{
		{UnitID: "go:x", Witness: affected.Witness{Kind: affected.WitnessUnboundedReader}},
		{UnitID: "driver:make:Makefile#x", Witness: affected.Witness{Kind: affected.WitnessUnboundedReader}},
		{UnitID: "go:y", Witness: affected.Witness{Kind: affected.WitnessDirectSource}},
	}); got != 1 {
		t.Errorf("unresolvedPackages = %d, want 1", got)
	}
	empty, err := compactAffected(affectedReceipt{Plan: affected.Plan{Scope: affected.ScopeBounded}})
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := json.Marshal(empty); !bytes.Contains(body, []byte(`"selectedFlat":[]`)) {
		t.Errorf("an empty selection must still carry selectedFlat: %s", body)
	}
}

// AFP-V0-9987 fail-closed rule: the reporting members never change the
// selection. On one committed range, the compact plan, its selectedFlat, its
// text form and the --full plan name the same units in the same order, and
// the unresolved package of a plan stays selected.
func TestAffectedNewMembersNeverChangeSelection_AFPV09987(t *testing.T) {
	t.Parallel()
	root := impactCLIRepository(t)
	base := coreFreezeParent(t, root)
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base)
	if code != 0 {
		t.Fatalf("compact: exit %d: %s", code, stderr)
	}
	var document affectedFlatDocument
	if err := json.Unmarshal([]byte(stdout), &document); err != nil {
		t.Fatal(err)
	}
	var planUnits, flatUnits []string
	for _, selection := range document.Plan.Selected {
		planUnits = append(planUnits, selection.UnitID)
	}
	for _, selection := range document.SelectedFlat {
		flatUnits = append(flatUnits, selection.UnitID)
	}
	if len(planUnits) == 0 || !slices.Equal(planUnits, flatUnits) || document.Summary.Selected != len(planUnits) || document.Summary.UnresolvedPackages == nil {
		t.Fatalf("plan %q, selectedFlat %q, summary %+v", planUnits, flatUnits, document.Summary)
	}
	code, full, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--full")
	if code != 0 {
		t.Fatalf("full: exit %d: %s", code, stderr)
	}
	var fullDocument affectedFlatDocument
	if err := json.Unmarshal([]byte(full), &fullDocument); err != nil {
		t.Fatal(err)
	}
	var fullUnits []string
	for _, selection := range fullDocument.Plan.Selected {
		fullUnits = append(fullUnits, selection.UnitID)
	}
	if !slices.Equal(planUnits, fullUnits) || fullDocument.SelectedFlat != nil || fullDocument.Summary.UnresolvedPackages != nil {
		t.Fatalf("--full selects %q (want %q) or gained a reporting member: %s", fullUnits, planUnits, full)
	}
	code, text, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--format", "text")
	if code != 0 {
		t.Fatalf("text: exit %d: %s", code, stderr)
	}
	var textUnits []string
	for line := range strings.Lines(text) {
		if rest, ok := strings.CutPrefix(line, "selected "); ok {
			textUnits = append(textUnits, strings.Fields(rest)[0])
		}
	}
	if !slices.Equal(planUnits, textUnits) || strings.Contains(text, "selectedFlat") || strings.Contains(text, "unresolved") {
		t.Fatalf("text selects %q (want %q) or changed:\n%s", textUnits, planUnits, text)
	}

	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"docs/a.md", "a/a.go", "README.md"} {
		plan := affected.Select(graph, []string{path})
		before := slices.Clone(plan.Selected)
		compact, err := compactAffected(affectedReceipt{Plan: plan})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(plan.Selected, before) || len(compact.Plan.Selected) != len(before) || len(compact.SelectedFlat) != len(before) {
			t.Fatalf("%s: projection changed the selection: %+v", path, compact)
		}
		if !slices.ContainsFunc(before, func(s affected.Selection) bool { return s.UnitID == "go:any" }) || compact.Summary.UnresolvedPackages != 1 {
			t.Fatalf("%s: the unresolved package go:any must stay selected and counted: %+v", path, compact.SelectedFlat)
		}
	}
}

// affectedWitnessShape is the one AFP-V0-9988 witness object; a decoder that
// rejects unknown members proves no form carries another member.
type affectedWitnessShape struct {
	Kind         string                   `json:"kind"`
	DirtyPath    string                   `json:"dirtyPath"`
	ViaLength    *int                     `json:"viaLength"`
	Via          []string                 `json:"via"`
	AnyDirtyPath *bool                    `json:"anyDirtyPath"`
	Cause        *affected.UnboundedCause `json:"cause"`
	Glob         string                   `json:"glob"`
	ReadScope    *affected.ReadScopeCause `json:"readScope"`
}

func decodeAffectedWitness(t *testing.T, form string, raw json.RawMessage) affectedWitnessShape {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var witness affectedWitnessShape
	if err := decoder.Decode(&witness); err != nil {
		t.Fatalf("%s witness %s is not the documented shape: %v", form, raw, err)
	}
	if witness.Kind == "" || witness.DirtyPath == "" {
		t.Fatalf("%s witness lacks kind or dirtyPath: %s", form, raw)
	}
	if (witness.AnyDirtyPath != nil) != (witness.Kind == affected.WitnessUnboundedReader) || witness.AnyDirtyPath != nil && !*witness.AnyDirtyPath {
		t.Fatalf("%s witness anyDirtyPath disagrees with its kind: %s", form, raw)
	}
	return witness
}

// AFP-V0-9988 (extends TSP-V0-7311): the compact, --full and --by-path forms
// emit one witness shape, and a unit both plans select agrees in every member.
func TestAffectedWitnessOneShapeAcrossModes_AFPV09988(t *testing.T) {
	t.Parallel()
	graph, err := affected.Build(context.Background(), t.TempDir(), byPathLanguage{})
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{"docs/a.md", "docs/b.md"}
	full := affected.Select(graph, paths)
	compact, _ := compactAffectedSelections(affected.SelectLinked(graph, paths).Selected)
	byPath := affectedByPath(affectedReceipt{Plan: full}, graph, paths)
	type witnessed struct {
		Witness json.RawMessage `json:"witness"`
		UnitID  string          `json:"unitId"`
	}
	decodeAll := func(form string, objects []any) map[string]affectedWitnessShape {
		shapes := map[string]affectedWitnessShape{}
		for _, object := range objects {
			body, err := json.Marshal(object)
			if err != nil {
				t.Fatal(err)
			}
			var entry witnessed
			if err := json.Unmarshal(body, &entry); err != nil {
				t.Fatal(err)
			}
			shapes[entry.UnitID] = decodeAffectedWitness(t, form, entry.Witness)
		}
		return shapes
	}
	var fullObjects, compactObjects, byPathObjects []any
	for _, selection := range full.Selected {
		fullObjects = append(fullObjects, selection)
	}
	for _, selection := range compact {
		compactObjects = append(compactObjects, selection)
	}
	for _, unit := range byPath.Shared {
		byPathObjects = append(byPathObjects, unit)
	}
	for _, entry := range byPath.Paths {
		for _, unit := range entry.Additions {
			byPathObjects = append(byPathObjects, unit)
		}
	}
	fullShapes, compactShapes, byPathShapes := decodeAll("--full", fullObjects), decodeAll("compact", compactObjects), decodeAll("--by-path", byPathObjects)
	if len(fullShapes) != 3 || len(compactShapes) != 3 || len(byPathShapes) != 3 {
		t.Fatalf("fixture must select go:any, go:a and go:b in each form: %v %v %v", fullShapes, compactShapes, byPathShapes)
	}
	for unit, fullWitness := range fullShapes {
		if fullWitness.Via == nil || fullWitness.ViaLength != nil {
			t.Fatalf("--full witness of %s must carry via and no viaLength: %+v", unit, fullWitness)
		}
		for form, other := range map[string]affectedWitnessShape{"compact": compactShapes[unit], "--by-path": byPathShapes[unit]} {
			if other.ViaLength == nil || *other.ViaLength != len(fullWitness.Via) {
				t.Fatalf("%s witness of %s: viaLength %v, --full via %q", form, unit, other.ViaLength, fullWitness.Via)
			}
			if form == "--by-path" && other.Via != nil {
				t.Fatalf("--by-path witness of %s carries via: %+v", unit, other)
			}
			if other.Kind != fullWitness.Kind || !reflect.DeepEqual(other.Cause, fullWitness.Cause) || other.Glob != fullWitness.Glob || !reflect.DeepEqual(other.ReadScope, fullWitness.ReadScope) {
				t.Fatalf("%s witness of %s disagrees with --full: %+v vs %+v", form, unit, other, fullWitness)
			}
			if form == "compact" && other.DirtyPath != fullWitness.DirtyPath {
				t.Fatalf("compact witness of %s names %s, --full %s", unit, other.DirtyPath, fullWitness.DirtyPath)
			}
		}
	}
	if fullShapes["go:any"].Cause == nil {
		t.Fatalf("fixture lost its unbounded cause: %+v", fullShapes["go:any"])
	}

	// The CLI documents decode the same way.
	root := impactCLIRepository(t)
	base := coreFreezeParent(t, root)
	for _, arguments := range [][]string{{"affected", "--base", base}, {"affected", "--base", base, "--full"}} {
		code, stdout, stderr := runCLI(t, append([]string{"--root", root}, arguments...)...)
		if code != 0 {
			t.Fatalf("%q: exit %d: %s", arguments, code, stderr)
		}
		var document struct {
			Plan struct {
				Selected []witnessed `json:"selected"`
			} `json:"plan"`
		}
		if err := json.Unmarshal([]byte(stdout), &document); err != nil {
			t.Fatal(err)
		}
		if len(document.Plan.Selected) == 0 {
			t.Fatalf("%q selected nothing", arguments)
		}
		for _, selection := range document.Plan.Selected {
			decodeAffectedWitness(t, strings.Join(arguments, " "), selection.Witness)
		}
	}
}

// AFP-V0-9988: --json is --format json; the output is the default output.
func TestAffectedJSONFlagIsTheDefaultJSON_AFPV09988(t *testing.T) {
	t.Parallel()
	invocation, err := parseAffectedOptions([]string{"--json"})
	if err != nil || invocation.Format != "json" {
		t.Fatalf("--json: %+v %v", invocation, err)
	}
	if invocation, err = parseAffectedOptions([]string{"--json", "--format", "json"}); err != nil || invocation.Format != "json" {
		t.Fatalf("--json --format json: %+v %v", invocation, err)
	}
	for _, arguments := range [][]string{{"--json", "--format", "text"}, {"--format=text", "--json"}} {
		if _, err := parseAffectedOptions(arguments); err == nil || !strings.Contains(err.Error(), "--json cannot be combined with --format text") || !strings.Contains(err.Error(), "JSON by default") {
			t.Errorf("%q: %v", arguments, err)
		}
	}
	for _, arguments := range [][]string{{"--json", "--json"}, {"--json=true"}} {
		if _, err := parseAffectedOptions(arguments); err == nil || !strings.Contains(err.Error(), "argument --json") {
			t.Errorf("%q: %v", arguments, err)
		}
	}
	root := impactCLIRepository(t)
	base := coreFreezeParent(t, root)
	for _, form := range [][]string{{"--base", base}, {"--base", base, "--summary"}, {"--base", base, "--full"}} {
		arguments := append([]string{"--root", root, "affected"}, form...)
		code, want, stderr := runCLI(t, arguments...)
		if code != 0 {
			t.Fatalf("%q: exit %d: %s", form, code, stderr)
		}
		code, got, stderr := runCLI(t, append(arguments, "--json")...)
		if code != 0 || got != want {
			t.Fatalf("%q --json: exit %d, stderr %s, output differs:\n%s\nwant\n%s", form, code, stderr, got, want)
		}
	}
	code, stdout, stderr := runCLI(t, "--root", root, "affected", "--base", base, "--json", "--format", "text")
	if code != 2 || stdout != "" || !strings.Contains(stderr, "JSON by default") {
		t.Fatalf("--json --format text: exit %d stdout %q stderr %s", code, stdout, stderr)
	}
	if !strings.Contains(affectedHelp, "--json") {
		t.Fatal("affected --help does not record --json")
	}
}
