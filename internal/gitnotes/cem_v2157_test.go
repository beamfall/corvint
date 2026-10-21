package gitnotes

import (
	"context"
	"testing"
)

// CEM-V1-067 (V1-2157): with --notes-tip the anchor note is read from the
// captured live refs/notes/corvint commit by object ID. A notes ref moved
// A -> B -> A between the tip capture and the unmoved check passes that
// check, so the anchor row must still reflect only A's note, never B's.
func TestCEMV1067PinnedNotesTipReadIgnoresARefMovedAwayAndBack(t *testing.T) {
	pinVerdictClock(t)
	root := fixtureRepo(t)
	ctx := context.Background()
	if _, err := AnchorWithVerdict(ctx, root, mapPath, "HEAD", root, true); err != nil {
		t.Fatal(err)
	}
	a := fixtureGit(t, root, nil, "rev-parse", Ref)
	want := anchorState(t, root, "HEAD")
	if want == "" || want == "malformed" {
		t.Fatalf("A's anchor row is %q", want)
	}
	// B: the same notes ref with HEAD's anchor note replaced by a malformed one.
	attach(t, root, "HEAD", []byte("not a pointer\n"))
	b := fixtureGit(t, root, nil, "rev-parse", Ref)
	if anchorState(t, root, "HEAD") != "malformed" || b == a {
		t.Fatal("B did not change the anchor note")
	}
	fixtureGit(t, root, nil, "update-ref", Ref, a)

	previous := anchorRead
	t.Cleanup(func() { anchorRead = previous })
	anchorRead = func(stage string) {
		switch stage {
		case "before":
			fixtureGit(t, root, nil, "update-ref", Ref, b)
		case "after":
			fixtureGit(t, root, nil, "update-ref", Ref, a)
		}
	}
	result, err := ProvenanceWithNotesTip(ctx, root, "HEAD", a)
	if err != nil {
		t.Fatal(err)
	}
	if tip := result["notes_tip"].(map[string]any); tip["state"] != "current" || tip["live"] != a {
		t.Fatalf("notes tip: %v", tip)
	}
	got := ""
	for _, row := range result["evidence"].([]map[string]any) {
		if row["kind"] == KindAnchor {
			got, _ = row["state"].(string)
		}
	}
	if got != want {
		t.Fatalf("a ref moved A->B->A leaked B's anchor note: state %q, want %q", got, want)
	}
}
