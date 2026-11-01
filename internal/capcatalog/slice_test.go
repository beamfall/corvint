package capcatalog

import (
	"encoding/json"
	"slices"
	"testing"
)

// CAP-V0-950: every command of the embedded catalog slices to at most
// MaxCommandSliceBytes; the slice carries the catalog entry with only trailing
// write statements omitted and counted, and an unknown name lists every name.
func TestCAPV0950EveryCommandSlicesUnderTheBudget(t *testing.T) {
	var catalog Catalog
	if err := json.Unmarshal(Snapshot(), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, command := range catalog.Commands {
		encoded, _, ok := Slice(Snapshot(), command.Name)
		if !ok || len(encoded) > MaxCommandSliceBytes || encoded[len(encoded)-1] != '\n' {
			t.Fatalf("%s: ok=%v bytes=%d", command.Name, ok, len(encoded))
		}
		var slice CommandSlice
		if err := json.Unmarshal(encoded, &slice); err != nil {
			t.Fatal(err)
		}
		kept := len(slice.Command.WriteStatements)
		if slice.Schema != CommandSliceSchema || slice.CatalogSchema != catalog.Schema || slice.FullCatalog != "corvint help --json" ||
			kept+slice.WriteStatementsOmitted != len(command.WriteStatements) || !slices.Equal(slice.Command.WriteStatements, command.WriteStatements[:kept]) {
			t.Fatalf("%s: slice %+v", command.Name, slice)
		}
		slice.Command.WriteStatements = command.WriteStatements
		want, _ := json.Marshal(command)
		got, _ := json.Marshal(slice.Command)
		if string(want) != string(got) {
			t.Fatalf("%s: entry differs from the catalog's", command.Name)
		}
	}
	if _, names, ok := Slice(Snapshot(), "nope"); ok || len(names) != len(catalog.Commands) {
		t.Fatalf("unknown ok=%v names=%d", ok, len(names))
	}
}
