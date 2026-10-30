package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// sealPortableLane runs the daily loop for HEAD against base with one
// citation plan, as docs/DOGFOOD.md §4 prescribes, and returns the bind commit.
func sealPortableLane(t *testing.T, run portableRun, root, base, plan string) string {
	t.Helper()
	inputsDir := t.TempDir()
	citations := filepath.Join(inputsDir, "citations.tsv")
	intents := filepath.Join(inputsDir, "intents")
	for path, content := range map[string]string{citations: plan, intents: "intent.md\n"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inputs := []string{"DOGFOOD_TASK=Answer two.", "DOGFOOD_VERIFY=true", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 1 || !strings.Contains(stderr, "local-outcome: record-index-failed") {
		t.Fatalf("first pass exit=%d stderr=%s", code, stderr)
	}
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	bind := cemGit(t, root, "rev-parse", "HEAD")
	if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code != 0 {
		t.Fatalf("bind exit=%d stderr=%s", code, stderr)
	}
	if code, _, stderr := run.exec(t, root, nil, "dogfood", "seal", base); code != 0 {
		t.Fatalf("seal exit=%d stderr=%s", code, stderr)
	}
	return bind
}

// DCW-V0-041: a citation plan may cite code at BASE, not only spec spans,
// and may cite one hunk more than once; an unregistered relation and a path
// absent at BASE each name their fix. DCW-V0-039: two such sealed changes
// merged into one batch pass batch-check with the real verifier, and a commit
// no sealed change binds fails it.
func TestDogfoodBatchCheckDCWV0039SealedLanesWithCodeCitations(t *testing.T) {
	t.Parallel()
	run := portableDogfoodRunner(t)
	root, base := portableDogfoodRepo(t)
	bindA := sealPortableLane(t, run, root, base, "1\tfixture/fixture.go\t1:1\timplementation\n1\tintent.md\t5:5\tspecification\n")
	sealA := cemGit(t, root, "rev-parse", "HEAD")
	var cem struct {
		Hunks []struct {
			Basis []struct {
				Relation string `json:"relation"`
			} `json:"basis"`
		} `json:"hunks"`
	}
	raw := cemGit(t, root, "show", "HEAD:.corvint/changes/"+bindA+".cem.json")
	if err := json.Unmarshal([]byte(raw), &cem); err != nil || len(cem.Hunks) != 1 || len(cem.Hunks[0].Basis) != 2 ||
		cem.Hunks[0].Basis[0].Relation != "implementation" || cem.Hunks[0].Basis[1].Relation != "specification" {
		t.Fatalf("code citation basis %s err=%v", raw, err)
	}

	// Lane B runs as in its own clone: lane A's local trace store names
	// commits lane B's HEAD cannot reach.
	if err := os.RemoveAll(filepath.Join(root, ".context-corvint")); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "checkout", "-q", "-b", "lane-b", base)
	cemWrite(t, root, "fixture/extra.go", "package fixture\nfunc Extra() int { return 3 }\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "extra")
	inputsDir := t.TempDir()
	intents := filepath.Join(inputsDir, "intents")
	if err := os.WriteFile(intents, []byte("intent.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for plan, fix := range map[string]string{
		"1\tfixture/fixture.go\t1:1\timplements\n": "  cem-cite: citation-plan-relation-unknown\n    fix: RELATION, the fourth field of a cite row, is one of call-site, decision, dependency, implementation, incident, specification, test-claim",
		"1\tfixture/extra.go\t1:1\tdependency\n":   "  cem-cite: missing-evidence\n    fix: plan row 1 cites a path that is not a regular file at BASE",
	} {
		citations := filepath.Join(inputsDir, "bad.tsv")
		if err := os.WriteFile(citations, []byte(plan), 0o644); err != nil {
			t.Fatal(err)
		}
		inputs := []string{"DOGFOOD_TASK=Extra.", "DOGFOOD_VERIFY=true", "DOGFOOD_OUTCOME=passed", "DOGFOOD_CITATIONS=" + citations, "DOGFOOD_INTENTS_FILE=" + intents}
		if code, _, stderr := run.exec(t, root, inputs, "dogfood", "change", base); code == 0 || !strings.Contains(stderr, fix) {
			t.Fatalf("plan %q exit=%d stderr=%s", plan, code, stderr)
		}
		if err := os.Remove(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
			t.Fatal(err)
		}
	}
	bindB := sealPortableLane(t, run, root, base, "1\tfixture/fixture.go\t1:1\tdependency\n")
	sealB := cemGit(t, root, "rev-parse", "HEAD")

	cemGit(t, root, "checkout", "-q", "-b", "batch", base)
	cemGit(t, root, "merge", "-q", "--no-ff", "--no-edit", sealA)
	cemGit(t, root, "merge", "-q", "--no-ff", "--no-edit", sealB)
	code, stdout, stderr := run.exec(t, root, nil, "dogfood", "batch-check", base)
	if code != 0 || stderr != "" || !strings.Contains(stdout, "CONSTITUENT bind="+bindA+" base="+base+" hunks=1 supported=1 unknown=0 bootstrapUnknown=0 reasonedUnknown=0 state=accepted\n") ||
		!strings.Contains(stdout, "CONSTITUENT bind="+bindB) || !strings.HasSuffix(stdout, "dogfood-batch-check: PASS constituents=2 commits=8\n") {
		t.Fatalf("batch-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	merged := cemGit(t, root, "rev-parse", "HEAD")

	// DCW-V0-111 (V1-1927): in an alternates-backed clone the real verifier
	// refuses every constituent, and batch-check repeats its reason code and
	// names the clone-layout fix.
	alternates := filepath.Join(root, ".git/objects/info/alternates")
	if err := os.WriteFile(alternates, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = run.exec(t, root, nil, "dogfood", "batch-check", base)
	if code != 1 || !strings.Contains(stdout, "CONSTITUENT bind="+bindA) || !strings.Contains(stderr, "  bind="+bindA+" state=unsupported code=unsupported-object-alternates\n") ||
		!strings.Contains(stderr, "  fix: unsupported-object-alternates: the clone borrows objects through .git/objects/info/alternates") {
		t.Fatalf("alternates batch-check exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if err := os.Remove(alternates); err != nil {
		t.Fatal(err)
	}
	cemWrite(t, root, "AGENTS.md", "# Fixture authority\nUse intent.md.\n")
	cemGit(t, root, "commit", "-qam", "unbound fix-up")
	fixup := cemGit(t, root, "rev-parse", "HEAD")
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "batch-check", base); code != 1 || !strings.Contains(stderr, "FAIL unbound-commits count=1\n  "+fixup+"\n") {
		t.Fatalf("unbound exit=%d stderr=%s", code, stderr)
	}

	// A sealed map reused on a bind commit carrying an extra committed edit
	// fails: canonical verification derives the patch from the bind commit
	// itself, so the omitted edit cannot pass as bound.
	cemGit(t, root, "checkout", "-q", "-b", "forged", bindA+"^")
	cemWrite(t, root, "fixture/omitted.go", "package fixture\nfunc Omitted() int { return 4 }\n")
	cemGit(t, root, "add", ".")
	cemGit(t, root, "commit", "-qm", "omitted edit")
	cemWrite(t, root, ".corvint/change.cem.json", raw+"\n")
	cemGit(t, root, "add", ".corvint/change.cem.json")
	cemGit(t, root, "commit", "-qm", "chore: bind change evidence")
	forged := cemGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, ".corvint/changes"), 0o755); err != nil {
		t.Fatal(err)
	}
	cemGit(t, root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+forged+".cem.json")
	cemGit(t, root, "commit", "-qm", "chore: seal change evidence")
	sealForged := cemGit(t, root, "rev-parse", "HEAD")
	cemGit(t, root, "checkout", "-q", "-b", "forged-batch", base)
	cemGit(t, root, "merge", "-q", "--no-ff", "--no-edit", sealForged)
	if code, _, stderr = run.exec(t, root, nil, "dogfood", "batch-check", base); code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-policy count=1\n  bind="+forged+" state=") {
		t.Fatalf("forged constituent exit=%d stderr=%s", code, stderr)
	}

	// DCW-V0-042, DCW-V0-040: a sealed CEM that differs from its bind
	// commit's map refuses sealed-cem-in-change and still names batch-check;
	// nothing is written.
	cemGit(t, root, "checkout", "-q", "-b", "drifted", merged)
	sealedA := ".corvint/changes/" + bindA + ".cem.json"
	cemWrite(t, root, sealedA, raw+"\n\n")
	cemGit(t, root, "commit", "-qam", "drift lane A's sealed CEM")
	if code, _, stderr := run.exec(t, root, nil, "dogfood", "change", base); code != 2 ||
		!strings.Contains(stderr, "REFUSE sealed-cem-in-change\n  "+sealedA+": sealed-cem-drift bind="+bindA+"\n") ||
		!strings.Contains(stderr, "  fix: for a merge batch of sealed changes, run corvint dogfood batch-check "+base+" instead of rebinding it") {
		t.Fatalf("drifted rebind exit=%d stderr=%s", code, stderr)
	}
	if status := cemGit(t, root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("worktree after refused rebind: %s", status)
	}

	// DCW-V0-042: the merged batch itself binds, checks and seals against
	// the original BASE: each merged sealed CEM is re-verified and admitted,
	// and its file is one more hunk the plan cites.
	cemGit(t, root, "checkout", "-q", "-b", "batch-bind", merged)
	first, second := bindA, bindB
	if second < first {
		first, second = second, first
	}
	spec := "\tintent.md\t5:5\tspecification\n"
	inputsDir = t.TempDir()
	intents = filepath.Join(inputsDir, "intents")
	if err := os.WriteFile(intents, []byte("intent.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	inputs := []string{"DOGFOOD_TASK=Batch.", "DOGFOOD_VERIFY=true", "DOGFOOD_OUTCOME=passed", "DOGFOOD_INTENTS_FILE=" + intents}
	code, _, stderr = run.exec(t, root, inputs, "dogfood", "change", base)
	for _, bind := range []string{first, second} {
		if !strings.Contains(stderr, "dogfood-change: NOTE sealed-cem-merged bind="+bind+" base="+base+" bootstrapUnknown=0\n") {
			t.Fatalf("merged batch pass exit=%d stderr=%s", code, stderr)
		}
	}
	if err := os.Remove(filepath.Join(root, ".corvint/change.cem.json")); err != nil {
		t.Fatal(err)
	}
	plan := "1" + spec + "2" + spec + "3\tfixture/fixture.go\t1:1\tdependency\n4\tfixture/fixture.go\t1:1\tdependency\n"
	bindBatch := sealPortableLane(t, run, root, base, plan)
	want := []string{".corvint/changes/" + first + ".cem.json", ".corvint/changes/" + second + ".cem.json", ".corvint/changes/" + bindBatch + ".cem.json"}
	sort.Strings(want)
	if sealed := cemGit(t, root, "ls-tree", "--name-only", "HEAD", ".corvint/changes/"); sealed != strings.Join(want, "\n") {
		t.Fatalf("sealed CEMs after binding the batch: %s", sealed)
	}
}
