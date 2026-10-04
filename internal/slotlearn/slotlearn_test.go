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
	if planned {
		// The writer blanks a planned row's path and size.
		return `{"ts":"2026-09-23T00:00:00Z","tool":"Read","bytes":0,"size_known":false,"packet":"sha256:p","planned":true}`
	}
	return fmt.Sprintf(`{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":%q,"bytes":10,"size_known":true,"packet":"sha256:p","planned":false}`, path)
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

// V1-0740: a row neither writer could have produced never becomes a label;
// the readers count it as rejected and ReadLabels reports the count.
func TestLTAV0009RowsOutsideTheWriterContractAreNotLabels(t *testing.T) {
	root := t.TempDir()
	writeLedger(t, root, "unplanned-reads.jsonl", []string{
		`{}`,
		`{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":"../../escape.go","bytes":10,"size_known":true,"packet":"sha256:p","planned":false}`,
		`{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":"negative.go","bytes":-5,"size_known":true,"packet":"sha256:p","planned":false}`,
		`{"kind":"forged","tool":"Read","path":"forged.go","bytes":10,"size_known":true,"packet":"sha256:p","planned":false}`,
		`{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":"","bytes":10,"size_known":true,"packet":"sha256:p","planned":false}`,
		unplannedRow("kept/read.go", false),
	})
	writeLedger(t, root, "self-observations.jsonl", []string{
		`{}`,
		`{"kind":"event","event":"file-change","missState":"OBSERVED","touchedPaths":["../../escape.go"],"rankedPaths":[]}`,
		`{"kind":"event","event":"file-change","missState":"OBSERVED","touchedPaths":["/abs/miss.go"],"rankedPaths":[]}`,
		`{"kind":"event","event":"file-change","missState":"OBSERVED","touchedPaths":["kept/miss.go"],"rankedPaths":[]}`,
	})
	labels, err := ReadLabels(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"kept/miss.go", "kept/read.go"}; !reflect.DeepEqual(labels.Paths, want) {
		t.Fatalf("labels = %v, want %v", labels.Paths, want)
	}
	if labels.RejectedRows != 8 || labels.LedgerCut {
		t.Fatalf("labels = %+v, want 8 rejected rows and no cut", labels)
	}
	if value := labelsValue(labels); value["rejected_rows"] != 8 || value["ledger_cut"] != false {
		t.Fatalf("labels value = %v", value)
	}
}

// V1-0740: a ledger over its byte cap is reported as cut alongside Truncated.
func TestLTAV0009LedgerCutIsReported(t *testing.T) {
	root := t.TempDir()
	rows := []string{}
	for number := 0; len(strings.Join(rows, "\n")) <= 128*1024; number++ {
		rows = append(rows, unplannedRow(fmt.Sprintf("pkg/p%04d.go", number%200), false))
	}
	writeLedger(t, root, "unplanned-reads.jsonl", rows)
	labels, err := ReadLabels(root)
	if err != nil {
		t.Fatal(err)
	}
	if !labels.LedgerCut || labels.Truncated || labels.RejectedRows != 0 {
		t.Fatalf("labels = %+v, want a cut ledger with no rejected rows", labels)
	}
	if labelsValue(labels)["ledger_cut"] != true {
		t.Fatal("labels value hides the cut ledger")
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

// The admit path writes through the pinned store, so a symlinked store is
// refused and nothing is written outside the repository.
func TestLTAV0011AdmitRefusesASymlinkedStoreAndWritesNothingOutside(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	store := filepath.Join(root, filepath.Dir(filepath.FromSlash(contextindex.SlotWeightsPath)))
	if err := os.Symlink(outside, store); err != nil {
		t.Fatal(err)
	}
	err := writeAdmitted(root, contextindex.SlotWeights{"test": 2}, map[string]any{
		"goldens_sha256": "sha256:" + strings.Repeat("a", 64), "revision": strings.Repeat("b", 40), "heldout_cases": 2,
		"baseline": map[string]any{"critical_misses": 0, "must_include_hits": 1, "top5_hits": 1},
		"arm":      map[string]any{"critical_misses": 0, "must_include_hits": 2, "top5_hits": 2},
	})
	if err == nil {
		t.Fatal("admit through a symlinked store succeeded")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("outside directory written: %v, %v", entries, err)
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
