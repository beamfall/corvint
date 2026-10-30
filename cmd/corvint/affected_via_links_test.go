package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/ruby"
)

// affectedChainSelections builds the selections of a require chain depth units
// deep: unit n's chain is units 1..n, and each unit also has a test user whose
// chain is units 1..n followed by the user, as the perf-budget Ruby fixture
// selects its models and specs (V1-2647).
func affectedChainSelections(depth int) []affected.Selection {
	selections := make([]affected.Selection, 0, 2*depth)
	chain := make([]string, 0, depth)
	for n := 1; n <= depth; n++ {
		unit := fmt.Sprintf("ruby:app/models/model_%05d.rb", n)
		chain = append(chain, unit)
		spec := fmt.Sprintf("ruby:spec/models/model_%05d_spec.rb", n)
		selections = append(selections,
			affected.Selection{UnitID: unit, Tests: []string{"t"}, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "app/models/model_00001.rb", Via: slices.Clone(chain)}},
			affected.Selection{UnitID: spec, Tests: []string{"t"}, Witness: affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "app/models/model_00001.rb", Via: append(slices.Clone(chain), spec)}})
	}
	return selections
}

// TestAFPV0640CompactWitnessChainsAreSharedLinks: AFP-V0-640 (V1-2647). The
// compact receipt states each witness chain once through plan.viaLinks; every
// selection's viaLength and the links rebuild exactly the chain --full carries,
// and the encoded plan grows linearly with chain depth, not with its square.
func TestAFPV0640CompactWitnessChainsAreSharedLinks(t *testing.T) {
	t.Parallel()
	encodedSize := func(depth int) int {
		selections := affectedChainSelections(depth)
		compact, links := compactAffectedSelections(selections)
		if len(links) != 2*depth-1 {
			t.Fatalf("depth %d: %d links, want one per non-head unit (%d)", depth, len(links), 2*depth-1)
		}
		for index, selection := range selections {
			witness := compact[index].Witness
			if witness.Via != nil || witness.ViaLength != len(selection.Witness.Via) || witness.Kind != selection.Witness.Kind || witness.DirtyPath != selection.Witness.DirtyPath {
				t.Fatalf("selection %s: compact witness %+v", selection.UnitID, witness)
			}
			chain, ok := expandAffectedVia(links, compact[index].UnitID, witness.ViaLength)
			if !ok || !slices.Equal(chain, selection.Witness.Via) {
				t.Fatalf("selection %s rebuilds %v, want %v", selection.UnitID, chain, selection.Witness.Via)
			}
		}
		encoded, err := gokernel.CanonicalJSON(affectedCompactPlan{Selected: compact, ViaLinks: links})
		if err != nil {
			t.Fatal(err)
		}
		return len(encoded)
	}
	small, large := encodedSize(400), encodedSize(1600)
	// Four times the depth: a linear document is about four times the bytes,
	// a quadratic one sixteen.
	if ratio := float64(large) / float64(small); ratio > 4.5 {
		t.Fatalf("compact plan grows %.1fx for 4x the chain depth (%d -> %d bytes), want linear", ratio, small, large)
	}
}

// TestAFPV0640CompactWitnessKeepsAChainTheLinksCannotState: AFP-V0-640. A
// chain that disagrees with a prefix an earlier selection recorded, or that
// does not end at its own unit, keeps its full via and records no link, so the
// links never rebuild a chain the plan did not carry.
func TestAFPV0640CompactWitnessKeepsAChainTheLinksCannotState(t *testing.T) {
	t.Parallel()
	witness := func(via ...string) affected.Witness {
		return affected.Witness{Kind: affected.WitnessDependency, DirtyPath: "a.go", Via: via}
	}
	selections := []affected.Selection{
		{UnitID: "a", Witness: witness("a")},
		{UnitID: "c", Witness: witness("a", "b", "c")},
		{UnitID: "d", Witness: witness("x", "b", "d")},
		{UnitID: "e", Witness: witness("a", "e", "z")},
		{UnitID: "f", Witness: witness("a", "b", "c", "f")},
	}
	compact, links := compactAffectedSelections(selections)
	for index, want := range []bool{false, false, true, true, false} {
		if listed := compact[index].Witness.Via != nil; listed != want {
			t.Fatalf("selection %s: full via listed %v, want %v", compact[index].UnitID, listed, want)
		}
		if compact[index].Witness.ViaLength != len(selections[index].Witness.Via) {
			t.Fatalf("selection %s: viaLength %d", compact[index].UnitID, compact[index].Witness.ViaLength)
		}
	}
	if want := map[string]string{"b": "a", "c": "b", "f": "c"}; fmt.Sprint(links) != fmt.Sprint(want) {
		t.Fatalf("links %v, want %v: a rejected chain must record nothing", links, want)
	}
	for _, index := range []int{0, 1, 4} {
		chain, ok := expandAffectedVia(links, compact[index].UnitID, compact[index].Witness.ViaLength)
		if !ok || !slices.Equal(chain, selections[index].Witness.Via) {
			t.Fatalf("selection %s rebuilds %v", compact[index].UnitID, chain)
		}
	}
	if _, ok := expandAffectedVia(links, "d", 3); ok {
		t.Fatal("a chain with a missing link must not rebuild")
	}
	if _, links := compactAffectedSelections(selections[:1]); links != nil {
		t.Fatalf("one-unit chains need no links, got %v", links)
	}
}

// TestAFPV0641LinkedSelectionCompactsLikeSelect: AFP-V0-641 (V1-2979). On a
// Ruby require chain shaped like the perf-budget fixture, the default receipt
// compacted from chains held by reference encodes byte-identically to the one
// compacted from Select's copied chains, with the same tiers, and the links
// still rebuild every chain --full carries.
func TestAFPV0641LinkedSelectionCompactsLikeSelect(t *testing.T) {
	t.Parallel()
	const depth = 40
	tree := map[string]string{
		"Gemfile":                          "source \"https://rubygems.org\"\ngem \"rails\"\ngem \"rspec-rails\"\n",
		"app/models/application_record.rb": "class ApplicationRecord\nend\n",
		"spec/spec_helper.rb":              "require \"rspec\"\n",
	}
	for n := 1; n <= depth; n++ {
		model, class := fmt.Sprintf("model_%05d", n), fmt.Sprintf("Model%05d", n)
		body := fmt.Sprintf("require_relative \"application_record\"\n\nclass %s < ApplicationRecord\nend\n", class)
		if n > 1 {
			body = fmt.Sprintf("require_relative \"model_%05d\"\n", n-1) + body
		}
		tree["app/models/"+model+".rb"] = body
		tree["app/services/"+model+"_service.rb"] = fmt.Sprintf("require_relative \"../models/%s\"\n\nclass %sService\nend\n", model, class)
		tree["spec/models/"+model+"_spec.rb"] = fmt.Sprintf("require \"spec_helper\"\nrequire_relative \"../../app/models/%s\"\n\nRSpec.describe %s do\nend\n", model, class)
	}
	root := t.TempDir()
	for name, body := range tree {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := affected.Build(context.Background(), root, ruby.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, dirty := range [][]string{{"app/models/model_00001.rb"}, {"app/models/model_00020.rb", "spec/models/model_00003_spec.rb"}} {
		full := affected.Select(graph, dirty)
		linked := affected.SelectLinked(graph, dirty)
		if len(full.Selected) < depth/2 {
			t.Fatalf("%v: selected %d units, want the chain", dirty, len(full.Selected))
		}
		encode := func(plan affected.Plan) string {
			compact, links := compactAffectedSelections(plan.Selected)
			encoded, err := gokernel.CanonicalJSON(map[string]any{"plan": affectedCompactPlan{Selected: compact, ViaLinks: links}, "tiers": tierAffectedSelections(plan.Selected)})
			if err != nil {
				t.Fatal(err)
			}
			for index, selection := range full.Selected {
				chain, ok := expandAffectedVia(links, compact[index].UnitID, compact[index].Witness.ViaLength)
				if compact[index].Witness.Via != nil {
					chain, ok = compact[index].Witness.Via, true
				}
				if !ok || !slices.Equal(chain, selection.Witness.Via) {
					t.Fatalf("%v: %s rebuilds %v, want %v", dirty, selection.UnitID, chain, selection.Witness.Via)
				}
			}
			return string(encoded)
		}
		if encode(linked) != encode(full) {
			t.Fatalf("%v: compact receipt from linked chains differs from Select's", dirty)
		}
		linked.MaterializeVia()
		encodedFull, _ := full.Canonical()
		encodedLinked, _ := linked.Canonical()
		if string(encodedFull) != string(encodedLinked) {
			t.Fatalf("%v: --full plan differs after materializing", dirty)
		}
	}
}
