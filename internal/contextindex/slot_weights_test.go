package contextindex

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLTAV0011SlotWeightOrderIsIdentityByDefaultAndStable(t *testing.T) {
	rows := []contextRow{{kind: "definition", path: "a"}, {kind: "test", path: "b"}, {kind: "lexical", path: "c"}, {kind: "test", path: "d"}}
	if got := orderBySlotWeight(append([]contextRow{}, rows...), nil); !reflect.DeepEqual(got, rows) {
		t.Fatalf("default order changed: %v", got)
	}
	got := orderBySlotWeight(append([]contextRow{}, rows...), SlotWeights{"test": 1, "lexical": -1})
	paths := []string{}
	for _, row := range got {
		paths = append(paths, row.path)
	}
	if strings.Join(paths, ",") != "b,d,a,c" {
		t.Fatalf("weighted order = %v", paths)
	}
}

func TestLTAV0011WeightedPacketDisclosesTheTraceAndNilIsTaskContext(t *testing.T) {
	index := taskContextFixture(t)
	task := "where does Split demux keys"
	plain, err := TaskContext(context.Background(), index, task, "", 5)
	if err != nil {
		t.Fatal(err)
	}
	unweighted, err := TaskContextWeighted(context.Background(), index, task, "", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	plainBytes, _ := json.Marshal(plain)
	unweightedBytes, _ := json.Marshal(unweighted)
	if string(plainBytes) != string(unweightedBytes) {
		t.Fatalf("nil trace changed the packet:\n%s\n%s", plainBytes, unweightedBytes)
	}
	weighted, err := TaskContextWeighted(context.Background(), index, task, "", 5, &AdmittedSlotWeights{Weights: SlotWeights{"test": 2}, SHA256: "sha256:x"})
	if err != nil {
		t.Fatal(err)
	}
	disclosed, _ := weighted["learned_slot_weights"].(map[string]any)
	if disclosed["path"] != SlotWeightsPath || disclosed["sha256"] != "sha256:x" {
		t.Fatalf("trace not disclosed: %v", weighted["learned_slot_weights"])
	}
	results := mapsFromAny(weighted["results"])
	if len(results) == 0 || results[0]["kind"] != "test" {
		t.Fatalf("test slot not first under weight: %v", results)
	}
}

func TestLTAV0012AdmittedSlotWeightsLoaderFailsClosed(t *testing.T) {
	write := func(t *testing.T, content string) string {
		t.Helper()
		root := t.TempDir()
		path := filepath.Join(root, filepath.FromSlash(SlotWeightsPath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	if admitted, err := LoadAdmittedSlotWeights(t.TempDir()); admitted != nil || err != nil {
		t.Fatalf("absent file = %v, %v", admitted, err)
	}
	admitted, err := LoadAdmittedSlotWeights(write(t, `{"schemaVersion":1,"weights":{"test":2},"evaluation":{}}`))
	if err != nil || admitted.Weights["test"] != 2 || !strings.HasPrefix(admitted.SHA256, "sha256:") {
		t.Fatalf("valid file = %v, %v", admitted, err)
	}
	refused := map[string]string{
		"out of range":     `{"schemaVersion":1,"weights":{"test":3},"evaluation":{}}`,
		"unknown relation": `{"schemaVersion":1,"weights":{"frame":1},"evaluation":{}}`,
		"unknown field":    `{"schemaVersion":1,"weights":{},"evaluation":{},"extra":1}`,
		"schema":           `{"schemaVersion":2,"weights":{},"evaluation":{}}`,
		"oversize":         `{"schemaVersion":1,"weights":{},"evaluation":"` + strings.Repeat("x", maxSlotWeightsBytes) + `"}`,
	}
	for name, content := range refused {
		if _, err := LoadAdmittedSlotWeights(write(t, content)); err == nil || !strings.Contains(err.Error(), "corvint eval --reset-slot-weights") {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere.json")
	if err := os.WriteFile(target, []byte(`{"schemaVersion":1,"weights":{},"evaluation":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, filepath.FromSlash(SlotWeightsPath))
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAdmittedSlotWeights(root); err == nil {
		t.Fatal("symlinked trace was loaded")
	}
}

func TestLTAV0009LiveContextPathCannotReachTheLedgers(t *testing.T) {
	output, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, output)
	}
	for _, forbidden := range []string{"/internal/unplannedread", "/internal/observations", "/internal/slotlearn"} {
		if strings.Contains(string(output), forbidden+"\n") {
			t.Fatalf("contextindex depends on %s", forbidden)
		}
	}
}
