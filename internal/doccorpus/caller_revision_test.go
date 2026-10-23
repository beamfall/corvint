package doccorpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// DCP-V1-332: a reader with the checkout relates the caller revision to the
// source revision as equal, descendant, ancestor or diverged; every reader
// that cannot observe the ancestry reports unknown with a fixed reason.
func TestDCPV1332RelateCallerRevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Corpus test")
	git(t, root, "config", "user.email", "corpus@example.invalid")
	commit := func(name string) string {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
		git(t, root, "add", name)
		git(t, root, "commit", "-qm", name)
		return git(t, root, "rev-parse", "HEAD")
	}
	base := commit("a")
	child := commit("b")
	git(t, root, "checkout", "-q", "-b", "side", base)
	side := commit("c")
	absent := strings.Repeat("f", 40)
	ctx := context.Background()
	for _, c := range []struct {
		name, root, source, caller, relation, reason string
	}{
		{"equal", root, base, base, "equal", callerEqualReason},
		{"equal without git", "", base, base, "equal", callerEqualReason},
		{"descendant", root, base, child, "descendant", callerDescendantReason},
		{"ancestor", root, child, base, "ancestor", callerAncestorReason},
		{"diverged", root, child, side, "diverged", callerDivergedReason},
		{"no git", "", base, child, "unknown", callerNoGitReason},
		{"absent caller", root, base, absent, "unknown", callerAbsentReason},
		{"absent source", root, absent, base, "unknown", callerAbsentReason},
		{"no repository", filepath.Join(root, "missing"), base, child, "unknown", callerRepositoryReason},
	} {
		got, err := relateCallerRevision(ctx, c.root, c.source, c.caller)
		if err != nil || got.Relation != c.relation || got.Reason != c.reason || got.Revision != c.caller || got.SourceRevision != c.source {
			t.Fatalf("%s: %+v %v", c.name, got, err)
		}
	}
	// A shallow history cannot prove non-ancestry, so it is unknown.
	if err := os.WriteFile(filepath.Join(root, ".git", "shallow"), []byte(base+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := relateCallerRevision(ctx, root, base, child); err != nil || got.Relation != "unknown" || got.Reason != callerAbsentReason {
		t.Fatalf("shallow: %+v %v", got, err)
	}
	if err := os.Remove(filepath.Join(root, ".git", "shallow")); err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := relateCallerRevision(cancelled, root, base, child); err == nil {
		t.Fatalf("cancelled context answered %+v", got)
	}
}

// DCP-V1-332: a static reader reports unknown, a malformed caller revision is
// refused, and a request without one keeps the envelope bytes unchanged.
func TestDCPV1332CallerRevisionEnvelope(t *testing.T) {
	t.Parallel()
	a := buildAdoption(t, nil)
	source := a.Manifest.Repository.Revision
	other := strings.Repeat("a", 40)
	if other == source {
		other = strings.Repeat("b", 40)
	}
	plain, err := Query(a, Request{Operation: "info"}, "unknown", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "caller_revision") {
		t.Fatal("caller_revision emitted without a request")
	}
	for caller, want := range map[string]string{source: "equal", other: "unknown"} {
		r, err := Query(a, Request{Operation: "info", CallerRevision: caller}, "unknown", nil)
		if err != nil {
			t.Fatal(err)
		}
		got := r.Envelope.CallerRevision
		if got == nil || got.Relation != want || got.SourceRevision != source || want == "unknown" && got.Reason != callerNoGitReason {
			t.Fatalf("static caller revision %s: %+v", caller, got)
		}
	}
	for _, bad := range []string{"main", strings.ToUpper(other), other[:39], other + "0"} {
		if _, err := Query(a, Request{Operation: "info", CallerRevision: bad}, "unknown", nil); err == nil {
			t.Fatalf("caller revision %q admitted", bad)
		}
	}
	// A live reader at the fixture checkout observes the ancestry.
	root, m := adoptionFixture(t, nil)
	live, err := Build(context.Background(), root, m)
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := LiveFreshness(context.Background(), root, live)
	if err != nil {
		t.Fatal(err)
	}
	head := git(t, root, "rev-parse", "HEAD")
	r, err := QueryLive(context.Background(), live, Request{Operation: "info", CallerRevision: head}, overlay)
	if err != nil {
		t.Fatal(err)
	}
	want := "descendant"
	if head == live.Manifest.Repository.Revision {
		want = "equal"
	}
	if got := r.Envelope.CallerRevision; got == nil || got.Relation != want {
		t.Fatalf("live caller revision: %+v", got)
	}
}
