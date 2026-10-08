package mutation_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/ticket"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestKHNV0025_RepositoryPayload: a KNOWHOW_ADD may name a repository alias;
// the writer records it on the entry with the qualified anchors, a payload
// without it encodes exactly as before, and every decoder refuses an alias
// that is not a token or an anchor outside "<alias>/".
func TestKHNV0025_RepositoryPayload(t *testing.T) {
	withRepo := func(alias string, anchors ...wire.Value) wire.Value {
		p := knowHowAdd("the e2e harness needs the fixture server", "", "", anchors...)
		p.Obj.Set("repository", str(alias))
		return p
	}
	pre := fixture.Ticket("AT-01")
	post := step(t, owner, nil, pre, mutation.OpKnowHowAdd, withRepo("e2e", khAnchor("e2e/src/a.go", khBlobA)), "1")
	sameExceptKnowHow(t, pre, post)
	if e := post.KnowHow[0]; e.Repository != "e2e" || e.Anchors[0].Path != "e2e/src/a.go" {
		t.Fatalf("repository entry not composed from the payload: %+v", e)
	}
	back, err := ticket.Decode(post.Encode())
	if err != nil || !bytes.Equal(back.Encode(), post.Encode()) {
		t.Fatalf("repository record round trip: %v", err)
	}
	legacy := knowHowAdd("t", "", "", khAnchor("a.go", khBlobA))
	if bytes.Contains(wire.Encode(legacy), []byte("repository")) {
		t.Fatal("a payload without a repository grew the key")
	}
	plain := step(t, owner, nil, pre, mutation.OpKnowHowAdd, legacy, "1")
	if bytes.Contains(plain.Encode(), []byte(`"repository"`)) {
		t.Fatal("an entry without a repository grew the key")
	}

	rev := string(pre.Revision)
	for name, p := range map[string]wire.Value{
		"unprefixed anchor":  withRepo("e2e", khAnchor("src/a.go", khBlobA)),
		"one anchor outside": withRepo("e2e", khAnchor("e2e/a.go", khBlobA), khAnchor("work/b.go", khBlobB)),
		"alias only":         withRepo("e2e", khAnchor("e2e", khBlobA)),
		"alias not a token":  withRepo("_e2e", khAnchor("_e2e/a.go", khBlobA)),
		"alias too long":     withRepo(strings.Repeat("a", 65), khAnchor(strings.Repeat("a", 65)+"/a.go", khBlobA)),
		"empty alias":        withRepo("", khAnchor("/a.go", khBlobA)),
		"null alias": func() wire.Value {
			p := knowHowAdd("t", "", "", khAnchor("a.go", khBlobA))
			p.Obj.Set("repository", wire.Null())
			return p
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpKnowHowAdd, p)); err == nil {
				t.Fatalf("payload decoded: %s", wire.Encode(p))
			}
		})
	}
	reconfirm := obj("note", str("1"), "anchors", wire.Array(khAnchor("e2e/src/a.go", khBlobB)), "commit", str(khCommit),
		"attempt", wire.Null(), "generation", wire.Null())
	if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpKnowHowReconfirm, reconfirm)); err != nil {
		t.Fatalf("a RECONFIRM payload of qualified anchors: %v", err)
	}
	reconfirm.Obj.Set("repository", str("e2e"))
	if _, err := mutation.Decode(envelope("p", owner, "AT-01", rev, mutation.OpKnowHowReconfirm, reconfirm)); err == nil {
		t.Fatal("a RECONFIRM payload carrying repository decoded")
	}
}
