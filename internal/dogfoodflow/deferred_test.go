package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/workflow"
)

// DCW-V0-370: deferred rows leave the citation plan before its cite rows are
// read, keep plan order, and are refused unless they meet the CEM-V1-643
// record rules; the cite count excludes them.
func TestDCWV0370SplitsDeferredPlanRows(t *testing.T) {
	cite := "1\ta.txt\t1:1\tspecification\n"
	stripped, rows, valid := splitDeferredRows([]byte(cite + "deferred\tdocs/specs/b.md\tlater slice\ndeferred\tdocs/specs/a.md\tnext batch\n"))
	if !valid || string(stripped) != cite {
		t.Fatalf("split = %q, %t; want the cite row", stripped, valid)
	}
	if want := []deferral{{"docs/specs/b.md", "later slice"}, {"docs/specs/a.md", "next batch"}}; !slices.Equal(rows, want) {
		t.Fatalf("rows = %v; want %v", rows, want)
	}
	if stripped, rows, valid := splitDeferredRows([]byte("deferred\tdocs/a.md\tlater\n")); !valid || len(stripped) != 0 || stripped == nil || len(rows) != 1 {
		t.Fatalf("deferral-only plan = %q, %v, %t", stripped, rows, valid)
	}
	if stripped, rows, valid := splitDeferredRows([]byte(cite)); !valid || string(stripped) != cite || rows != nil {
		t.Fatalf("plan without deferrals = %q, %v, %t", stripped, rows, valid)
	}
	five := ""
	for index := range 5 {
		five += fmt.Sprintf("deferred\tdocs/%d.md\tlater\n", index)
	}
	for name, plan := range map[string]string{
		"two fields":     "deferred\tdocs/a.md\n",
		"four fields":    "deferred\tdocs/a.md\tlater\tnote\n",
		"repeated path":  "deferred\tdocs/a.md\tlater\ndeferred\tdocs/a.md\tagain\n",
		"over the bound": five,
		"empty reason":   "deferred\tdocs/a.md\t\n",
		"long reason":    "deferred\tdocs/a.md\t" + strings.Repeat("r", 201) + "\n",
		"absolute path":  "deferred\t/docs/a.md\tlater\n",
		"dot segment":    "deferred\tdocs/../a.md\tlater\n",
	} {
		if _, _, valid := splitDeferredRows([]byte(plan)); valid {
			t.Errorf("%s: plan %q admitted", name, plan)
		}
	}
	plan := filepath.Join(t.TempDir(), "cites.tsv")
	if err := os.WriteFile(plan, []byte(cite+"deferred\tdocs/a.md\tlater\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	c := &change{options: ChangeOptions{Citations: plan}}
	if got, valid := c.validateCitationPlan(); !valid || string(got) != cite || c.citationCount != 1 || len(c.deferrals) != 1 {
		t.Fatalf("validateCitationPlan = %q, %t, count %d, deferrals %v", got, valid, c.citationCount, c.deferrals)
	}
}

// DCW-V0-371: the strict status argv gains --max-deferred-intents only for a
// change that records deferrals, so every other change keeps the argv a BASE
// verifier that predates the flag reads.
func TestDCWV0371StatusArgumentsBoundDeferrals(t *testing.T) {
	oid := strings.Repeat("a", 40)
	root := t.TempDir()
	plain := statusArguments(root, oid, oid, 2, 0)
	if slices.Contains(plain, "--max-deferred-intents") {
		t.Fatalf("argv without deferrals = %q", plain)
	}
	bounded := statusArguments(root, oid, oid, 2, 3)
	if !slices.Equal(bounded[:len(plain)-2], plain[:len(plain)-2]) || !slices.Equal(bounded[len(plain)-2:len(plain)], []string{"--max-deferred-intents", "3"}) {
		t.Fatalf("bounded argv = %q; plain %q", bounded, plain)
	}
	for _, args := range [][]string{plain, bounded, bounded[:len(bounded)-2]} {
		if !ValidAggregateVerifierInvocation(root, args) {
			t.Fatalf("aggregate matcher refused %q", args)
		}
	}
	status := []string{"cem", "status", "--map", ".corvint/change.cem.json", "--expected-base", oid, "--target", oid, "--max-unknown", "0", "--max-mechanical", "0"}
	for _, args := range [][]string{
		append(append([]string{}, status...), "--max-deferred-intents", "0"),
		append(append([]string{}, status...), "--max-deferred-intents", "5"),
		append(append([]string{}, status...), "--max-deferred-intents", "01"),
		append(append([]string{}, status...), "--max-deferred-intents", "x"),
		append(append([]string{}, status...), "--max-deferred-intent", "1"),
		append(append([]string{}, status...), "--artifacts", root, "--max-deferred-intents", "1"),
		append(append([]string{}, status...), "--max-deferred-intents", "1", "--artifacts", t.TempDir()),
		append(append([]string{}, status...), "--max-deferred-intents", "1", "--max-deferred-intents", "1"),
	} {
		if ValidAggregateVerifierInvocation(root, args) {
			t.Errorf("aggregate matcher admitted %q", args)
		}
	}
}

// DCW-V0-371: a check reads the bound from the deferral records the range
// adds, and an unreadable record counts nothing.
func TestDCWV0371RecordedDeferralsCountAddedRecords(t *testing.T) {
	repo := newBatchRepo(t)
	entries := []workflow.DeferredIntent{{ObligationCount: 2, Path: "docs/a.md", Reason: "later"}, {ObligationCount: 1, Path: "docs/b.md", Reason: "next"}}
	record, err := workflow.EncodeDeferredIntents(entries)
	if err != nil {
		t.Fatal(err)
	}
	repo.write(workflow.DeferredRecordPath(record), string(record))
	target := repo.commit("record")
	f := &flow{ctx: context.Background(), root: repo.root, stderr: io.Discard}
	if count, paths := f.recordedDeferrals(repo.base, target); count != 2 || !slices.Equal(paths, []string{"docs/a.md", "docs/b.md"}) {
		t.Fatalf("recordedDeferrals = %d, %v", count, paths)
	}
	if count, paths := f.recordedDeferrals(target, target); count != 0 || paths != nil {
		t.Fatalf("empty range = %d, %v", count, paths)
	}
	repo.write(workflow.DeferredIntentsDir+"/bad.json", "{}\n")
	bad := repo.commit("bad record")
	if count, _ := f.recordedDeferrals(target, bad); count != 0 {
		t.Fatalf("invalid record counted %d", count)
	}
}

// deferralRunner is the reseal runner that also accepts `cem defer`, keeping
// each deferral's --intent and --reason.
func (r *resealRepo) deferralRunner(defers *[][]string) Runner {
	base := r.runner()
	return Runner{Path: base.Path, Run: func(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
		if len(args) == 8 && args[0] == "cem" && args[1] == "defer" {
			*defers = append(*defers, []string{args[5], args[7]})
			return 0
		}
		return base.Run(ctx, root, args, stdout, stderr)
	}}
}

// deferralChange runs dogfood change over a prepared map of the given
// (id, path) hunks and returns the report rows named prefix.
func (r *resealRepo) deferralChange(intents string, defers *[][]string, hunks ...[2]string) []string {
	r.t.Helper()
	rows := []string{}
	for _, hunk := range hunks {
		rows = append(rows, `{"disposition":"unknown","id":"hunk:`+hunk[0]+`","path":"`+hunk[1]+`"}`)
	}
	r.prepared = `{"hunks":[` + strings.Join(rows, ",") + "]}\n"
	r.cites = nil
	r.stderr.Reset()
	options := ChangeOptions{Root: r.root, Base: r.base, Steps: r.deferralRunner(defers), Citations: r.plan}
	if intents != "" {
		options.IntentsFile = filepath.Join(r.t.TempDir(), "intents")
		if err := os.WriteFile(options.IntentsFile, []byte(intents), 0o666); err != nil {
			r.t.Fatal(err)
		}
	}
	if _, err := Change(context.Background(), options, &r.stderr); err != nil {
		r.t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(r.root, ".corvint/dogfood-report.json"))
	if err != nil {
		r.t.Fatal(err)
	}
	found := []string{}
	for _, line := range strings.Split(string(report), "\n") {
		if strings.Contains(line, `"name": "cem-`) {
			found = append(found, strings.TrimSpace(line))
		}
	}
	return found
}

func rowNamed(rows []string, name string) string {
	for _, row := range rows {
		if strings.Contains(row, `"name": "`+name+`"`) {
			return row
		}
	}
	return ""
}

func (r *resealRepo) addDeferredRow(row string) {
	r.t.Helper()
	plan, err := os.ReadFile(r.plan)
	if err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(r.plan, append(plan, row...), 0o666); err != nil {
		r.t.Fatal(err)
	}
}

// DCW-V0-370: a deferred plan row runs `cem defer` on the prepared map before
// any cite, and a deferred path the intent manifest keeps is refused before
// any record is written.
func TestDCWV0370DeferredRowsRunCemDefer(t *testing.T) {
	repo := newResealRepo(t, false)
	repo.addDeferredRow("deferred\tdocs/specs/later.md\tnext slice\n")
	var defers [][]string
	rows := repo.deferralChange("", &defers, [2]string{"a", "a.txt"}, [2]string{"b", "b.txt"})
	if row := rowNamed(rows, "cem-defer-001"); !strings.Contains(row, `"status": "PRODUCED"`) {
		t.Fatalf("cem-defer-001 = %q; rows %v\n%s", row, rows, repo.stderr.String())
	}
	if want := [][]string{{"docs/specs/later.md", "next slice"}}; fmt.Sprint(defers) != fmt.Sprint(want) {
		t.Fatalf("defers = %v; want %v", defers, want)
	}
	if want := [][]string{{"1", "a.txt", "1:1"}, {"2", "b.txt", "1:1"}}; fmt.Sprint(repo.cites) != fmt.Sprint(want) {
		t.Fatalf("cites = %v; want %v (the deferred row is not a cite)", repo.cites, want)
	}

	defers = nil
	rows = repo.deferralChange("docs/specs/later.md\n", &defers, [2]string{"a", "a.txt"}, [2]string{"b", "b.txt"})
	if row := rowNamed(rows, "cem-defer"); !strings.Contains(row, `"status": "NOT_PRODUCED"`) || !strings.Contains(row, "deferred-intent-kept") {
		t.Fatalf("cem-defer = %q; rows %v", row, rows)
	}
	if len(defers) != 0 {
		t.Fatalf("kept intent still deferred: %v", defers)
	}
	if !strings.Contains(repo.stderr.String(), "dogfood-change: FAIL cem-defer deferred-intent-kept paths=docs/specs/later.md") {
		t.Fatalf("stderr lacks the refusal:\n%s", repo.stderr.String())
	}
}

// DCW-V0-372: the rebind's map holds the deferral record the bind commit
// added; the plan's ordinal rows shifted by it are remapped, and the record
// hunk is cited against the DCW-V0-372 bullet at BASE without a plan row.
func TestDCWV0372CitesTheDeferralRecordHunk(t *testing.T) {
	repo := newResealRepo(t, false)
	repo.write(sealAuthority, "# spec\n\n- `DCW-V0-371`: other\n- `DCW-V0-372`: cites the record\n  hunk automatically\n- `DCW-V0-373`: other\n")
	repo.base = repo.commit("spec")
	testGit(t, repo.root, "checkout", "-q", "-b", "deferring", repo.base)
	repo.write("a.txt", "A1\nA2\na3\n")
	repo.write("b.txt", "B1\nB2\nb3\n")
	repo.commit("deferring work")
	repo.addDeferredRow("deferred\tdocs/specs/later.md\tnext slice\n")
	var defers [][]string
	if row := rowNamed(repo.deferralChange("", &defers, [2]string{"a", "a.txt"}, [2]string{"b", "b.txt"}), "cem-cite"); !strings.Contains(row, `"status": "PRODUCED"`) {
		t.Fatalf("bind cem-cite = %q\n%s", row, repo.stderr.String())
	}
	record := workflow.DeferredIntentsDir + "/" + strings.Repeat("e", 64) + ".json"
	want := [][]string{{"2", "a.txt", "1:1"}, {"3", "b.txt", "1:1"}, {"hunk:rec", sealAuthority, "4:5"}}
	for pass := 1; pass <= 2; pass++ {
		rows := repo.deferralChange("", &defers, [2]string{"rec", record}, [2]string{"a", "a.txt"}, [2]string{"b", "b.txt"})
		if row := rowNamed(rows, "cem-cite"); !strings.Contains(row, `"status": "PRODUCED"`) {
			t.Fatalf("rebind %d cem-cite = %q\n%s", pass, row, repo.stderr.String())
		}
		if fmt.Sprint(repo.cites) != fmt.Sprint(want) {
			t.Errorf("rebind %d cites = %v; want %v", pass, repo.cites, want)
		}
	}
}

// DCW-V0-370: the bind commit carries the deferral record the bind pass wrote
// beside the sidecar, so the deferral needs no extra commit, and a rerun
// still recognizes that commit as the bind commit.
func TestFinishLaneDCWV0370BindCommitCarriesTheDeferralRecord(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write("a.txt", "changed\n")
	change := repo.commit("change")
	record := workflow.DeferredIntentsDir + "/" + strings.Repeat("e", 64) + ".json"
	repo.write(finishLaneSidecar, "{}\n")
	repo.write(record, "{}\n")
	repo.write("untouched.txt", "kept out\n")
	var stdout, stderr bytes.Buffer
	lane := &finishLane{flow: flow{ctx: context.Background(), prefix: "dogfood-finish-lane", root: repo.root, stderr: &stderr}, stdout: &stdout, clock: newLaneClock()}
	if code, err := boundary(context.Background(), func() int { lane.commitBind(); return 0 }, func() {}); err != nil || code != 0 {
		t.Fatalf("commitBind = %d, %v\n%s%s", code, err, stdout.String(), stderr.String())
	}
	bind := testGit(t, repo.root, "rev-parse", "HEAD")
	changed := testGit(t, repo.root, "diff-tree", "-r", "--no-commit-id", "--name-only", change, bind)
	if want := finishLaneSidecar + "\n" + record; strings.TrimSpace(changed) != want {
		t.Fatalf("bind commit changed %q; want %q", changed, want)
	}
	if !strings.Contains(testGit(t, repo.root, "status", "--porcelain"), "untouched.txt") {
		t.Fatal("bind commit took an unrelated path")
	}
	if !lane.isBindCommit(bind) {
		t.Fatal("the bind commit with its record is not recognized")
	}
	testGit(t, repo.root, "checkout", "-q", "-b", "record-only", change)
	repo.write(workflow.DeferredIntentsDir+"/other.json", "{}\n")
	testGit(t, repo.root, "add", "--", workflow.DeferredIntentsDir)
	testGit(t, repo.root, "commit", "-q", "-m", bindSubject)
	if lane.isBindCommit(testGit(t, repo.root, "rev-parse", "HEAD")) {
		t.Fatal("a record-only commit is recognized as the bind commit")
	}
}
