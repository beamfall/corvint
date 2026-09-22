package necessity

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// necessityRepository commits one fixture tree and returns its root.
func necessityRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{{"init", "-q"}, {"add", "-A"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "fixture"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
	return root
}

// governedRepository has one governing instruction file, which the packet
// carries as its critical anchor, beside a redundant cochange file.
func governedRepository(t *testing.T) string {
	t.Helper()
	return necessityRepository(t, map[string]string{
		"go.mod":              "module example.test/ctx\n\ngo 1.27.0\n",
		"AGENTS.md":           "# Agent contract\n\nSplit keys carefully.\n",
		"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"cache/other.go":      "package cache\n\nfunc Unrelated() {}\n",
	})
}

func labelledRows(t *testing.T, packet map[string]any) map[string]map[string]any {
	t.Helper()
	block, ok := packet["necessity"].(map[string]any)
	if !ok {
		t.Fatalf("no necessity block: %v", packet)
	}
	rows := make(map[string]map[string]any)
	for _, item := range block["labels"].([]any) {
		row := item.(map[string]any)
		rows[row["path"].(string)] = row
	}
	return rows
}

// NEC-V0-004(a): the governing file is the packet's only critical anchor, so
// removing it degrades the packet and it is labelled load-bearing.
func TestRemovingAnAnchorPathIsLoadBearing(t *testing.T) {
	root := governedRepository(t)
	packet, err := Resolve(context.Background(), Options{Root: root, Task: "does `Split` keep empty keys", Subject: "cache/demux.go"})
	if err != nil {
		t.Fatal(err)
	}
	row, ok := labelledRows(t, packet)["AGENTS.md"]
	if !ok {
		t.Fatalf("AGENTS.md not included: %v", packet["results"])
	}
	if row["label"] != labelLoadBearing || row["change"] != "anchor lost: governing AGENTS.md" {
		t.Fatalf("AGENTS.md row = %v", row)
	}
}

// NEC-V0-004(b): a co-change row carries no anchor and no verdict, so the
// packet holds its shape without it and it is labelled supporting.
func TestRemovingARedundantPathIsSupporting(t *testing.T) {
	root := governedRepository(t)
	packet, err := Resolve(context.Background(), Options{Root: root, Task: "does `Split` keep empty keys", Subject: "cache/demux.go"})
	if err != nil {
		t.Fatal(err)
	}
	row, ok := labelledRows(t, packet)["cache/other.go"]
	if !ok {
		t.Fatalf("cache/other.go not included: %v", packet["results"])
	}
	if row["label"] != labelSupporting || row["reason"] != "counterfactual" {
		t.Fatalf("cache/other.go row = %v", row)
	}
}

// NEC-V0-005: the recompile budget is the limit capped at MaxReruns; every
// included path past it is unlabelled with the budget reason, never guessed.
func TestBudgetLeavesRemainingPathsUnlabelled(t *testing.T) {
	files := map[string]string{
		"go.mod":         "module example.test/ctx\n\ngo 1.27.0\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
	}
	for index := range 16 {
		name := string(rune('a' + index))
		files["cache/f"+name+".go"] = "package cache\n\nfunc Split" + string(rune('A'+index)) + "() {}\n"
	}
	root := necessityRepository(t, files)
	packet, err := Resolve(context.Background(), Options{Root: root, Task: "does `Split` keep empty keys", Subject: "cache/demux.go", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	block := packet["necessity"].(map[string]any)
	if block["reruns"] != MaxReruns || block["budget"] != MaxReruns {
		t.Fatalf("budget = %v reruns = %v", block["budget"], block["reruns"])
	}
	labelled, unlabelled := 0, 0
	for _, item := range block["labels"].([]any) {
		row := item.(map[string]any)
		if row["label"] == labelUnlabelled {
			if row["reason"] != "budget" {
				t.Fatalf("unlabelled row without the budget reason: %v", row)
			}
			unlabelled++
			continue
		}
		labelled++
	}
	if labelled != MaxReruns || unlabelled == 0 {
		t.Fatalf("labelled = %d unlabelled = %d", labelled, unlabelled)
	}
}

// NEC-V0-006: a packet with no answer is emitted unchanged and carries no
// label, rather than labelling an abstention.
func TestAbstainingPacketCarriesNoLabels(t *testing.T) {
	root := necessityRepository(t, map[string]string{
		"go.mod":         "module example.test/ctx\n\ngo 1.27.0\n",
		"cache/demux.go": "package cache\n\nfunc Split(key string) string { return key }\n",
	})
	packet, err := Resolve(context.Background(), Options{Root: root, Task: "reconcile zzqqx with wwvvy in the ppmm subsystem"})
	if err != nil {
		t.Fatal(err)
	}
	block := packet["necessity"].(map[string]any)
	if packet["state"] != "NO_CANDIDATES" || block["abstained"] != true || block["reruns"] != 0 {
		t.Fatalf("abstention = %v state = %v", block, packet["state"])
	}
	if rows := block["labels"].([]any); len(rows) != 0 {
		t.Fatalf("labels on an abstention: %v", rows)
	}
}

// NEC-V0-003: the counterfactual index is a copy; every read-side table the
// original carries is byte-identical after the removal.
func TestWithoutLeavesTheOriginalIndexUnchanged(t *testing.T) {
	root := governedRepository(t)
	index, err := contextindex.BuildContext(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	sources := slices.Sorted(maps.Keys(index.Sources))
	tracked := slices.Sorted(maps.Keys(index.Tracked))
	symbols := slices.Clone(index.Symbols)
	reduced := without(index, "cache/other.go")
	if !slices.Equal(slices.Sorted(maps.Keys(index.Sources)), sources) || !slices.Equal(slices.Sorted(maps.Keys(index.Tracked)), tracked) {
		t.Fatal("without mutated the original index tables")
	}
	if !slices.Equal(index.Symbols, symbols) {
		t.Fatal("without mutated the original symbol table")
	}
	if _, present := reduced.Sources["cache/other.go"]; present {
		t.Fatal("reduced index still carries the removed source")
	}
	if _, present := reduced.Tracked["cache/other.go"]; present {
		t.Fatal("reduced index still tracks the removed path")
	}
	if reduced.Vocabulary != nil {
		t.Fatal("reduced index carried the original term table")
	}
}

// NEC-V0-003: the document, feature, scenario and marker tables are keyed by
// record id, not by path, so the counterfactual must compare each entry's own
// path. A record left behind would keep answering for the removed file.
func TestWithoutDropsRecordTablesByPath(t *testing.T) {
	removed, kept := "cache/other.go", "cache/demux.go"
	records := func() map[string]contextindex.Record {
		return map[string]contextindex.Record{
			"REC-1": {ID: "REC-1", Path: removed, Line: 3},
			"REC-2": {ID: "REC-2", Path: kept, Line: 4},
		}
	}
	index := &contextindex.Index{
		Sources:   map[string]contextindex.Source{removed: {Path: removed}, kept: {Path: kept}},
		Tracked:   map[string]struct{}{removed: {}, kept: {}},
		Documents: records(), Features: records(), Scenarios: records(),
		Markers: map[string][]contextindex.Marker{
			"REC-1": {{Path: removed, Line: 3}, {Path: kept, Line: 9}},
			"REC-2": {{Path: removed, Line: 5}},
		},
	}
	reduced := without(index, removed)

	for name, table := range map[string]map[string]contextindex.Record{
		"documents": reduced.Documents, "features": reduced.Features, "scenarios": reduced.Scenarios,
	} {
		if len(table) != 1 || table["REC-2"].Path != kept {
			t.Fatalf("reduced %s = %v", name, table)
		}
	}
	if len(reduced.Markers) != 1 {
		t.Fatalf("reduced markers = %v", reduced.Markers)
	}
	if bucket := reduced.Markers["REC-1"]; len(bucket) != 1 || bucket[0].Path != kept {
		t.Fatalf("reduced marker bucket = %v", bucket)
	}
	if len(index.Documents) != 2 || len(index.Features) != 2 || len(index.Scenarios) != 2 {
		t.Fatal("without mutated an original record table")
	}
	if len(index.Markers["REC-1"]) != 2 || len(index.Markers["REC-2"]) != 1 {
		t.Fatalf("without mutated the original marker table: %v", index.Markers)
	}
}

// NEC-V0-007: every labelled answer carries the fixed disclosure, so no reader
// takes a label for proof of relevance.
func TestLabelledAnswerCarriesTheDisclosure(t *testing.T) {
	root := governedRepository(t)
	packet, err := Resolve(context.Background(), Options{Root: root, Task: "does `Split` keep empty keys", Subject: "cache/demux.go"})
	if err != nil {
		t.Fatal(err)
	}
	if packet["necessity"].(map[string]any)["disclosure"] != Disclosure {
		t.Fatalf("disclosure = %v", packet["necessity"])
	}
}

// NEC-V0-004: a cancellation that arrives after the packet itself already
// compiled, while a later counterfactual recompile is reading co-change
// history, is exactly the "a counterfactual recompile refuses" failure mode:
// that path (and every path still to run) lands unlabelled/"error" and the
// sweep keeps going rather than corrupting or losing the other labels. This
// confirms bug-hunt item (2): the sweep does exit 0 with error labels on a
// mid-sweep cancellation, and that is the documented, deliberate resilience
// behavior of NEC-V0-004, not a defect — a transient per-path failure (of
// which cancellation is one cause among others) must never sink the whole
// read-only answer.
func TestCancellationDuringTheSweepDegradesGracefully(t *testing.T) {
	root := governedRepository(t)
	options := Options{Root: root, Task: "does `Split` keep empty keys", Subject: "cache/demux.go", Limit: DefaultLimit}
	index, err := loadIndex(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := contextindex.TaskContext(context.Background(), index, options.Task, options.Subject, DefaultLimit)
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	block := label(cancelled, index, packet, options)
	if block["abstained"] != false {
		t.Fatalf("unexpected abstention: %v", block)
	}
	labels, _ := block["labels"].([]any)
	if len(labels) == 0 {
		t.Fatal("expected the included paths to still be reported")
	}
	for _, item := range labels {
		row := item.(map[string]any)
		if row["label"] != labelUnlabelled || row["reason"] != "error" {
			t.Fatalf("row %v: expected every recompile to refuse once cancelled", row)
		}
	}
}
