package verify

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestCEMV1690OverlappingSpansShareOneBlob is CEM-V1-690: 64 overlapping
// near-full spans of one large blob are all accepted, and every retained span
// is a capacity-clipped window into one copy of the blob, so retained span
// storage is the blob's size rather than 64 times it.
func TestCEMV1690OverlappingSpansShareOneBlob(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const author, email, date = "A", "a@example.test", "2024-01-01T00:00:00Z"
	gitAt(t, root, author, email, date, "init", "-q")
	const size, records = 4 << 20, 64
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('a' + i%23)
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	gitAt(t, root, author, email, date, "add", ".")
	gitAt(t, root, author, email, date, "commit", "-qm", "base")
	base := gitAt(t, root, author, email, date, "rev-parse", "HEAD^{tree}")
	blob := gitAt(t, root, author, email, date, "rev-parse", "HEAD:big.txt")
	repository, err := gitauth.Open(root, gitrun.NewBudget(4096, 120e9))
	if err != nil {
		t.Fatal(err)
	}
	document := &wire.Map{}
	for i := 0; i < records; i++ {
		span := wire.Span{Start: int64(i), End: int64(size - records + i)}
		digest := sha256.Sum256(content[span.Start:span.End])
		spanSha256 := hex.EncodeToString(digest[:])
		document.Evidence = append(document.Evidence, wire.Evidence{
			ID: wire.EvidenceIdentity(blob, "big.txt", span, spanSha256), BlobOid: blob,
			Path: "big.txt", Span: span, SpanSha256: spanSha256,
		})
	}
	spans, err := checkEvidence(context.Background(), repository, document, base)
	if err != nil {
		t.Fatalf("checkEvidence refused 64 overlapping spans of one blob: %v", err)
	}
	first := document.Evidence[0]
	origin := uintptr(unsafe.Pointer(unsafe.SliceData(spans[first.ID])))
	for _, record := range document.Evidence {
		got := spans[record.ID]
		if !bytes.Equal(got, content[record.Span.Start:record.Span.End]) {
			t.Fatalf("span %d bytes differ from the blob", record.Span.Start)
		}
		if cap(got) != len(got) {
			t.Fatalf("span %d cap %d, want clipped to len %d", record.Span.Start, cap(got), len(got))
		}
		offset := uintptr(unsafe.Pointer(unsafe.SliceData(got))) - origin
		if offset != uintptr(record.Span.Start-first.Span.Start) {
			t.Fatalf("span %d is not a window into the first retained blob copy (offset %d)", record.Span.Start, offset)
		}
	}
}

// errCountingContext counts Err calls: ClassifySpanContext checks its context
// before each search, so the count bounds the searches it ran.
type errCountingContext struct {
	context.Context
	calls int
}

func (c *errCountingContext) Err() error {
	c.calls++
	return c.Context.Err()
}

// TestCEMV1691DriftStopsAtTheSecondMatch is CEM-V1-691: an adversarial
// repeated-byte needle in a repeated-byte target classifies after at most two
// searches (the uncapped count compared the 1 MiB needle at every one of 3 Mi
// offsets), SpanOccurrences counts exactly in linear time, and a cancelled
// context stops classification with an error. Linearity is a scaling ratio of
// this process's own runs, not a wall-clock constant, so a loaded -race shard
// cannot fail it; the stated 2 s bound is asserted without -race.
func TestCEMV1691DriftStopsAtTheSecondMatch(t *testing.T) {
	t.Parallel()
	haystack := bytes.Repeat([]byte("a"), 4<<20)
	needle := bytes.Repeat([]byte("a"), 1<<20)
	started := time.Now()
	counted := &errCountingContext{Context: context.Background()}
	status, span, err := ClassifySpanContext(counted, true, false, haystack, needle, wire.Span{Start: 0, End: 1 << 20})
	if err != nil || status != DriftAmbiguous || span != nil {
		t.Fatalf("ClassifySpanContext = %v, %v, %v; want ambiguous", status, span, err)
	}
	// One check before each of the two searches and one on return.
	if counted.calls > 3 {
		t.Fatalf("ClassifySpanContext checked its context %d times, want at most 3 (two searches)", counted.calls)
	}
	if got, want := SpanOccurrences(haystack, needle), 3<<20+1; got != want {
		t.Fatalf("SpanOccurrences = %d, want %d", got, want)
	}
	if elapsed := time.Since(started); !raceEnabled && elapsed > 2*time.Second {
		t.Fatalf("repeated-byte classification took %v, want under 2s", elapsed)
	}
	// Sixteen times the needle and target cost 16x when linear and 256x when
	// the needle is compared at every offset; 64x splits the two by 4x each
	// way. Each attempt takes the best of three interleaved runs of both
	// sizes; a load spike can inflate one attempt, while a quadratic search is
	// over the bound in every one, so the first attempt inside it decides.
	run := func(n int) time.Duration {
		haystack, needle := bytes.Repeat([]byte("a"), 4*n), bytes.Repeat([]byte("a"), n)
		started := time.Now()
		ClassifySpanContext(context.Background(), true, false, haystack, needle, wire.Span{})
		SpanOccurrences(haystack, needle)
		return time.Since(started)
	}
	const attempts = 5
	linear := false
	for attempt := 1; attempt <= attempts && !linear; attempt++ {
		small, large := time.Duration(1<<63-1), time.Duration(1<<63-1)
		for range 3 {
			small, large = min(small, run(64<<10)), min(large, run(1<<20))
		}
		t.Logf("attempt %d: repeated-byte classify+count best of 3: 64 KiB needle %v, 1 MiB needle %v (%.1fx)", attempt, small, large, float64(large)/float64(small))
		linear = large <= 64*small
	}
	if !linear {
		t.Fatalf("16x the input cost over 64x in each of %d attempts: not linear", attempts)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := ClassifySpanContext(ctx, true, false, haystack, needle, wire.Span{}); err == nil {
		t.Fatal("ClassifySpanContext ignored a cancelled context")
	}
	for _, tc := range []struct {
		haystack, needle string
		want             int
	}{{"aaaa", "aa", 3}, {"abababa", "aba", 3}, {"abc", "d", 0}, {"ab", "abc", 0}, {"abc", "", 0}, {"xx", "x", 2}} {
		if got := SpanOccurrences([]byte(tc.haystack), []byte(tc.needle)); got != tc.want {
			t.Errorf("SpanOccurrences(%q, %q) = %d, want %d", tc.haystack, tc.needle, got, tc.want)
		}
	}
}
