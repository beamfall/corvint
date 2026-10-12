package dogfoodflow

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type batchRepo struct {
	t    *testing.T
	root string
	base string
	args [][]string
	// status is the fake verifier's exit status for every constituent.
	status int
	// output, when set, replaces the fake verifier's canonical status line.
	output func(base, bind string) string
}

// newBatchRepo is a repository whose base commit has two files that lanes
// edit independently.
func newBatchRepo(t *testing.T) *batchRepo {
	t.Helper()
	repo := &batchRepo{t: t, root: t.TempDir()}
	testGit(t, repo.root, "init", "-q", "-b", "main")
	repo.write("a.txt", "a1\na2\na3\n")
	repo.write("b.txt", "b1\nb2\nb3\n")
	repo.commit("base")
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	return repo
}

func (r *batchRepo) write(path, data string) {
	r.t.Helper()
	full := filepath.Join(r.root, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o777); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(data), 0o666); err != nil {
		r.t.Fatal(err)
	}
}

func (r *batchRepo) commit(message string) string {
	r.t.Helper()
	testGit(r.t, r.root, "add", "-A")
	testGit(r.t, r.root, "commit", "-q", "-m", message)
	return testGit(r.t, r.root, "rev-parse", "HEAD")
}

// lane binds and seals one change on its own branch from base, as the daily
// loop does, and returns the bind and seal commits.
func (r *batchRepo) lane(name, path, data, cem string) (string, string) {
	r.t.Helper()
	testGit(r.t, r.root, "checkout", "-q", "-b", name, r.base)
	r.write(path, data)
	r.commit("work " + name)
	if cem == "" {
		cem = `{"spec": "cem/0.2", "baseRevision": "` + r.base + `", "hunks": []}` + "\n"
	}
	r.write(".corvint/change.cem.json", cem)
	bind := r.commit("chore: bind change evidence")
	if err := os.MkdirAll(filepath.Join(r.root, ".corvint/changes"), 0o777); err != nil {
		r.t.Fatal(err)
	}
	testGit(r.t, r.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bind+".cem.json")
	testGit(r.t, r.root, "commit", "-q", "-m", "chore: seal change evidence")
	return bind, testGit(r.t, r.root, "rev-parse", "HEAD")
}

// batch merges the sealed lane tips, in order, onto a branch from base.
func (r *batchRepo) batch(tips ...string) {
	r.t.Helper()
	testGit(r.t, r.root, "checkout", "-q", "-b", "batch", r.base)
	for _, tip := range tips {
		testGit(r.t, r.root, "merge", "-q", "--no-ff", "--no-edit", tip)
	}
}

func (r *batchRepo) check() (int, string, string) {
	r.t.Helper()
	r.args = nil
	var stdout, stderr bytes.Buffer
	code, err := BatchCheck(context.Background(), BatchCheckOptions{Root: r.root, Base: r.base, Verifier: r.verifier()}, &stdout, &stderr)
	if err != nil {
		r.t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// verifier is the fake constituent verifier: canonical ready-for-ci output of
// the base and bind it is asked about, unless status or output say otherwise.
func (r *batchRepo) verifier() Runner {
	return Runner{Path: "/never-executed/corvint", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		r.args = append(r.args, args)
		base, bind := args[5], args[7]
		if r.output != nil {
			fmt.Fprint(stdout, r.output(base, bind))
			return r.status
		}
		state := map[bool]string{true: "ready-for-ci", false: "incomplete"}[r.status == 0]
		fmt.Fprintf(stdout, `{"counts":{"mechanical":0,"supported":1,"total":1,"unknown":0},"ok":%t,"state":"%s","tool":"cem-status",`+
			`"verification":{"assurance":"canonical","baseRevision":"%s","targetRevision":"%s","valid":true}}`+"\n", r.status == 0, state, base, bind)
		return r.status
	}}
}

// admit runs dogfood change's sealed-CEM admission and inherited-CEM check
// (DCW-V0-042, DCW-V0-043) for base..HEAD with the fake verifier.
func (r *batchRepo) admit(base string) (int, string) {
	r.t.Helper()
	var stderr bytes.Buffer
	c := &change{flow: flow{ctx: context.Background(), prefix: "dogfood-change", root: r.root, stderr: &stderr}, options: ChangeOptions{Steps: r.verifier()}}
	c.base, c.target = base, testGit(r.t, r.root, "rev-parse", "HEAD")
	code, err := boundary(c.ctx, func() int {
		inherited := c.inheritedCEM()
		c.admitSealedCEMs(inherited)
		c.requireArchivedBaseCEM(inherited)
		return 0
	}, func() {})
	if err != nil {
		r.t.Fatal(err)
	}
	return code, stderr.String()
}

// DCW-V0-042: dogfood change admits merged sealed changes that re-verify and
// refuses any other file under .corvint/changes/, naming each path.
func TestChangeDCWV0042AdmitsOnlyVerifiableSealedCEMs(t *testing.T) {
	repo := newBatchRepo(t)
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	bindB, sealB := repo.lane("lane-b", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(sealA, sealB)
	code, stderr := repo.admit(repo.base)
	for _, bind := range []string{bindA, bindB} {
		if code != 0 || !strings.Contains(stderr, "dogfood-change: NOTE sealed-cem-merged bind="+bind+" base="+repo.base+" bootstrapUnknown=0\n") {
			t.Fatalf("admit exit=%d stderr=%q", code, stderr)
		}
	}
	if len(repo.args) != 2 {
		t.Fatalf("verifier runs %v", repo.args)
	}
	repo.status = 1
	if code, stderr = repo.admit(repo.base); code != 2 || !strings.HasPrefix(stderr, "dogfood-change: REFUSE sealed-cem-in-change\n  .corvint/changes/") ||
		!strings.Contains(stderr, ": constituent-cem-policy bind=") {
		t.Fatalf("unverified exit=%d stderr=%q", code, stderr)
	}
	repo.status = 0
	repo.write(".corvint/changes/"+bindA+".cem.json", "{}\n")
	repo.write(".corvint/changes/notes.json", "{}\n")
	repo.commit("edit sealed evidence")
	code, stderr = repo.admit(sealA)
	for _, want := range []string{
		"  .corvint/changes/" + bindA + ".cem.json: sealed-cem-modified status=M\n",
		"  .corvint/changes/notes.json: unrecognized-sealed-cem\n",
		"  fix: for a merge batch of sealed changes, run corvint dogfood batch-check " + sealA + " instead of rebinding it (DCW-V0-039)\n",
	} {
		if code != 2 || !strings.Contains(stderr, want) {
			t.Fatalf("modified exit=%d stderr=%q; want %q", code, stderr, want)
		}
	}
	// More added sealed files than a batch admits refuse by count, unverified.
	overflow := testGit(t, repo.root, "rev-parse", "HEAD")
	for i := 0; i <= batchConstituentLimit; i++ {
		repo.write(fmt.Sprintf(".corvint/changes/%040x.cem.json", i), "{}\n")
	}
	repo.commit("too many sealed files")
	repo.args = nil
	if code, stderr = repo.admit(overflow); code != 2 || len(repo.args) != 0 ||
		!strings.HasPrefix(stderr, fmt.Sprintf("dogfood-change: REFUSE sealed-cem-in-change\n  sealed CEMs=%d exceed %d\n", batchConstituentLimit+1, batchConstituentLimit)) {
		t.Fatalf("overflow exit=%d verifier=%v stderr=%q", code, repo.args, stderr)
	}
}

// objects lists the loose objects and packs, which a read-only check must
// leave unchanged.
func (r *batchRepo) objects() string {
	r.t.Helper()
	var names []string
	_ = filepath.Walk(filepath.Join(r.root, ".git/objects"), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			names = append(names, path)
		}
		return nil
	})
	return strings.Join(names, "\n")
}

// DCW-V0-039: a batch of sealed changes merged without manual resolution
// passes; every constituent is re-verified against its own base and bind
// commit, and the uncertainty the batch cannot observe is printed.
func TestBatchCheckDCWV0039PassesSealedConstituents(t *testing.T) {
	repo := newBatchRepo(t)
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	bindB, sealB := repo.lane("lane-b", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(sealA, sealB)
	before := repo.objects()
	code, stdout, stderr := repo.check()
	if code != 0 || stderr != "" {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"dogfood-batch-check: CONSTITUENT bind=" + bindA + " base=" + repo.base + " hunks=1 supported=1 unknown=0 bootstrapUnknown=0 reasonedUnknown=0 state=ready-for-ci\n",
		"dogfood-batch-check: CONSTITUENT bind=" + bindB + " base=" + repo.base,
		"derivation=ort\n",
		" commits=8 constituents=2 merges first-parent-tree=0 ort=2 ort-ours=0 ort-theirs=0 bootstrapUnknown=0\n",
		"dogfood-batch-check: NOTE ocm NOT_REVERIFIED sealed-constituent\n",
		"dogfood-batch-check: NOTE composition NOT_OBSERVED merged-change-interaction\n",
		"dogfood-batch-check: NOTE local-outcome NOT_PRODUCED batch-check\n",
		"dogfood-batch-check: NOTE verifier-independence NOT_OBSERVED single-verifier verifier=unavailable\n",
		"dogfood-batch-check: PASS constituents=2 commits=8\n",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("missing %q in %s", want, stdout)
		}
	}
	// Constituents run in bind-commit order, so bindA is either call.
	wantArgs := []string{"cem", "status", "--map", ".corvint/changes/" + bindA + ".cem.json", "--expected-base", repo.base, "--target", bindA, "--max-unknown", "0", "--max-mechanical", "0"}
	if len(repo.args) != 2 || strings.Join(repo.args[0], " ") != strings.Join(wantArgs, " ") && strings.Join(repo.args[1], " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("verifier argv %q", repo.args)
	}
	if after := repo.objects(); after != before {
		t.Fatalf("batch-check wrote objects:\n%s\nwas\n%s", after, before)
	}
	if status := testGit(t, repo.root, "status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("worktree changed: %s", status)
	}
}

// DCW-V0-039: a one-sided conflict resolution is derivable from the parents,
// and the dropped side is printed rather than hidden.
func TestBatchCheckDCWV0039DerivesOneSidedResolution(t *testing.T) {
	repo := newBatchRepo(t)
	_, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "a.txt", "a1\nB2\na3\n", "")
	repo.batch(sealA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-X", "ours", sealB)
	before := repo.objects()
	code, stdout, stderr := repo.check()
	if code != 0 || !strings.Contains(stdout, "derivation=ort-ours\n") || !strings.Contains(stdout, "dogfood-batch-check: NOTE conflict-side-dropped merges=1\n") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	if after := repo.objects(); after != before {
		t.Fatalf("merge-tree wrote into the repository:\n%s", after)
	}
}

// DCW-V0-039 and DCW-V0-040: a manual merge resolution and a commit no sealed
// change binds fail, each naming the commit, the paths and the recovery.
func TestBatchCheckDCWV0040NamesUnboundContentAndRecovery(t *testing.T) {
	repo := newBatchRepo(t)
	_, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "a.txt", "a1\nB2\na3\n", "")
	repo.batch(sealA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-X", "ours", sealB)
	repo.write("a.txt", "a1\nAB2\na3\n")
	testGit(t, repo.root, "add", "a.txt")
	testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	repo.write("b.txt", "b1\nb2\nfix\n")
	fixup := repo.commit("orchestrator fix-up")
	code, _, stderr := repo.check()
	for _, want := range []string{
		"dogfood-batch-check: FAIL merge-resolution-unbound count=1\n  " + merge + " paths=a.txt\n  fix: redo the merge with git merge -X ours (or -X theirs)",
		"dogfood-batch-check: FAIL unbound-commits count=1\n  " + fixup + "\n  fix: bind each listed commit through a sealed change",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("exit=%d missing %q in %s", code, want, stderr)
		}
	}
	if code != 1 {
		t.Fatalf("exit=%d", code)
	}
}

// DCW-V0-039: sealed CEMs are immutable; a drifted copy, an edited CEM that
// BASE already sealed, an unsealed sidecar and a failing constituent policy
// each fail.
func TestBatchCheckDCWV0039RefusesDriftedEvidence(t *testing.T) {
	repo := newBatchRepo(t)
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(sealA)
	repo.write(".corvint/changes/"+bindA+".cem.json", `{"baseRevision": "`+repo.base+`", "hunks": [], "edited": true}`+"\n")
	repo.write(".corvint/change.cem.json", "{}\n")
	repo.commit("edit sealed evidence")
	code, _, stderr := repo.check()
	for _, want := range []string{"FAIL unsealed-cem-at-head count=1\n", "FAIL sealed-cem-drift count=1\n  bind=" + bindA + "\n"} {
		if code != 1 || !strings.Contains(stderr, want) {
			t.Fatalf("exit=%d missing %q in %s", code, want, stderr)
		}
	}
	// Once BASE contains the sealed CEM, editing it is a modification.
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD~1")
	repo.write(".corvint/changes/"+bindA+".cem.json", "{}\n")
	repo.commit("edit again")
	if code, _, stderr = repo.check(); code != 1 || !strings.Contains(stderr, "FAIL sealed-cem-modified count=1\n  .corvint/changes/"+bindA+".cem.json status=M\n") {
		t.Fatalf("modified exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-039: a constituent whose sealed CEM fails its own policy fails the
// batch; the bootstrap allowance is the whole-file no-evidence unknowns
// absent at the constituent's base, and each one is printed.
func TestBatchCheckDCWV0039BootstrapAllowanceIsVisible(t *testing.T) {
	repo := newBatchRepo(t)
	cem := func(base string) string {
		return `{"spec": "cem/0.3", "baseRevision": "` + base + `", "hunks": [` +
			`{"disposition": "unknown", "reason": "no-evidence", "path": "spec.md", "oldRange": {"count": 0, "start": 0}},` +
			`{"disposition": "unknown", "reason": "no-evidence", "path": "a.txt", "oldRange": {"count": 0, "start": 2}}]}` + "\n"
	}
	bind, seal := repo.lane("lane-a", "spec.md", "# new spec\n", cem(repo.base))
	repo.batch(seal)
	repo.status = 1
	code, stdout, stderr := repo.check()
	if !strings.Contains(stdout, " bootstrapUnknown=1 reasonedUnknown=0 state=incomplete\n  bootstrap-unknown spec.md\n") || repo.args[0][9] != "1" {
		t.Fatalf("bootstrap stdout=%s argv=%q", stdout, repo.args)
	}
	if code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-policy count=1\n  bind="+bind+" state=incomplete\n  fix: rebind and reseal") {
		t.Fatalf("policy exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-046 (V1-1258): a sealed CEM that kept hunks unknown with a recorded
// reason is admitted beside the bootstrap allowance and counted visibly; the
// prepared no-evidence hunk on an existing file is not reasoned and is not.
// The fake verifier applies --max-unknown as cem status does, so the pair
// shows the batch outcome, not only the count.
func TestBatchCheckDCWV0046ReasonedUnknownAllowance(t *testing.T) {
	hunk := func(reason string, start int) string {
		return fmt.Sprintf(`{"disposition": "unknown", "reason": "%s", "path": "a.txt", "oldRange": {"count": 1, "start": %d}}`, reason, start)
	}
	for _, tc := range []struct {
		name  string
		hunks []string
		code  int
		state string
	}{
		{"reasoned only", []string{hunk("insufficient-evidence", 1), hunk("conflicting-evidence", 2)}, 0, "ready-for-ci"},
		{"one unreasoned", []string{hunk("insufficient-evidence", 1), hunk("conflicting-evidence", 2), hunk("no-evidence", 3)}, 1, "incomplete"},
	} {
		repo := newBatchRepo(t)
		cem := `{"spec": "cem/0.3", "baseRevision": "` + repo.base + `", "hunks": [` + strings.Join(tc.hunks, ",") + "]}\n"
		_, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", cem)
		repo.batch(seal)
		unknown := len(tc.hunks)
		repo.output = func(base, bind string) string {
			allowed, _ := strconv.Atoi(repo.args[len(repo.args)-1][9])
			ready := unknown <= allowed
			repo.status = map[bool]int{true: 0, false: 1}[ready]
			state := map[bool]string{true: "ready-for-ci", false: "incomplete"}[ready]
			return fmt.Sprintf(`{"counts":{"mechanical":0,"supported":0,"total":%d,"unknown":%d},"ok":%t,"state":"%s","tool":"cem-status",`+
				`"verification":{"assurance":"canonical","baseRevision":"%s","targetRevision":"%s","valid":true}}`+"\n", unknown, unknown, ready, state, base, bind)
		}
		code, stdout, stderr := repo.check()
		if !strings.Contains(stdout, " bootstrapUnknown=0 reasonedUnknown=2 state="+tc.state+"\n") || repo.args[0][9] != "2" {
			t.Fatalf("%s: stdout=%s argv=%q", tc.name, stdout, repo.args)
		}
		if code != tc.code || (tc.code == 1) != strings.Contains(stderr, "FAIL constituent-cem-policy count=1\n") {
			t.Fatalf("%s: exit=%d stderr=%s", tc.name, code, stderr)
		}
	}
}

// DCW-V0-039: the bounds and preconditions refuse with exit 2 before any
// constituent is verified.
func TestBatchCheckDCWV0039Refusals(t *testing.T) {
	repo := newBatchRepo(t)
	if code, _, stderr := repo.check(); code != 2 || stderr != "dogfood-batch-check: REFUSE empty-range\n" {
		t.Fatalf("empty exit=%d stderr=%s", code, stderr)
	}
	_, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(seal)
	repo.write("untracked.txt", "x\n")
	if code, _, stderr := repo.check(); code != 2 || stderr != "dogfood-batch-check: REFUSE dirty-worktree\n" {
		t.Fatalf("dirty exit=%d stderr=%s", code, stderr)
	}
	if err := os.Remove(filepath.Join(repo.root, "untracked.txt")); err != nil {
		t.Fatal(err)
	}
	testGit(t, repo.root, "checkout", "-q", "-b", "other", repo.base)
	repo.write("b.txt", "other\n")
	repo.commit("other")
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	testGit(t, repo.root, "checkout", "-q", "batch")
	if code, _, stderr := repo.check(); code != 2 || stderr != "dogfood-batch-check: REFUSE base-not-ancestor-of-head\n" {
		t.Fatalf("not ancestor exit=%d stderr=%s", code, stderr)
	}
	repo.base = "no-such-revision"
	if code, _, stderr := repo.check(); code != 2 || stderr != "dogfood-batch-check: REFUSE base-unavailable\n" {
		t.Fatalf("unavailable exit=%d stderr=%s", code, stderr)
	}
	if len(repo.args) != 0 {
		t.Fatalf("refusal ran the verifier: %q", repo.args)
	}
}

// DCW-V0-040: each refusal of a change too large for the single loop names
// the split, seal and batch-check recovery, with BASE filled in, rather than
// a raised limit.
func TestChangeDCWV0040NamesBatchRecoveryForOversizedChanges(t *testing.T) {
	base := strings.Repeat("b", 40)
	c := &change{flow: flow{base: base}}
	for _, row := range []step{
		{name: "cem-prepare", reason: "git-budget-exceeded"},
		{name: "ocm-prepare-001", reason: "git-budget-exceeded"},
		{name: "local-outcome", reason: "admitted-path-limit"},
	} {
		if hint := c.fixHint(row); !strings.Contains(hint, "corvint dogfood batch-check "+base+" instead of rebinding it") {
			t.Fatalf("%s:%s hint %q", row.name, row.reason, hint)
		}
	}
}

// DCW-V0-039: only canonical evidence covers a constituent. A cem/0.1 map,
// whose verifier reads a cached patch, is refused before verification, and a
// zero exit without canonical ready-for-ci output of exactly this bind and
// base is not evidence.
func TestBatchCheckDCWV0039RequiresCanonicalVerification(t *testing.T) {
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", `{"spec": "cem/0.1", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	repo.batch(seal)
	if code, _, stderr := repo.check(); code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-invalid count=1\n  bind="+bind+"\n") || len(repo.args) != 0 {
		t.Fatalf("cem/0.1 exit=%d stderr=%s argv=%q", code, stderr, repo.args)
	}
	repo = newBatchRepo(t)
	bind, seal = repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(seal)
	counts := `"counts":{"supported":1,"total":1,"unknown":0},`
	canonical := func(state, assurance, target, counts string) string {
		return `{` + counts + `"ok":true,"state":"` + state + `","tool":"cem-status","verification":{"assurance":"` + assurance +
			`","baseRevision":"` + repo.base + `","targetRevision":"` + target + `","valid":true}}` + "\n"
	}
	// Control: the complete canonical response passes.
	repo.output = func(string, string) string { return canonical("ready-for-ci", "canonical", bind, counts) }
	if code, stdout, stderr := repo.check(); code != 0 || !strings.Contains(stdout, " hunks=1 supported=1 unknown=0 ") {
		t.Fatalf("control exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	for name, output := range map[string]string{
		"empty":        "",
		"malformed":    "not json\n",
		"incomplete":   canonical("incomplete", "canonical", bind, counts),
		"structural":   canonical("ready-for-ci", "structural", bind, counts),
		"other-target": canonical("ready-for-ci", "canonical", repo.base, counts),
		"no-counts":    canonical("ready-for-ci", "canonical", bind, ""),
	} {
		repo.output = func(string, string) string { return output }
		code, stdout, stderr := repo.check()
		if code != 1 || !strings.Contains(stderr, "FAIL constituent-unverified count=1\n  bind="+bind+" state=") || strings.Contains(stdout, "PASS") {
			t.Fatalf("%s exit=%d stdout=%s stderr=%s", name, code, stdout, stderr)
		}
		if (name == "empty" || name == "no-counts") && !strings.Contains(stdout, " hunks=NOT_OBSERVED supported=NOT_OBSERVED unknown=NOT_OBSERVED ") {
			t.Fatalf("invented counts: %s", stdout)
		}
	}
}

// DCW-V0-039: the check is read-only even against hostile repository
// configuration. A configured merge driver that would reproduce a manual
// resolution neither runs nor makes it derivable, and a configured fsmonitor
// hook never runs.
func TestBatchCheckDCWV0039IgnoresRepositoryDriversAndHooks(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write(".gitattributes", "a.txt merge=forge\n")
	repo.commit("attributes")
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	_, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "a.txt", "a1\nB2\na3\n", "")
	repo.batch(sealA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-X", "ours", sealB)
	repo.write("a.txt", "a1\nAB2\na3\n")
	testGit(t, repo.root, "add", "a.txt")
	testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	scratch := t.TempDir()
	driverRan, hookRan, hook := filepath.Join(scratch, "driver-ran"), filepath.Join(scratch, "hook-ran"), filepath.Join(scratch, "fsmonitor")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+hookRan+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	testGit(t, repo.root, "config", "merge.forge.driver", "printf 'a1\\nAB2\\na3\\n' > %A; touch '"+driverRan+"'")
	testGit(t, repo.root, "config", "core.fsmonitor", hook)
	code, _, stderr := repo.check()
	for _, marker := range []string{driverRan, hookRan} {
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("batch-check executed repository configuration %s: %v", marker, err)
		}
	}
	if code != 1 || !strings.Contains(stderr, "FAIL merge-resolution-unbound count=1\n  "+merge+" paths=a.txt\n") {
		t.Fatalf("driver-derived merge exit=%d stderr=%s", code, stderr)
	}
	entries, err := os.ReadDir(repo.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".merge_file_") {
			t.Fatalf("merge driver temporary file in the worktree: %s", entry.Name())
		}
	}
}

// DCW-V0-039: a root commit an unrelated history brings in is unbound
// content, reported rather than a panic.
func TestBatchCheckDCWV0039ReportsUnrelatedRootCommit(t *testing.T) {
	repo := newBatchRepo(t)
	_, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	testGit(t, repo.root, "checkout", "-q", "--orphan", "orphan")
	testGit(t, repo.root, "rm", "-rq", "--cached", ".")
	for _, path := range []string{"a.txt", "b.txt", ".corvint"} {
		if err := os.RemoveAll(filepath.Join(repo.root, path)); err != nil {
			t.Fatal(err)
		}
	}
	repo.write("orphan.txt", "orphan\n")
	root := repo.commit("orphan root")
	repo.batch(seal)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "--allow-unrelated-histories", root)
	code, _, stderr := repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL unbound-commits count=1\n  "+root+"\n") {
		t.Fatalf("root commit exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-039: the cleanliness probe is Core's hardened status, so a
// repository-configured clean filter never runs; the check refuses instead.
func TestBatchCheckDCWV0039RefusesRepositoryFilters(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write(".gitattributes", "a.txt filter=hostile\n")
	repo.commit("attributes")
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	_, seal := repo.lane("lane-a", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(seal)
	marker := filepath.Join(t.TempDir(), "filter-ran")
	testGit(t, repo.root, "config", "filter.hostile.clean", "touch '"+marker+"'; cat")
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(repo.root, "a.txt"), later, later); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := repo.check()
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("batch-check ran a repository clean filter: %v", err)
	}
	if code != 2 || !strings.HasPrefix(stderr, "dogfood-batch-check: REFUSE worktree-status-unavailable\n") || !strings.Contains(stderr, "  fix: ") || len(repo.args) != 0 {
		t.Fatalf("filter exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-039: a temporary directory inside the repository's Git directory is
// refused before any scratch file is created there.
func TestBatchCheckDCWV0039RefusesScratchInsideRepository(t *testing.T) {
	repo := newBatchRepo(t)
	_, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(seal)
	inside := filepath.Join(repo.root, ".git", "scratch")
	if err := os.Mkdir(inside, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", inside)
	code, _, stderr := repo.check()
	if code != 2 || stderr != "dogfood-batch-check: REFUSE scratch-inside-repository\n  fix: set TMPDIR to an existing directory outside the repository and its Git directory\n" {
		t.Fatalf("scratch exit=%d stderr=%s", code, stderr)
	}
	if entries, err := os.ReadDir(inside); err != nil || len(entries) != 0 {
		t.Fatalf("scratch written inside the repository: %v %v", entries, err)
	}
}
