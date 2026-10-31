package transaction

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestLGLV0800_AttemptMemoMatchesFreshDecode (V1-3186): the per-receipt
// attempt memo returns what a fresh externalPostBytes and DecodeAttempt
// return, errors included, reads each post's bytes once however many binding
// audits ask, and starts afresh for a different receipt.
func TestLGLV0800_AttemptMemoMatchesFreshDecode(t *testing.T) {
	_, a, _ := untouchedContext(t)
	good, err := a.Encode()
	if err != nil {
		t.Fatal(err)
	}
	notRecord := []byte("not a record")
	malformed := []byte(`{"phase":"BUILT","phaseSinceSeq":"2"}` + "\n")
	blobs := map[wire.Digest][]byte{}
	reads := map[wire.Digest]int{}
	post := func(b []byte) snapshot.PostEntry {
		d := wire.Sum(b)
		blobs[d] = b
		return snapshot.PostEntry{Path: "attempts/a.json", Sha256: &d, BlobSha256: &d}
	}
	absent := wire.Sum([]byte("absent"))
	lookup := func(d wire.Digest) ([]byte, bool) { reads[d]++; b, ok := blobs[d]; return b, ok }
	rc := &snapshot.Receipt{Seq: wire.SizeOf(2), Post: []snapshot.PostEntry{
		post(good), post(notRecord), post(malformed),
		{Path: "attempts/b.json", Sha256: &absent, BlobSha256: &absent},
	}}
	errText := func(err error) string {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	var memo TicketPosts
	for i := range rc.Post {
		fresh := (*TicketPosts)(nil).attempt(rc, i, lookup)
		for range 4 {
			got := memo.attempt(rc, i, lookup)
			if errText(got.rawErr) != errText(fresh.rawErr) || errText(got.decErr) != errText(fresh.decErr) || !reflect.DeepEqual(got.at, fresh.at) {
				t.Fatalf("post %d: memo %+v, fresh %+v", i, got, fresh)
			}
		}
	}
	if memo.attempt(rc, 0, lookup).at == nil {
		t.Fatal("valid attempt post did not decode")
	}
	for d, n := range reads {
		if n != 2 { // one fresh read and one memoized read
			t.Fatalf("blob %s read %d times, want 2", d, n)
		}
	}
	next := &snapshot.Receipt{Seq: wire.SizeOf(3), Post: []snapshot.PostEntry{post(notRecord)}}
	if got := memo.attempt(next, 0, lookup); got.decErr == nil {
		t.Fatalf("a different receipt reused the prior receipt's post 0: %+v", got)
	}
}
