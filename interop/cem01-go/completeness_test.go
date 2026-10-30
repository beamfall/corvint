package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// TestCCPV0009PortableCompletenessVectors runs every public vector of
// protocol/cem-completeness-1 through the portable reader. The native
// verifier asserts the same vectors, so both implementations agree on the
// outcome, the independence label and every explicit entry (CCP-V0-009).
func TestCCPV0009PortableCompletenessVectors(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable artifact roots are only supported on Darwin/Linux")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate completeness_test.go")
	}
	dir := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "protocol", "cem-completeness-1"))
	var packet struct {
		CompletenessProfile string `json:"completenessProfile"`
		ArtifactRoot        string `json:"artifactRoot"`
		Vectors             []struct {
			Name         string              `json:"name"`
			Map          string              `json:"map"`
			Inventory    *string             `json:"inventory"`
			MapProducer  string              `json:"mapProducer"`
			Target       string              `json:"target"`
			Outcome      string              `json:"outcome"`
			Independence *string             `json:"independence"`
			Missing      []completenessEntry `json:"missing"`
			NotObserved  []completenessEntry `json:"notObserved"`
		} `json:"vectors"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "VECTORS.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.CompletenessProfile != completenessProfile || len(packet.Vectors) < 5 {
		t.Fatalf("packet profile %q with %d vectors", packet.CompletenessProfile, len(packet.Vectors))
	}
	for _, v := range packet.Vectors {
		raw, err := os.ReadFile(filepath.Join(dir, v.Map))
		if err != nil {
			t.Fatal(err)
		}
		var inventory []byte
		if v.Inventory != nil {
			if inventory, err = os.ReadFile(filepath.Join(dir, *v.Inventory)); err != nil {
				t.Fatal(err)
			}
		}
		r := verifyCompleteness(context.Background(), raw, filepath.Join(dir, packet.ArtifactRoot), inventory, v.MapProducer, v.Target)
		b, _ := json.Marshal(r)
		want := func(e []completenessEntry) []completenessEntry {
			if e == nil {
				return []completenessEntry{}
			}
			return e
		}
		if r.Profile != completenessResultProfile || r.ReferenceIntegrity != "NOT_EVALUATED" || r.MapSHA256 != shaHex(raw) || r.Outcome != v.Outcome || !reflect.DeepEqual(r.Missing, want(v.Missing)) || !reflect.DeepEqual(r.NotObserved, want(v.NotObserved)) {
			t.Fatalf("%s: got %s, want outcome %s missing %+v notObserved %+v", v.Name, b, v.Outcome, v.Missing, v.NotObserved)
		}
		if (v.Independence == nil) != (r.Inventory == nil) || r.Inventory != nil && (r.Inventory.Independence != *v.Independence || r.Inventory.Authentication != "NOT_OBSERVED" || r.Inventory.MapProducer != v.MapProducer) {
			t.Fatalf("%s: inventory provenance %s, want independence %v", v.Name, b, v.Independence)
		}
		if r.Outcome == "PASS" && r.Inventory.Independence != "INDEPENDENT" {
			t.Fatalf("%s: PASS without an independent inventory: %s", v.Name, b)
		}
	}
}
