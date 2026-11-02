package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"syscall"
	"testing"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/snapshot"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// sealedRefusals runs one complete audit of repo with the segment
// read-ahead off and one with it on, calling arm before each, and returns
// both errors as text.
func sealedRefusals(t *testing.T, repo *intent.Repository, arm func()) (sequential, prefetched string) {
	t.Helper()
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range []bool{false, true} {
		restore := journal.SetSegmentPrefetchForTest(on)
		arm()
		r := journalReader(repo, head)
		r.SelectState = true
		_, err := r.Audit()
		restore()
		if on {
			prefetched = fmt.Sprint(err)
		} else {
			sequential = fmt.Sprint(err)
		}
	}
	return sequential, prefetched
}

// LGL-V0-813: a transient read error loading a segment is reported exactly
// as the sequential sealed walk reports it -- the read-ahead keeps it and
// never retries the load into a success.
func TestLGLV0813_TransientSegmentLoadErrorIsNotRetried(t *testing.T) {
	// Not parallel: it switches process-wide test seams.
	repo := historyStore(t, 3*snapshot.SegmentReceipts+40)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	m, err := boundManifest(repo, head)
	if err != nil {
		t.Fatal(err)
	}
	target := snapshot.SegmentName(m.Segments[1].First, m.Segments[1].Last)
	var failures int
	defer journal.SetSegmentLoadFaultForTest(func(name string) error {
		if name == target && failures == 0 {
			failures++
			return &os.PathError{Op: "read", Path: name, Err: syscall.EIO}
		}
		return nil
	})()
	seq, pre := sealedRefusals(t, repo, func() { failures = 0 })
	if seq == "<nil>" || !strings.Contains(seq, "input/output error") {
		t.Fatalf("sequential walk did not report the injected EIO: %s", seq)
	}
	if seq != pre {
		t.Fatalf("refusals differ:\nsequential %s\nprefetched %s", seq, pre)
	}
}

// LGL-V0-813: a segment that seals a request an earlier segment already
// sealed, after one of its own, is refused on that duplicate path by both
// walks: a refused load leaves the view's requests unchanged, so nothing is
// reported against the segment's own earlier request.
func TestLGLV0813_MidSegmentDuplicateRefusedTheSameWay(t *testing.T) {
	// Not parallel: it switches the process-wide segment read-ahead.
	repo := historyStore(t, 3*snapshot.SegmentReceipts+40)
	if err := sealSegmentsLocked(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	head, err := readHead(repo)
	if err != nil {
		t.Fatal(err)
	}
	m, err := boundManifest(repo, head)
	if err != nil {
		t.Fatal(err)
	}
	read := func(e snapshot.SegmentEntry) *snapshot.Segment {
		t.Helper()
		raw, err := os.ReadFile(segmentPath(repo, snapshot.SegmentName(e.First, e.Last)))
		if err != nil {
			t.Fatal(err)
		}
		s, err := snapshot.DecodeSegment(raw, e.First, e.Last)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	one, two := read(m.Segments[0]), read(m.Segments[1])
	// The duplicate must sort after at least one of segment two's own
	// requests, so a load that kept requests as it went would have kept
	// that one before refusing.
	var dup snapshot.SegmentFile
	for _, f := range one.Requests {
		if f.Path > two.Requests[0].Path {
			dup = f
			break
		}
	}
	if dup.Path == "" {
		t.Skip("no request of segment one sorts after segment two's first")
	}
	two.Requests = append(two.Requests, dup)
	sort.Slice(two.Requests, func(i, j int) bool { return two.Requests[i].Path < two.Requests[j].Path })
	raw, err := snapshot.EncodeSegment(two)
	if err != nil {
		t.Fatal(err)
	}
	e := &m.Segments[1]
	if err := os.WriteFile(segmentPath(repo, snapshot.SegmentName(e.First, e.Last)), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	e.Sha256, e.Bytes, e.Requests = wire.Sum(raw), uint64(len(raw)), uint64(len(two.Requests))
	manifestRaw := wire.EncodeFile(m.Value())
	if err := os.WriteFile(segmentPath(repo, snapshot.SegmentManifestName(m.Through)), manifestRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	next := *head
	next.Segments = &snapshot.SegmentBinding{Through: head.Segments.Through, ManifestSha256: wire.Sum(manifestRaw)}
	if err := os.WriteFile(statePath(repo, "head.json"), wire.EncodeFile(next.Value()), 0o600); err != nil {
		t.Fatal(err)
	}
	seq, pre := sealedRefusals(t, repo, func() {})
	if !strings.Contains(seq, dup.Path) || !strings.Contains(seq, "request sealed in more than one segment") {
		t.Fatalf("sequential walk did not refuse the duplicate %s: %s", dup.Path, seq)
	}
	if seq != pre {
		t.Fatalf("refusals differ:\nsequential %s\nprefetched %s", seq, pre)
	}
}
