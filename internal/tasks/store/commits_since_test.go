package store_test

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/store"
)

// ATR-V0-351: the commits a worker made since its launch HEAD, newest first
// and bounded; outside a repository the listing fails.
func TestATRV0351_CommitsSinceListsNewCommitsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	commit := func(msg string) string {
		gitRun(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", msg)
		head, err := store.ResolveCommit(dir, "HEAD")
		if err != nil {
			t.Fatal(err)
		}
		return head
	}
	gitRun(t, dir, "init", "-q")
	base := commit("base")
	if got, err := store.CommitsSince(dir, base, 5); err != nil || len(got) != 0 {
		t.Fatalf("no new commits: %v, %v", got, err)
	}
	one, two, three := commit("one"), commit("two"), commit("three")
	if got, err := store.CommitsSince(dir, base, 5); err != nil || !reflect.DeepEqual(got, []string{three, two, one}) {
		t.Fatalf("commits %v, %v; want %v", got, err, []string{three, two, one})
	}
	if got, err := store.CommitsSince(dir, base, 2); err != nil || !reflect.DeepEqual(got, []string{three, two}) {
		t.Fatalf("bounded commits %v, %v", got, err)
	}
	if _, err := store.CommitsSince(t.TempDir(), base, 5); err == nil {
		t.Fatal("a directory outside a repository listed commits")
	}
}
