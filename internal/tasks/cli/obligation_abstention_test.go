package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// abstentionGit runs git in dir with stdin and returns its trimmed output.
func abstentionGit(t *testing.T, dir, stdin string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	c.Stdin = strings.NewReader(stdin)
	out, err := c.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// looseObject is the path of oid's loose object file in repo.
func looseObject(repo, oid string) string {
	return filepath.Join(repo, ".git", "objects", oid[:2], oid[2:])
}

// damageObject overwrites oid's loose object with the bytes of with's, the
// way a damaged or foreign-rewritten object store would serve them: Git
// reads it back as with's type, size and content under oid's name.
func damageObject(t *testing.T, repo, oid, with string) {
	t.Helper()
	raw, err := os.ReadFile(looseObject(repo, with))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(looseObject(repo, oid), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(looseObject(repo, oid), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestTOLV0101_SubjectAbsentUnreadableMismatchAtReview: a reviewed subject
// whose bytes at the reviewed commit no longer match the review is
// SUBJECT_MISMATCH_AT_REVIEW, one over the 1 MiB read bound there is
// SUBJECT_UNREADABLE_AT_REVIEW, and one the reviewed commit's tree no longer
// holds is SUBJECT_ABSENT_AT_REVIEW, in both the boundary and the worklist;
// none falls back to HEAD (TOL-V0-101, V1-1962). The object store is damaged
// to reach them, since a commit's content is otherwise fixed by its id.
func TestTOLV0101_SubjectAbsentUnreadableMismatchAtReview(t *testing.T) {
	app, appHead := separateTestRepo(t)
	r, id, _ := obligationRepoWith(t, testRepoPolicy(map[string]string{"app": app}))
	seedObligations(t, r.Root, id)
	held := reviewFile(t, "1", "none", verdict("rev-a", "partial", "none", "misses the error branch"))
	if x := reviewWitness(t, r.Root, id, "rv-app", appHead, "AC-1", held, "--test-repository", "app"); x.res.Outcome != wire.OutcomeOK || field(x.res.Items[0], "reviewOutcome").Str != "HELD" {
		t.Fatalf("test-repository review: %s", x.stdout)
	}
	blob := abstentionGit(t, app, "", "rev-parse", appHead+":e2e/login.spec.ts")
	tree := abstentionGit(t, app, "", "rev-parse", appHead+":e2e")
	expect := func(reason string) {
		t.Helper()
		var subjects []wire.Value
		for _, e := range field(obligationsBoundaryRead(t, r.Root, id), "entries").Arr {
			if field(e, "id").Str == "AC-1" {
				subjects = append(subjects, field(e, "reviewedSubject"))
			}
		}
		v := worklistRead(t, r.Root, "--target", id)
		for _, e := range append(append([]wire.Value{}, field(v, "items").Arr...), field(v, "excluded").Arr...) {
			if field(e, "id").Str == "AC-1" {
				subjects = append(subjects, field(e, "reviewedSubject"))
			}
		}
		if len(subjects) != 2 {
			t.Fatalf("want AC-1 in the boundary and the worklist, got %d: %s", len(subjects), wire.Encode(v))
		}
		for _, s := range subjects {
			if field(s, "status").Str != "UNAVAILABLE" || field(s, "reason").Str != reason || field(s, "commit").Str != appHead ||
				field(s, "repository").Str != "app" || field(s, "path").Str != "e2e/login.spec.ts" {
				t.Fatalf("reviewedSubject want UNAVAILABLE/%s: %s", reason, wire.Encode(s))
			}
		}
	}

	other := abstentionGit(t, app, obligationSpec+"// other bytes\n", "hash-object", "-w", "--stdin")
	damageObject(t, app, blob, other)
	expect("SUBJECT_MISMATCH_AT_REVIEW")

	huge := abstentionGit(t, app, strings.Repeat("x", 1<<20+1), "hash-object", "-w", "--stdin")
	damageObject(t, app, blob, huge)
	expect("SUBJECT_UNREADABLE_AT_REVIEW")

	without := abstentionGit(t, app, "100644 blob "+other+"\tother.spec.ts\n", "mktree")
	damageObject(t, app, tree, without)
	expect("SUBJECT_ABSENT_AT_REVIEW")
}

// TestTOLV0108_CapabilityBlobOverBoundAndUnlistableBaseline: a --contains
// search whose only candidate blob is over the 1 MiB read bound abstains
// SEARCH_BOUND_EXCEEDED, and beside a readable match reports truncated; a
// baseline commit whose tree cannot be listed, because its tree object is
// missing or holds a path over the 64 KiB entry bound, abstains
// ENVIRONMENT_BASELINE_UNRESOLVED with no baseline commit
// (TOL-V0-107..109, V1-1962).
func TestTOLV0108_CapabilityBlobOverBoundAndUnlistableBaseline(t *testing.T) {
	r, _, _ := obligationRepoWith(t, func(p wire.Value) {
		p.Obj.Set("obligations", wire.ObjectValue(wire.NewObject().Set("workerWitness", wire.Bool(false)).
			Set("environmentBaselines", wire.ObjectValue(wire.NewObject().Set(".", wire.String("refs/heads/env"))))))
	})
	fixture.Write(t, filepath.Join(r.Root, "big", "huge.txt"), []byte(strings.Repeat("needle\n", (1<<20)/7+1)))
	fixture.Write(t, filepath.Join(r.Root, "big", "small.txt"), []byte("hay\n"))
	commitTree(t, r.Root, "blob over the bound")
	git(t, r.Root, "branch", "env", "HEAD")
	if v := capabilityRead(t, r.Root, "--path", "big", "--contains", "needle"); field(v, "status").Str != "UNKNOWN" ||
		field(v, "reason").Str != "SEARCH_BOUND_EXCEEDED" || len(field(v, "matches").Arr) != 0 || field(field(v, "baseline"), "commit").Kind != wire.KindString {
		t.Fatalf("blob over the bound: %s", wire.Encode(v))
	}
	if v := capabilityRead(t, r.Root, "--path", "big", "--contains", "hay"); field(v, "status").Str != "PRESENT" ||
		strList(v, "matches") != "big/small.txt" || !field(v, "truncated").Bool {
		t.Fatalf("readable match beside a blob over the bound: %s", wire.Encode(v))
	}

	unlistable := func(name, top string) {
		t.Helper()
		commit := abstentionGit(t, r.Root, "", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit-tree", top, "-m", name)
		git(t, r.Root, "branch", "-f", "env", commit)
	}
	expect := func(name string) {
		t.Helper()
		v := capabilityRead(t, r.Root, "--path", "gone")
		b := field(v, "baseline")
		if field(v, "status").Str != "UNKNOWN" || field(v, "reason").Str != "ENVIRONMENT_BASELINE_UNRESOLVED" || len(field(v, "matches").Arr) != 0 ||
			field(b, "commit").Kind != wire.KindNull || field(b, "ref").Str != "refs/heads/env" {
			t.Fatalf("%s: %s", name, wire.Encode(v))
		}
		// The baseline commit itself still resolves: only its listing fails.
		abstentionGit(t, r.Root, "", "rev-parse", "--verify", "refs/heads/env^{commit}")
		if err := exec.Command("git", "-C", r.Root, "ls-tree", "-r", "refs/heads/env").Run(); err == nil && name == "missing tree" {
			t.Fatalf("%s: the baseline tree still lists", name)
		}
	}

	leafBlob := abstentionGit(t, r.Root, "only under the missing tree\n", "hash-object", "-w", "--stdin")
	leaf := abstentionGit(t, r.Root, "100644 blob "+leafBlob+"\tf.txt\n", "mktree")
	top := abstentionGit(t, r.Root, "040000 tree "+leaf+"\tgone\n", "mktree")
	unlistable("missing tree", top)
	if err := os.Remove(looseObject(r.Root, top)); err != nil {
		t.Fatal(err)
	}
	expect("missing tree")

	long := abstentionGit(t, r.Root, "100644 blob "+leafBlob+"\t"+strings.Repeat("z", 64<<10+16)+"\n", "mktree")
	unlistable("long path", abstentionGit(t, r.Root, "040000 tree "+long+"\tgone\n", "mktree"))
	expect("long path")
}
