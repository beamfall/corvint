package workflow

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCAXV0010CoverWritesNotInstrumentedState checks CAX-V0-010 and
// CAX-V0-011: --states-output writes a cem-coverage-states/1 document that
// names a hunk whose path the profile never names not-instrumented, bound to
// the exact written map; the map bytes equal a run without the flag; the
// validator accepts it and refuses a state that disagrees with the witness, a
// different map, and a non-canonical document; and an aliasing path refuses
// before anything is written.
func TestCAXV0010CoverWritesNotInstrumentedState(t *testing.T) {
	t.Parallel()
	root, base, target := makeGoRepo(t, importReorderTarget, whisperTarget)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "cover.out", "mode: set\nexample.com/m/other/b.go:1.1,1.20 1 1\n")
	options := CoverOptions{MapPath: wire.ExcludedCEMPath, Coverprofile: "cover.out", TestRun: "go test ./other/...", Output: witnessMap}
	if _, err := openSession(t, root).Cover(ctx(), options); err != nil {
		t.Fatal(err)
	}
	plain, err := os.ReadFile(filepath.Join(root, witnessMap))
	if err != nil {
		t.Fatal(err)
	}
	options.StatesOutput = ".corvint/witness.states.json"
	result, err := openSession(t, root).Cover(ctx(), options)
	if err != nil {
		t.Fatal(err)
	}
	mapBytes, err := os.ReadFile(filepath.Join(root, witnessMap))
	if err != nil || !bytes.Equal(mapBytes, plain) {
		t.Fatalf("the map changed under --states-output: %v", err)
	}
	statesPath, _ := result["coverageStates"].(string)
	document, err := os.ReadFile(statesPath)
	if err != nil {
		t.Fatalf("coverageStates %q: %v", statesPath, err)
	}
	states, err := wire.ValidateCoverageStates(document, mapBytes)
	if err != nil {
		t.Fatalf("validate: %v\n%s", err, document)
	}
	hunks := readMapAt(t, root, witnessMap).Hunks
	if states.Profile != wire.CoverageStatesProfile || len(states.Hunks) != len(hunks) || len(hunks) != 1 ||
		states.Hunks[0] != (wire.CoverageStateHunk{ID: hunks[0].ID, State: wire.CoverageNotInstrumented}) ||
		hunks[0].Coverage.State != wire.CoverageUncovered {
		t.Fatalf("states = %+v, map hunks %+v", states, hunks)
	}
	refused := map[string][]byte{
		"covered for an uncovered witness": bytes.Replace(document, []byte(`"not-instrumented"`), []byte(`"covered"`), 1),
		"unknown state":                    bytes.Replace(document, []byte(`"not-instrumented"`), []byte(`"partial"`), 1),
		"non-canonical":                    bytes.Replace(document, []byte(`{"hunks"`), []byte(`{ "hunks"`), 1),
		"unknown member":                   bytes.Replace(document, []byte(`{"hunks"`), []byte(`{"extra":1,"hunks"`), 1),
		"other profile":                    bytes.Replace(document, []byte(wire.CoverageStatesProfile), []byte("cem-coverage-states/2"), 1),
	}
	for name, edited := range refused {
		if bytes.Equal(edited, document) {
			t.Fatalf("%s: the edit changed nothing", name)
		}
		if _, err := wire.ValidateCoverageStates(edited, mapBytes); err == nil {
			t.Errorf("%s: validated", name)
		}
	}
	if _, err := wire.ValidateCoverageStates(document, append(append([]byte{}, mapBytes...), ' ')); err == nil {
		t.Error("states validated against different map bytes")
	}

	// An instrumented hunk no block reaches stays uncovered in the states.
	writeFile(t, root, "cover.out", "mode: count\nexample.com/m/pkg/a.go:9.41,9.72 1 7\nexample.com/m/pkg/a.go:12.43,12.71 1 0\n")
	result, err = openSession(t, root).Cover(ctx(), options)
	if err != nil {
		t.Fatal(err)
	}
	document, _ = os.ReadFile(result["coverageStates"].(string))
	mapBytes, _ = os.ReadFile(filepath.Join(root, witnessMap))
	if states, err := wire.ValidateCoverageStates(document, mapBytes); err != nil || states.Hunks[0].State != wire.CoverageUncovered {
		t.Fatalf("instrumented uncovered: %v %+v", err, states)
	}

	for _, alias := range []string{witnessMap, wire.ExcludedCEMPath, strings.ToUpper(witnessMap), witnessMap + ".lock", filepath.Join(root, "states.json")} {
		bad := options
		bad.StatesOutput = alias
		if _, err := openSession(t, root).Cover(ctx(), bad); cemcode.CodeOf(err) != cemcode.InvalidArguments {
			t.Errorf("--states-output %s: got %v, want invalid-arguments", alias, err)
		}
	}
	if after, _ := os.ReadFile(filepath.Join(root, witnessMap)); !bytes.Equal(after, mapBytes) {
		t.Error("a refused --states-output changed the map")
	}
}
