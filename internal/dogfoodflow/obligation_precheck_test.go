package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// obligationRepository commits one intent document under dir per count, each
// declaring that many requirements, and returns the root, the base and the
// manifest path.
func obligationRepository(t *testing.T, dir string, counts ...int) (root, base, manifest string) {
	t.Helper()
	root = t.TempDir()
	testGit(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("base\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	testGit(t, root, "add", "a.txt")
	testGit(t, root, "commit", "-q", "-m", "base")
	base = testGit(t, root, "rev-parse", "HEAD")
	if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(dir)), 0o777); err != nil {
		t.Fatal(err)
	}
	list := new(strings.Builder)
	for index, count := range counts {
		spec := new(strings.Builder)
		spec.WriteString("# Intent\n\n## Requirements\n\n")
		for requirement := 1; requirement <= count; requirement++ {
			fmt.Fprintf(spec, "- `PRE%d-V0-%03d`: keep the bound.\n", index+1, requirement)
		}
		spec.WriteString("\n## Non-goals\n")
		path := fmt.Sprintf("%s/intent-%d.md", dir, index+1)
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte(spec.String()), 0o666); err != nil {
			t.Fatal(err)
		}
		list.WriteString(path + "\n")
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "intents")
	manifest = filepath.Join(t.TempDir(), "intents")
	if err := os.WriteFile(manifest, []byte(list.String()), 0o666); err != nil {
		t.Fatal(err)
	}
	return root, base, manifest
}

// TestOCMV0028OverBoundIntentsRefuseBeforeAnyStep (OCM-V0-028, V1-2893): a
// manifest whose intents derive more obligations than the fixed aggregate
// bound refuses before any Corvint step runs or the previous evidence is
// cleared, and names the fitting subset; a manifest at the bound proceeds.
// The real bound admits every well-formed manifest (V1-2971), so the refusal
// runs at a lowered bound of 256.
func TestOCMV0028OverBoundIntentsRefuseBeforeAnyStep(t *testing.T) {
	defer func(bound int) { aggregateObligationBound = bound }(aggregateObligationBound)
	aggregateObligationBound = 256
	root, base, manifest := obligationRepository(t, "docs/specs", 200, 40, 30)
	evidence := filepath.Join(testGit(t, root, "rev-parse", "--absolute-git-dir"), "corvint")
	kept := filepath.Join(evidence, "cem-status.json")
	if err := os.MkdirAll(evidence, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(kept, []byte("previous\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stderr bytes.Buffer
	code, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, IntentsFile: manifest}, &stderr)
	if err != nil || code != 2 {
		t.Fatalf("code=%d err=%v stderr:\n%s", code, err, &stderr)
	}
	for _, want := range []string{
		"dogfood-change: REFUSE too-many-obligations\n",
		"derive 270 obligations at HEAD, over the fixed OCM aggregate bound 256; per scope 1=200, 2=40, 3=30;",
		"  fix: keep in DOGFOOD_INTENTS_FILE: docs/specs/intent-1.md=200 docs/specs/intent-2.md=40 (total 240 of bound 256); bind separately as a staged change: docs/specs/intent-3.md=30",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("refusal lacks %q:\n%s", want, &stderr)
		}
	}
	if calls != 0 {
		t.Errorf("%d Corvint steps ran before the refusal", calls)
	}
	if got, _ := os.ReadFile(kept); string(got) != "previous\n" {
		t.Errorf("the previous evidence was cleared: %q", got)
	}

	root, base, manifest = obligationRepository(t, "docs/specs", 200, 56)
	stderr.Reset()
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, IntentsFile: manifest}, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "too-many-obligations") || calls == 0 {
		t.Fatalf("a manifest at the bound was refused or ran no step (%d calls):\n%s", calls, &stderr)
	}

	// A record change naming its own build-log entries declares no intent and
	// prepares no OCM scope, so their requirement lists are never counted.
	root, base, manifest = obligationRepository(t, "docs/build-log", 200, 57)
	stderr.Reset()
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, IntentsFile: manifest}, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "too-many-obligations") {
		t.Fatalf("a record change's own entries were counted as obligations:\n%s", &stderr)
	}
}

// TestOCMV0028DerivedBoundAdmitsSummedScopes (V1-2971, OCM-V0-021): at the
// derived aggregate bound, three intents summing to 271 obligations, past one
// map's 256, proceed to the steps instead of refusing.
func TestOCMV0028DerivedBoundAdmitsSummedScopes(t *testing.T) {
	if aggregateObligationBound != 4096 {
		t.Fatalf("aggregate obligation bound = %d, want 16 scopes x 256", aggregateObligationBound)
	}
	root, base, manifest := obligationRepository(t, "docs/specs", 151, 105, 15)
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stderr bytes.Buffer
	if _, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, IntentsFile: manifest}, &stderr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stderr.String(), "too-many-obligations") || calls == 0 {
		t.Fatalf("271 obligations over three scopes were refused or ran no step (%d calls):\n%s", calls, &stderr)
	}
}

// TestDCWV0961OverBoundSingleIntentRefusesBeforeAnyStep (V1-3047): an intent
// whose Requirements section alone is over one OCM map's 256 obligations
// refuses before any step runs, naming that intent and not the one within the
// bound; the count used to fail, the precheck skipped and ocm-prepare refused
// only after the bind commit.
func TestDCWV0961OverBoundSingleIntentRefusesBeforeAnyStep(t *testing.T) {
	root, base, manifest := obligationRepository(t, "docs/specs", 257, 3)
	calls := 0
	steps := Runner{Path: "corvint", Run: func(context.Context, string, []string, io.Writer, io.Writer) int {
		calls++
		return 1
	}}
	var stderr bytes.Buffer
	code, err := Change(context.Background(), ChangeOptions{Root: root, Base: base, Steps: steps, IntentsFile: manifest}, &stderr)
	if err != nil || code != 2 || calls != 0 {
		t.Fatalf("code=%d err=%v calls=%d stderr:\n%s", code, err, calls, &stderr)
	}
	for _, want := range []string{
		"dogfood-change: REFUSE too-many-obligations\n",
		"  intent docs/specs/intent-1.md in DOGFOOD_INTENTS_FILE derives more obligations at HEAD than one OCM map admits (256); no step ran",
		"  fix: drop it from DOGFOOD_INTENTS_FILE and cite the BASE lines of it the change depends on, or list a companion spec",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("refusal lacks %q:\n%s", want, &stderr)
		}
	}
	if strings.Contains(stderr.String(), "intent-2.md") {
		t.Errorf("the intent within the bound was named:\n%s", &stderr)
	}
}
