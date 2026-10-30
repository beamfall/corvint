package slotlearn

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// LTA-V0-018: the learning report names its three stages, pins the ledgers
// labels came from with their rejections by reason, carries the fixed
// non-learning arm, and records the cross-repository and cross-time splits as
// NOT_RUN rather than implying them.
func TestLTAV0018LearnReportRecordsSplitProvenanceAndFixedBaseline(t *testing.T) {
	root := t.TempDir()
	result, admitted, err := Learn(context.Background(), root, filepath.Join(root, "absent-goldens.json"), true)
	if err != nil || admitted {
		t.Fatalf("learn with no ledgers = %v, %v", admitted, err)
	}
	split := result["split"].(map[string]any)
	for _, key := range []string{"cross_repository", "cross_time"} {
		if value, _ := split[key].(string); !strings.HasPrefix(value, notRun) {
			t.Fatalf("%s = %v", key, split[key])
		}
	}
	if value, _ := split["final_evaluation"].(string); !strings.HasPrefix(value, notRun) {
		t.Fatalf("no proposal was scored but final_evaluation = %v", split["final_evaluation"])
	}
	ledgers := split["label_collection"].(map[string]any)["ledgers"].([]any)
	if len(ledgers) != 2 || ledgers[0].(map[string]any)["state"] != "absent" {
		t.Fatalf("absent ledgers = %v", ledgers)
	}
	baseline := result["fixed_baseline"].(map[string]any)
	if baseline["learns"] != false || !strings.Contains(baseline["arm"].(string), "nil slot weights") {
		t.Fatalf("fixed baseline = %v", baseline)
	}

	writeLedger(t, root, "unplanned-reads.jsonl", []string{unplannedRow("cache/demux_test.go", false), `{"ts":"2026-09-23T00:00:00Z","tool":"Read"`})
	labels, err := ReadLabels(root)
	if err != nil {
		t.Fatal(err)
	}
	value := labelsValue(labels)
	total := 0
	for _, count := range labels.RejectedReasons {
		total += count
	}
	if total != labels.RejectedRows || len(value["rejected_by_reason"].(map[string]any)) != len(labels.RejectedReasons) {
		t.Fatalf("rejections by reason %v do not sum to %d", labels.RejectedReasons, labels.RejectedRows)
	}
	for reason := range labels.RejectedReasons {
		if !strings.HasPrefix(reason, "unplanned-reads:") && !strings.HasPrefix(reason, "self-observations:") {
			t.Fatalf("rejection reason %q names no ledger", reason)
		}
	}
	provenance := ledgerProvenance(root)[0].(map[string]any)
	raw, _ := os.ReadFile(filepath.Join(root, ".corvint", "unplanned-reads.jsonl"))
	if provenance["state"] != "present" || provenance["bytes"] != int64(len(raw)) || !strings.HasPrefix(provenance["sha256"].(string), "sha256:") {
		t.Fatalf("ledger provenance = %v", provenance)
	}
	after, _ := os.ReadFile(filepath.Join(root, ".corvint", "unplanned-reads.jsonl"))
	if string(after) != string(raw) {
		t.Fatal("reading provenance changed the ledger")
	}
}

// LTA-V0-018: with no admitted trace, with live-read ledgers present but
// nothing admitted, and after Reset of an admitted trace, the context packet
// is byte-identical to the shipped default; only an admitted trace changes it.
func TestLTAV0018DefaultContextIsByteIdenticalWhenLearningIsAbsentOrReset(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":              "module example.test/fixture\n\ngo 1.27.0\n",
		"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"cache/notes.md":      "# cache\n\nThe cache demuxes keys.\n",
	}
	for relative, content := range files {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, arguments := range [][]string{
		{"init", "-q"}, {"config", "user.email", "corvint@example.test"}, {"config", "user.name", "Corvint Test"},
		{"add", "."}, {"commit", "-qm", "fixture"},
	} {
		command := exec.Command("git", append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0"}, arguments...)...)
		command.Dir = root
		command.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Skipf("git %v: %v %s", arguments, err, output)
		}
	}
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet := func() string {
		t.Helper()
		admitted, err := contextindex.LoadAdmittedSlotWeights(root)
		if err != nil {
			t.Fatal(err)
		}
		value, err := contextindex.TaskContextWeighted(context.Background(), index, "where does Split demux keys", "", 5, admitted)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return string(encoded)
	}
	plain, err := contextindex.TaskContext(context.Background(), index, "where does Split demux keys", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	shipped, _ := json.Marshal(plain)
	if absent := packet(); absent != string(shipped) {
		t.Fatalf("absent learning changed the packet:\n%s\n%s", absent, shipped)
	}
	writeLedger(t, root, "unplanned-reads.jsonl", []string{unplannedRow("cache/demux_test.go", false)})
	if withLedger := packet(); withLedger != string(shipped) {
		t.Fatal("a live-read ledger changed the packet without an admitted trace")
	}
	if err := writeAdmitted(root, contextindex.SlotWeights{"test": 2}, map[string]any{
		"goldens_sha256": "sha256:" + strings.Repeat("a", 64), "revision": strings.Repeat("b", 40), "heldout_cases": 2,
		"baseline": map[string]any{"critical_misses": 0, "must_include_hits": 1, "top5_hits": 1},
		"arm":      map[string]any{"critical_misses": 0, "must_include_hits": 2, "top5_hits": 2},
	}); err != nil {
		t.Fatal(err)
	}
	if learned := packet(); learned == string(shipped) {
		t.Fatal("an admitted trace left the packet unchanged; the identity checks above prove nothing")
	}
	if removed, err := Reset(root); !removed || err != nil {
		t.Fatalf("reset = %v, %v", removed, err)
	}
	if reset := packet(); reset != string(shipped) {
		t.Fatalf("reset did not restore the default packet:\n%s\n%s", reset, shipped)
	}
}
