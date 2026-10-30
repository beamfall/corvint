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

// rubyRangeFixture commits count Ruby files and then changes every one of
// them, the V1-2959 reproduction.
func rubyRangeFixture(t *testing.T, count int) (*contextindex.Index, string) {
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
	write := func(value int) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(root, "lib"), 0o755); err != nil {
			t.Fatal(err)
		}
		for position := 1; position <= count; position++ {
			body := fmt.Sprintf("def f%03d; %d; end\n", position, value)
			if err := os.WriteFile(filepath.Join(root, "lib", fmt.Sprintf("f%03d.rb", position)), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(1)
	fixtureGit("add", ".")
	fixtureGit("commit", "--quiet", "-m", "base")
	base := fixtureGit("rev-parse", "HEAD")
	write(2)
	fixtureGit("commit", "--quiet", "-am", "change")
	index, err := contextindex.Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index, base
}

// TestCompileNamesTheAdmissionPacketBound pins AGW-V0-038 (V1-2959): an
// admitted path the 50-result admission packet ranks out is NOT_RUN with
// ADMISSION_PACKET_BOUND, admission is PARTIAL and its detail names every such
// path and the bound; past 100 paths the PATH_BOUND refusals sit beside it.
func TestCompileNamesTheAdmissionPacketBound(t *testing.T) {
	for _, test := range []struct {
		files, bounded, pathBound int
	}{{60, 10, 0}, {101, 50, 1}} {
		t.Run(fmt.Sprint(test.files), func(t *testing.T) {
			index, base := rubyRangeFixture(t, test.files)
			report, err := Compile(context.Background(), index, Options{Base: base})
			if err != nil {
				t.Fatal(err)
			}
			if report.Range.Admission != "PARTIAL" || !strings.Contains(report.Range.AdmissionDetail, "50-result admission packet bound (ADMISSION_PACKET_BOUND)") {
				t.Fatalf("admission = %s %q", report.Range.Admission, report.Range.AdmissionDetail)
			}
			reasons := map[string]int{}
			for _, obligation := range report.Obligations {
				if obligation.Kind != KindChange {
					continue
				}
				reasons[obligation.Reason]++
				if obligation.Reason == "ADMISSION_PACKET_BOUND" {
					if obligation.Verdict != NotRun || !strings.Contains(report.Range.AdmissionDetail, obligation.Path) {
						t.Fatalf("bounded obligation %+v is not NOT_RUN or not named in %q", obligation, report.Range.AdmissionDetail)
					}
				}
			}
			if reasons["ADMISSION_PACKET_BOUND"] != test.bounded || reasons["ADMISSION_REFUSED_PATH_BOUND"] != test.pathBound ||
				reasons["ADMISSION_PACKET_BOUND"]+reasons["ADMISSION_REFUSED_PATH_BOUND"]+reasons["NO_CLOSING_AUTHORITY"] != test.files {
				t.Fatalf("change obligation reasons = %v", reasons)
			}
			if (test.pathBound != 0) != strings.Contains(report.Range.AdmissionDetail, "(PATH_BOUND)") {
				t.Fatalf("PATH_BOUND naming in %q", report.Range.AdmissionDetail)
			}
			found := false
			for _, precondition := range report.Preconditions {
				found = found || precondition.ID == "WITNESS-P8-ADMISSION-PACKET-BOUND"
			}
			if !found {
				t.Fatalf("preconditions lack WITNESS-P8: %+v", report.Preconditions)
			}
		})
	}
}
