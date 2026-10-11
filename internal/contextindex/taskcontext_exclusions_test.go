package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"testing"
)

// TCP-V0-065: a real Git Latin-1 path is disclosed even when neither the task
// nor a subject names it. Snapshot reads preserve the same gap and packet.
func TestTaskContextDisclosesNonUTF8Exclusions(t *testing.T) {
	for _, clean := range []bool{true, false} {
		t.Run(fmt.Sprintf("clean=%v", clean), func(t *testing.T) {
			root := nonUTF8Repository(t, clean)
			index, err := Build(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			want := Exclusion{Path: "latin/caf�.go", Reason: "unsafe-or-non-utf8-path"}
			if !exclusionIn(index.Exclusions, want) {
				t.Fatalf("fixture exclusion missing: %v", index.Exclusions)
			}
			packet, err := TaskContext(context.Background(), index, "Explain MintToken", "", 20)
			if err != nil {
				t.Fatal(err)
			}
			if packet["state"] != "READY" || packet["subject"] != nil {
				t.Fatalf("ordinary unnamed-subject context refused: %v", packet)
			}
			exclusions, ok := packet["exclusions"].(map[string]any)
			if !ok {
				t.Fatal("TCP-V0-065: packet does not disclose the unnamed exclusion")
			}
			samples := mapsFromAny(exclusions["samples"])
			if exclusions["count"] != 1 || len(samples) != 1 || samples[0]["path"] != want.Path || samples[0]["reason"] != want.Reason {
				t.Fatalf("TCP-V0-065: exclusions = %v", exclusions)
			}
			for _, row := range mapsFromAny(packet["results"]) {
				if row["id"] == want.Path {
					t.Fatal("exclusion was presented as ranked evidence")
				}
			}
			wire := contextExclusionWire(t, packet)
			if packet["schema_version"] != 1 || !json.Valid(wire) {
				t.Fatalf("unexpected wire encoding: %s", wire)
			}
			if _, err := WriteSnapshot(index); err != nil {
				t.Fatal(err)
			}
			loaded, hit, err := LoadSnapshot(context.Background(), root)
			if err != nil || !hit {
				t.Fatalf("snapshot hit=%v err=%v", hit, err)
			}
			for i := 0; i < 2; i++ {
				reloaded, err := TaskContext(context.Background(), loaded, "Explain MintToken", "", 20)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(wire, contextExclusionWire(t, reloaded)) {
					t.Fatal("TCP-V0-065: eager, repeated, and snapshot packet bytes differ")
				}
			}
		})
	}
}

// TCP-V0-065: cap and deterministic order do not depend on index insertion
// order; stripping the additive field leaves every prior packet member intact.
func TestTaskContextExclusionsAreBoundedAndAdditive(t *testing.T) {
	index := recipeFixtureIndex(t)
	baseline := recipePacket(t, index, "", recipeFixtureTask)
	for i := 12; i >= 0; i-- {
		index.Exclusions = append(index.Exclusions, Exclusion{fmt.Sprintf("excluded/%02d.go", i), "source exceeds size bound"})
	}
	index.Exclusions = append(index.Exclusions, Exclusion{"excluded/00.go", "generated-file header excluded"})
	index.UnsupportedSuffixCount = 2
	original := slices.Clone(index.Exclusions)
	packet := recipePacket(t, index, "", recipeFixtureTask)
	exclusions := packet["exclusions"].(map[string]any)
	samples := mapsFromAny(exclusions["samples"])
	if exclusions["count"] != 16 || len(samples) != 10 {
		t.Fatalf("TCP-V0-065: bounded summary = %v", exclusions)
	}
	for i, sample := range samples {
		path := fmt.Sprintf("excluded/%02d.go", max(i-1, 0))
		reason := "source exceeds size bound"
		if i == 0 {
			reason = "generated-file header excluded"
		}
		if len(sample) != 2 || sample["path"] != path || sample["reason"] != reason {
			t.Fatalf("sample %d = %v, want %s / %s", i, sample, path, reason)
		}
	}
	if !reflect.DeepEqual(index.Exclusions, original) {
		t.Fatal("packet sampling mutated the index")
	}
	empty := recipePacket(t, index, "", "zzunknownword")
	if !reflect.DeepEqual(empty["exclusions"], exclusions) {
		t.Fatal("exclusion disclosure depends on task relevance")
	}
	slices.Reverse(index.Exclusions)
	if !bytes.Equal(contextExclusionWire(t, packet), contextExclusionWire(t, recipePacket(t, index, "", recipeFixtureTask))) {
		t.Fatal("input order changed packet bytes")
	}
	// This consumer knows exactly the prior top-level shape. Unknown members
	// are ignored while all known JSON values survive decoding unchanged.
	var old struct {
		Tool          json.RawMessage `json:"tool"`
		OK            json.RawMessage `json:"ok"`
		Mutates       json.RawMessage `json:"mutates"`
		SchemaVersion json.RawMessage `json:"schema_version"`
		Revision      json.RawMessage `json:"revision"`
		State         json.RawMessage `json:"state"`
		Subject       json.RawMessage `json:"subject"`
		Request       json.RawMessage `json:"request"`
		Coverage      json.RawMessage `json:"coverage"`
		Results       json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(contextExclusionWire(t, packet), &old); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(contextExclusionWire(t, old), &decoded); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(contextExclusionWire(t, decoded), contextExclusionWire(t, baseline)) {
		t.Fatal("TCP-V0-065: old consumer sees changed ranking or prior packet members")
	}
	delete(packet, "exclusions")
	if !bytes.Equal(contextExclusionWire(t, packet), contextExclusionWire(t, baseline)) {
		t.Fatal("TCP-V0-065: disclosure changed prior packet bytes")
	}
}

// TCP-V0-065: no exclusions means no new member and unchanged frozen bytes.
func TestTaskContextExclusionsAbsentKeepsFrozenBytes(t *testing.T) {
	index := recipeFixtureIndex(t)
	if len(index.Exclusions)+index.UnsupportedSuffixCount != 0 {
		t.Fatal("no-exclusion fixture drifted")
	}
	packet := recipePacket(t, index, "", recipeFixtureTask)
	if _, present := packet["exclusions"]; present {
		t.Fatal("zero-count disclosure must be absent")
	}
	wire, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(append(wire, '\n'), golden) {
		t.Fatal("TCP-V0-065: no-exclusion packet differs from frozen bytes")
	}
}

// TCP-V0-065: suffix-only exclusions count even without a recorded sample.
func TestTaskContextExclusionsCountUnsupportedSuffixes(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":    "module example.test/suffix\n\ngo 1.27.0\n",
		"main.go":   "package main\nfunc MintToken() {}\n",
		"asset.bin": "not indexed\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if index.UnsupportedSuffixCount != 1 || len(index.Exclusions) != 0 {
		t.Fatal("suffix-only fixture drifted")
	}
	packet, err := TaskContext(context.Background(), index, "Explain MintToken", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	got := packet["exclusions"]
	want := map[string]any{"count": 1, "samples": []any{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TCP-V0-065: suffix disclosure = %v, want %v", got, want)
	}
}

func contextExclusionWire(t *testing.T, value any) []byte {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return wire
}
