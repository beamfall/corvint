package witness

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// perPathFixture commits a range with one admitted Go change, one path whose
// name the range profile refuses, one deletion and one path with '@' and '+'
// in its name.
func perPathFixture(t *testing.T) (*contextindex.Index, string) {
	t.Helper()
	return perPathFixtureWith(t, 0)
}

// perPathFixtureWith is perPathFixture whose change also adds generated
// generated text files, sorting after every other changed path.
func perPathFixtureWith(t *testing.T, generated int) (*contextindex.Index, string) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := initTestRepo(t, git)
	fixtureGit := func(args ...string) string {
		t.Helper()
		command := exec.Command(git, append([]string{"-c", "maintenance.auto=false", "-c", "gc.auto=0", "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test"}, args...)...)
		command.Dir, command.Env = root, gitEnvironment()
		raw, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git %v: %v: %s", args, err, raw)
		}
		return strings.TrimSpace(string(raw))
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, filepath.Dir(name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/fixture\n\ngo 1.22\n")
	write("a/a.go", "package a\n\n// A is used by b.\nfunc A() int { return 1 }\n")
	write("b/b.go", "package b\n\nimport \"example.test/fixture/a\"\n\n// B calls A.\nfunc B() int { return a.A() }\n")
	write("README.md", "fixture readme with its own content\n")
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	write("a/a.go", "package a\n\n// A is used by b.\nfunc A() int { return 2 }\n")
	write("notes/a b.txt", "a name with a space\n")
	write("assets/@scope/c++.txt", "admitted characters\n")
	for position := 0; position < generated; position++ {
		write(fmt.Sprintf("zz/generated-%03d.txt", position), fmt.Sprintf("generated %d\n", position))
	}
	fixtureGit("rm", "--quiet", "README.md")
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "change")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base
}

// TestCompileRefusesOnlyTheOffendingPaths pins AGW-V0-004 (V1-2796): an
// unsafe path and a deletion are each their own NOT_RUN obligation naming the
// rule, the rest of the range is still admitted and closed over, and the
// precondition names the refused paths.
func TestCompileRefusesOnlyTheOffendingPaths(t *testing.T) {
	index, base := perPathFixture(t)
	report, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if report.Range.Admission != "PARTIAL" || !strings.Contains(report.Range.AdmissionDetail, "notes/a b.txt (UNSAFE_PATH)") ||
		!strings.Contains(report.Range.AdmissionDetail, "README.md (DELETED)") {
		t.Fatalf("admission %s: %s", report.Range.Admission, report.Range.AdmissionDetail)
	}
	if report.Range.Closure != "COMPUTED" {
		t.Fatalf("closure %s: %s; the admitted Go path must still be closed over", report.Range.Closure, report.Range.ClosureDetail)
	}
	reasons, verdicts := map[string]string{}, map[string]string{}
	for _, obligation := range report.Obligations {
		reasons[obligation.ID] = obligation.Reason
		verdicts[obligation.ID] = string(obligation.Verdict)
	}
	for id, want := range map[string]string{
		"change:notes/a b.txt": "ADMISSION_REFUSED_UNSAFE_PATH",
		"change:README.md":     "ADMISSION_REFUSED_DELETED",
	} {
		if reasons[id] != want || verdicts[id] != string(NotRun) {
			t.Errorf("%s reason %q verdict %s, want %q NOT_RUN", id, reasons[id], verdicts[id], want)
		}
	}
	for _, id := range []string{"change:a/a.go", "change:assets/@scope/c++.txt", "impact:reverse-import:b/b.go"} {
		if _, ok := reasons[id]; !ok {
			t.Errorf("obligation %s missing; obligations %v", id, reasons)
		}
		if strings.HasPrefix(reasons[id], "ADMISSION_REFUSED") {
			t.Errorf("%s was refused: %s", id, reasons[id])
		}
	}
	got := identifiers(preconditions(report))
	if !strings.Contains(got, "WITNESS-P7-PATHS-REFUSED-PER-PATH") || strings.Contains(got, "WITNESS-P4") {
		t.Errorf("preconditions %s: want P7 and no P4", got)
	}
}

// TestCompileRefusesPathsPastThePathBoundEachAlone pins AGW-V0-035: a range
// over the 100-path bound is PARTIAL, not REFUSED. Its paths past the bound in
// path order are each NOT_RUN ADMISSION_REFUSED_PATH_BOUND, and the admitted
// Go change is still closed over.
func TestCompileRefusesPathsPastThePathBoundEachAlone(t *testing.T) {
	index, base := perPathFixtureWith(t, 100)
	report, err := Compile(context.Background(), index, Options{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	if report.Range.Admission != "PARTIAL" || !strings.Contains(report.Range.AdmissionDetail, "(PATH_BOUND)") {
		t.Fatalf("admission %s: %s", report.Range.Admission, report.Range.AdmissionDetail)
	}
	if report.Range.Closure != "COMPUTED" {
		t.Fatalf("closure %s: %s", report.Range.Closure, report.Range.ClosureDetail)
	}
	bounded := 0
	for _, obligation := range report.Obligations {
		if obligation.Reason == "ADMISSION_REFUSED_PATH_BOUND" {
			bounded++
			if obligation.Verdict != NotRun || !strings.HasPrefix(obligation.Path, "zz/generated-") {
				t.Errorf("%s %s past the bound", obligation.ID, obligation.Verdict)
			}
		}
		if obligation.ID == "change:a/a.go" && strings.HasPrefix(obligation.Reason, "ADMISSION_REFUSED") {
			t.Errorf("the admitted Go change was refused: %s", obligation.Reason)
		}
	}
	// 104 changed paths (one deletion among them) leave four past the bound.
	if bounded != 4 {
		t.Errorf("%d PATH_BOUND obligations, want 4", bounded)
	}
}

// TestResolveCommitAcceptsAnyGitRevision pins AGW-V0-005: a branch, a relative
// revision and an abbreviated ID resolve to the full commit object ID; an
// option-like or unknown revision refuses.
func TestResolveCommitAcceptsAnyGitRevision(t *testing.T) {
	index, base := packetFixture(t, "example.test/fixture")
	for _, revision := range []string{"HEAD~1", base[:12], base} {
		got, err := ResolveCommit(context.Background(), index.Root, revision)
		if err != nil || got != base {
			t.Errorf("%s resolved to %q, %v; want %s", revision, got, err, base)
		}
	}
	got, err := ResolveCommit(context.Background(), index.Root, "HEAD")
	if err != nil || got != index.CommitRevision {
		t.Errorf("HEAD resolved to %q, %v; want %s", got, err, index.CommitRevision)
	}
	for _, revision := range []string{"", "--all", "-h", "no-such-branch", strings.Repeat("a", 40), "HEAD\nHEAD"} {
		if got, err := ResolveCommit(context.Background(), index.Root, revision); err == nil {
			t.Errorf("%q resolved to %s; want a refusal", revision, got)
		}
	}
}
