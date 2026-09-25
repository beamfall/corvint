package contextindex

import (
	"bytes"
	"context"
	"errors"
	"io"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// gitBlobHashAllocationPayload is deliberately constructed before benchmark
// timing so the allocation ratchet measures only object framing and hashing.
var gitBlobHashAllocationPayload = bytes.Repeat([]byte("0123456789abcdef"), 48*1024)

func TestGitBlobHashMatchesKnownGitObjectIDs(t *testing.T) {
	for _, test := range []struct {
		name, format, data, want string
	}{
		{"sha1-empty", "sha1", "", "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"},
		{"sha1-hello", "sha1", "hello\n", "ce013625030ba8dba906f756967f9e9ca394464a"},
		{"sha256-empty", "sha256", "", "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"},
		{"sha256-hello", "sha256", "hello\n", "2cf8d83d9ee29543b34a87727421fdecb7e3f3a183d337639025de576db9ebb4"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := gitBlobHash([]byte(test.data), test.format); got != test.want {
				t.Fatalf("gitBlobHash(%q, %q) = %q, want %q", test.data, test.format, got, test.want)
			}
		})
	}
}

func TestGitBlobHashLargePayloadAllocationRatchet(t *testing.T) {
	allocations := testing.AllocsPerRun(100, func() {
		result := gitBlobHash(gitBlobHashAllocationPayload, "sha256")
		runtime.KeepAlive(result)
	})
	if allocations > 4 {
		t.Fatalf("gitBlobHash allocations = %.0f, want <= 4", allocations)
	}
	result := testing.Benchmark(benchmarkGitBlobHashLargePayload)
	if bytes := result.AllocedBytesPerOp(); bytes > 1024 {
		t.Fatalf("gitBlobHash allocation bytes = %d, want <= 1024", bytes)
	}
}

func BenchmarkGitBlobHashLargePayload(b *testing.B) {
	benchmarkGitBlobHashLargePayload(b)
}

func benchmarkGitBlobHashLargePayload(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(gitBlobHashAllocationPayload)))
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result := gitBlobHash(gitBlobHashAllocationPayload, "sha256")
		runtime.KeepAlive(result)
	}
}

func TestValidObjectIDRejectsMalformedTreeBlobIDs(t *testing.T) {
	for _, test := range []struct {
		format, value string
		want          bool
	}{
		{"sha1", strings.Repeat("a", 40), true},
		{"sha256", strings.Repeat("f", 64), true},
		{"sha1", strings.Repeat("a", 39), false},
		{"sha1", strings.Repeat("g", 40), false},
		{"sha256", strings.Repeat("A", 64), false},
	} {
		if got := validObjectID(test.value, test.format); got != test.want {
			t.Fatalf("validObjectID(%q, %q) = %v, want %v", test.value, test.format, got, test.want)
		}
	}
}

func TestParseStatusPreservesRenameEndpoints(t *testing.T) {
	paths, err := parseStatus([]byte("R  internal/token/renamed.go\x00internal/token/token.go\x00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/token/renamed.go", "internal/token/token.go"}
	if !slicesEqual(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestParseStatusRejectsMalformedAndNonUTF8Paths(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload []byte
		message string
	}{
		{"missing-terminator", []byte("?? path.py"), "malformed"},
		{"malformed-field", []byte("broken\x00"), "malformed"},
		{"empty-field", []byte("?? path.py\x00\x00"), "malformed"},
		{"non-utf8", []byte{'?', '?', ' ', 0xff, 0}, "not valid UTF-8"},
		{"missing-rename-source", []byte("R  renamed.go\x00"), "empty path"},
		{"empty-rename-source", []byte("R  renamed.go\x00\x00"), "empty path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseStatus(test.payload)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("parseStatus(%q) error = %v, want %q", test.payload, err, test.message)
			}
		})
	}
}

func TestParseBlobBatchPreservesBoundedImmutableBlobViews(t *testing.T) {
	leftOID := strings.Repeat("a", 40)
	sharedOID := strings.Repeat("b", 40)
	rightOID := strings.Repeat("c", 40)
	entries := []treeEntry{
		{path: "left.go", oid: leftOID, mode: "100644", size: len("left")},
		{path: "alias.go", oid: sharedOID, mode: "100644", size: len("shared")},
		{path: "shared.go", oid: sharedOID, mode: "100644", size: len("shared")},
		{path: "right.go", oid: rightOID, mode: "100644", size: len("right")},
	}
	raw := blobBatch(entries, []string{"left", "shared", "shared", "right"})
	blobs, err := parseBlobBatch(raw, entries)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(blobs[sharedOID]); got != "shared" {
		t.Fatalf("shared OID bytes = %q", got)
	}
	left := blobs[leftOID]
	if cap(left) != len(left) {
		t.Fatalf("left blob capacity = %d, want %d", cap(left), len(left))
	}
	left[0] = 'L'
	if got := string(blobs[rightOID]); got != "right" {
		t.Fatalf("direct mutation crossed blob boundary: %q", got)
	}
	extended := append(left, '!')
	if got := string(extended); got != "Left!" {
		t.Fatalf("appended blob = %q", got)
	}
	if got := string(blobs[rightOID]); got != "right" {
		t.Fatalf("append crossed blob boundary: %q", got)
	}
	runtime.GC()
	if got := string(blobs[rightOID]); got != "right" {
		t.Fatalf("blob bytes did not survive batch lifetime: %q", got)
	}
}

func TestParseBlobBatchRejectsHostileFraming(t *testing.T) {
	oid := strings.Repeat("a", 40)
	entries := []treeEntry{{path: "hostile.go", oid: oid, mode: "100644", size: 4}}
	for _, raw := range [][]byte{
		[]byte(oid + " blob 4\nbody"),
		[]byte(oid + " blob 4\nbody\x00"),
		[]byte(oid + " blob 5\nbody\n"),
		[]byte(oid + " tree 4\nbody\n"),
	} {
		if _, err := parseBlobBatch(raw, entries); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("parseBlobBatch(%q) error = %v, want malformed batch", raw, err)
		}
	}
}

func TestBlobAdmissionCountsEveryPathBeforeOIDDeduplication(t *testing.T) {
	oid := strings.Repeat("a", 40)
	entries := []treeEntry{{path: "left.go", oid: oid, size: maxBatchBytes - 128}, {path: "right.go", oid: oid, size: maxBatchBytes - 128}}
	if err := validateBlobAdmission(entries); err == nil {
		t.Fatal("repeated OID bypassed full path aggregate admission")
	}
	if _, err := uniqueBlobEntries([]treeEntry{{path: "left.go", oid: oid, size: 1}, {path: "right.go", oid: oid, size: 2}}); err == nil {
		t.Fatal("repeated OID with incompatible sizes accepted")
	}
}

// TestBlobAdmissionRefusalNamesTotalAndRemedyWithoutALanguage is V1-0339: a
// TypeScript repository over the aggregate bound was told its "native Go"
// index was too large, with neither the measured size nor a way out.
func TestBlobAdmissionRefusalNamesTotalAndRemedyWithoutALanguage(t *testing.T) {
	entries := []treeEntry{{path: "web/app.ts", size: maxBatchBytes - 128}, {path: "web/lib.ts", size: 1}}
	var refusal *Error
	if err := validateBlobAdmission(entries); !errors.As(err, &refusal) || refusal.Code != "unsupported-impact-repository" {
		t.Fatalf("error = %#v", err)
	}
	want := "repository index sources total 134217601 bytes in 2 files, 134217857 with per-file framing, over the 134217728-byte (128 MiB) aggregate bound; paths under vendor/, node_modules/, dist/, build/, target/ or generated/ are not admitted"
	if refusal.Message != want {
		t.Fatalf("message = %q, want %q", refusal.Message, want)
	}
}

func TestBlobBatchInputDeduplicatesRepeatedOIDsInFirstSeenOrder(t *testing.T) {
	left, right := strings.Repeat("a", 40), strings.Repeat("b", 40)
	unique, err := uniqueBlobEntries([]treeEntry{{path: "left.go", oid: left, size: 1}, {path: "alias.go", oid: right, size: 1}, {path: "again.go", oid: left, size: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(blobBatchInput(unique)), left+"\n"+right+"\n"; got != want {
		t.Fatalf("batch input=%q want=%q", got, want)
	}
}

func TestParseBlobBatchLargeCorpusAllocationRatchet(t *testing.T) {
	const count = 256
	entries := make([]treeEntry, 0, count)
	payloads := make([]string, 0, count)
	for index := 0; index < count; index++ {
		payload := strings.Repeat(string(rune('a'+index%26)), 1024)
		entries = append(entries, treeEntry{path: "internal/file.go", oid: strings.Repeat(string(rune('a'+index%8)), 40), mode: "100644", size: len(payload)})
		payloads = append(payloads, payload)
	}
	unique, err := uniqueBlobEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	if len(unique) != 8 {
		t.Fatalf("unique entries = %d, want 8", len(unique))
	}
	raw := blobBatch(unique, payloads[:len(unique)])
	allocations := testing.AllocsPerRun(100, func() {
		blobs, err := parseBlobBatch(raw, unique)
		if err != nil {
			t.Fatal(err)
		}
		runtime.KeepAlive(blobs)
	})
	if allocations > 3 {
		t.Fatalf("parseBlobBatch allocations = %.0f, want <= 3", allocations)
	}
	result := testing.Benchmark(func(b *testing.B) { benchmarkParseBlobBatchRepeated(b, raw, unique) })
	if bytes := result.AllocedBytesPerOp(); bytes > 1024 {
		t.Fatalf("parseBlobBatch bytes/op = %d, want <= 1024", bytes)
	}
	if allocations := result.AllocsPerOp(); allocations > 3 {
		t.Fatalf("parseBlobBatch allocs/op = %d, want <= 3", allocations)
	}
}

func BenchmarkParseBlobBatchRepeatedOIDs(b *testing.B) {
	entries, raw := repeatedBlobBatchBenchmarkFixture()
	benchmarkParseBlobBatchRepeated(b, raw, entries)
}

func repeatedBlobBatchBenchmarkFixture() ([]treeEntry, []byte) {
	entries := make([]treeEntry, 0, 512)
	payloads := make([]string, 0, 512)
	for index := 0; index < 512; index++ {
		payload := strings.Repeat(string(rune('a'+index%8)), 1024)
		entries = append(entries, treeEntry{path: "internal/file.go", oid: strings.Repeat(string(rune('a'+index%8)), 40), mode: "100644", size: len(payload)})
		payloads = append(payloads, payload)
	}
	unique, err := uniqueBlobEntries(entries)
	if err != nil {
		panic(err)
	}
	return unique, blobBatch(unique, payloads[:len(unique)])
}
func benchmarkParseBlobBatchRepeated(b *testing.B, raw []byte, entries []treeEntry) {
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	b.ReportMetric(float64(len(raw)), "retained-bytes/op")
	for index := 0; index < b.N; index++ {
		blobs, err := parseBlobBatch(raw, entries)
		if err != nil {
			b.Fatal(err)
		}
		runtime.KeepAlive(blobs)
	}
}

func blobBatch(entries []treeEntry, payloads []string) []byte {
	var batch bytes.Buffer
	for index, entry := range entries {
		batch.WriteString(entry.oid)
		batch.WriteString(" blob ")
		batch.WriteString(strconv.Itoa(len(payloads[index])))
		batch.WriteByte('\n')
		batch.WriteString(payloads[index])
		batch.WriteByte('\n')
	}
	return batch.Bytes()
}

// TestBoundedBufferEnforcesLimitThroughReadFrom pins the byte cap against the
// path os/exec actually uses. exec collects a non-*os.File stdout with
// io.Copy, which prefers an io.ReaderFrom destination over Write. While
// boundedBuffer embedded bytes.Buffer, that promoted ReadFrom ran instead of
// the capping Write, so maxIdentityBytes, maxStatusBytes, maxTreeBytes and
// maxBatchBytes were all inert. Exercising Write alone does not catch this.
func TestBoundedBufferEnforcesLimitThroughReadFrom(t *testing.T) {
	const limit = 64
	payload := strings.Repeat("A", 100_000)

	for _, test := range []struct {
		name  string
		write func(*boundedBuffer)
	}{
		{"io.Copy", func(buffer *boundedBuffer) {
			readerWithoutWriterTo := struct{ io.Reader }{strings.NewReader(payload)}
			_, _ = io.Copy(buffer, readerWithoutWriterTo)
		}},
		{"direct-write", func(buffer *boundedBuffer) {
			_, _ = buffer.Write([]byte(payload))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := &boundedBuffer{limit: limit}
			test.write(buffer)
			if !buffer.exceeded {
				t.Errorf("exceeded = false, want true after writing %d bytes past a %d-byte limit",
					len(payload), limit)
			}
			if got := len(buffer.Bytes()); got > limit {
				t.Errorf("retained %d bytes, want at most %d", got, limit)
			}
		})
	}
}

func TestGitFailsClosedOnOutputOverflow(t *testing.T) {
	raw, err := git(context.Background(), testRepository(t), 1, nil, "rev-parse", "HEAD")
	if err == nil || err.Error() != "Git output exceeds its byte limit" {
		t.Fatalf("git overflow error = %v, want Git output exceeds its byte limit", err)
	}
	if raw != nil {
		t.Fatalf("git overflow output = %q, want nil", raw)
	}
}

// TestBoundedBufferPromotesNoBypassingWriter fails if bytes.Buffer is ever
// re-embedded. ReadFrom is deliberately implemented here and enforces the cap
// itself (proved by TestBoundedBufferEnforcesLimitThroughReadFrom); the
// remaining methods have no cap-enforcing implementation, so satisfying their
// interfaces can only mean a promotion that reaches the underlying buffer
// directly.
func TestBoundedBufferPromotesNoBypassingWriter(t *testing.T) {
	var buffer any = &boundedBuffer{limit: 1}
	if _, declared := buffer.(io.ReaderFrom); !declared {
		t.Error("*boundedBuffer must implement io.ReaderFrom; without it io.Copy stages through a fresh 32 KiB buffer per Git subprocess")
	}
	if _, promoted := buffer.(io.StringWriter); promoted {
		t.Error("*boundedBuffer satisfies io.StringWriter; WriteString will bypass the byte limit")
	}
}
