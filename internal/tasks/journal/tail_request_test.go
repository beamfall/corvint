package journal

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// V1-0917: a tail receipt that posts a request already posted before the
// read checkpoint is refused exactly as the complete audit refuses it.
func TestPSRV0190_CheckpointTailRefusesPrefixRequestDuplicate(t *testing.T) {
	repo, r := setup(t)
	id := "r1"
	path, _ := snapshot.RequestPath(id)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 2, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	cp := checkpointed(t, repo, r)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 3, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	_, full := r.Audit()
	requireCode(t, full, wire.CodeJournalForked)
	r.Checkpoint = cp
	res, err := r.Audit()
	requireCode(t, err, wire.CodeJournalForked)
	if res != nil && res.ProjectionAgreement == "AGREES" {
		t.Fatal("checkpoint read agreed with a duplicated request")
	}
}

// V1-2352: a forged duplicate whose tail receipt declares a null pre-state
// (the request file removed before it is re-posted) is refused on the read
// checkpoint route exactly as the complete audit refuses it.
func TestPSRV0203_CheckpointTailRefusesNullPreStateDuplicate(t *testing.T) {
	repo, r := setup(t)
	id := "r1"
	path, _ := snapshot.RequestPath(id)
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 2, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	cp := checkpointed(t, repo, r)
	remove(t, filepath.Join(repo.StateDir, path))
	appendReceipt(t, repo, "MUTATION", map[string][]byte{path: requestBytes(id, 3, wire.Sum([]byte("mutation")), false)}, id, true, true, false)
	_, full := r.Audit()
	requireCode(t, full, wire.CodeJournalForked)
	r.Checkpoint = cp
	res, err := r.Audit()
	requireCode(t, err, wire.CodeJournalForked)
	if res != nil && res.ProjectionAgreement == "AGREES" {
		t.Fatal("checkpoint read agreed with a duplicated request")
	}
}

// CAL-V0-321: the read checkpoint carries a request filter that holds every
// request path posted at or before its sequence, round-trips its codec, and
// is identical whether derived from a complete audit or from a writer audit
// resumed at a writer checkpoint (whose prefix requests the walk only looks
// up). A checkpoint without the filter, or with a malformed one, cannot
// serve a read: the read runs the complete audit.
func TestCALV0321_CheckpointRequestFilter(t *testing.T) {
	repo, r := refreshStore(t)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil {
		t.Fatal(err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil {
		t.Fatal(err)
	}
	appendRequest(t, repo, "tail-0")
	complete := checkpointed(t, repo, r)
	if complete.Requests == nil {
		t.Fatal("complete audit derived a read checkpoint without a request filter")
	}
	for i := 0; i < 6; i++ {
		p, _ := snapshot.RequestPath(fmt.Sprintf("refresh-%d", i))
		if !complete.Requests.mayHold(p) {
			t.Fatalf("filter misses prefix request %s", p)
		}
	}
	tailPath, _ := snapshot.RequestPath("tail-0")
	if !complete.Requests.mayHold(tailPath) {
		t.Fatal("filter misses the head's own request")
	}
	writer, _, err := r.AuditForWriter(wc, "", false)
	if err != nil || writer.Mode != ModeWriter {
		t.Fatalf("writer audit: %+v %v", writer, err)
	}
	if cp := writer.Checkpoint(); cp == nil || !bytes.Equal(cp.Encode(), complete.Encode()) {
		t.Fatal("writer-resumed read checkpoint differs from the complete audit's")
	}

	r.Checkpoint = complete
	if res, err := r.Audit(); err != nil || res.Mode != ModeCheckpoint {
		t.Fatalf("filtered checkpoint read: %+v %v", res, err)
	}
	bare := *complete
	bare.Requests = nil
	r.Checkpoint = &bare
	if res, err := r.Audit(); err != nil || res.Mode != ModeFull {
		t.Fatalf("a checkpoint without a request filter served the read: %+v %v", res, err)
	}

	for name, mutate := range map[string]func(*wire.Object){
		"bits not a power of two": func(o *wire.Object) { o.Set("bits", str("96")) },
		"bits below the minimum":  func(o *wire.Object) { o.Set("bits", str("32")) },
		"data length":             func(o *wire.Object) { o.Set("data", str("AAAA")) },
		"unknown key":             func(o *wire.Object) { o.Set("hashes", str("6")) },
	} {
		cp, _ := wire.Parse(complete.Encode())
		f, _ := cp.Obj.Get("requests")
		mutate(f.Obj)
		if _, err := DecodeCheckpoint(wire.EncodeFile(cp)); err == nil {
			t.Errorf("%s: malformed request filter decoded", name)
		}
	}
}

// CAL-V0-321: the filter never answers "not posted" for a request it holds,
// at every size step, and its false-positive rate stays below 0.1%.
func TestCALV0321_RequestFilterHasNoFalseNegatives(t *testing.T) {
	for _, n := range []int{0, 1, 3, 4, 5, 1000, 4096} {
		f := newRequestFilter(n)
		if m := len(f.bits) * 8; m < minRequestFilterBits || m&(m-1) != 0 || (n > 4 && m < n*requestFilterBitsPer) {
			t.Fatalf("n=%d: %d bits", n, m)
		}
		for i := 0; i < n; i++ {
			p, _ := snapshot.RequestPath(fmt.Sprintf("held-%d", i))
			d, _ := requestDigest(p)
			f.add(d)
		}
		for i := 0; i < n; i++ {
			p, _ := snapshot.RequestPath(fmt.Sprintf("held-%d", i))
			if !f.mayHold(p) {
				t.Fatalf("n=%d: false negative for %s", n, p)
			}
		}
		if n < 1000 {
			continue
		}
		hits := 0
		const probes = 100000
		for i := 0; i < probes; i++ {
			p, _ := snapshot.RequestPath(fmt.Sprintf("absent-%d", i))
			if f.mayHold(p) {
				hits++
			}
		}
		if hits*1000 > probes {
			t.Fatalf("n=%d: false-positive rate %d/%d", n, hits, probes)
		}
	}
}

// CAL-V0-321: a seal anchor and a writer checkpoint, which embed the
// checkpoint codec as their header, never carry the read checkpoint's request
// filter; one that does is refused rather than decoded.
func TestCALV0321_EmbeddedHeadersRefuseRequestFilter(t *testing.T) {
	_, r := refreshStore(t)
	full, observed, err := r.AuditForWriteObserved()
	if err != nil {
		t.Fatal(err)
	}
	wc, err := full.WriterCheckpoint(observed.Files)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := encodeAnchor(wc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeAnchor(anchor); err != nil {
		t.Fatalf("plain anchor: %v", err)
	}
	base, delta, err := wc.Encode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeWriterCheckpoint(delta, base); err != nil {
		t.Fatalf("plain writer checkpoint: %v", err)
	}
	filtered := *wc
	filtered.Checkpoint.Requests = newRequestFilter(1)
	if anchor, err := encodeAnchor(&filtered); err == nil {
		if _, err := decodeAnchor(anchor); err == nil {
			t.Fatal("a seal anchor carrying a request filter decoded")
		}
	}
	if base, delta, err := filtered.Encode(); err == nil {
		if _, err := DecodeWriterCheckpoint(delta, base); err == nil {
			t.Fatal("a writer checkpoint carrying a request filter decoded")
		}
	}
}
