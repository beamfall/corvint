package journal

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/iotest"
)

// TestReadAllSizedMatchesReadAll pins the stat-sized native read to
// io.ReadAll's bytes and errors for files that match, shrank below and grew
// past their stat size, under the same max+1 limit reader.
func TestReadAllSizedMatchesReadAll(t *testing.T) {
	data := bytes.Repeat([]byte("receipt-bytes-"), 400)
	boom := errors.New("boom")
	for _, c := range []struct {
		name string
		r    func() io.Reader
		size int64
	}{
		{"exact", func() io.Reader { return bytes.NewReader(data) }, int64(len(data))},
		{"grew", func() io.Reader { return iotest.HalfReader(bytes.NewReader(data)) }, 5},
		{"shrank", func() io.Reader { return bytes.NewReader(data[:9]) }, int64(len(data))},
		{"limited", func() io.Reader { return io.LimitReader(bytes.NewReader(data), 100) }, int64(len(data))},
		{"negative", func() io.Reader { return bytes.NewReader(data[:3]) }, -1},
		{"error", func() io.Reader { return io.MultiReader(bytes.NewReader(data[:10]), iotest.ErrReader(boom)) }, 10},
	} {
		want, wantErr := io.ReadAll(c.r())
		got, err := readAllSized(c.r(), c.size)
		if !bytes.Equal(got, want) || !errors.Is(err, wantErr) && err != wantErr {
			t.Fatalf("%s: got %d bytes %v, want %d bytes %v", c.name, len(got), err, len(want), wantErr)
		}
	}
}
