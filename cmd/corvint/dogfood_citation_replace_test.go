package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// DCW-V0-390..392 (V1-3168): after two applied staged plans, a corrected
// stage-one plan that changes only hunk 1 is refused with the
// DOGFOOD_CITATION_REPLACE entry to rerun with; that rerun replaces hunk 1's
// citations only, with no map deletion or stage restart, so hunks 2 and 3 keep
// theirs byte for byte; a stale digest leaves the map intact; and the bound,
// committed map replays the same inputs to a complete pass and dogfood check
// PASS.
func TestDogfoodCitationReplaceCorrectsOneHunkOfAnAppliedStage(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, _ := portableDogfoodRepo(t)
	var before, after strings.Builder
	for i := 0; i < 3; i++ {
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
	cemGit(t, root, "commit", "-qm", "base for three hunks")
	base := cemGit(t, root, "rev-parse", "HEAD")
	cemWrite(t, root, "fixture/many.go", "package fixture\n"+after.String())
	cemGit(t, root, "commit", "-qam", "three hunks")
	inputsDir := t.TempDir()
	intents := filepath.Join(inputsDir, "intents")
	cemWrite(t, inputsDir, "intents", "intent.md\n")
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=go test ./fixture", "DOGFOOD_OUTCOME=passed", "DOGFOOD_INTENTS_FILE=" + intents}
	run.exec(t, root, inputs, "dogfood", "change", base)
	mapPath := filepath.Join(root, ".corvint/change.cem.json")
	type hunk struct {
		ID    string          `json:"id"`
		Basis json.RawMessage `json:"basis"`
	}
	read := func() ([]byte, []hunk) {
		t.Helper()
		data, err := os.ReadFile(mapPath)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct{ Hunks []hunk }
		if err = json.Unmarshal(data, &doc); err != nil || len(doc.Hunks) != 3 {
			t.Fatalf("map hunks=%d err=%v", len(doc.Hunks), err)
		}
		return data, doc.Hunks
	}
	_, hunks := read()
	pass := func(name, plan string, extra ...string) (int, string) {
		t.Helper()
		cemWrite(t, inputsDir, name, plan)
		env := append(append(inputs[:len(inputs):len(inputs)], "DOGFOOD_CITATIONS="+filepath.Join(inputsDir, name)), extra...)
		code, _, stderr := run.exec(t, root, env, "dogfood", "change", base)
		return code, stderr
	}
	// Stage one cites hunks 1 and 2 and keeps hunk 3 unknown; stage two
	// names only hunk 3, which is still uncited.
	stages := []string{
		fmt.Sprintf("%s\tintent.md\t5:5\tspecification\n%s\tintent.md\t6:6\tspecification\n%s\tunknown\tinsufficient-evidence\tcited later\n", hunks[0].ID, hunks[1].ID, hunks[2].ID),
		fmt.Sprintf("%s\tintent.md\t1:1\tspecification\n", hunks[2].ID),
	}
	for index, plan := range stages {
		if code, stderr := pass(fmt.Sprintf("stage-%d.tsv", index+1), plan); code != 1 || strings.Contains(stderr, "  cem-cite: citation") {
			t.Fatalf("stage %d exit=%d stderr=%s", index+1, code, stderr)
		}
	}
	applied, stagedHunks := read()

	corrected := fmt.Sprintf("%s\tintent.md\t3:3\tspecification\n%s\tintent.md\t6:6\tspecification\n", hunks[0].ID, hunks[1].ID)
	code, stderr := pass("corrected.tsv", corrected)
	entry := regexp.MustCompile(`DOGFOOD_CITATION_REPLACE=(\S+)`).FindStringSubmatch(stderr)
	if code != 1 || !strings.Contains(stderr, "  cem-cite: citation-plan-recites-cited-hunk\n") || entry == nil || !strings.HasPrefix(entry[1], hunks[0].ID+"=") {
		t.Fatalf("corrected plan exit=%d stderr=%s", code, stderr)
	}
	if now, _ := read(); !bytes.Equal(now, applied) {
		t.Fatal("the refused corrected plan changed the map")
	}

	code, stderr = pass("corrected.tsv", corrected, "DOGFOOD_CITATION_REPLACE="+entry[1])
	if code != 1 || !strings.Contains(stderr, "NOTE cem-cite replaced hunk="+hunks[0].ID) || strings.Contains(stderr, "  cem-cite: ") {
		t.Fatalf("guarded replacement exit=%d stderr=%s", code, stderr)
	}
	replaced, replacedHunks := read()
	if bytes.Equal(replacedHunks[0].Basis, stagedHunks[0].Basis) || strings.Count(string(replacedHunks[0].Basis), "evidenceId") != 1 {
		t.Fatalf("hunk 1 basis was not replaced: %s", replacedHunks[0].Basis)
	}
	for i := 1; i < 3; i++ {
		if !bytes.Equal(replacedHunks[i].Basis, stagedHunks[i].Basis) {
			t.Fatalf("hunk %d basis changed: %s -> %s", i+1, stagedHunks[i].Basis, replacedHunks[i].Basis)
		}
	}

	// The digest read before the replacement is stale for a further
	// correction, which cites nothing.
	code, stderr = pass("again.tsv", strings.Replace(corrected, "3:3", "4:4", 1), "DOGFOOD_CITATION_REPLACE="+entry[1])
	if code != 1 || !strings.Contains(stderr, "  cem-cite: citation-replace-digest-mismatch\n") {
		t.Fatalf("stale digest exit=%d stderr=%s", code, stderr)
	}
	if now, _ := read(); !bytes.Equal(now, replaced) {
		t.Fatal("a stale-digest replacement changed the map")
	}

	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	code, stderr = pass("corrected.tsv", corrected, "DOGFOOD_CITATION_REPLACE="+entry[1])
	if code != 0 || !strings.Contains(stderr, "NOTE cem-cite replace-already-applied hunk="+hunks[0].ID) {
		t.Fatalf("replayed bind exit=%d stderr=%s", code, stderr)
	}
	code, stdout, stderr := run.exec(t, root, nil, "dogfood", "check", base)
	if code != 0 || !strings.Contains(stdout, "hunks=3 supported=3 unknown=0") || !strings.Contains(stdout, "dogfood-check: PASS") {
		t.Fatalf("check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}
