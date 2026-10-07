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

// knowHowRepo is a checkout whose first commit holds a.go, b.go and
// docs/x.md; it returns the root, that commit and each file's blob.
func knowHowRepo(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	gitRun(t, root, "init", "-q", "-b", "main")
	for p, body := range map[string]string{"a.go": "package a\n", "b.go": "package b\n", "docs/x.md": "# x\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, root, "add", ".")
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "base")
	blobs := map[string]string{}
	for _, p := range []string{"a.go", "b.go", "docs/x.md"} {
		blobs[p] = gitOut(t, root, "rev-parse", "HEAD:"+p)
	}
	return root, gitOut(t, root, "rev-parse", "HEAD"), blobs
}

func knowHowEntry(seq int64, at string, anchors ...ticket.KnowHowAnchor) ticket.KnowHowEntry {
	return ticket.KnowHowEntry{Seq: wire.CountOf(seq), Operation: ticket.KnowHowAdd, Text: "note " + at, Anchors: anchors,
		Routes: []string{}, Commit: strings.Repeat("c", 40), ActorID: "russell", ActorRole: "OWNER", RecordedAt: wire.Timestamp(at)}
}

// TestKHNV0001_PinsResolveTheWritersCommit: one batched lookup pins the
// commit and each anchor's blob; a missing file, a directory or an unknown
// revision is refused, and no checkout is refused.
func TestKHNV0001_PinsResolveTheWritersCommit(t *testing.T) {
	root, head, blobs := knowHowRepo(t)
	commit, got, err := store.KnowHowPins(root, "HEAD", []string{"a.go", "docs/x.md"})
	if err != nil || commit != head || got[0] != blobs["a.go"] || got[1] != blobs["docs/x.md"] {
		t.Fatalf("pins: %s %v %v", commit, got, err)
	}
	if c, _, err := store.KnowHowPins(root, head, []string{"b.go"}); err != nil || c != head {
		t.Fatalf("explicit commit: %s %v", c, err)
	}
	for name, c := range map[string]struct{ rev, path string }{
		"missing file":     {"HEAD", "gone.go"},
		"directory":        {"HEAD", "docs"},
		"unknown revision": {strings.Repeat("d", 40), "a.go"},
	} {
		if _, _, err := store.KnowHowPins(root, c.rev, []string{c.path}); err == nil {
			t.Fatalf("%s pinned", name)
		}
	}
	if _, _, err := store.KnowHowPins("", "HEAD", []string{"a.go"}); err == nil {
		t.Fatal("pinned without a checkout")
	}
}

// TestKHNV0005_FreshnessIsComputedAtReadTime: an anchor whose blob equals
// its pin is CURRENT, a changed one STALE, a deleted one UNKNOWN; a note is
// STALE over UNKNOWN over CURRENT; an uncommitted edit does not count; no Git
// or no commit makes every note UNKNOWN, never CURRENT.
func TestKHNV0005_FreshnessIsComputedAtReadTime(t *testing.T) {
	root, _, blobs := knowHowRepo(t)
	a := ticket.KnowHowAnchor{Path: "a.go", Blob: blobs["a.go"]}
	b := ticket.KnowHowAnchor{Path: "b.go", Blob: blobs["b.go"]}
	x := ticket.KnowHowAnchor{Path: "docs/x.md", Blob: blobs["docs/x.md"]}
	notes := func() []store.KnowHowNote {
		return []store.KnowHowNote{
			{TicketID: "T", Entry: knowHowEntry(1, "2026-10-01T00:00:00Z", a)},
			{TicketID: "T", Entry: knowHowEntry(2, "2026-10-02T00:00:00Z", a, b)},
			{TicketID: "T", Entry: knowHowEntry(3, "2026-10-03T00:00:00Z", a, x)},
			{TicketID: "T", Entry: knowHowEntry(4, "2026-10-04T00:00:00Z", b, x)},
		}
	}
	fresh := notes()
	head := store.ResolveKnowHowFreshness(root, fresh)
	for _, n := range fresh {
		if n.Freshness != store.KnowHowCurrent {
			t.Fatalf("unchanged note %s is %s", n.Entry.Seq, n.Freshness)
		}
	}
	if head != gitOut(t, root, "rev-parse", "HEAD") {
		t.Fatalf("head %q", head)
	}

	// An uncommitted edit is not observed.
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package a // dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := notes()
	store.ResolveKnowHowFreshness(root, dirty)
	if dirty[0].Freshness != store.KnowHowCurrent {
		t.Fatalf("dirty tree changed freshness: %s", dirty[0].Freshness)
	}

	// Commit a change to b.go and delete docs/x.md.
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("package b // changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, root, "checkout", "-q", "--", "a.go")
	gitRun(t, root, "rm", "-q", "docs/x.md")
	gitRun(t, root, "add", "b.go")
	gitRun(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "change")
	later := notes()
	store.ResolveKnowHowFreshness(root, later)
	want := []struct {
		note    string
		anchors []string
	}{
		{store.KnowHowCurrent, []string{store.KnowHowCurrent}},
		{store.KnowHowStale, []string{store.KnowHowCurrent, store.KnowHowStale}},
		{store.KnowHowUnknown, []string{store.KnowHowCurrent, store.KnowHowUnknown}},
		{store.KnowHowStale, []string{store.KnowHowStale, store.KnowHowUnknown}},
	}
	for i, w := range want {
		if later[i].Freshness != w.note || strings.Join(later[i].Anchors, ",") != strings.Join(w.anchors, ",") {
			t.Fatalf("note %d: %s %v, want %s %v", i+1, later[i].Freshness, later[i].Anchors, w.note, w.anchors)
		}
	}

	// Ordering: CURRENT, UNKNOWN, STALE; newest first within a state.
	store.SortKnowHow(later)
	var order []string
	for _, n := range later {
		order = append(order, string(n.Entry.Seq))
	}
	if strings.Join(order, ",") != "1,3,4,2" {
		t.Fatalf("order %v", order)
	}

	for name, dir := range map[string]string{"no checkout": "", "not a repository": t.TempDir()} {
		none := notes()
		if h := store.ResolveKnowHowFreshness(dir, none); h != "" {
			t.Fatalf("%s: head %q", name, h)
		}
		for _, n := range none {
			if n.Freshness != store.KnowHowUnknown {
				t.Fatalf("%s: note %s is %s", name, n.Entry.Seq, n.Freshness)
			}
		}
	}
	unborn := t.TempDir()
	gitRun(t, unborn, "init", "-q", "-b", "main")
	none := notes()
	if h := store.ResolveKnowHowFreshness(unborn, none); h != "" || none[0].Freshness != store.KnowHowUnknown {
		t.Fatalf("unborn HEAD: %q %s", h, none[0].Freshness)
	}
}

// TestKHNV0006_SelectionAndProjection: selection is by exact anchor path or
// a trailing-"/" prefix, over active notes only, optionally of one home
// ticket; the projection keeps the ordered prefix that fits the byte cap and
// counts the rest.
func TestKHNV0006_SelectionAndProjection(t *testing.T) {
	blob := strings.Repeat("a", 40)
	one, two := fixture.Ticket("AT-01"), fixture.Ticket("AT-02")
	reason := "r"
	sup := wire.Count("1")
	replaced := knowHowEntry(2, "2026-10-02T00:00:00Z", ticket.KnowHowAnchor{Path: "internal/a.go", Blob: blob})
	replaced.Supersedes, replaced.Reason = &sup, &reason
	one.KnowHow = []ticket.KnowHowEntry{knowHowEntry(1, "2026-10-01T00:00:00Z", ticket.KnowHowAnchor{Path: "internal/a.go", Blob: blob}), replaced}
	two.KnowHow = []ticket.KnowHowEntry{knowHowEntry(1, "2026-10-03T00:00:00Z", ticket.KnowHowAnchor{Path: "docs/x.md", Blob: blob})}
	q, _ := wire.ParseQueueID("", one.TicketID.QueueID())
	inv, err := ticket.NewInventory(q, []*ticket.Record{one, two})
	if err != nil {
		t.Fatal(err)
	}
	seqs := func(ns []store.KnowHowNote) string {
		var out []string
		for _, n := range ns {
			out = append(out, n.TicketID[len(n.TicketID)-5:]+"#"+string(n.Entry.Seq))
		}
		return strings.Join(out, ",")
	}
	for name, c := range map[string]struct {
		paths []string
		home  string
		want  string
	}{
		"all":            {nil, "", "AT-01#2,AT-02#1"},
		"exact":          {[]string{"internal/a.go"}, "", "AT-01#2"},
		"prefix":         {[]string{"docs/"}, "", "AT-02#1"},
		"no slash":       {[]string{"internal"}, "", ""},
		"home":           {nil, fixture.TicketID("AT-02"), "AT-02#1"},
		"home and paths": {[]string{"docs/"}, fixture.TicketID("AT-01"), ""},
	} {
		if got := seqs(store.SelectKnowHow(inv, c.paths, c.home)); got != c.want {
			t.Fatalf("%s: %q, want %q", name, got, c.want)
		}
	}

	var notes []store.KnowHowNote
	for i := 0; i < 10; i++ {
		e := knowHowEntry(int64(i+1), "2026-10-01T00:00:00Z", ticket.KnowHowAnchor{Path: "a.go", Blob: blob})
		e.Text = strings.Repeat("t", 400)
		notes = append(notes, store.KnowHowNote{TicketID: "T", Entry: e, Anchors: []string{store.KnowHowCurrent}, Freshness: store.KnowHowCurrent})
	}
	items, omitted := store.ProjectKnowHow(notes, true, store.KnowHowDeliveryMaxBytes)
	if len(items) == 0 || omitted == 0 || len(items)+omitted != len(notes) {
		t.Fatalf("projection: %d items, %d omitted", len(items), omitted)
	}
	if n := len(wire.Encode(wire.Array(items...))); n > store.KnowHowDeliveryMaxBytes {
		t.Fatalf("projection is %d bytes", n)
	}
	compact := string(wire.Encode(items[0]))
	if strings.Contains(compact, `"blob"`) || strings.Contains(compact, `"actor"`) || !strings.Contains(compact, `"freshness":"CURRENT"`) {
		t.Fatalf("compact item %s", compact)
	}
	if items, omitted := store.ProjectKnowHow(notes[:1], false, store.KnowHowDeliveryMaxBytes); len(items) != 1 || omitted != 0 || !strings.Contains(string(wire.Encode(items[0])), `"blob"`) {
		t.Fatal("full item lacks its pins")
	}
}
