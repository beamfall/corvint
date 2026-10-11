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

type batchRepo struct {
	t    *testing.T
	root string
	base string
	args [][]string
	// status is the fake verifier's exit status for every constituent.
	status int
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
		cem = `{"baseRevision": "` + r.base + `", "hunks": []}` + "\n"
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
	verifier := Runner{Path: "/never-executed/corvint", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		r.args = append(r.args, args)
		fmt.Fprintf(stdout, `{"counts":{"mechanical":0,"supported":1,"total":1,"unknown":0},"state":"%s"}`+"\n", map[bool]string{true: "ready-for-ci", false: "incomplete"}[r.status == 0])
		return r.status
	}}
	var stdout, stderr bytes.Buffer
	code, err := BatchCheck(context.Background(), BatchCheckOptions{Root: r.root, Base: r.base, Verifier: verifier}, &stdout, &stderr)
	if err != nil {
		r.t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
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
		"dogfood-batch-check: CONSTITUENT bind=" + bindA + " base=" + repo.base + " hunks=1 supported=1 unknown=0 bootstrapUnknown=0 state=ready-for-ci\n",
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
		return `{"baseRevision": "` + base + `", "hunks": [` +
			`{"disposition": "unknown", "reason": "no-evidence", "path": "spec.md", "oldRange": {"count": 0, "start": 0}},` +
			`{"disposition": "unknown", "reason": "no-evidence", "path": "a.txt", "oldRange": {"count": 0, "start": 2}}]}` + "\n"
	}
	bind, seal := repo.lane("lane-a", "spec.md", "# new spec\n", cem(repo.base))
	repo.batch(seal)
	repo.status = 1
	code, stdout, stderr := repo.check()
	if !strings.Contains(stdout, " bootstrapUnknown=1 state=incomplete\n  bootstrap-unknown spec.md\n") || repo.args[0][9] != "1" {
		t.Fatalf("bootstrap stdout=%s argv=%q", stdout, repo.args)
	}
	if code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-policy count=1\n  bind="+bind+" state=incomplete\n  fix: rebind and reseal") {
		t.Fatalf("policy exit=%d stderr=%s", code, stderr)
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
