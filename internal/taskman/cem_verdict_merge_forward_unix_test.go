//go:build darwin || linux

package taskman

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestTCBV0020GateGrantsOnlyAtTheSealCommitOfAMergedForwardLane: when main
// moved during the lane, the merge commit and a post-seal merge of main into
// the lane each carry other files, so both refuse sealed-tree-mismatch with the
// lane's one sealed map found, while the seal commit, which the merge reaches
// through its second parent, grants, as does a later commit with the seal's
// tree (TCB-V0-020, TCB-V0-022).
func TestTCBV0020GateGrantsOnlyAtTheSealCommitOfAMergedForwardLane(t *testing.T) {
	f := newVerdictFixture(t, nil)
	moved := f.commitAt(t, f.base, func() {
		if err := os.WriteFile(filepath.Join(f.root, "docs", "other.txt"), []byte("main moved\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	f.git("checkout", "-q", "--detach", f.seal)
	f.git("merge", "-q", "--no-edit", moved)
	forward := f.git("rev-parse", "HEAD")
	f.git("checkout", "-q", "--detach", moved)
	f.git("merge", "-q", "--no-ff", "--no-edit", forward)
	merge := f.git("rev-parse", "HEAD")
	if f.git("rev-parse", merge+"^2") != forward || f.git("rev-parse", forward+"^1") != f.seal {
		t.Fatal("fixture: the seal must reach the merge through its second parent")
	}
	for _, candidate := range []string{merge, forward} {
		in := f.input(PredicateTouchedAnchored, "src/")
		in.CandidateCommit = candidate
		view := requireVerdictRefusal(t, in, CEMVerdictProducerMismatch, "sealed-tree-mismatch")
		if view.Map == nil || view.Map.Path != f.sealed || view.Map.Target != f.bind || view.Verdict != nil {
			t.Fatalf("a merged-forward candidate must name the lane's sealed map and reach no consumer: %+v %+v", view.Map, view.Verdict)
		}
	}
	if result, view := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "src/")); !result.Granted || view.Inputs.CandidateCommit != f.seal {
		t.Fatalf("the seal commit must grant: %s", result.Document)
	}
	// The gate binds trees: a later commit with the seal's tree is the same candidate.
	empty := f.git("commit-tree", f.seal+"^{tree}", "-p", f.seal, "-m", "empty")
	same := f.input(PredicateTouchedAnchored, "src/")
	same.CandidateCommit = empty
	if result, _ := evaluateVerdict(t, same); !result.Granted {
		t.Fatalf("a tree-identical post-seal commit presents the sealed content: %s", result.Document)
	}
}

// TestTCBV0023ExactKeyNamingADirectoryRefuses: under touched-anchored a key
// without a trailing "/" that names a directory at the base or the candidate
// refuses touch-key-names-directory with its inputs pinned, never widening
// the key; an unreadable directory listing fails closed; all-anchored, whose
// keys govern nothing, is not checked.
func TestTCBV0023ExactKeyNamingADirectoryRefuses(t *testing.T) {
	f := newVerdictFixture(t, nil)
	for _, touch := range [][]string{{"src"}, {"src/", "lib"}, {"src/app.txt", "docs"}} {
		view := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, touch...), CEMVerdictInputInvalid, "touch-key-names-directory")
		if view.Inputs == nil || view.CandidateTree == nil || view.Map != nil || view.Verdict != nil || len(view.Governed) != 0 {
			t.Fatalf("a directory key must refuse with inputs pinned and before the map: %+v", view)
		}
	}
	// A directory the change removes still names one at the base.
	removed := f.commitAt(t, f.seal, func() {
		if err := os.RemoveAll(filepath.Join(f.root, "lib")); err != nil {
			t.Fatal(err)
		}
	})
	onlyBase := f.input(PredicateTouchedAnchored, "src/", "lib")
	onlyBase.CandidateCommit = removed
	requireVerdictRefusal(t, onlyBase, CEMVerdictInputInvalid, "touch-key-names-directory")
	// A directory the change adds names one only at the candidate.
	added := f.commitAt(t, f.seal, func() {
		if err := os.MkdirAll(filepath.Join(f.root, "fresh"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.root, "fresh", "x.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	onlyCandidate := f.input(PredicateTouchedAnchored, "src/", "fresh")
	onlyCandidate.CandidateCommit = added
	requireVerdictRefusal(t, onlyCandidate, CEMVerdictInputInvalid, "touch-key-names-directory")
	// Exact file keys and keys naming nothing keep the Tasks exact-path rule.
	if result, _ := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "src/app.txt", "absent/name")); !result.Granted {
		t.Fatalf("exact file keys must still grant: %s", result.Document)
	}
	requireVerdictRefusal(t, f.input(PredicateAllAnchored, "src"), CEMVerdictPredicateFailed, "tier-below-anchored")
	// A candidate whose tree names a missing subtree cannot be listed.
	listing := missingSubtreeCommit(t, f)
	unreadable := f.input(PredicateTouchedAnchored, "src/", "absent/name")
	unreadable.CandidateCommit = listing
	requireVerdictRefusal(t, unreadable, CEMVerdictUnverifiable, "git-read-failed")
	// Without an exact key nothing is listed, and the same candidate reads on.
	unreadable.TouchPaths = []string{"src/"}
	requireVerdictRefusal(t, unreadable, CEMVerdictMapMissing, "")
}

// missingSubtreeCommit commits, on the seal, a root tree whose one entry is a
// subtree object the repository does not hold.
func missingSubtreeCommit(t *testing.T, f verdictFixture) string {
	t.Helper()
	git := func(stdin string, args ...string) string {
		t.Helper()
		command := exec.Command("git", args...)
		command.Dir = f.root
		command.Stdin = strings.NewReader(stdin)
		command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	tree := git("040000 tree "+strings.Repeat("e", len(f.seal))+"\tghost\n", "mktree", "--missing")
	return git("", "commit-tree", tree, "-p", f.seal, "-m", "missing subtree")
}

// TestTCBV0023DirectoryListingFramingFailsClosed: a listing that is not a
// sequence of NUL-terminated "<mode> tree <oid>\t<path>" records is unreadable,
// so the gate refuses git-read-failed rather than reading past it.
func TestTCBV0023DirectoryListingFramingFailsClosed(t *testing.T) {
	const oid = "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		listing string
		names   bool
		fails   bool
	}{
		{listing: "", names: false},
		{listing: "040000 tree " + oid + "\tsrc\x00", names: true},
		{listing: "040000 tree " + oid + "\tlib\x00", names: false},
		{listing: "040000 tree " + oid + "\tsrc", fails: true},
		{listing: "040000 tree " + oid + "\tlib\x00\x00", fails: true},
		{listing: "040000 tree " + oid + "\t\x00", fails: true},
		{listing: "100644 blob " + oid + "\tsrc\x00", fails: true},
		{listing: "040000 tree " + oid + "src\x00", fails: true},
	} {
		git := func(...string) ([]byte, error) { return []byte(tc.listing), nil }
		names, err := exactKeyNamesDirectory(git, []string{"src"}, "c")
		if (err != nil) != tc.fails || names != tc.names {
			t.Fatalf("listing %q: names=%v err=%v", tc.listing, names, err)
		}
	}
}
