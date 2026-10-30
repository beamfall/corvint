package verify

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

type completenessVector struct {
	Name         string              `json:"name"`
	Map          string              `json:"map"`
	Inventory    *string             `json:"inventory"`
	MapProducer  string              `json:"mapProducer"`
	Target       string              `json:"target"`
	Outcome      string              `json:"outcome"`
	Independence *string             `json:"independence"`
	Missing      []CompletenessEntry `json:"missing"`
	NotObserved  []CompletenessEntry `json:"notObserved"`
}

// TestCCPV0009NativeCompletenessVectors runs every public vector of
// protocol/cem-completeness-1 through the native completeness verifier: a
// matching complete inventory passes, an omitted failed retry, a narrowed
// selection, a missing receipt and every other omission refuse with their
// exact missing entries, a self-authored inventory never passes, and no
// inventory is NOT_OBSERVED (CCP-V0-004..CCP-V0-009). The portable reader
// asserts the same vectors.
func TestCCPV0009NativeCompletenessVectors(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("stable artifact roots are only supported on Darwin/Linux")
	}
	dir := filepath.Join(repositoryRoot(t), "protocol/cem-completeness-1")
	var packet struct {
		CompletenessProfile string               `json:"completenessProfile"`
		ArtifactRoot        string               `json:"artifactRoot"`
		Vectors             []completenessVector `json:"vectors"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "VECTORS.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &packet); err != nil {
		t.Fatal(err)
	}
	if packet.CompletenessProfile != CompletenessProfile || len(packet.Vectors) < 5 {
		t.Fatalf("packet profile %q with %d vectors", packet.CompletenessProfile, len(packet.Vectors))
	}
	for _, v := range packet.Vectors {
		raw, err := os.ReadFile(filepath.Join(dir, v.Map))
		if err != nil {
			t.Fatal(err)
		}
		o := CompletenessOptions{ArtifactRoot: filepath.Join(dir, packet.ArtifactRoot), MapProducer: v.MapProducer, Target: v.Target}
		if v.Inventory != nil {
			if o.Inventory, err = os.ReadFile(filepath.Join(dir, *v.Inventory)); err != nil {
				t.Fatal(err)
			}
		}
		r := Completeness(context.Background(), raw, o)
		b, _ := json.Marshal(r)
		if r.Profile != CompletenessResultProfile || r.CompletenessProfile != CompletenessProfile || r.ReferenceIntegrity != "NOT_EVALUATED" || r.MapSha256 != stableDigest(raw) {
			t.Fatalf("%s: result header %s", v.Name, b)
		}
		if r.Outcome != v.Outcome || !reflect.DeepEqual(r.Missing, nonNil(v.Missing)) || !reflect.DeepEqual(r.NotObserved, nonNil(v.NotObserved)) {
			t.Fatalf("%s: got %s, want outcome %s missing %+v notObserved %+v", v.Name, b, v.Outcome, v.Missing, v.NotObserved)
		}
		switch {
		case v.Independence == nil && r.Inventory != nil, v.Independence != nil && (r.Inventory == nil || r.Inventory.Independence != *v.Independence):
			t.Fatalf("%s: inventory provenance %s, want independence %v", v.Name, b, v.Independence)
		}
		if r.Inventory != nil && (r.Inventory.Authentication != CompletenessNotObserved || r.Inventory.MapProducer != v.MapProducer) {
			t.Fatalf("%s: provenance must carry the map producer and unauthenticated identity: %s", v.Name, b)
		}
		if r.Outcome == CompletenessPass && (r.Inventory == nil || r.Inventory.Independence != InventoryIndependent) {
			t.Fatalf("%s: PASS without an independent inventory: %s", v.Name, b)
		}
	}
}

func nonNil(e []CompletenessEntry) []CompletenessEntry {
	if e == nil {
		return []CompletenessEntry{}
	}
	return e
}

// TestCCPV0006RequiredTestSelectedButUnrecordedRefusesReceiptMissing: a
// required test the cited plan selects but no cited receipt records refuses
// receipt-missing, and an incomplete receipt is not observed (CCP-V0-005,
// CCP-V0-006). The case needs a receipt a valid map cannot carry, so it is
// judged directly.
func TestCCPV0006RequiredTestSelectedButUnrecordedRefusesReceiptMissing(t *testing.T) {
	t.Parallel()
	base, target, digest := "d39fdcfe8330ebe1b7158e0fcc23020626d41947", "f8cd50b5509eb7176a79ef4e0787904feb66af71", "5e0af7807953ff823e91bb5a8eea2b0681b11a7e46dfc7b3e05641c79a052d30"
	inputs := map[string]string{"greet.go": "0540c64566bdc430b91135791858588a6f8a4df3a6918b1064795b373d566645"}
	run := citedRun{ReceiptSha256: digest, Readable: true, Complete: false, Runner: "go-test", Project: "p", Selection: []string{"p::A", "p::B"}, InputFiles: inputs, Outcomes: map[string]string{"p::A": "PASSED"}}
	inv := runInventory{BaseRevision: base, TargetRevision: target, RequiredTests: []inventoryTest{{"go-test", "p", "p::A"}, {"go-test", "p", "p::B"}}, Runs: []inventoryRun{{Runner: "go-test", Project: "p", Attempt: 1, ReceiptSha256: digest, InputFiles: inputs, Outcomes: []inventoryOutcome{{"p::A", "PASSED"}}}}}
	r := newCompletenessResult([]byte("{}"))
	r.Inventory = &InventoryProvenance{Independence: InventoryIndependent}
	judgeCompleteness(&r, base, target, []citedRun{run}, inv)
	r = r.finish()
	want := []CompletenessEntry{{Reason: "receipt-missing", Runner: "go-test", Project: "p", Test: "p::B"}}
	if r.Outcome != CompletenessRefuse || !reflect.DeepEqual(r.Missing, want) || len(r.NotObserved) != 1 || r.NotObserved[0].Reason != "receipt-incomplete" {
		t.Fatalf("got %+v", r)
	}
}

// TestCCPV0003InventoryRefusals: a malformed, unknown-member, duplicate-key,
// wrong-profile or producer-less inventory refuses inventory-invalid and the
// result never passes (CCP-V0-003).
func TestCCPV0003InventoryRefusals(t *testing.T) {
	t.Parallel()
	valid := `{"profile":"cem-run-inventory/1","producer":"ci","baseRevision":"d39fdcfe8330ebe1b7158e0fcc23020626d41947","targetRevision":"f8cd50b5509eb7176a79ef4e0787904feb66af71","requiredTests":[],"runs":[]}`
	if _, p, err := decodeRunInventory([]byte(valid), "map"); err != nil || p.Independence != InventoryIndependent {
		t.Fatalf("valid inventory: %v %+v", err, p)
	}
	for name, raw := range map[string]string{
		"not-json":      `{`,
		"unknown":       `{"extra":1,` + valid[1:],
		"duplicate-key": `{"producer":"x",` + valid[1:],
		"profile":       `{"profile":"cem-run-inventory/0"` + valid[len(`{"profile":"cem-run-inventory/1"`):],
		"no-producer":   `{"profile":"cem-run-inventory/1","producer":""` + valid[len(`{"profile":"cem-run-inventory/1","producer":"ci"`):],
		"null-runs":     valid[:len(valid)-len(`[]}`)] + `null}`,
		"trailing":      valid + `{}`,
	} {
		r := Completeness(context.Background(), []byte("{}"), CompletenessOptions{Inventory: []byte(raw), MapProducer: "map", Target: "t"})
		if r.Outcome != CompletenessRefuse || len(r.Missing) != 1 || r.Missing[0].Reason != "inventory-invalid" || r.Inventory == nil || r.Inventory.Independence != CompletenessNotObserved {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}
