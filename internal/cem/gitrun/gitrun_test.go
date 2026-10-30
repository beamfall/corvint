package gitrun

import (
	"io"
	"strings"
	"testing"
)

// TestLimitedBufferSatisfiesNeitherReaderFromNorStringWriter guards against a
// future embedding of bytes.Buffer into limitedBuffer. os/exec collects a
// non-*os.File stdout with io.Copy, which prefers an io.ReaderFrom
// destination over the capping Write below; an embedded bytes.Buffer would
// promote ReadFrom (and WriteString/WriteByte/WriteRune) and silently bypass
// every StdoutLimit and StderrLimit in this package. See the equivalent fix
// and tests in internal/contextindex/git.go (boundedBuffer).
func TestLimitedBufferSatisfiesNeitherReaderFromNorStringWriter(t *testing.T) {
	var buffer any = &limitedBuffer{limit: 1}
	if _, promoted := buffer.(io.ReaderFrom); promoted {
		t.Error("*limitedBuffer satisfies io.ReaderFrom; io.Copy would bypass the byte limit")
	}
	if _, promoted := buffer.(io.StringWriter); promoted {
		t.Error("*limitedBuffer satisfies io.StringWriter; WriteString would bypass the byte limit")
	}
}

// TestLimitedBufferEnforcesLimitThroughIOCopy pins the cap against the path
// os/exec actually uses to collect a non-*os.File stdout.
func TestLimitedBufferEnforcesLimitThroughIOCopy(t *testing.T) {
	const limit = 64
	payload := strings.Repeat("A", 100_000)

	buffer := newLimitedBuffer(limit, 0, nil)
	n, err := io.Copy(buffer, strings.NewReader(payload))
	if err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if int(n) != len(payload) {
		t.Fatalf("io.Copy reported %d bytes copied, want %d", n, len(payload))
	}
	data, exceeded := buffer.snapshot()
	if !exceeded {
		t.Errorf("exceeded = false, want true after writing %d bytes past a %d-byte limit", len(payload), limit)
	}
	if len(data) > limit {
		t.Errorf("retained %d bytes, want at most %d", len(data), limit)
	}
}

func TestNewLimitedBufferPreallocatesCappedAtLimit(t *testing.T) {
	tests := []struct {
		name         string
		limit, hint  int
		wantCapacity int
	}{
		{"hint below limit preallocates the hint", 1000, 100, 100},
		{"hint above limit is clamped to the limit", 100, 1000, 100},
		{"zero hint preallocates nothing", 1000, 0, 0},
		{"negative hint preallocates nothing", 1000, -5, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer := newLimitedBuffer(test.limit, test.hint, nil)
			if got := cap(buffer.data); got != test.wantCapacity {
				t.Errorf("cap(data) = %d, want %d", got, test.wantCapacity)
			}
		})
	}
}

// TestLimitedBufferWriteWithHintMatchesUnhinted pins that a size hint changes
// only the allocation shape, never the retained bytes.
func TestLimitedBufferWriteWithHintMatchesUnhinted(t *testing.T) {
	payload := []byte(strings.Repeat("B", 5000))
	const limit = 10_000
	const chunkSize = 4096

	unhinted := newLimitedBuffer(limit, 0, nil)
	hinted := newLimitedBuffer(limit, len(payload), nil)
	for offset := 0; offset < len(payload); offset += chunkSize {
		end := offset + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		if _, err := unhinted.Write(payload[offset:end]); err != nil {
			t.Fatalf("unhinted Write: %v", err)
		}
		if _, err := hinted.Write(payload[offset:end]); err != nil {
			t.Fatalf("hinted Write: %v", err)
		}
	}
	gotUnhinted, _ := unhinted.snapshot()
	gotHinted, _ := hinted.snapshot()
	if string(gotUnhinted) != string(payload) {
		t.Fatalf("unhinted payload = %q, want %q", gotUnhinted, payload)
	}
	if string(gotHinted) != string(payload) {
		t.Fatalf("hinted payload = %q, want %q", gotHinted, payload)
	}
}

// BenchmarkLimitedBufferWrite drives limitedBuffer.Write the way os/exec's
// io.Copy would for `corvint init`'s cat-file --batch payload (~53MB),
// comparing the unhinted growth-ladder path against a caller-supplied size
// hint capped at the limit.
func BenchmarkLimitedBufferWrite(b *testing.B) {
	const payloadSize = 53 << 20 // matches the measured init payload
	const chunkSize = 32 << 10   // typical io.Copy chunk size
	chunk := make([]byte, chunkSize)

	b.Run("unhinted", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buffer := newLimitedBuffer(payloadSize+1, 0, nil)
			for written := 0; written < payloadSize; written += chunkSize {
				_, _ = buffer.Write(chunk)
			}
		}
	})

	b.Run("hinted", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			buffer := newLimitedBuffer(payloadSize+1, payloadSize+1, nil)
			for written := 0; written < payloadSize; written += chunkSize {
				_, _ = buffer.Write(chunk)
			}
		}
	})
}
