package journal

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/fixture"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// requestHistory records request R1 early, pads n ticket receipts, derives
// a writer checkpoint, then records R2 in the tail.
func requestHistory(t *testing.T, n int) (*fixture.Repo, Reader, *WriterCheckpoint) {
	t.Helper()
	repo, r := setup(t)
	record := func(id string) {
		rp, err := snapshot.RequestPath(id)
		if err != nil {
			t.Fatal(err)
		}
		h, err := snapshot.DecodeHead(read(t, filepath.Join(repo.StateDir, "head.json")))
		if err != nil {
			t.Fatal(err)
		}
		appendReceipt(t, repo, "MUTATION", map[string][]byte{rp: requestBytes(id, h.LastSeq.Uint64()+1, wire.Sum([]byte(id)), false)}, id, true, true, false)
	}
	record("R1")
	for i := 0; i < n; i++ {
		tk := fixture.Ticket("A")
		body := fmt.Sprintf("pad %d", i)
		tk.Body = &body
		appendReceipt(t, repo, "MUTATION", map[string][]byte{ticketPath("A"): tk.Encode()}, "", true, true, false)
	}
	full, observed, err := r.AuditForWriteObserved()
	if err != nil || full.Mode != ModeFull {
		t.Fatalf("complete audit: %v", err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil || wc == nil {
		t.Fatalf("writer checkpoint: %v", err)
	}
	record("R2")
	return repo, r, wc
}

func lookupBoth(t *testing.T, r Reader, wc *WriterCheckpoint, id string) (complete, routed [3]any, reads int) {
	t.Helper()
	plain := RequestIndex{Reader: r}
	e, f, err := plain.Lookup(id)
	complete = [3]any{e, f, fmt.Sprint(err)}
	idx := RequestIndex{Reader: r, WriterCheckpoint: func() *WriterCheckpoint { return wc }}
	old := afterNativeRead
	afterNativeRead = func(p string) {
		if strings.HasPrefix(p, "receipts/") {
			reads++
		}
	}
	e2, f2, err2 := idx.Lookup(id)
	afterNativeRead = old
	routed = [3]any{e2, f2, fmt.Sprint(err2)}
	if f && (plain.TicketID != idx.TicketID) {
		t.Fatalf("%s: ticket %q vs %q", id, plain.TicketID, idx.TicketID)
	}
	return complete, routed, reads
}

// LGL-V0-811: a lookup served by the writer route returns what the
// complete lookup returns. An absent request and one recorded after the
// checkpoint read the same few receipts whatever the history length; one
// recorded at most MaxWriterTail receipts before the checkpoint is served
// by walking back to its receipt; an older one falls back to the complete
// audit (CAL-V0-190's bound) and still answers the same.
func TestLGLV0811_RequestLookupFromTheWriterCheckpoint(t *testing.T) {
	reads := map[string][2]int{}
	for i, n := range []int{40, 400} {
		_, r, wc := requestHistory(t, n)
		for _, id := range []string{"R1", "R2", "R3"} {
			complete, routed, got := lookupBoth(t, r, wc, id)
			if !reflect.DeepEqual(complete, routed) {
				t.Fatalf("n=%d %s: complete %v, routed %v", n, id, complete, routed)
			}
			v := reads[id]
			v[i] = got
			reads[id] = v
		}
	}
	for _, id := range []string{"R2", "R3"} {
		if reads[id][0] != reads[id][1] || reads[id][1] > 4 {
			t.Fatalf("%s: receipt reads %v at 40 and 400 receipts; want the same small count", id, reads[id])
		}
	}
	if r1 := reads["R1"]; r1[0] > 40+4 || r1[1] <= 400 {
		t.Fatalf("R1: receipt reads %v; want a walk back to its receipt at 40 and the complete audit at 400", r1)
	}
}

// LGL-V0-811: a tampered request afterimage, or a tampered receipt the route
// reads, is refused exactly as the complete lookup refuses it; a forged
// checkpoint only falls back.
func TestLGLV0811_TamperedRequestStillRefused(t *testing.T) {
	t.Run("afterimage", func(t *testing.T) {
		repo, r, wc := requestHistory(t, 40)
		rp, _ := snapshot.RequestPath("R1")
		fixture.Write(t, filepath.Join(repo.StateDir, rp), requestBytes("R1", 2, wire.Sum([]byte("other")), false))
		complete, routed, _ := lookupBoth(t, r, wc, "R1")
		if !reflect.DeepEqual(complete, routed) || complete[2] == "<nil>" {
			t.Fatalf("complete %v, routed %v", complete, routed)
		}
	})
	t.Run("tail receipt", func(t *testing.T) {
		repo, r, wc := requestHistory(t, 40)
		h, _ := snapshot.DecodeHead(read(t, filepath.Join(repo.StateDir, "head.json")))
		name, _ := snapshot.ReceiptName(h.LastSeq.Uint64())
		p := filepath.Join(repo.StateDir, "receipts", name)
		fixture.Write(t, p, []byte(strings.Replace(string(read(t, p)), `"requestId":"R2"`, `"requestId":"R9"`, 1)))
		complete, routed, _ := lookupBoth(t, r, wc, "R2")
		if !reflect.DeepEqual(complete, routed) || complete[2] == "<nil>" {
			t.Fatalf("complete %v, routed %v", complete, routed)
		}
	})
	t.Run("forged checkpoint", func(t *testing.T) {
		_, r, wc := requestHistory(t, 40)
		forged := *wc
		forged.ReceiptSha256 = wire.Sum([]byte("forged"))
		complete, routed, _ := lookupBoth(t, r, &forged, "R1")
		if !reflect.DeepEqual(complete, routed) {
			t.Fatalf("complete %v, routed %v", complete, routed)
		}
	})
}
