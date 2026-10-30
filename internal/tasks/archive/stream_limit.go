package archive

import (
	"github.com/Beamfall/corvint/internal/tasks/wire"
	"io"
)

func streamLimitError(limit uint64) error {
	return wire.Errorf(wire.CodeLimitExceeded, "stream", "encoded tar stream exceeds %d bytes", limit)
}

// streamWriter refuses a write before it would grow the staging file beyond the
// complete tar budget. Previously staged bytes are private and are removed.
type streamWriter struct {
	w        io.Writer
	limit, n uint64
	exceeded bool
}

func (w *streamWriter) Write(p []byte) (int, error) {
	if uint64(len(p)) > w.limit-w.n {
		w.exceeded = true
		return 0, streamLimitError(w.limit)
	}
	n, err := w.w.Write(p)
	w.n += uint64(n)
	return n, err
}

// streamReader limits actual encoded bytes, including PAX headers and padding.
// At the boundary a one-byte lookahead distinguishes exact EOF from overflow;
// the extra byte is never passed to the tar decoder or retained.
type streamReader struct {
	sourceErr error // first non-EOF source error survives tar/io.ReadFull
	r         io.Reader
	limit, n  uint64
	exceeded  bool
}

func (r *streamReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	remaining := r.limit - r.n
	if remaining == 0 {
		var one [1]byte
		n, err := r.readSource(one[:])
		if n != 0 {
			r.exceeded = true
			return 0, streamLimitError(r.limit)
		}
		return 0, err
	}
	if uint64(len(p)) > remaining {
		p = p[:int(remaining)]
	}
	n, err := r.readSource(p)
	r.n += uint64(n)
	return n, err
}

func (r *streamReader) readSource(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF && r.sourceErr == nil {
		r.sourceErr = err
	}
	return n, err
}
