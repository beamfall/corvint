package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// repositoryTMPDIR points TMPDIR at an existing directory inside the
// repository's worktree and returns it, so a run that honoured it would
// export the target tree there.
func repositoryTMPDIR(t *testing.T, root string) string {
	t.Helper()
	inside := filepath.Join(root, "scratch-tmp")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	// TMP and TEMP control os.TempDir() on Windows, TMPDIR elsewhere (V1-1489).
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, inside)
	}
	if os.TempDir() != inside {
		t.Fatalf("os.TempDir() is %q, want %q", os.TempDir(), inside)
	}
	return inside
}

// assertNothingExported fails when anything was written under the
// repository-local TMPDIR.
func assertNothingExported(t *testing.T, inside string) {
	t.Helper()
	entries, err := os.ReadDir(inside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("the export was placed inside the repository: %v", entries)
	}
}

// TestCEMV1250HunkRevertRefusesRepositoryTMPDIR pins that --hunk-revert never
// exports into a TMPDIR inside the repository: every row is not-run
// infrastructure and nothing is written there.
func TestCEMV1250HunkRevertRefusesRepositoryTMPDIR(t *testing.T) {
	root, base, target := makeRevertRepo(t)
	hunks := prepareRevert(t, root, base, target, [3][3]string{
		{"pkg/calc/calc_test.go", "5:5", "test-claim"},
		{"pkg/calc/broken_test.go", "5:5", "test-claim"},
		{"pkg/calc/abs_test.go", "5:5", "test-claim"},
	})
	inside := repositoryTMPDIR(t, root)
	report, rows := revertRows(t, verifyStable(t, root, base, target, true))
	for _, hunk := range hunks {
		if row := rows[hunk.ID]; row.Result != verify.RevertNotRun || row.Reason != verify.RevertInfrastructure {
			t.Fatalf("row %s = %+v, want not-run infrastructure", hunk.ID, row)
		}
	}
	if report.Held != 0 || report.NotRun != 3 {
		t.Fatalf("report = %+v", report)
	}
	assertNothingExported(t, inside)
}

// TestCEMV1250GuardingTestsRefusesRepositoryTMPDIR pins the same refusal,
// through the same helper, for --guarding-tests: no row is guarded and no
// plan row is printed.
func TestCEMV1250GuardingTestsRefusesRepositoryTMPDIR(t *testing.T) {
	root, base, target := makeRevertRepo(t)
	if _, err := openSession(t, root).Prepare(ctx(), PrepareOptions{Base: base, Target: target}); err != nil {
		t.Fatal(err)
	}
	inside := repositoryTMPDIR(t, root)
	_, report, rows := verifyGuards(t, root, base, target)
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want 3", rows)
	}
	for id, row := range rows {
		if row.Result != verify.GuardNotRun || row.Reason != verify.RevertInfrastructure {
			t.Fatalf("row %s = %+v, want not-run infrastructure", id, row)
		}
	}
	if report.Guarded != 0 || report.NotRun != 3 || len(report.Plan) != 0 {
		t.Fatalf("report = %+v", report)
	}
	assertNothingExported(t, inside)
}
