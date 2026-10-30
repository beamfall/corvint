package testsource

import (
	"context"
	"strings"
	"testing"
)

// TestDuplicatesCallerDecisions proves caller-supplied decision files resolve pairs after the
// committed file, need a reviewer, bind to the test file blob when one is given so an edit reopens
// the pair, and that every resolved pair and ignored decision names its source
// (TSA-V0-660, TSA-V0-661, TSA-V0-662).
func TestDuplicatesCallerDecisions(t *testing.T) {
	root := t.TempDir()
	third := strings.ReplaceAll(cartA, "'cart'", "'cart copy'")
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": cartA, "e2e/b.spec.ts": cartB, "e2e/c.spec.ts": third})
	repo := OpenRepo(context.Background(), root)
	blobA := gitTest(t, root, "rev-parse", "HEAD:e2e/a.spec.ts")
	options := duplicateOptions()
	options.MaxLineTests = 3
	const (
		a = "e2e/a.spec.ts :: cart > adds an item"
		b = "e2e/b.spec.ts :: basket gains an item"
		c = "e2e/c.spec.ts :: cart copy > adds an item"
	)
	caller := `{"schema":"corvint-test-duplicate-decisions/0","decisions":[
 {"decision":"distinct-from","test":"` + a + `","other":"` + b + `","reason":"guest vs signed-in","reviewer":"ada","at":"2026-10-27","testBlob":"` + blobA[:12] + `"},
 {"decision":"distinct-from","test":"` + c + `","other":"` + b + `","reason":"copy flow"},
 {"decision":"covered-by","test":"` + a + `","other":"` + c + `","obligation":"CART-1","reviewer":"bob","at":"yesterday"}
]}`
	options.CallerDecisions = []DecisionFile{{Name: "review.json", Data: []byte(caller)}}
	report, err := Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Resolved) != 1 || report.Resolved[0].DecisionSource != "caller:review.json" || report.Resolved[0].Reviewer != "ada" ||
		report.Resolved[0].At != "2026-10-27" || len(report.Unresolved) != 2 {
		t.Fatalf("caller decision: resolved=%+v unresolved=%+v", report.Resolved, report.Unresolved)
	}
	problems := []string{}
	for _, ignored := range report.IgnoredDecisions {
		if ignored.Source != "caller:review.json" {
			t.Fatalf("ignored source = %+v", ignored)
		}
		problems = append(problems, ignored.Problem)
	}
	if strings.Join(problems, ",") != "missing-reviewer,invalid-at" {
		t.Fatalf("ignored problems = %v", problems)
	}

	// A committed decision for the same pair outranks the caller's, which is ignored as superseded.
	commitFiles(t, root, map[string]string{DecisionsPath: `{"schema":"corvint-test-duplicate-decisions/0","decisions":[
 {"decision":"covered-by","test":"` + b + `","other":"` + a + `","obligation":"CART-2"}]}`})
	blobA = gitTest(t, root, "rev-parse", "HEAD:e2e/a.spec.ts")
	report, err = Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Resolved) != 1 || report.Resolved[0].DecisionSource != RepositoryDecisionSource || report.Resolved[0].Obligation != "CART-2" ||
		report.IgnoredDecisions[0].Problem != "superseded" {
		t.Fatalf("committed precedence: resolved=%+v ignored=%+v", report.Resolved, report.IgnoredDecisions)
	}

	// Editing the test file changes its blob: the caller's blob-bound decision reopens the pair.
	commitFiles(t, root, map[string]string{DecisionsPath: "", "e2e/a.spec.ts": cartA + "// edited\n"})
	report, err = Duplicates(repo, options)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Resolved) != 0 || len(report.Unresolved) != 3 || report.IgnoredDecisions[0].Problem != "blob-changed" {
		t.Fatalf("edited test: resolved=%+v ignored=%+v", report.Resolved, report.IgnoredDecisions)
	}
	if blobA == gitTest(t, root, "rev-parse", "HEAD:e2e/a.spec.ts") {
		t.Fatal("fixture edit did not change the blob")
	}

	options.CallerDecisions = []DecisionFile{{Name: "bad.json", Data: []byte(`{"schema":"other","decisions":[]}`)}}
	if _, err := Duplicates(repo, options); !hasCode(err, "tests-decisions-invalid") || !strings.Contains(err.Error(), "bad.json") {
		t.Fatalf("malformed caller file error = %v", err)
	}
	options.CallerDecisions = make([]DecisionFile, MaxDecisionFiles+1)
	if _, err := Duplicates(repo, options); !hasCode(err, "tests-bound-exceeded") {
		t.Fatalf("too many files error = %v", err)
	}
}

// TestDuplicatesUnpinnedWarningTSAV0853 proves a valid caller decision without both blob pins is
// warned about as unpinned, while a fully pinned one is not.
func TestDuplicatesUnpinnedWarningTSAV0853(t *testing.T) {
	root := t.TempDir()
	commitFiles(t, root, map[string]string{"e2e/a.spec.ts": cartA, "e2e/b.spec.ts": cartB})
	repo := OpenRepo(context.Background(), root)
	blobA := gitTest(t, root, "rev-parse", "HEAD:e2e/a.spec.ts")
	blobB := gitTest(t, root, "rev-parse", "HEAD:e2e/b.spec.ts")
	options := duplicateOptions()
	const (
		a = "e2e/a.spec.ts :: cart > adds an item"
		b = "e2e/b.spec.ts :: basket gains an item"
	)
	run := func(decision string) DuplicatesReport {
		options.CallerDecisions = []DecisionFile{{Name: "r.json", Data: []byte(`{"schema":"corvint-test-duplicate-decisions/0","decisions":[` + decision + `]}`)}}
		report, err := Duplicates(repo, options)
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	base := `{"decision":"distinct-from","test":"` + a + `","other":"` + b + `","reason":"r","reviewer":"ada"`
	report := run(base + `,"testBlob":"` + blobA[:12] + `"}`)
	if len(report.Resolved) != 1 || len(report.Warnings) != 1 || report.Warnings[0].Warning != "unpinned" ||
		report.Warnings[0].Test != a || report.Warnings[0].Source != "caller:r.json" {
		t.Fatalf("half pinned: resolved=%+v warnings=%+v", report.Resolved, report.Warnings)
	}
	report = run(base + `,"testBlob":"` + blobA[:12] + `","otherBlob":"` + blobB[:12] + `"}`)
	if len(report.Resolved) != 1 || len(report.Warnings) != 0 {
		t.Fatalf("fully pinned: %+v", report.Warnings)
	}
}
