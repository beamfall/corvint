package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
	"github.com/Beamfall/corvint/internal/evalrepo"
)

func slotWeightsRun(t *testing.T, root string, arguments ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := runContext(context.Background(), append([]string{"--root", root}, arguments...), strings.NewReader(""), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeSlotLedgers(t *testing.T, root string) {
	t.Helper()
	unplanned := `{"ts":"2026-09-23T00:00:00Z","tool":"Read","path":"cache/demux_test.go","bytes":10,"size_known":true,"packet":"sha256:p","planned":false}` + "\n"
	observed := `{"kind":"event","event":"file-change","miss_state":"OBSERVED","touched_paths":["cache/demux_test.go"],"ranked_paths":[]}` + "\n"
	for name, content := range map[string]string{"unplanned-reads.jsonl": unplanned, "self-observations.jsonl": observed} {
		if err := os.MkdirAll(filepath.Join(root, ".corvint"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".corvint", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// ignoreSlotLedgers commits the repository's own ignore rules for the ledgers
// and the learned trace store (/.gitignore), as a real checkout has them.
func ignoreSlotLedgers(t *testing.T, root string) {
	t.Helper()
	rules := ".context-corvint/\n.corvint/self-observations.jsonl\n.corvint/unplanned-reads.jsonl\n"
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(rules), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"add", ".gitignore"}, {"-c", "user.name=t", "-c", "user.email=t@x", "commit", "-qm", "ignore"}} {
		command := exec.Command("git", arguments...)
		command.Dir = root
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", arguments, err, output)
		}
	}
}

func TestLTAV0009LedgersNeverChangeContextOrQueryOutput(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	ignoreSlotLedgers(t, root)
	commands := [][]string{{"context", "--task", "where is Split", "--subject", "cache/demux.go"}, {"context", "--task", "where is Split"}, {"query", "--task", "where is Split"}}
	baseline := []string{}
	for _, command := range commands {
		code, stdout, stderr := slotWeightsRun(t, root, command...)
		if code != 0 {
			t.Fatalf("%v: exit %d: %s", command, code, stderr)
		}
		baseline = append(baseline, stdout)
	}
	writeSlotLedgers(t, root)
	for position, command := range commands {
		if _, stdout, _ := slotWeightsRun(t, root, command...); stdout != baseline[position] {
			t.Fatalf("%v output changed with populated ledgers:\n%s\n%s", command, baseline[position], stdout)
		}
	}
}

func heldoutSlotGolden(t *testing.T, root string) string {
	t.Helper()
	cases := []any{}
	for number := 0; len(cases) < 3; number++ {
		id := fmt.Sprintf("slot-case-%d", number)
		if evalrepo.CaseSplit(id) != evalrepo.SplitHeldout {
			continue
		}
		cases = append(cases, map[string]any{"id": id, "mode": "query", "text": "where is Split", "limit": 5,
			"expected": map[string]any{"must_include": []string{"symbol:cache/demux.go:Split"}, "critical": []string{}, "relevant": []string{}}})
	}
	raw, err := json.Marshal(map[string]any{"schemaVersion": 1, "cases": cases})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "goldens.json")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLTAV0010GateReportsTheDeltaAndRefusesWithoutImprovement(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	ignoreSlotLedgers(t, root)
	writeSlotLedgers(t, root)
	code, stdout, stderr := slotWeightsRun(t, root, "eval", "--learn-slot-weights", "--goldens", heldoutSlotGolden(t, root), "--admit")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var payload struct {
		Mutates     bool `json:"mutates"`
		SlotWeights struct {
			Labels     map[string]any `json:"labels"`
			Evaluation struct {
				HeldoutCases int `json:"heldout_cases"`
				Proposals    []struct {
					Delta map[string]any `json:"delta"`
				} `json:"proposals"`
			} `json:"evaluation"`
			Decision map[string]any `json:"decision"`
		} `json:"slot_weights"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatal(err)
	}
	decision := payload.SlotWeights.Decision
	if payload.Mutates || decision["admitted"] != false || decision["reason"] != "no held-out improvement" {
		t.Fatalf("payload = %s", stdout)
	}
	if payload.SlotWeights.Evaluation.HeldoutCases != 3 || len(payload.SlotWeights.Evaluation.Proposals) == 0 || payload.SlotWeights.Evaluation.Proposals[0].Delta["classification"] != "not distinguished" {
		t.Fatalf("evaluation = %s", stdout)
	}
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(contextindex.SlotWeightsPath))); !os.IsNotExist(err) {
		t.Fatalf("refused trace was written: %v", err)
	}
}

func TestLTAV0012ResetRestoresTheDefaultPacket(t *testing.T) {
	t.Parallel()
	root := taskContextRepository(t)
	ignoreSlotLedgers(t, root)
	command := []string{"context", "--task", "where is Split"}
	_, baseline, _ := slotWeightsRun(t, root, command...)
	path := filepath.Join(root, filepath.FromSlash(contextindex.SlotWeightsPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	evaluation := `{"goldens_sha256":"sha256:` + strings.Repeat("a", 64) + `","revision":"` + strings.Repeat("b", 40) +
		`","heldout_cases":2,"baseline":{"critical_misses":0,"must_include_hits":1,"top5_hits":1},` +
		`"arm":{"critical_misses":0,"must_include_hits":2,"top5_hits":2}}`
	if err := os.WriteFile(path, []byte(`{"schemaVersion":1,"weights":{"test":2},"evaluation":`+evaluation+`}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, weighted, _ := slotWeightsRun(t, root, command...); weighted == baseline || !strings.Contains(weighted, "learned_slot_weights") {
		t.Fatalf("admitted trace not applied:\n%s", weighted)
	}
	// A hand-written file without the gate's evaluation block is refused, not
	// applied (LTA-V0-011); so is an out-of-range weight.
	for _, content := range []string{
		`{"schemaVersion":1,"weights":{"test":2},"evaluation":"anything"}`,
		`{"schemaVersion":1,"weights":{"test":2}}`,
		`{"schemaVersion":1,"weights":{"test":9},"evaluation":` + evaluation + `}`,
	} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, stdout, stderr := slotWeightsRun(t, root, command...); code != 2 || stdout != "" || !strings.Contains(stderr, "corvint eval --reset-slot-weights") {
			t.Fatalf("malformed trace %s: exit %d: %s%s", content, code, stdout, stderr)
		}
	}
	code, stdout, stderr := slotWeightsRun(t, root, "eval", "--reset-slot-weights")
	if code != 0 || !strings.Contains(stdout, `"removed":true`) {
		t.Fatalf("reset: exit %d: %s%s", code, stdout, stderr)
	}
	if _, restored, _ := slotWeightsRun(t, root, command...); restored != baseline {
		t.Fatalf("reset did not restore the default packet:\n%s\n%s", baseline, restored)
	}
}
