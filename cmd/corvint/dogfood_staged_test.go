package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCW-V0-037: execute the documented staged route on one 279-hunk commit.
func TestDogfoodSingleCommitStagedCitationPlans(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	var before, after strings.Builder
	for i := 0; i < 279; i++ {
		fmt.Fprintf(&before, "// item %d old\n", i)
		fmt.Fprintf(&after, "// item %d new\n", i)
		for j := 0; j < 8; j++ {
			line := fmt.Sprintf("// unchanged %d %d\n", i, j)
			before.WriteString(line)
			after.WriteString(line)
		}
	}
	cemWrite(t, root, "fixture/many.go", "package fixture\n"+before.String())
	cemGit(t, root, "add", "fixture/many.go")
	cemGit(t, root, "commit", "-qm", "base for oversized patch")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/many.go", "package fixture\n"+after.String())
	cemGit(t, root, "commit", "-qam", "one oversized commit")
	inputsDir := t.TempDir()
	intents := filepath.Join(inputsDir, "intents")
	cemWrite(t, inputsDir, "intents", "intent.md\n")
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_INTENTS_FILE=" + intents}
	run.exec(t, root, inputs, "dogfood", "change", base)
	mapPath := filepath.Join(root, ".corvint/change.cem.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hunks []struct {
			ID          string `json:"id"`
			Disposition string `json:"disposition"`
		} `json:"hunks"`
	}
	if err = json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Hunks) != 279 {
		t.Fatalf("hunks=%d", len(doc.Hunks))
	}
	plans := []string{}
	for stage, start := range []int{0, 256} {
		end := start + 256
		if end > len(doc.Hunks) {
			end = len(doc.Hunks)
		}
		var plan strings.Builder
		// Two spans, so the stages together do not cite one span for every
		// hunk (DCW-V0-048, DCW-V0-883).
		for index, h := range doc.Hunks[start:end] {
			fmt.Fprintf(&plan, "%s\tintent.md\t%d:%d\tspecification\n", h.ID, 5+index%2, 5+index%2)
		}
		name := fmt.Sprintf("plan-%d.tsv", stage)
		cemWrite(t, inputsDir, name, plan.String())
		plans = append(plans, filepath.Join(inputsDir, name))
		code, _, stderr := run.exec(t, root, append(inputs[:len(inputs):len(inputs)], "DOGFOOD_CITATIONS="+plans[stage]), "dogfood", "change", base)
		if code != 1 || strings.Contains(stderr, "  cem-cite:") && !strings.Contains(stderr, "the plan was added") {
			t.Fatalf("stage %d exit=%d stderr=%s", stage, code, stderr)
		}
		data, err = os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		supported := 0
		for _, h := range doc.Hunks {
			if h.Disposition == "supported" {
				supported++
			}
		}
		if supported != end {
			t.Fatalf("stage=%d supported=%d want=%d", stage, supported, end)
		}
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "bind fully cited sidecar")
	code, _, stderr := run.exec(t, root, append(inputs, "DOGFOOD_CITATIONS="+plans[1]), "dogfood", "change", base)
	if code != 0 {
		t.Fatalf("final change exit=%d stderr=%s", code, stderr)
	}
	code, stdout, stderr := run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.Contains(stdout, "hunks=279 supported=279 unknown=0 mechanical=0") || !strings.Contains(stdout, "dogfood-check: PASS") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
