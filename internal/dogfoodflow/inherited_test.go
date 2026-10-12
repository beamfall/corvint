package dogfoodflow

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newInheritedRepo is a repository that seals, whose base still tracks an
// unsealed CEM, as main did after the w4 batch merge (V1-1198). It returns
// the commit that wrote that CEM.
func newInheritedRepo(t *testing.T) (*batchRepo, string) {
	t.Helper()
	repo := newBatchRepo(t)
	repo.write(".corvint/changes/"+strings.Repeat("e", 40)+".cem.json", "{}\n")
	repo.commit("an earlier sealed change")
	repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	writer := repo.commit("chore: bind change evidence, never sealed")
	repo.base = writer
	return repo, writer
}

// archive runs the exact recovery command a refusal printed.
func (r *batchRepo) archive(stderr string) {
	r.t.Helper()
	_, fix, found := strings.Cut(stderr, "  fix: ")
	fix, _, _ = strings.Cut(fix, "\n")
	if !found || !strings.HasPrefix(fix, "mkdir -p .corvint/changes && git cat-file blob ") {
		r.t.Fatalf("no archive command in %q", stderr)
	}
	command := exec.Command("sh", "-c", fix)
	command.Dir = r.root
	command.Env = append(command.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	if output, err := command.CombinedOutput(); err != nil {
		r.t.Fatalf("%s: %v\n%s", fix, err, output)
	}
}

// DCW-V0-043: dogfood change refuses, before any step, a change that would
// replace BASE's unsealed CEM without archiving it, and names the exact
// command; after that command the change is admitted with the archive noted,
// even while the worktree's sidecar already holds a newer map.
func TestChangeDCWV0043RefusesUnarchivedInheritedCEM(t *testing.T) {
	repo, writer := newInheritedRepo(t)
	testGit(t, repo.root, "checkout", "-q", "-b", "lane", repo.base)
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("work")
	code, stderr := repo.admit(repo.base)
	blob := testGit(t, repo.root, "rev-parse", repo.base+":.corvint/change.cem.json")
	archive := ".corvint/changes/" + writer + ".cem.json"
	want := "dogfood-change: REFUSE unarchived-base-cem\n" +
		"  BASE tracks an unsealed .corvint/change.cem.json (last written by " + writer + ") that no .corvint/changes/ file at HEAD keeps; this change replaces it, so dogfood-seal would refuse\n" +
		"  fix: mkdir -p .corvint/changes && git cat-file blob " + blob + " > " + archive + " && git add " + archive +
		" && git commit -q -m \"chore: archive inherited change evidence\" -- " + archive + "\n" +
		"  then rerun dogfood-change with the same BASE; the plan cites the archived file's hunk (DCW-V0-043)\n"
	if code != 2 || stderr != want {
		t.Fatalf("exit=%d stderr=%q\nwant %q", code, stderr, want)
	}
	repo.write(".corvint/change.cem.json", "a newer prepared map\n")
	repo.archive(stderr)
	if status := testGit(t, repo.root, "status", "--porcelain"); status != "M .corvint/change.cem.json" {
		t.Fatalf("archive committed more than its file: %q", status)
	}
	if code, stderr = repo.admit(repo.base); code != 0 || stderr != "dogfood-change: NOTE inherited-cem-archived NOT_REVERIFIED path="+archive+"\n" || len(repo.args) != 0 {
		t.Fatalf("archived exit=%d stderr=%q verifier=%v", code, stderr, repo.args)
	}
}

// DCW-V0-042, DCW-V0-043: the full command refuses before it clears the
// previous run's step evidence, so the refusal destroys nothing.
func TestChangeDCWV0043RefusalKeepsPreviousEvidence(t *testing.T) {
	repo, _ := newInheritedRepo(t)
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("work")
	previous := filepath.Join(repo.root, ".git", "corvint", "coordination-query.stderr")
	if err := os.MkdirAll(filepath.Dir(previous), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(previous, []byte("previous run\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code, err := Change(context.Background(), ChangeOptions{Root: repo.root, Base: repo.base, Steps: repo.verifier()}, &stderr)
	if err != nil || code != 2 || !strings.HasPrefix(stderr.String(), "dogfood-change: REFUSE unarchived-base-cem\n") {
		t.Fatalf("exit=%d err=%v stderr=%q", code, err, stderr.String())
	}
	if data, err := os.ReadFile(previous); err != nil || string(data) != "previous run\n" {
		t.Fatalf("previous evidence lost: %q %v", data, err)
	}
}

// DCW-V0-043: a repository that never sealed keeps rebinding its tracked CEM
// in place.
func TestChangeDCWV0043IgnoresRepositoriesThatNeverSealed(t *testing.T) {
	repo := newBatchRepo(t)
	repo.write(".corvint/change.cem.json", "{}\n")
	repo.base = repo.commit("bind in place")
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("work")
	if code, stderr := repo.admit(repo.base); code != 0 || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
}

// DCW-V0-043: batch-check accepts BASE's inherited CEM archived by a lane,
// whether the archive commit lies in a constituent's range or before it, and
// still fails a batch that drops it unarchived.
func TestBatchCheckDCWV0043AcceptsTheInheritedArchive(t *testing.T) {
	repo, writer := newInheritedRepo(t)
	archive := ".corvint/changes/" + writer + ".cem.json"
	// Lane A archives first and binds against the original base.
	testGit(t, repo.root, "checkout", "-q", "-b", "lane-a", repo.base)
	_, stderr := repo.admit(repo.base)
	repo.archive(stderr)
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("work lane-a")
	repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+" \n")
	bindA := repo.commit("chore: bind change evidence")
	testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bindA+".cem.json")
	sealA := repo.commit("chore: seal change evidence")
	// Lane B renames the inherited CEM into the archive, then binds against
	// that archive commit, as the w6 lane did.
	testGit(t, repo.root, "checkout", "-q", "-b", "lane-b", repo.base)
	testGit(t, repo.root, "mv", ".corvint/change.cem.json", archive)
	archiveB := repo.commit("chore: archive inherited change evidence")
	repo.write("b.txt", "b1\nB2\nb3\n")
	repo.commit("work lane-b")
	repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+archiveB+`", "hunks": []}`+"\n")
	bindB := repo.commit("chore: bind change evidence")
	testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bindB+".cem.json")
	sealB := repo.commit("chore: seal change evidence")
	repo.batch(sealA, sealB)
	code, stdout, stderr := repo.check()
	if code != 0 || stderr != "" || !strings.Contains(stdout, "dogfood-batch-check: NOTE inherited-cem-archived NOT_REVERIFIED path="+archive+"\n") ||
		!strings.HasSuffix(stdout, "PASS constituents=2 commits=10\n") || len(repo.args) != 2 {
		t.Fatalf("exit=%d stdout=%s stderr=%s verifier=%v", code, stdout, stderr, repo.args)
	}
	// A batch whose lane drops the inherited CEM without archiving it fails.
	testGit(t, repo.root, "checkout", "-q", "-b", "lane-c", repo.base)
	testGit(t, repo.root, "rm", "-q", ".corvint/change.cem.json")
	repo.commit("drop the inherited CEM")
	if code, _, stderr = repo.check(); code != 1 || !strings.Contains(stderr, "FAIL unsealed-cem-at-head count=1\n") {
		t.Fatalf("dropped exit=%d stderr=%s", code, stderr)
	}
}

// DCW-V0-043: once BASE itself keeps the inherited CEM's archive, a later
// sealed lane that replaces the sidecar passes batch-check.
func TestBatchCheckDCWV0043AcceptsAnArchiveAlreadyAtBase(t *testing.T) {
	repo, writer := newInheritedRepo(t)
	_, stderr := repo.admit(repo.base)
	repo.archive(stderr)
	repo.base = testGit(t, repo.root, "rev-parse", "HEAD")
	testGit(t, repo.root, "checkout", "-q", "-b", "lane", repo.base)
	repo.write("a.txt", "a1\nA2\na3\n")
	repo.commit("work lane")
	repo.write(".corvint/change.cem.json", `{"spec": "cem/1.0", "baseRevision": "`+repo.base+`", "hunks": []}`+"\n")
	bind := repo.commit("chore: bind change evidence")
	testGit(t, repo.root, "mv", ".corvint/change.cem.json", ".corvint/changes/"+bind+".cem.json")
	seal := repo.commit("chore: seal change evidence")
	repo.batch(seal)
	code, stdout, stderr := repo.check()
	if code != 0 || stderr != "" || !strings.Contains(stdout, "NOTE inherited-cem-archived NOT_REVERIFIED path=.corvint/changes/"+writer+".cem.json\n") ||
		!strings.HasSuffix(stdout, "PASS constituents=1 commits=4\n") || len(repo.args) != 1 {
		t.Fatalf("exit=%d stdout=%s stderr=%s verifier=%v", code, stdout, stderr, repo.args)
	}
}
