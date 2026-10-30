package affected

import (
	"context"
	"strings"
	"testing"
)

// advisoryChain is core <-advisory- service <- client, plus a unit that names
// core both ways.
func advisoryChain() fake {
	return fake{name: "x", units: []Unit{
		{ID: "x:core", Sources: []string{"core.x"}},
		{ID: "x:service", Sources: []string{"service.x"}, Tests: []string{"service_test.x"}, AdvisoryImports: []string{"x:core"}},
		{ID: "x:client", Sources: []string{"client.x"}, Tests: []string{"client_test.x"}, Imports: []string{"x:service"}},
		{ID: "x:direct", Sources: []string{"direct.x"}, Tests: []string{"direct_test.x"}, Imports: []string{"x:core"}, AdvisoryImports: []string{"x:core"}},
		{ID: "x:solo", Sources: []string{"solo.x"}, Tests: []string{"solo_test.x"}},
	}}
}

// TestAdvisoryEdgeSelectsWithAnAdvisoryWitness_RUBYAFFECTEDV0009 proves an
// advisory edge selects like an import, marks every unit reached through it,
// yields to an explicit edge, and enters the digest only when present.
func TestAdvisoryEdgeSelectsWithAnAdvisoryWitness_RUBYAFFECTEDV0009(t *testing.T) {
	graph, err := Build(context.Background(), t.TempDir(), advisoryChain())
	if err != nil {
		t.Fatal(err)
	}
	plan := Select(graph, []string{"core.x"})
	want := map[string]string{
		"x:service": WitnessAdvisoryConstant,
		"x:client":  WitnessAdvisoryConstant,
		"x:direct":  WitnessDependency,
	}
	got := witnesses(plan)
	for id, kind := range want {
		if got[id].Kind != kind {
			t.Fatalf("%s witness=%+v want %s; plan=%+v", id, got[id], kind, plan)
		}
	}
	if _, selected := got["x:solo"]; selected || plan.Scope != ScopeBounded {
		t.Fatalf("scope=%s selected=%v", plan.Scope, plan.SelectedTests())
	}
	if via := got["x:client"].Via; len(via) != 3 || via[0] != "x:core" || via[2] != "x:client" {
		t.Fatalf("client chain=%v", via)
	}

	plain, err := Build(context.Background(), t.TempDir(), chain())
	if err != nil {
		t.Fatal(err)
	}
	body, err := plain.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "advisoryImports") {
		t.Fatalf("a graph without advisory edges projects them: %s", body)
	}
	advised := chain()
	advised.units[3].AdvisoryImports = []string{"x:core"}
	edged, err := Build(context.Background(), t.TempDir(), advised)
	if err != nil {
		t.Fatal(err)
	}
	edgedBody, err := edged.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if plain.Digest() == edged.Digest() || !strings.Contains(string(edgedBody), `"advisoryImports":["x:core"]`) {
		t.Fatalf("advisory edge is outside the digest: %s", edgedBody)
	}
	invalid := chain()
	invalid.units[3].AdvisoryImports = []string{"x:mid", "x:core"}
	if _, err := Build(context.Background(), t.TempDir(), invalid); err == nil {
		t.Fatal("unsorted advisory edges were accepted")
	}
}
