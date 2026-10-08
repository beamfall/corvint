package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/store"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

func repositoryEntry(seq int64, alias string, anchors ...ticket.KnowHowAnchor) ticket.KnowHowEntry {
	for i := range anchors {
		anchors[i].Path = alias + "/" + anchors[i].Path
	}
	e := knowHowEntry(seq, "2026-10-08T00:00:00Z", anchors...)
	e.Repository = alias
	return e
}

// TestKHNV0026_QualifiedAnchorsMatchTouchPaths: a repository note's stored
// paths carry its alias, so the unchanged KHN-V0-006 rule matches it against
// a qualified touchPath or --path and never against the bare path.
func TestKHNV0026_QualifiedAnchorsMatchTouchPaths(t *testing.T) {
	blob := strings.Repeat("a", 40)
	one := fixture.Ticket("AT-01")
	one.KnowHow = []ticket.KnowHowEntry{
		repositoryEntry(1, "e2e", ticket.KnowHowAnchor{Path: "src/a.go", Blob: blob}),
		knowHowEntry(2, "2026-10-08T00:00:00Z", ticket.KnowHowAnchor{Path: "src/a.go", Blob: blob}),
	}
	q, _ := wire.ParseQueueID("", one.TicketID.QueueID())
	inv, err := ticket.NewInventory(q, []*ticket.Record{one})
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		paths []string
		want  string
	}{
		"repository prefix": {[]string{"e2e/"}, "1"},
		"qualified exact":   {[]string{"e2e/src/a.go"}, "1"},
		"bare path":         {[]string{"src/a.go"}, "2"},
		"other repository":  {[]string{"work/"}, ""},
	} {
		var got []string
		for _, n := range store.SelectKnowHow(inv, c.paths, "") {
			got = append(got, string(n.Entry.Seq))
		}
		if strings.Join(got, ",") != c.want {
			t.Fatalf("%s: %v, want %q", name, got, c.want)
		}
	}
}

// TestKHNV0027_RepositoryFreshness: a repository note resolves at the HEAD
// of the root its alias is mapped to, using the path below the alias; an
// unmapped alias is UNKNOWN with a warning and never resolves against the
// store checkout, even where that checkout holds an identical blob; a note
// without a repository resolves exactly as before.
func TestKHNV0027_RepositoryFreshness(t *testing.T) {
	storeRoot, storeHead, storeBlobs := knowHowRepo(t)
	e2eRoot, e2eHead, e2eBlobs := knowHowRepo(t)
	if err := os.WriteFile(filepath.Join(e2eRoot, "b.go"), []byte("package b // changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, e2eRoot, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-am", "change b")
	e2eHead2 := gitOut(t, e2eRoot, "rev-parse", "HEAD")
	if e2eHead2 == e2eHead {
		t.Fatal("fixture commit did not move")
	}
	notes := []store.KnowHowNote{
		{TicketID: "T", Entry: knowHowEntry(1, "2026-10-08T00:00:00Z", ticket.KnowHowAnchor{Path: "a.go", Blob: storeBlobs["a.go"]})},
		{TicketID: "T", Entry: repositoryEntry(2, "e2e", ticket.KnowHowAnchor{Path: "a.go", Blob: e2eBlobs["a.go"]})},
		{TicketID: "T", Entry: repositoryEntry(3, "e2e", ticket.KnowHowAnchor{Path: "b.go", Blob: e2eBlobs["b.go"]})},
		{TicketID: "T", Entry: repositoryEntry(4, "work", ticket.KnowHowAnchor{Path: "a.go", Blob: storeBlobs["a.go"]})},
	}
	repos := map[string]string{"e2e": e2eRoot}
	head := store.ResolveKnowHowRepositories(storeRoot, repos, notes)
	if head != storeHead {
		t.Fatalf("head %s, want the store checkout's %s", head, storeHead)
	}
	for i, want := range []struct{ freshness, repoHead string }{
		{store.KnowHowCurrent, ""},
		{store.KnowHowCurrent, e2eHead2},
		{store.KnowHowStale, e2eHead2},
		{store.KnowHowUnknown, ""},
	} {
		if notes[i].Freshness != want.freshness || notes[i].RepositoryHead != want.repoHead {
			t.Fatalf("note %d: %s at %q, want %s at %q", i+1, notes[i].Freshness, notes[i].RepositoryHead, want.freshness, want.repoHead)
		}
	}
	if notes[1].Entry.Anchors[0].Path != "e2e/a.go" {
		t.Fatalf("resolution rewrote the stored path: %+v", notes[1].Entry.Anchors)
	}
	warnings := store.KnowHowRepositoryWarnings(repos, notes)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "KNOWHOW_REPOSITORY: repository work is not mapped") {
		t.Fatalf("warnings: %v", warnings)
	}

	full := string(wire.Encode(store.KnowHowNoteValue(notes[1], false)))
	compact := string(wire.Encode(store.KnowHowNoteValue(notes[3], true)))
	legacy := string(wire.Encode(store.KnowHowNoteValue(notes[0], false)))
	if !strings.Contains(full, `"repository":{"alias":"e2e","head":"`+e2eHead2+`"}`) ||
		!strings.Contains(compact, `"repository":{"alias":"work","head":null}`) || strings.Contains(legacy, "repository") {
		t.Fatalf("projection:\n%s\n%s\n%s", full, compact, legacy)
	}

	// The legacy entry point resolves every repository note as unmapped.
	again := append([]store.KnowHowNote{}, notes...)
	store.ResolveKnowHowFreshness(storeRoot, again)
	if again[0].Freshness != store.KnowHowCurrent || again[1].Freshness != store.KnowHowUnknown {
		t.Fatalf("legacy resolution: %+v", again)
	}
}

// TestKHNV0027_RepositoryArguments: --repo takes ALIAS=ROOT with a token
// alias and a Git work-tree top level, resolved against the working
// directory; anything else, or an alias given twice, is refused.
func TestKHNV0027_RepositoryArguments(t *testing.T) {
	root, _, _ := knowHowRepo(t)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	alias, got, err := store.KnowHowRepositoryArg(filepath.Dir(root), "e2e="+filepath.Base(root))
	if err != nil || alias != "e2e" || got != resolved {
		t.Fatalf("relative root: %s %s %v", alias, got, err)
	}
	plain := t.TempDir()
	for name, arg := range map[string]string{
		"no equals":         root,
		"empty root":        "e2e=",
		"alias not a token": "_e2e=" + root,
		"alias too long":    strings.Repeat("a", 65) + "=" + root,
		"subdirectory":      "e2e=" + filepath.Join(root, "docs"),
		"not a repository":  "e2e=" + plain,
		"missing":           "e2e=" + filepath.Join(plain, "gone"),
	} {
		if _, _, err := store.KnowHowRepositoryArg(plain, arg); err == nil || wire.CodeOf(err) != wire.CodeMalformed ||
			!strings.Contains(err.Error(), "KNOWHOW_REPOSITORY") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if _, err := store.KnowHowRepositoryArgs(plain, []string{"e2e=" + root, "e2e=" + root}); err == nil {
		t.Fatal("a repeated alias was admitted")
	}
	if m, err := store.KnowHowRepositoryArgs(plain, nil); err != nil || m != nil {
		t.Fatalf("no --repo: %v %v", m, err)
	}
}
