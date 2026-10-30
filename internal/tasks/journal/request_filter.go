package journal

import (
	"encoding/base64"
	"encoding/binary"
	"math/bits"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// requestFilterHashes is the number of filter bits one request path sets.
// The path's SHA-256 digest is already uniform, so the positions are double
// hashing of its first two 64-bit little-endian words, without rehashing
// (PSR-V0-203).
const requestFilterHashes = 11

// Filter sizing: at least requestFilterBitsPer bits per request, rounded up
// to a power of two, so a read's false-positive rate per tail request is
// about 0.05% at 16 bits and falls as the size rounds up.
const (
	requestFilterBitsPer = 16
	minRequestFilterBits = 64
	// maxRequestFilterBits covers 2,100,000 requests at 16 bits each
	// rounded up (2^26) and bounds what a reader decodes: 8 MiB of bits.
	maxRequestFilterBits = 1 << 26
)

// RequestFilter is a no-false-negative set of the request paths receipts
// 1..Seq posted, carried by a read checkpoint (CAL-V0-321). A path it does
// not hold was certainly not posted before the checkpoint; a path it holds
// may have been, and the read declines the checkpoint route so the complete
// audit classifies it (PSR-V0-203). It never changes what a read concludes.
type RequestFilter struct {
	bits []byte
}

func newRequestFilter(n int) *RequestFilter {
	want := uint64(n) * requestFilterBitsPer
	m := uint64(minRequestFilterBits)
	if want > m {
		m = 1 << bits.Len64(want-1)
	}
	if m > maxRequestFilterBits {
		m = maxRequestFilterBits
	}
	return &RequestFilter{bits: make([]byte, m/8)}
}

func (f *RequestFilter) positions(d []byte, fn func(i uint64)) {
	mask := uint64(len(f.bits)*8 - 1)
	h1, h2 := binary.LittleEndian.Uint64(d), binary.LittleEndian.Uint64(d[8:])|1
	for k := uint64(0); k < requestFilterHashes; k++ {
		fn((h1 + k*h2) & mask)
	}
}

// add records request path digest d (32 bytes).
func (f *RequestFilter) add(d []byte) {
	f.positions(d, func(i uint64) { f.bits[i/8] |= 1 << (i % 8) })
}

// mayHold reports whether request path p may have been posted at or before
// the checkpoint. A path that is not a canonical request path may.
func (f *RequestFilter) mayHold(p string) bool {
	d, ok := requestDigest(p)
	if !ok {
		return true
	}
	held := true
	f.positions(d, func(i uint64) { held = held && f.bits[i/8]&(1<<(i%8)) != 0 })
	return held
}

func (f *RequestFilter) value() wire.Value {
	o := wire.NewObject()
	o.Set("bits", wire.String(string(wire.SizeOf(uint64(len(f.bits)*8)))))
	o.Set("data", wire.String(base64.StdEncoding.EncodeToString(f.bits)))
	return wire.ObjectValue(o)
}

func readRequestFilter(r *wire.Reader) *RequestFilter {
	r.Closed("bits", "data")
	m := r.Field("bits").Size().Uint64()
	data := r.Field("data").String()
	if r.Err() != nil {
		return nil
	}
	if m < minRequestFilterBits || m > maxRequestFilterBits || m&(m-1) != 0 {
		r.Fail(wire.CodeMalformed, "request filter bits must be a power of two in 64..2^26")
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || uint64(len(raw))*8 != m || base64.StdEncoding.EncodeToString(raw) != data {
		r.Fail(wire.CodeMalformed, "request filter data is not the canonical encoding of its bits")
		return nil
	}
	return &RequestFilter{bits: raw}
}
