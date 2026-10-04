package contextindex

import (
	"bytes"
	"context"
	"errors"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestParseStatusNamesNonUTF8PathsInDisplayForm(t *testing.T) {
	paths, err := parseStatus([]byte("?? caf\xe9.txt\x00R  new\xff.go\x00old.go\x00"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"caf\uFFFD.txt", "new\uFFFD.go", "old.go"}
	if !slicesEqual(paths, want) {
		t.Fatalf("IDX-SNAP-V0-024: paths = %q, want %q", paths, want)
	}
}

func TestParseStatusRejectsMalformedRecords(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload []byte
		message string
	}{
		{"missing-terminator", []byte("?? path.py"), "malformed"},
		{"malformed-field", []byte("broken\x00"), "malformed"},
		{"empty-field", []byte("?? path.py\x00\x00"), "malformed"},
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

// ALO-V0-017: the shared executor retains index errors and charges each real
// process, including failed starts and status/index calls nested by its caller.
func TestAggregateGitExecutorErrorParity(t *testing.T) {
	if os.PathSeparator != '/' {
		t.Skip("Unix executable fixture")
	}
	for _, tc := range []struct {
		name, script string
		limit        int
	}{
		{"exit-empty", "exit 7", 1024},
		{"exit-detail", "echo detail >&2; exit 9", 1024},
		{"stdout-over", "printf 123456789", 4},
		{"stderr-over", "i=0; while [ $i -lt 7000 ]; do printf 1234567890 >&2; i=$((i+1)); done", 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "git")
			if err := os.WriteFile(path, []byte("#!/bin/sh\n"+tc.script+"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			ctx := context.WithValue(context.Background(), gitExecutionKey{}, gitExecution{executable: path, environment: os.Environ()})
			_, legacy := gitRaw(ctx, root, tc.limit, 0, nil, "status")
			budget, _ := gitrun.NewOperationBudget(64, time.Now().Add(time.Minute), false)
			_, aggregate := gitRaw(gitrun.WithOperationBudget(ctx, budget), root, tc.limit, 0, nil, "status")
			if legacy == nil || aggregate == nil || legacy.Error() != aggregate.Error() {
				t.Fatalf("legacy=%v aggregate=%v", legacy, aggregate)
			}
			if budget.Used() != 1 {
				t.Fatalf("physical spawns=%d", budget.Used())
			}
		})
	}
}

func TestAggregateIndexAdmissionAndPhysicalBatchBoundary(t *testing.T) {
	// Independent 128-byte-per-file admission oracle, with every source below
	// the 1,000,000-byte source limit. No production sizing helper sets want.
	const limit = 128 * 1024 * 1024
	const count = 135
	entries := make([]treeEntry, count)
	remaining := limit - count*128
	for i := range entries {
		size := 999999
		if i == count-1 {
			size = remaining
		}
		entries[i] = treeEntry{path: "source/f" + strconv.Itoa(i) + ".go", size: size, oid: strings.Repeat("a", 40)}
		remaining -= size
	}
	if remaining != 0 || entries[count-1].size <= 0 || entries[count-1].size > 1000000 {
		t.Fatal("independent fixture invalid")
	}
	if err := validateBlobAdmission(entries); err != nil {
		t.Fatalf("exact admission: %v", err)
	}
	entries[count-1].size++
	if err := validateBlobAdmission(entries); err == nil || !strings.Contains(err.Error(), "134217729 with per-file framing") {
		t.Fatalf("plus-one admission: %v", err)
	}

	// The physical cat-file ceiling has smaller actual headers than the
	// conservative admission allowance. Exercise it separately, without
	// mislabelling this lower-level acquisition as end-to-end admission.
	root := t.TempDir()
	git := func(input []byte, args ...string) []byte {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", root}, args...)...)
		command.Stdin = bytes.NewReader(input)
		out, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
		return out
	}
	git(nil, "init", "-q")
	physical := make([]treeEntry, 0, count)
	actualTotal := 0
	for i := 0; i < count; i++ {
		size := 999999
		if i == count-1 {
			// This remaining body has six decimal digits; its exact SHA-1
			// header plus trailing newline is 40+6+6+1+1 bytes.
			size = limit - actualTotal - (40 + 6 + 6 + 1 + 1)
		}
		payload := bytes.Repeat([]byte{'x'}, size)
		copy(payload, []byte("package source\n// blob "+strconv.Itoa(i)+"\n"))
		oid := strings.TrimSpace(string(git(payload, "hash-object", "-w", "--stdin")))
		if len(oid) != 40 || size > 1000000 || size < 100000 {
			t.Fatal("physical fixture identity/size invalid")
		}
		actualTotal += len(oid) + len(" blob ") + len(strconv.Itoa(size)) + 1 + size + 1
		physical = append(physical, treeEntry{path: "source/f" + strconv.Itoa(i) + ".go", oid: oid, size: size})
	}
	if actualTotal != limit {
		t.Fatalf("independent physical framing total=%d", actualTotal)
	}
	if err := validateBlobAdmission(physical); err == nil {
		t.Fatal("expected earlier conservative end-to-end refusal")
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	blobs, err := fetchBlobs(context.Background(), root, physical)
	if err != nil || len(blobs) != count {
		t.Fatalf("exact physical acquisition len=%d: %v", len(blobs), err)
	}
	for _, entry := range physical {
		if len(blobs[entry.oid]) != entry.size {
			t.Fatal("truncated blob")
		}
	}
	runtime.ReadMemStats(&after)
	t.Logf("physical exact bytes=%d elapsed=%s totalAllocDelta=%d", actualTotal, time.Since(start), after.TotalAlloc-before.TotalAlloc)
	blobs = nil
	runtime.GC()
	last := &physical[count-1]
	payload := bytes.Repeat([]byte{'y'}, last.size+1)
	last.size++
	last.oid = strings.TrimSpace(string(git(payload, "hash-object", "-w", "--stdin")))
	if _, err = fetchBlobs(context.Background(), root, physical); err == nil {
		t.Fatal("physical plus-one acquisition accepted")
	}
}

// IDX-SNAP-V0-017 and ALO-V0-017: the budgeted executor returns byte-identical
// successful Git output, with and without stdin, so the re-pinned analyzer
// audit needs no schema bump.
func TestAggregateGitExecutorSuccessParity(t *testing.T) {
	root := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "parity@example.invalid"},
		{"config", "user.name", "Parity"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "café \"q\".go"), []byte("package p\n// é\x00\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "parity"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	tree, err := gitRaw(context.Background(), root, 1<<20, 0, nil, "ls-tree", "-r", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	oid := strings.Fields(string(tree))[2]
	for _, call := range []struct {
		stdin []byte
		args  []string
	}{
		{nil, []string{"ls-tree", "-r", "-z", "--long", "HEAD"}},
		{nil, []string{"ls-tree", "-r", "HEAD"}},
		{nil, []string{"log", "--format=%H%x00%T%x00%s", "HEAD"}},
		{[]byte(oid + "\n"), []string{"cat-file", "--batch"}},
	} {
		legacy, err := gitRaw(context.Background(), root, 1<<20, 64, call.stdin, call.args...)
		if err != nil {
			t.Fatal(err)
		}
		budget, _ := gitrun.NewOperationBudget(64, time.Now().Add(time.Minute), false)
		aggregate, err := gitRaw(gitrun.WithOperationBudget(context.Background(), budget), root, 1<<20, 64, call.stdin, call.args...)
		if err != nil || !bytes.Equal(legacy, aggregate) || len(legacy) == 0 || budget.Used() != 1 {
			t.Fatalf("%v: err=%v equal=%v used=%d", call.args, err, bytes.Equal(legacy, aggregate), budget.Used())
		}
	}
}
