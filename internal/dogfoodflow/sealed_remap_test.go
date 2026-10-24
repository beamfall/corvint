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

// resealRepo is a lane that cites a.txt and b.txt by ordinal, then merges a
// sealed change (as merging origin/main does) and reseals with the same plan.
type resealRepo struct {
	*batchRepo
	plan     string
	sealPath string
	prepared string
	cites    [][]string
	stderr   bytes.Buffer
}

func newResealRepo(t *testing.T, withSpec bool) *resealRepo {
	t.Helper()
	repo := &resealRepo{batchRepo: newBatchRepo(t)}
	if withSpec {
		repo.write(sealAuthority, "# spec\n\n- `DCW-V0-042`: admits merged sealed CEMs\n  and cites their hunks\n- `DCW-V0-043`: other\n")
		repo.base = repo.commit("spec")
	}
	bind, _ := repo.laneFrom("sealed", repo.base, "c.txt", "c1\n", "")
	repo.sealPath = ".corvint/changes/" + bind + ".cem.json"
	testGit(t, repo.root, "checkout", "-q", "-b", "work", repo.base)
	repo.write("a.txt", "A1\na2\na3\n")
	repo.write("b.txt", "B1\nb2\nb3\n")
	repo.commit("work")
	repo.plan = filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(repo.plan, []byte("1\ta.txt\t1:1\tspecification\n2\tb.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	return repo
}

func (r *resealRepo) runner() Runner {
	return Runner{Path: "corvint", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		switch {
		case len(args) > 1 && args[0] == "cem" && args[1] == "prepare":
			if err := os.MkdirAll(filepath.Join(r.root, ".corvint"), 0o777); err != nil {
				return 1
			}
			if err := os.WriteFile(filepath.Join(r.root, ".corvint/change.cem.json"), []byte(r.prepared), 0o666); err != nil {
				return 1
			}
			return 0
		case len(args) > 9 && args[0] == "cem" && args[1] == "cite":
			r.cites = append(r.cites, []string{args[5], args[7], args[9]})
			return 0
		case len(args) > 7 && args[0] == "cem" && args[1] == "status" && args[3] == r.sealPath:
			fmt.Fprint(stdout, fakeStableStatus(true, 1, 1, args[5], args[7]))
			return 0
		}
		return 1
	}}
}

// reseal runs dogfood change over a map of the hunk IDs, each on its own path
// ("seal" stands for the merged sealed CEM file), and returns the cem-cite row.
func (r *resealRepo) reseal(ids ...string) string {
	r.t.Helper()
	hunks := []string{}
	for _, id := range ids {
		path := id + ".txt"
		if id == "seal" {
			path = r.sealPath
		}
		hunks = append(hunks, "    {\n      \"disposition\": \"unknown\",\n      \"id\": \"hunk:"+id+"\",\n      \"path\": \""+path+"\"\n    }")
	}
	r.prepared = "{\n  \"hunks\": [\n" + strings.Join(hunks, ",\n") + "\n  ]\n}\n"
	r.cites = nil
	r.stderr.Reset()
	if _, err := Change(context.Background(), ChangeOptions{Root: r.root, Base: r.base, Steps: r.runner(), Citations: r.plan}, &r.stderr); err != nil {
		r.t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(r.root, ".corvint/dogfood-report.json"))
	if err != nil {
		r.t.Fatal(err)
	}
	for _, line := range strings.Split(string(report), "\n") {
		if strings.Contains(line, `"name": "cem-cite"`) {
			return strings.TrimSpace(line)
		}
	}
	r.t.Fatalf("report has no cem-cite row:\n%s", report)
	return ""
}

func (r *resealRepo) mergeSeal() {
	r.t.Helper()
	testGit(r.t, r.root, "merge", "-q", "--no-ff", "--no-edit", "sealed")
}

// DCW-V0-340, DCW-V0-341 (V1-1638): after merging a sealed change, a reseal
// reuses the prior ordinal plan. Its rows are remapped in memory through the
// recorded hunk IDs past the inserted sealed-CEM hunk, which is cited against
// the DCW-V0-042 admission span at BASE; the plan file and its binding are
// left as they were, and a rerun remaps the same way.
func TestChangeDCWV0340RemapsOrdinalsShiftedByAdmittedSeals(t *testing.T) {
	repo := newResealRepo(t, true)
	if got := repo.reseal("a", "b"); !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("first pass cem-cite = %s; want PRODUCED\n%s", got, repo.stderr.String())
	}
	binding := filepath.Join(testGit(t, repo.root, "rev-parse", "--absolute-git-dir"), "corvint", "citation-plan-binding")
	recorded, err := os.ReadFile(binding)
	if err != nil {
		t.Fatal(err)
	}
	planBytes, err := os.ReadFile(repo.plan)
	if err != nil {
		t.Fatal(err)
	}
	repo.mergeSeal()
	want := [][]string{{"2", "a.txt", "1:1"}, {"3", "b.txt", "1:1"}, {"hunk:seal", sealAuthority, "3:4"}}
	for pass := 1; pass <= 2; pass++ {
		if got := repo.reseal("seal", "a", "b"); !strings.Contains(got, `"status": "PRODUCED"`) {
			t.Fatalf("reseal %d cem-cite = %s; want PRODUCED\n%s", pass, got, repo.stderr.String())
		}
		if fmt.Sprint(repo.cites) != fmt.Sprint(want) {
			t.Errorf("reseal %d cites = %v; want %v", pass, repo.cites, want)
		}
		if note := "dogfood-change: NOTE citation-plan-remapped rows=2 sealedHunks=1 binding="; !strings.Contains(repo.stderr.String(), note) {
			t.Errorf("reseal %d stderr lacks %q:\n%s", pass, note, repo.stderr.String())
		}
		if after, _ := os.ReadFile(repo.plan); !bytes.Equal(after, planBytes) {
			t.Errorf("reseal %d rewrote the plan file: %q", pass, after)
		}
		if after, _ := os.ReadFile(binding); !bytes.Equal(after, recorded) {
			t.Errorf("reseal %d rewrote the binding: %q; want %q", pass, after, recorded)
		}
	}
	// Any other inserted hunk still shifts the ordinals and refuses.
	if got := repo.reseal("seal", "c", "a", "b"); !strings.Contains(got, `"reason": "citation-plan-map-mismatch"`) || len(repo.cites) != 0 {
		t.Errorf("reseal past an inserted ordinary hunk: cem-cite = %s cites=%v; want citation-plan-map-mismatch", got, repo.cites)
	}
}

// DCW-V0-341: without the DCW-V0-042 admission span at BASE the merged
// sealed-CEM hunk stays owed by the plan, so the reseal refuses rather than
// cite it against nothing.
func TestChangeDCWV0341SealedHunkOwedWithoutAdmissionSpan(t *testing.T) {
	repo := newResealRepo(t, false)
	if got := repo.reseal("a", "b"); !strings.Contains(got, `"status": "PRODUCED"`) {
		t.Fatalf("first pass cem-cite = %s; want PRODUCED\n%s", got, repo.stderr.String())
	}
	repo.mergeSeal()
	if got := repo.reseal("seal", "a", "b"); !strings.Contains(got, `"reason": "citation-plan-map-mismatch"`) || len(repo.cites) != 0 {
		t.Errorf("reseal without the admission span: cem-cite = %s cites=%v; want citation-plan-map-mismatch", got, repo.cites)
	}
}

// DCW-V0-341: the appended sealed-hunk rows count toward the uniform-plan
// check, so a plan whose own rows all cite the admission span is refused, and
// they share the 256-row budget, so a partial plan over 256 unmarked hunks
// stays admissible for staging.
func TestChangeDCWV0341AppendedRowsJoinUniformAndStagingChecks(t *testing.T) {
	repo := newResealRepo(t, true)
	repo.mergeSeal()
	uniform := fmt.Sprintf("2\t%s\t3:4\tspecification\n3\t%s\t3:4\tspecification\n", sealAuthority, sealAuthority)
	if err := os.WriteFile(repo.plan, []byte(uniform), 0o666); err != nil {
		t.Fatal(err)
	}
	if got := repo.reseal("seal", "a", "b"); !strings.Contains(got, `"reason": "citation-plan-uniform"`) || len(repo.cites) != 0 {
		t.Errorf("plan completed uniform by the appended row: cem-cite = %s cites=%v; want citation-plan-uniform", got, repo.cites)
	}
	if err := os.WriteFile(repo.plan, []byte("2\ta.txt\t1:1\tspecification\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	ids := []string{"seal"}
	for i := 1; i <= 256; i++ {
		ids = append(ids, fmt.Sprintf("h%d", i))
	}
	if got := repo.reseal(ids...); !strings.Contains(got, `"status": "PRODUCED"`) || len(repo.cites) != 2 {
		t.Errorf("partial plan over 256 unmarked hunks: cem-cite = %s cites=%v; want PRODUCED with the plan row and the seal row", got, repo.cites)
	}
}
