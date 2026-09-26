package slotlearn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

func writeLedger(t *testing.T, root, name string, rows []string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".corvint", name), []byte(strings.Join(rows, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func unplannedRow(path string, planned bool) string {
	return fmt.Sprintf(`{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":%q,"bytes":10,"size_known":true,"packet":"sha256:p","planned":%t}`, path, planned)
}

func TestLTAV0009ServingSlotTable(t *testing.T) {
	for path, want := range map[string]string{
		"cache/demux_test.go": "test", "tests/test_x.py": "test", "web/a.spec.ts": "test", "src/test/A.java": "test",
		"README.md": "documentation", "docs/guide.rst": "documentation", "notes.txt": "documentation",
		"site/page.mdx": "documentation", "docs/guide.html": "lexical",
		"cache/demux.go": "definition", "web/app.ts": "definition",
	} {
		if got := ServingSlot(path); got != want {
			t.Fatalf("%s = %s, want %s", path, got, want)
		}
		if !slices.Contains(contextindex.LearnableSlots, want) {
			t.Fatalf("%s serves %s, which no learned weight may reorder", path, want)
		}
	}
}

func TestLTAV0009LabelsAreBoundedAndPlannedReadsAreNotLabels(t *testing.T) {
	root := t.TempDir()
	rows := []string{unplannedRow("already/in/packet.go", true)}
	for number := range MaxLabelPaths + 44 {
		rows = append(rows, unplannedRow(fmt.Sprintf("pkg/p%03d.go", number), false))
	}
	writeLedger(t, root, "unplanned-reads.jsonl", rows)
	labels, err := ReadLabels(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(labels.Paths) != MaxLabelPaths || !labels.Truncated || labels.PlannedReads != 1 || labels.Paths[0] != "pkg/p000.go" {
		t.Fatalf("labels = %+v", labels)
	}
	for _, path := range labels.Paths {
		if path == "already/in/packet.go" {
			t.Fatal("a planned re-read became a negative label")
		}
	}
}

func TestLTAV0009ProposalsAreBoundedDeterministicAndInRange(t *testing.T) {
	classes := map[string]int{"definition": 2, "test": 5, "lexical": 2}
	first := Propose(classes)
	if !reflect.DeepEqual(first, Propose(classes)) || len(first) != MaxProposals {
		t.Fatalf("proposals = %v", first)
	}
	want := []contextindex.SlotWeights{{"test": 1}, {"test": 2}, {"definition": 1}, {"definition": 2}}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("proposals = %v, want %v", first, want)
	}
	for _, weights := range first {
		if err := contextindex.ValidateSlotWeights(weights); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLTAV0010NoLabelsRefusesWithoutEvaluating(t *testing.T) {
	result, admitted, err := Learn(context.Background(), t.TempDir(), "/nonexistent/goldens.json", true)
	if err != nil || admitted {
		t.Fatalf("admitted=%v err=%v", admitted, err)
	}
	if result["decision"].(map[string]any)["reason"] != ReasonNoLabels {
		t.Fatalf("result = %v", result)
	}
}

func TestLTAV0012AdmittedTraceRoundTripsAndResetRestoresDefault(t *testing.T) {
	root := t.TempDir()
	if removed, err := Reset(root); removed || err != nil {
		t.Fatalf("reset of absent trace = %v, %v", removed, err)
	}
	if err := writeAdmitted(root, contextindex.SlotWeights{"test": 2}, map[string]any{
		"goldens_sha256": "sha256:" + strings.Repeat("a", 64), "revision": strings.Repeat("b", 40), "heldout_cases": 2,
		"baseline": map[string]any{"critical_misses": 0, "must_include_hits": 1, "top5_hits": 1},
		"arm":      map[string]any{"critical_misses": 0, "must_include_hits": 2, "top5_hits": 2},
	}); err != nil {
		t.Fatal(err)
	}
	admitted, err := contextindex.LoadAdmittedSlotWeights(root)
	if err != nil || admitted == nil || admitted.Weights["test"] != 2 {
		t.Fatalf("admitted = %v, %v", admitted, err)
	}
	if removed, err := Reset(root); !removed || err != nil {
		t.Fatalf("reset = %v, %v", removed, err)
	}
	if admitted, err := contextindex.LoadAdmittedSlotWeights(root); admitted != nil || err != nil {
		t.Fatalf("after reset = %v, %v", admitted, err)
	}
}

func TestLTAV0012ResetRefusesASymlinkedStoreAndRemovesALeafLink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, filepath.Base(contextindex.SlotWeightsPath))
	original := []byte(`{"outside":true}`)
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}
	store := filepath.Join(root, filepath.Dir(filepath.FromSlash(contextindex.SlotWeightsPath)))
	if err := os.Symlink(outside, store); err != nil {
		t.Fatal(err)
	}
	if removed, err := Reset(root); removed || err == nil {
		t.Fatalf("reset through a symlinked store = %v, %v", removed, err)
	}
	if after, err := os.ReadFile(target); err != nil || string(after) != string(original) {
		t.Fatalf("outside file changed: %q, %v", after, err)
	}
	if err := os.Remove(store); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(store, filepath.Base(contextindex.SlotWeightsPath))); err != nil {
		t.Fatal(err)
	}
	if removed, err := Reset(root); !removed || err != nil {
		t.Fatalf("reset of a leaf link = %v, %v", removed, err)
	}
	if after, err := os.ReadFile(target); err != nil || string(after) != string(original) {
		t.Fatalf("leaf link target changed: %q, %v", after, err)
	}
}

func TestLTAV0012PinnedStoreIgnoresASubstitutedDirectory(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	name := filepath.Base(contextindex.SlotWeightsPath)
	store := filepath.Join(root, filepath.Dir(filepath.FromSlash(contextindex.SlotWeightsPath)))
	if err := os.Mkdir(store, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{store, outside} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(directory), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	pinned, err := contextindex.OpenSlotWeightsStore(root)
	if err != nil || pinned == nil {
		t.Fatalf("open store = %v, %v", pinned, err)
	}
	defer pinned.Close()
	moved := filepath.Join(root, "moved")
	if err := os.Rename(store, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, store); err != nil {
		t.Fatal(err)
	}
	if err := pinned.Remove(name); err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(filepath.Join(outside, name)); err != nil || string(after) != outside {
		t.Fatalf("outside file changed: %q, %v", after, err)
	}
	if _, err := os.Lstat(filepath.Join(moved, name)); !os.IsNotExist(err) {
		t.Fatalf("pinned file not removed: %v", err)
	}
}
