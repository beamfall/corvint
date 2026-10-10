package intent

import (
	"bytes"
	"io"
	"runtime"
	"testing"
	"testing/iotest"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestReadAllBoundsAndBytes pins readAll's contract across hints and chunked
// readers: exact bytes for a file that matches, shrank below or grew past its
// stat hint, LIMIT_EXCEEDED once more than max bytes arrive, and no 32 KiB
// scratch buffer per small file (V1-1053).
func TestReadAllBoundsAndBytes(t *testing.T) {
	data := bytes.Repeat([]byte("0123456789abcdef"), 300) // 4800 bytes
	for _, c := range []struct {
		name string
		r    func() io.Reader
		max  int
		hint int
		want []byte
		err  string
	}{
		{"exact hint", func() io.Reader { return bytes.NewReader(data) }, 1 << 20, len(data), data, ""},
		{"one byte reads", func() io.Reader { return iotest.OneByteReader(bytes.NewReader(data)) }, 1 << 20, len(data), data, ""},
		{"grew past hint", func() io.Reader { return iotest.HalfReader(bytes.NewReader(data)) }, 1 << 20, 10, data, ""},
		{"shrank below hint", func() io.Reader { return bytes.NewReader(data[:7]) }, 1 << 20, len(data), data[:7], ""},
		{"negative hint", func() io.Reader { return bytes.NewReader(data[:100]) }, 200, -1, data[:100], ""},
		{"at max", func() io.Reader { return bytes.NewReader(data[:64]) }, 64, 64, data[:64], ""},
		{"over max with hint", func() io.Reader { return bytes.NewReader(data[:65]) }, 64, 64, nil, wire.CodeLimitExceeded},
		{"over max small hint", func() io.Reader { return iotest.HalfReader(bytes.NewReader(data)) }, 1000, 3, nil, wire.CodeLimitExceeded},
		{"empty", func() io.Reader { return bytes.NewReader(nil) }, 10, 0, []byte{}, ""},
	} {
		got, err := readAll(c.r(), "p", c.max, c.hint)
		if c.err != "" {
			if wire.CodeOf(err) != c.err || got != nil {
				t.Fatalf("%s: got %d bytes, %v; want %s", c.name, len(got), err, c.err)
			}
			continue
		}
		if err != nil || !bytes.Equal(got, c.want) {
			t.Fatalf("%s: got %d bytes, %v; want %d bytes", c.name, len(got), err, len(c.want))
		}
	}
	small := data[:3000]
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := 0; i < 100; i++ {
		if _, err := readAll(bytes.NewReader(small), "p", 1<<20, len(small)); err != nil {
			t.Fatal(err)
		}
	}
	runtime.ReadMemStats(&after)
	if per := (after.TotalAlloc - before.TotalAlloc) / 100; per > 2*uint64(len(small)) {
		t.Fatalf("readAll allocated %d bytes per %d-byte file", per, len(small))
	}
}
