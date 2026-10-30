package dogfoodflow

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
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
	// output, when set, replaces the fake verifier's stable status line.
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
	return r.laneFrom(name, r.base, path, data, cem)
}

// laneFrom is lane on a branch from, and bound against, the commit from.
func (r *batchRepo) laneFrom(name, from, path, data, cem string) (string, string) {
	r.t.Helper()
	testGit(r.t, r.root, "checkout", "-q", "-b", name, from)
	r.write(path, data)
	r.commit("work " + name)
	if cem == "" {
		cem = `{"spec": "cem/1.0", "baseRevision": "` + from + `", "hunks": []}` + "\n"
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

// verifier is the fake constituent verifier: the stable ACCEPT of the base and
// bind it is asked about, unless status or output say otherwise.
func (r *batchRepo) verifier() Runner {
	return Runner{Path: "/never-executed/corvint", Run: func(_ context.Context, _ string, args []string, stdout, _ io.Writer) int {
		r.args = append(r.args, args)
		base, bind := args[5], args[7]
		if r.output != nil {
			fmt.Fprint(stdout, r.output(base, bind))
			return r.status
		}
		fmt.Fprint(stdout, fakeStableStatus(r.status == 0, 1, 1, base, bind))
		return r.status
	}}
}

// fakeStableStatus is a stable `cem status` line: ACCEPT and accepted when ok,
// otherwise REJECT and incomplete.
func fakeStableStatus(ok bool, total, supported int, base, bind string) string {
	state, outcome := map[bool]string{true: "accepted", false: "incomplete"}[ok], map[bool]string{true: "ACCEPT", false: "REJECT"}[ok]
	return fmt.Sprintf(`{"counts":{"mechanical":0,"supported":%d,"total":%d,"unknown":%d},"ok":%t,"state":"%s","tool":"cem-status",`+
		`"stableVerification":{"accept":%t,"expectedBase":"%s","outcome":"%s","targetRevision":"%s"}}`+"\n",
		supported, total, total-supported, ok, state, ok, base, outcome, bind)
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
		"dogfood-batch-check: CONSTITUENT bind=" + bindA + " base=" + repo.base + " hunks=1 supported=1 unknown=0 bootstrapUnknown=0 reasonedUnknown=0 state=accepted\n",
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
	wantArgs := []string{"cem", "status", "--map", ".corvint/changes/" + bindA + ".cem.json", "--expected-base", repo.base, "--target", bindA, "--max-unknown", "0", "--max-mechanical", "0", "--artifacts", stableArtifactRoot(repo.root)}
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

// DCW-V0-039 and DCW-V0-073: a one-sided conflict resolution is derivable
// from the parents, but each hunk it drops is reported per file and hunk and
// fails until restored or dispositioned. V1-1786: the old check passed this
// merge with an aggregate count only.
func TestBatchCheckDCWV0073ReportsEachDroppedHunk(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write("c.txt", "c1\nc2\nc3\nc4\nc5\nc6\nc7\nc8\nc9\nc10\nc11\n")
	repo.base = repo.commit("third file")
	_, sealA := repo.lane("lane-a", "c.txt", "c1\nA2\nc3\nc4\nc5\nc6\nc7\nc8\nc9\nA10\nc11\n", "")
	_, sealB := repo.lane("lane-b", "c.txt", "c1\nB2\nB2b\nc3\nc4\nc5\nc6\nc7\nc8\nc9\nB10\nc11\n", "")
	repo.batch(sealA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-X", "ours", sealB)
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	before := repo.objects()
	code, stdout, stderr := repo.check()
	first := "path=c.txt hunk=1 kept=L2+1 dropped=2 side=" + sealB + " id=" + wantHunkID("c.txt", "A2\n", "B2\nB2b\n")
	second := "path=c.txt hunk=2 kept=L10+1 dropped=1 side=" + sealB + " id=" + wantHunkID("c.txt", "A10\n", "B10\n")
	if want := "dogfood-batch-check: MERGE " + merge + " derivation=ort-ours\n  dropped-hunk " + first + " coverage=none\n  dropped-hunk " +
		second + " coverage=none\n"; !strings.Contains(stdout, want) {
		t.Fatalf("exit=%d missing %q in %s", code, want, stdout)
	}
	if want := "dogfood-batch-check: FAIL merge-side-dropped count=2\n  " + merge + " " + first + "\n  " + merge + " " + second +
		"\n  fix: restore each listed hunk's dropped side"; code != 1 || !strings.Contains(stderr, want) {
		t.Fatalf("exit=%d missing %q in %s", code, want, stderr)
	}
	if after := repo.objects(); after != before {
		t.Fatalf("merge-tree wrote into the repository:\n%s", after)
	}

	// A disposition with a reason in the merge's message covers its hunk; one
	// without a reason does not.
	testGit(t, repo.root, "commit", "-q", "--amend", "-m", "merge lane-b\n\nCorvint-Dropped-Hunk: "+wantHunkID("c.txt", "A2\n", "B2\nB2b\n")+
		" lane A's line supersedes it\nCorvint-Dropped-Hunk: "+wantHunkID("c.txt", "A10\n", "B10\n")+"\n")
	merge = testGit(t, repo.root, "rev-parse", "HEAD")
	code, stdout, stderr = repo.check()
	if !strings.Contains(stdout, "  dropped-hunk "+first+" coverage=disposition\n") ||
		!strings.Contains(stderr, "FAIL merge-side-dropped count=1\n  "+merge+" "+second+"\n") || code != 1 {
		t.Fatalf("disposition: exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	// A sealed change based on the merge that restores the other dropped side
	// at HEAD covers it, and the batch passes with the tally.
	_, sealC := repo.laneFrom("lane-c", merge, "c.txt", "c1\nA2\nc3\nc4\nc5\nc6\nc7\nc8\nc9\nA10\nB10\nc11\n", "")
	testGit(t, repo.root, "checkout", "-q", "batch")
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", sealC)
	code, stdout, stderr = repo.check()
	if code != 0 || !strings.Contains(stdout, "  dropped-hunk "+second+" coverage=restored:") ||
		!strings.Contains(stdout, "dogfood-batch-check: NOTE conflict-side-dropped merges=1 hunks=2 restored=1 dispositioned=1\n") {
		t.Fatalf("exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
}

// wantHunkID is the DCW-V0-073 hunk id: the sha256 of the path, the kept lines
// and the dropped lines, NUL-separated.
func wantHunkID(path, kept, dropped string) string {
	sum := sha256.Sum256([]byte(path + "\x00" + kept + "\x00" + dropped))
	return hex.EncodeToString(sum[:])
}

// DCW-V0-073: a merge that keeps its first parent's tree over a second side
// Git would merge drops that whole side; it is UNKNOWN and fails.
func TestBatchCheckDCWV0073FirstParentTreeDropsSide(t *testing.T) {
	repo := newBatchRepo(t)
	_, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "b.txt", "b1\nB2\nb3\n", "")
	repo.batch(sealA)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-s", "ours", sealB)
	merge := testGit(t, repo.root, "rev-parse", "HEAD")
	code, stdout, stderr := repo.check()
	if want := "FAIL merge-side-dropped count=1\n  " + merge + " side=" + sealB + " hunk=UNKNOWN reason=second-side-discarded paths="; code != 1 ||
		!strings.Contains(stdout, "MERGE "+merge+" derivation=first-parent-tree\n") || !strings.Contains(stderr, want) {
		t.Fatalf("exit=%d missing %q stdout=%s stderr=%s", code, want, stdout, stderr)
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
		"dogfood-batch-check: FAIL merge-resolution-unbound count=1\n  " + merge + " paths=a.txt\n  fix: a hand-resolved merge is unbound: drop it, rebase the conflicting change's own work commits",
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

// DCW-V0-443 (V1-3104): one run lists every hand-resolved merge of the
// range, earliest first, with count equal to the merges listed, and names each
// merge's own rebind BASE and merged change, so all of them are recovered in
// one round. The per-merge and one-round fix lines fail on BASE.
func TestBatchCheckDCWV0443ListsEveryHandResolvedMerge(t *testing.T) {
	repo := newBatchRepo(t)
	_, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	_, sealB := repo.lane("lane-b", "a.txt", "a1\nB2\na3\n", "")
	_, sealC := repo.lane("lane-c", "b.txt", "b1\nC2\nb3\n", "")
	_, sealD := repo.lane("lane-d", "b.txt", "b1\nD2\nb3\n", "")
	handMerge := func(tip, path, data string) (string, string) {
		first := testGit(t, repo.root, "rev-parse", "HEAD")
		testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "-X", "ours", tip)
		repo.write(path, data)
		testGit(t, repo.root, "add", path)
		testGit(t, repo.root, "commit", "-q", "--amend", "--no-edit")
		return testGit(t, repo.root, "rev-parse", "HEAD"), first
	}
	repo.batch(sealA)
	early, earlyFirst := handMerge(sealB, "a.txt", "a1\nAB2\na3\n")
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", sealC)
	late, lateFirst := handMerge(sealD, "b.txt", "b1\nCD2\nb3\n")
	code, _, stderr := repo.check()
	want := "dogfood-batch-check: FAIL merge-resolution-unbound count=2\n  " + early + " paths=a.txt\n  " + late + " paths=b.txt\n" +
		"  fix: " + batchRecovery["merge-resolution-unbound"] + "\n" +
		"  fix: " + early + ": rebind the change merged from its second parent " + sealB + " with BASE=" + earlyFirst + " (its first parent), then remerge it there\n" +
		"  fix: " + late + ": rebind the change merged from its second parent " + sealD + " with BASE=" + lateFirst + " (its first parent), then remerge it there\n" +
		"  fix: recover all 2 listed merges in this round, in the listed order: redoing a merge changes the first parent of every later merge"
	if code != 1 || !strings.Contains(stderr, want) {
		t.Fatalf("exit=%d missing %q in %s", code, want, stderr)
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
		return `{"spec": "cem/1.0", "baseRevision": "` + base + `", "hunks": [` +
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
		{"reasoned only", []string{hunk("insufficient-evidence", 1), hunk("conflicting-evidence", 2)}, 0, "accepted"},
		{"one unreasoned", []string{hunk("insufficient-evidence", 1), hunk("conflicting-evidence", 2), hunk("no-evidence", 3)}, 1, "incomplete"},
	} {
		repo := newBatchRepo(t)
		cem := `{"spec": "cem/1.0", "baseRevision": "` + repo.base + `", "hunks": [` + strings.Join(tc.hunks, ",") + "]}\n"
		_, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", cem)
		repo.batch(seal)
		unknown := len(tc.hunks)
		repo.output = func(base, bind string) string {
			allowed, _ := strconv.Atoi(repo.args[len(repo.args)-1][9])
			ready := unknown <= allowed
			repo.status = map[bool]int{true: 0, false: 1}[ready]
			return fakeStableStatus(ready, unknown, 0, base, bind)
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

// DCW-V0-040 as amended by DCW-V0-373 and DCW-V0-374: a Git budget refusal
// names the scaled bound and the batch-check route, the legacy path refusal
// names the aggregate opt-in, and only the aggregate's own bounds route to
// splitting, with BASE filled in.
func TestChangeDCWV0040NamesBatchRecoveryForOversizedChanges(t *testing.T) {
	base := strings.Repeat("b", 40)
	c := &change{flow: flow{base: base}}
	for _, tc := range []struct {
		row  step
		want []string
	}{
		{step{name: "cem-prepare", reason: "git-budget-exceeded"}, []string{"DCW-V0-373", "corvint dogfood batch-check " + base}},
		{step{name: "ocm-prepare-001", reason: "git-budget-exceeded"}, []string{"DCW-V0-373", "corvint dogfood batch-check " + base}},
		{step{name: "local-outcome", reason: "admitted-path-limit"}, []string{"--aggregate-over-limit", "corvint dogfood change " + base}},
		{step{name: "local-outcome", reason: "aggregate-candidate-limit"}, []string{"4,096", "corvint dogfood batch-check " + base}},
		{step{name: "local-outcome", reason: "aggregate-receipt-byte-limit"}, []string{"4 MiB", "corvint dogfood batch-check " + base}},
	} {
		hint := c.fixHint(tc.row)
		for _, want := range tc.want {
			if !strings.Contains(hint, want) {
				t.Fatalf("%s:%s hint %q lacks %q", tc.row.name, tc.row.reason, hint, want)
			}
		}
	}
}

// DCW-V0-039, CEM-V1-202: only a stable ACCEPT covers a constituent. A zero
// exit without the accepted stable line of exactly this bind and base is not
// evidence, and neither is a 0.x ready-for-ci line.
func TestBatchCheckDCWV0039RequiresStableVerification(t *testing.T) {
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(seal)
	// Control: the complete stable response passes.
	repo.output = func(base, bind string) string { return fakeStableStatus(true, 1, 1, base, bind) }
	if code, stdout, stderr := repo.check(); code != 0 || !strings.Contains(stdout, " hunks=1 supported=1 unknown=0 ") {
		t.Fatalf("control exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}
	accepted := fakeStableStatus(true, 1, 1, repo.base, bind)
	for name, output := range map[string]string{
		"empty":        "",
		"malformed":    "not json\n",
		"incomplete":   strings.Replace(accepted, `"state":"accepted"`, `"state":"incomplete"`, 1),
		"reject":       strings.Replace(accepted, `"outcome":"ACCEPT"`, `"outcome":"REJECT"`, 1),
		"other-target": fakeStableStatus(true, 1, 1, repo.base, repo.base),
		"no-counts":    strings.Replace(accepted, `"counts":{"mechanical":0,"supported":1,"total":1,"unknown":0},`, "", 1),
		"legacy-line": `{"counts":{"supported":1,"total":1,"unknown":0},"ok":true,"state":"ready-for-ci","tool":"cem-status",` +
			`"verification":{"assurance":"canonical","baseRevision":"` + repo.base + `","targetRevision":"` + bind + `","valid":true}}` + "\n",
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

// CEM-V1-203: a retired cem/0.x seal that still equals its bind commit's
// sidecar is neither verified nor refused as invalid. batch-check notes it and
// gives it no coverage, so its commits fail unbound-commits until a verified
// cem/1.0 constituent's own range covers them; a branch that merged main and
// rebound with BASE before that seal passes. dogfood change admits it because
// the change binds the whole range itself.
func TestBatchCheckCEMV1203RetiredSealHasNoCoverage(t *testing.T) {
	for _, spec := range []string{"cem/0.1", "cem/0.2", "cem/0.3"} {
		repo := newBatchRepo(t)
		bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", `{"spec": "`+spec+`", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
		repo.batch(seal)
		code, stdout, stderr := repo.check()
		note := "dogfood-batch-check: NOTE sealed-cem-retired bind=" + bind + " spec=" + spec + " NOT_REVERIFIED coverage=none\n"
		if code != 1 || len(repo.args) != 0 || !strings.Contains(stdout, note) || !strings.Contains(stdout, "  fix: if its commits are unbound, rebind the branch") ||
			!strings.Contains(stderr, "FAIL unbound-commits count=3\n") || strings.Contains(stderr, "constituent-cem-invalid") {
			t.Fatalf("%s: exit=%d argv=%q stdout=%s stderr=%s", spec, code, repo.args, stdout, stderr)
		}
		if code, stderr := repo.admit(repo.base); code != 0 || len(repo.args) != 0 ||
			!strings.Contains(stderr, "dogfood-change: NOTE sealed-cem-retired bind="+bind+" spec="+spec+" NOT_REVERIFIED bound-by-this-change\n") {
			t.Fatalf("%s admit: exit=%d stderr=%s", spec, code, stderr)
		}
		// The lane merges main and rebinds as one cem/1.0 change whose base
		// precedes the retired seal: that constituent covers every commit.
		testGit(t, repo.root, "checkout", "-q", "lane-a")
		repo.write("a.txt", "a1\nA2\nA3\n")
		repo.commit("rework lane-a")
		repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
		rebind := repo.commit("chore: bind change evidence")
		testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+rebind+".cem.json")
		testGit(t, repo.root, "commit", "-q", "-m", "chore: seal change evidence")
		testGit(t, repo.root, "checkout", "-q", "batch")
		testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "lane-a")
		code, stdout, stderr = repo.check()
		if code != 0 || len(repo.args) != 1 || repo.args[0][7] != rebind || !strings.Contains(stdout, note) || !strings.Contains(stdout, " constituents=1 ") {
			t.Fatalf("%s rebound: exit=%d argv=%q stdout=%s stderr=%s", spec, code, repo.args, stdout, stderr)
		}
	}
	// A sealed map that declares no spec, or a malformed one, stays invalid.
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", `{"spec": "cem/0.4", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	repo.batch(seal)
	if code, _, stderr := repo.check(); code != 1 || !strings.Contains(stderr, "FAIL constituent-cem-invalid count=1\n  bind="+bind+"\n") || len(repo.args) != 0 {
		t.Fatalf("cem/0.4 exit=%d stderr=%s argv=%q", code, stderr, repo.args)
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

// DCW-V0-110 (V1-1463): a lane that merged the mainline forward past an
// unbound mainline commit still fails unbound-commits against its original
// BASE, and the failure names the merged mainline commit as the BASE that
// passes; checked from that BASE every commit of the lane is still checked. A
// commit made on the lane itself gets no such line.
func TestBatchCheckDCWV0110NamesTheMergedForwardBase(t *testing.T) {
	repo := newBatchRepo(t)
	original := repo.base
	testGit(t, repo.root, "checkout", "-q", "main")
	repo.write("b.txt", "b1\nb2\nb3\npolicy\n")
	carry := repo.commit("chore(tasks): carry policy to main")
	repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", "main")
	forward := testGit(t, repo.root, "rev-parse", "HEAD")
	code, stdout, stderr := repo.check()
	want := "dogfood-batch-check: FAIL unbound-commits count=1\n  " + carry + "\n  fix: bind each listed commit through a sealed change"
	hint := "  fix: every listed commit came in through merge " + forward + " from its merged parent " + carry +
		"; if that parent is a mainline commit (git merge-base --is-ancestor " + carry + " origin/main), its commits are the mainline's to check: rerun batch-check with BASE=" + carry
	if code != 1 || !strings.Contains(stderr, want) || !strings.Contains(stderr, hint) || strings.Contains(stdout, "PASS") {
		t.Fatalf("original base exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	repo.base = carry
	code, stdout, stderr = repo.check()
	if code != 0 || stderr != "" || !strings.Contains(stdout, "dogfood-batch-check: PASS constituents=1 commits=4\n") {
		t.Fatalf("merged-forward base exit=%d stdout=%s stderr=%s", code, stdout, stderr)
	}

	// The lane's own unbound commit fails from the merged-forward BASE too,
	// with no merged-parent line: no merge brought it in.
	repo.write("a.txt", "a1\nA2\na3\nlane fix-up\n")
	fixup := repo.commit("lane fix-up")
	code, _, stderr = repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL unbound-commits count=1\n  "+fixup+"\n") || strings.Contains(stderr, "came in through merge") {
		t.Fatalf("lane fix-up exit=%d stderr=%s", code, stderr)
	}
	repo.base = original
	code, _, stderr = repo.check()
	if code != 1 || !strings.Contains(stderr, "FAIL unbound-commits count=2\n") || strings.Contains(stderr, "came in through merge") {
		t.Fatalf("mixed unbound exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-111 (V1-1927): a constituent the verifier reports unsupported
// carries the verifier's reason code on its CONSTITUENT and failure lines, and
// a reason code outside the sealed change, such as an alternates-backed clone,
// prints its own fix line after the failure's.
func TestBatchCheckDCWV0111NamesTheVerifierReasonCode(t *testing.T) {
	repo := newBatchRepo(t)
	bind, seal := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	repo.batch(seal)
	fix := "  fix: unsupported-object-alternates: the clone borrows objects through .git/objects/info/alternates (git clone --reference or --shared), not a fault of the sealed change"
	for name, output := range map[string]string{
		"stdout": `{"mutates":false,"ok":false,"state":"unsupported","tool":"cem-status","stableVerification":{"code":"unsupported-object-alternates","outcome":"UNSUPPORTED","stage":"repository"}}` + "\n",
		"stderr": "",
	} {
		repo.status = 2
		repo.output = func(string, string) string { return output }
		verifier := repo.verifier()
		if name == "stderr" {
			// The refusal reaches stderr alone: its code becomes the state.
			verifier.Run = func(_ context.Context, _ string, _ []string, _, stderr io.Writer) int {
				fmt.Fprint(stderr, `{"code":"unsupported-object-alternates","ok":false}`+"\n")
				return 2
			}
		}
		var stdout, stderr bytes.Buffer
		code, err := BatchCheck(context.Background(), BatchCheckOptions{Root: repo.root, Base: repo.base, Verifier: verifier}, &stdout, &stderr)
		if err != nil {
			t.Fatal(err)
		}
		wantLine, wantDetail := " state=unsupported code=unsupported-object-alternates\n", "  bind="+bind+" state=unsupported code=unsupported-object-alternates\n"
		if name == "stderr" {
			wantLine, wantDetail = " state=unsupported-object-alternates\n", "  bind="+bind+" state=unsupported-object-alternates\n"
		}
		if code != 1 || !strings.Contains(stdout.String(), "CONSTITUENT bind="+bind) || !strings.Contains(stdout.String(), wantLine) ||
			!strings.Contains(stderr.String(), "FAIL constituent-cem-policy count=1\n"+wantDetail+"  fix: rebind and reseal") ||
			!strings.Contains(stderr.String(), fix) || strings.Count(stderr.String(), "  fix: unsupported-object-alternates") != 1 {
			t.Fatalf("%s exit=%d stdout=%s stderr=%s", name, code, stdout.String(), stderr.String())
		}
	}

	// A code with no recovery of its own, or one shaped like anything but a
	// reason code, adds no fix line, and a malformed code is not repeated.
	for code, shown := range map[string]bool{"unproven-mechanical": true, "Bad Code\n": false} {
		encoded, _ := json.Marshal(code)
		repo.output = func(string, string) string {
			return `{"ok":false,"state":"rejected","tool":"cem-status","stableVerification":{"code":` + string(encoded) + `,"outcome":"REJECT"}}` + "\n"
		}
		repo.status = 1
		exit, _, stderr := repo.check()
		if exit != 1 || strings.Contains(stderr, "  fix: "+code) || strings.Contains(stderr, " code=") != shown {
			t.Fatalf("%q exit=%d stderr=%s", code, exit, stderr)
		}
	}
}

// DCW-V0-039 (V1-1213): the scratch repository that derives merges reads no
// system attributes and no configured attributes file. Every scratch command
// carries GIT_ATTR_NOSYSTEM=1 (the system file cannot be redirected in a test,
// so its environment is asserted), and every repository read overrides
// core.attributesFile, shown by an attributes file that a configured
// scratch repository would otherwise apply.
func TestBatchCheckDCWV0039ScratchIgnoresSystemAndConfiguredAttributes(t *testing.T) {
	scratch := t.TempDir()
	b := &batch{flow: flow{ctx: context.Background(), prefix: "dogfood-batch-check", stderr: io.Discard}, scratch: scratch}
	environment := b.scratchCommand("version").Env
	for _, want := range []string{"GIT_ATTR_NOSYSTEM=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull} {
		if !slices.Contains(environment, want) {
			t.Fatalf("scratch command environment lacks %s: %q", want, environment)
		}
	}
	if _, status := b.runScratch("init", "-q", "--bare", "--template=", "repository.git"); status != 0 {
		t.Fatalf("scratch init status %d", status)
	}
	attributes := filepath.Join(scratch, "attributes")
	if err := os.WriteFile(attributes, []byte("* merge=forge filter=hostile\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	testGit(t, filepath.Join(scratch, "repository.git"), "config", "core.attributesFile", attributes)
	if output, status := b.runScratch("--git-dir=repository.git", "check-attr", "merge", "filter", "--", "a.txt"); status != 0 ||
		output != "a.txt: merge: forge\na.txt: filter: hostile\n" {
		t.Fatalf("control: configured attributes not applied without the override: status=%d %q", status, output)
	}
	output, status, bounded := b.readScratch(true, "check-attr", "merge", "filter", "--", "a.txt")
	if status != 0 || !bounded || output != "a.txt: merge: unspecified\na.txt: filter: unspecified\n" {
		t.Fatalf("repository read applied a configured attributes file: status=%d bounded=%v %q", status, bounded, output)
	}
}

// DCW-V0-113: a lane stacked on a sealed predecessor binds against the
// predecessor's seal commit, so only its own hunks bind and batch-check still
// verifies both; bound at the shared base instead, an aggregate bound refusal
// names the merged sealed change and that route.
func TestChangeDCWV0113StackedPredecessorBindsAgainstItsSeal(t *testing.T) {
	repo := newBatchRepo(t)
	bindA, sealA := repo.lane("lane-a", "a.txt", "a1\nA2\na3\n", "")
	testGit(t, repo.root, "checkout", "-q", "-b", "lane-b", repo.base)
	testGit(t, repo.root, "merge", "-q", "--no-ff", "--no-edit", sealA)
	repo.write("b.txt", "b1\nB2\nb3\n")
	repo.commit("work lane-b")

	newChange := func(base string) (*change, *bytes.Buffer) {
		var stderr bytes.Buffer
		c := &change{flow: flow{ctx: context.Background(), prefix: "dogfood-change", root: repo.root, stderr: &stderr}, options: ChangeOptions{Steps: repo.verifier()}}
		c.base, c.target = base, testGit(t, repo.root, "rev-parse", "HEAD")
		if _, err := boundary(c.ctx, func() int { c.admitSealedCEMs(c.inheritedCEM()); return 0 }, func() {}); err != nil {
			t.Fatal(err)
		}
		return c, &stderr
	}
	bound := step{"ocm-aggregate", "NOT_PRODUCED", "too-many-obligations"}

	// Bound at the shared base, the refusal names the stacked cause.
	c, stderr := newChange(repo.base)
	hint := c.fixHint(bound)
	for _, want := range []string{"stacked-predecessor: BASE " + repo.base + "..HEAD also merges 1 sealed change(s) (bind=" + bindA + " seal=" + sealA + ")", "BASE=<seal>", "batch-check", "DCW-V0-113"} {
		if !strings.Contains(hint, want) {
			t.Fatalf("hint %q lacks %q (stderr %q)", hint, want, stderr.String())
		}
	}
	for _, reason := range []string{"map-too-large", "too-many-claims"} {
		if got := c.fixHint(step{"ocm-aggregate", "NOT_PRODUCED", reason}); !strings.HasPrefix(got, "stacked-predecessor: ") {
			t.Fatalf("%s hint %q", reason, got)
		}
	}
	if got := c.fixHint(step{"ocm-aggregate", "NOT_PRODUCED", "intent-scope-drift"}); strings.Contains(got, "stacked-predecessor") {
		t.Fatalf("unrelated reason named the stacked cause: %q", got)
	}

	// Bound at the predecessor's seal, nothing sealed is in the range, the
	// verifier is not asked, and only this lane's path changes.
	repo.args = nil
	c, stderr = newChange(sealA)
	if len(c.mergedSeals) != 0 || len(repo.args) != 0 || strings.Contains(stderr.String(), "sealed-cem") {
		t.Fatalf("stacked bind admitted %v verifier=%v stderr=%q", c.mergedSeals, repo.args, stderr.String())
	}
	if got := c.fixHint(bound); strings.Contains(got, "stacked-predecessor") {
		t.Fatalf("no merged seal but hint %q", got)
	}
	if paths := testGit(t, repo.root, "diff", "--name-only", sealA, "HEAD"); paths != "b.txt" {
		t.Fatalf("stacked range paths %q", paths)
	}

	// Sealed against the predecessor's seal, the merge batch passes and
	// verifies each change against its own base.
	repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+sealA+`", "hunks": []}`+"\n")
	bindB := repo.commit("chore: bind change evidence")
	testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bindB+".cem.json")
	sealB := repo.commit("chore: seal change evidence")
	repo.batch(sealB)
	code, stdout, stderrText := repo.check()
	verified := map[string]string{}
	for _, args := range repo.args {
		verified[args[7]] = args[5]
	}
	if code != 0 || verified[bindA] != repo.base || verified[bindB] != sealA {
		t.Fatalf("batch exit=%d verified=%v stdout=%s stderr=%s", code, verified, stdout, stderrText)
	}
}
