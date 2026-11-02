package store_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
)

// sccGit runs git in dir with a fixed identity and clock and returns its
// trimmed stdout.
func sccGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL=a@example.test", "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@example.test",
		"GIT_AUTHOR_DATE=2026-10-19T00:00:00Z", "GIT_COMMITTER_DATE=2026-10-19T00:00:00Z")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// sccCommit writes name=content and commits it with subject.
func sccCommit(t *testing.T, dir, name, content, subject string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sccGit(t, dir, "add", "--", name)
	sccGit(t, dir, "commit", "-q", "-m", subject)
	return sccGit(t, dir, "rev-parse", "HEAD")
}

// sccRepo is a repository with main at base plus one commit, and a feature
// branch from base carrying commit X (feature.txt).
func sccRepo(t *testing.T) (dir, base, x string) {
	t.Helper()
	dir = t.TempDir()
	sccGit(t, dir, "init", "-q", "-b", "main")
	base = sccCommit(t, dir, "base.txt", "base\n", "base")
	sccGit(t, dir, "checkout", "-q", "-b", "feature")
	x = sccCommit(t, dir, "feature.txt", "one\ntwo\nthree\n", "add feature")
	sccGit(t, dir, "checkout", "-q", "main")
	sccCommit(t, dir, "other.txt", "other\n", "other work")
	return dir, base, x
}

func TestSCCV0001Ancestor(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccGit(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature", "feature")
	got := store.SourceContainmentOf(dir, "refs/heads/main", x)
	if got.State != store.SourceAncestor || got.Commit != x {
		t.Fatalf("merged feature commit: %+v, want ANCESTOR", got)
	}
}

func TestSCCV0001CherryPickIsEquivalent(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccGit(t, dir, "cherry-pick", x)
	picked := sccGit(t, dir, "rev-parse", "HEAD")
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceEquivalent || got.Equivalent != picked {
		t.Fatalf("cherry-picked commit: %+v, want EQUIVALENT %s", got, picked)
	}
}

func TestSCCV0001RebaseIsEquivalent(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccGit(t, dir, "checkout", "-q", "-b", "rebased", "feature")
	sccGit(t, dir, "rebase", "-q", "main")
	rebased := sccGit(t, dir, "rev-parse", "HEAD")
	sccGit(t, dir, "checkout", "-q", "main")
	sccGit(t, dir, "merge", "-q", "--ff-only", "rebased")
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceEquivalent || got.Equivalent != rebased || rebased == x {
		t.Fatalf("rebased commit: %+v, want EQUIVALENT %s", got, rebased)
	}
}

func TestSCCV0002ConflictResolvedRebaseIsPatchDiffers(t *testing.T) {
	dir, _, x := sccRepo(t)
	// main changes the same file first, so the rebased patch differs.
	sccCommit(t, dir, "feature.txt", "zero\n", "main writes feature.txt")
	sccGit(t, dir, "checkout", "-q", "-b", "resolved", "main")
	resolved := sccCommit(t, dir, "feature.txt", "zero\none\ntwo\nthree\n", "add feature")
	sccGit(t, dir, "checkout", "-q", "main")
	sccGit(t, dir, "merge", "-q", "--ff-only", "resolved")
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceAbsent || got.Reason != store.SourceReasonPatchDiffers || got.PatchDiffers != resolved {
		t.Fatalf("conflict-resolved rebase: %+v, want ABSENT PATCH_DIFFERS %s", got, resolved)
	}
}

func TestSCCV0002PlainAbsent(t *testing.T) {
	dir, _, x := sccRepo(t)
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceAbsent || got.Reason != store.SourceReasonNoEquivalent || got.PatchDiffers != "" {
		t.Fatalf("unmerged commit: %+v, want ABSENT NO_EQUIVALENT", got)
	}
}

func TestSCCV0002ChangeIDMatchesPatchDiffers(t *testing.T) {
	dir, _, _ := sccRepo(t)
	sccGit(t, dir, "checkout", "-q", "feature")
	sccCommit(t, dir, "g.txt", "g1\n", "first wording\n\nChange-Id: I0123abcd")
	y := sccGit(t, dir, "rev-parse", "HEAD")
	sccGit(t, dir, "checkout", "-q", "main")
	other := sccCommit(t, dir, "g.txt", "g2\n", "second wording\n\nChange-Id: I0123abcd")
	got := store.SourceContainmentOf(dir, "main", y)
	if got.State != store.SourceAbsent || got.Reason != store.SourceReasonPatchDiffers || got.PatchDiffers != other {
		t.Fatalf("same Change-Id, different patch: %+v, want PATCH_DIFFERS %s", got, other)
	}
}

func TestSCCV0002MergeCommitIsAbsentOnlyByAncestry(t *testing.T) {
	dir, _, _ := sccRepo(t)
	sccGit(t, dir, "checkout", "-q", "-b", "side", "main")
	sccGit(t, dir, "merge", "-q", "--no-ff", "-m", "merge feature into side", "feature")
	m := sccGit(t, dir, "rev-parse", "HEAD")
	sccGit(t, dir, "checkout", "-q", "main")
	got := store.SourceContainmentOf(dir, "main", m)
	if got.State != store.SourceAbsent || got.Reason != store.SourceReasonMergeCommit {
		t.Fatalf("merge commit off main: %+v, want ABSENT MERGE_COMMIT", got)
	}
	sccGit(t, dir, "merge", "-q", "--ff-only", "side")
	if got = store.SourceContainmentOf(dir, "main", m); got.State != store.SourceAncestor {
		t.Fatalf("merge commit merged: %+v, want ANCESTOR", got)
	}
}

func TestSCCV0003ScanBoundIsUnknown(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccGit(t, dir, "cherry-pick", x)
	defer store.SetSourceContainmentScanBoundForTest(1)()
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceUnknown || got.Reason != store.SourceReasonScanBound {
		t.Fatalf("over the scan bound: %+v, want UNKNOWN SCAN_BOUND", got)
	}
}

// SCC-V0-003: the deadline covers the whole answer; once it passes the
// answer is UNKNOWN GIT_FAILED, never a missing ref.
func TestSCCV0003DeadlineIsUnknown(t *testing.T) {
	dir, _, x := sccRepo(t)
	defer store.SetSourceContainmentTimeoutForTest(time.Nanosecond)()
	got := store.SourceContainmentOf(dir, "main", x)
	if got.State != store.SourceUnknown || got.Reason != store.SourceReasonGitFailed || !strings.Contains(got.Detail, "deadline") {
		t.Fatalf("past the deadline: %+v, want UNKNOWN GIT_FAILED naming the deadline", got)
	}
}

// SCC-V0-003: the deadline bounds every Git command of the answer, not only
// the patch-id log: a commit count that hangs is cut off at the deadline.
func TestSCCV0003DeadlineCoversEveryCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the hanging git wrapper is a POSIX shell script")
	}
	dir, _, x := sccRepo(t)
	real, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	wrapper := "#!/bin/sh\ncase \"$*\" in *--count*) exec sleep 30;; esac\nexec '" + real + "' \"$@\"\n"
	if err := fixture.WriteExecutable(filepath.Join(bin, "git"), []byte(wrapper), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	defer store.SetSourceContainmentTimeoutForTest(3 * time.Second)()
	start := time.Now()
	got := store.SourceContainmentOf(dir, "main", x)
	if elapsed := time.Since(start); elapsed > 20*time.Second {
		t.Fatalf("the answer took %v; the deadline did not cut off the hanging count", elapsed)
	}
	if got.State != store.SourceUnknown || got.Reason != store.SourceReasonGitFailed || !strings.Contains(got.Detail, "deadline") {
		t.Fatalf("hanging count: %+v, want UNKNOWN GIT_FAILED naming the deadline", got)
	}
}

// SCC-V0-003: a repository's log.showSignature never starts its signature
// verifier: the patch-id scan of a signed commit runs no gpg.program.
func TestSCCV0003SignedCommitStartsNoVerifier(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the marker verifier is a POSIX shell script")
	}
	dir, _, x := sccRepo(t)
	raw := sccGit(t, dir, "cat-file", "commit", x)
	head, msg, _ := strings.Cut(raw, "\n\n")
	signed := head + "\ngpgsig -----BEGIN PGP SIGNATURE-----\n \n AAAA\n -----END PGP SIGNATURE-----\n\n" + msg + "\n"
	c := exec.Command("git", "hash-object", "-t", "commit", "-w", "--stdin")
	c.Dir, c.Stdin = dir, strings.NewReader(signed)
	out, err := c.Output()
	if err != nil {
		t.Fatal(err)
	}
	xs := strings.TrimSpace(string(out))
	marker := filepath.Join(t.TempDir(), "verifier-ran")
	verifier := filepath.Join(t.TempDir(), "gpg")
	if err := fixture.WriteExecutable(verifier, []byte("#!/bin/sh\ntouch '"+marker+"'\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	sccGit(t, dir, "config", "log.showSignature", "true")
	sccGit(t, dir, "config", "gpg.program", verifier)
	if got := store.SourceContainmentOf(dir, "main", xs); got.State != store.SourceAbsent {
		t.Fatalf("signed commit: %+v, want ABSENT", got)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the containment scan started the repository's signature verifier")
	}
}

func TestSCCV0004MissingRefsAbstain(t *testing.T) {
	dir, _, x := sccRepo(t)
	if got := store.SourceContainmentOf(dir, "refs/heads/absent", x); got.State != store.SourceUnknown || got.Reason != store.SourceReasonMissingSource {
		t.Fatalf("missing source: %+v", got)
	}
	if got := store.SourceContainmentOf(dir, "main", strings.Repeat("e", 40)); got.State != store.SourceUnknown || got.Reason != store.SourceReasonMissingCommit {
		t.Fatalf("missing commit: %+v", got)
	}
}

func TestSCCV0004ShallowHistoryAbstains(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccGit(t, dir, "cherry-pick", x)
	clone := filepath.Join(t.TempDir(), "shallow")
	sccGit(t, dir, "clone", "-q", "--depth", "1", "--no-single-branch", "file://"+dir, clone)
	got := store.SourceContainmentOf(clone, "refs/remotes/origin/main", "refs/remotes/origin/feature")
	if got.State != store.SourceUnknown || got.Reason != store.SourceReasonShallowHistory {
		t.Fatalf("shallow clone: %+v, want UNKNOWN SHALLOW_HISTORY", got)
	}
}

func TestSCCV0011HoldConditionEvaluation(t *testing.T) {
	dir, _, x := sccRepo(t)
	sccCommit(t, dir, "pkg/observer.go", "package pkg\n\n// Observer watches jobs.\ntype JobObserver struct{}\n\nfunc (o *JobObserver) Watch() {}\n", "add observer")
	commit := func(c string) *ticket.HoldCondition {
		return &ticket.HoldCondition{Kind: ticket.HoldConditionCommitContained, Ref: "refs/heads/main", Commit: c}
	}
	cases := []struct {
		name string
		c    *ticket.HoldCondition
		want string
	}{
		{"commit absent", commit(x), store.HoldConditionBlocked},
		{"path exists", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "pkg/observer.go"}, store.HoldConditionUnblocked},
		{"directory exists", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "pkg/"}, store.HoldConditionUnblocked},
		{"file is not a directory", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "pkg/observer.go/"}, store.HoldConditionBlocked},
		{"path prefix is not a path", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "pkg/observer"}, store.HoldConditionBlocked},
		{"path missing", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/main", Path: "feature.txt"}, store.HoldConditionBlocked},
		{"symbol defined", &ticket.HoldCondition{Kind: ticket.HoldConditionSymbolDefined, Ref: "refs/heads/main", Path: "pkg/observer.go", Symbol: "JobObserver"}, store.HoldConditionUnblocked},
		{"method defined", &ticket.HoldCondition{Kind: ticket.HoldConditionSymbolDefined, Ref: "refs/heads/main", Path: "pkg/observer.go", Symbol: "Watch"}, store.HoldConditionUnblocked},
		{"symbol only mentioned", &ticket.HoldCondition{Kind: ticket.HoldConditionSymbolDefined, Ref: "refs/heads/main", Path: "pkg/observer.go", Symbol: "Observer"}, store.HoldConditionBlocked},
		{"symbol file missing", &ticket.HoldCondition{Kind: ticket.HoldConditionSymbolDefined, Ref: "refs/heads/main", Path: "pkg/absent.go", Symbol: "JobObserver"}, store.HoldConditionBlocked},
		{"ref missing", &ticket.HoldCondition{Kind: ticket.HoldConditionPathExists, Ref: "refs/heads/deploy", Path: "pkg/observer.go"}, store.HoldConditionUnknown},
		{"commit missing", commit(strings.Repeat("d", 40)), store.HoldConditionUnknown},
	}
	for _, tc := range cases {
		if got := store.EvaluateHoldCondition(dir, tc.c); got.State != tc.want {
			t.Errorf("%s: %+v, want %s", tc.name, got, tc.want)
		}
	}
	// The deploy ref moves to a descendant of X: the first read flips it.
	sccGit(t, dir, "merge", "-q", "--no-ff", "-m", "deploy feature", "feature")
	got := store.EvaluateHoldCondition(dir, commit(x))
	if got.State != store.HoldConditionUnblocked || !strings.Contains(got.Detail, "condition met at refs/heads/main "+got.RefCommit) {
		t.Fatalf("after the ref moved: %+v, want UNBLOCKED with the met ref", got)
	}
}

func TestSCCV0011SymbolBinaryAndOversizeAbstain(t *testing.T) {
	dir, _, _ := sccRepo(t)
	sccCommit(t, dir, "bin.dat", "type JobObserver\x00\n", "binary")
	big := "type JobObserver struct{}\n" + strings.Repeat("x", store.HoldConditionMaxBlobBytes)
	sccCommit(t, dir, "big.go", big, "big")
	for _, p := range []string{"bin.dat", "big.go"} {
		c := &ticket.HoldCondition{Kind: ticket.HoldConditionSymbolDefined, Ref: "refs/heads/main", Path: p, Symbol: "JobObserver"}
		if got := store.EvaluateHoldCondition(dir, c); got.State != store.HoldConditionUnknown {
			t.Errorf("%s: %+v, want UNKNOWN", p, got)
		}
	}
}
