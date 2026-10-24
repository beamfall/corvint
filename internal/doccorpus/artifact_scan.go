package doccorpus

import (
	"bytes"
	"runtime"
	"sync"
)

// boundaryScan is how a streamed or in-memory decode finds the elements of a split member
// (DCR-V1-551). A window region holding at least two segments of minSegment bytes is cut into at
// most segments segments that are scanned in parallel; a smaller region, and segments 1, is
// scanned sequentially. The zero value is the default: 32 KiB segments, one per GOMAXPROCS.
type boundaryScan struct{ minSegment, segments int }

// scanRegion is the most window bytes one parallel scan takes, so the decode workers start on
// the first elements of an in-memory artifact while the rest is still being scanned.
const scanRegion = 4 << 20

func (b boundaryScan) plan() (minSegment, segments int) {
	if b.segments == 0 {
		return 32 << 10, runtime.GOMAXPROCS(0)
	}
	return max(b.minSegment, 1), b.segments
}

// windowSpan is one element's bounds in the window.
type windowSpan struct{ begin, end int }

// parallelElements finds the split elements that follow one another from the scan position,
// which must be at the first byte of an element, and returns their bounds. They are exactly the
// elements the sequential scan finds from there, each directly followed by one comma, so the scan
// resumes after the last of them as if it had found them itself (DCR-V1-551):
//
//  1. The first element's bytes up to its first member name give a separator: a comma, then
//     those bytes. In a canonical artifact every element of a split member starts with the same
//     member name, and the separator cannot occur inside a string, where a quote is escaped.
//  2. The region is cut into segments that are scanned in parallel. The first starts at the scan
//     position; each other guesses that its first element starts just after its first separator.
//     From there each scans whole elements with scanValue, the sequential scan's own function,
//     until an element starts after the segment.
//  3. The runs are joined in order while each one's first element starts where its predecessor's
//     next element does; a segment with no separator is passed over while an element spans it.
//     The first segment starts at the scan position, so by induction every joined element starts
//     where the sequential scan's would; a wrong guess, an anomaly or the end of the window only
//     ends the join early.
//
// It returns nothing for a region too short to split, one already scanned or an element without
// a member name, so the sequential scan takes such elements one by one.
func (d *artifactDecoder) parallelElements() []windowSpan {
	minSegment, segments := d.boundary.plan()
	from := d.pos
	to := min(len(d.buf), from+scanRegion)
	k := min(segments, (to-from)/minSegment)
	if k < 2 || d.offset() < d.scanned {
		return nil
	}
	sep := elementSeparator(d.buf[from:to])
	if sep == nil {
		return nil
	}
	d.scanned = d.base + int64(to)
	data := d.buf
	bounds := make([]int, k+1)
	for j := range bounds {
		bounds[j] = from + (to-from)*j/k
	}
	runs := make([]elementRun, k)
	var wg sync.WaitGroup
	for j := range runs {
		wg.Go(func() { runs[j] = scanRun(data, bounds[j], bounds[j+1], sep, j == 0) })
	}
	wg.Wait()
	var spans []windowSpan
	next := from
	for j, r := range runs {
		if r.first != next {
			if r.first < 0 && next >= bounds[j+1] {
				continue // the element before next spans the segment
			}
			break
		}
		spans = append(spans, r.spans...)
		if r.next < 0 {
			break
		}
		next = r.next
	}
	return spans
}

// scanSeparatorName is the longest member name a separator holds.
const scanSeparatorName = 64

// elementSeparator is a comma followed by the bytes of the object element at data[0] up to the
// colon after its first member name, or nil when data holds no such name.
func elementSeparator(data []byte) []byte {
	if len(data) < 2 || data[0] != '{' || data[1] != '"' {
		return nil
	}
	for i := 2; i < len(data) && i < 2+scanSeparatorName; i++ {
		switch data[i] {
		case '\\':
			return nil
		case '"':
			if i+1 < len(data) && data[i+1] == ':' {
				return append([]byte{','}, data[:i+2]...)
			}
			return nil
		}
	}
	return nil
}

// elementRun is one segment's elements: the start of its first, or -1 when the segment holds no
// separator, those that start before the segment ends, and the start of the next element after
// them, or -1 when the run stopped first.
type elementRun struct {
	first, next int
	spans       []windowSpan
}

// scanRun scans the elements of data[lo:hi]. The first segment starts at an element; any other
// guesses that its first element starts just after its first separator.
func scanRun(data []byte, lo, hi int, sep []byte, first bool) elementRun {
	start := lo
	if !first {
		at := bytes.Index(data[lo-1:min(len(data), hi-1+len(sep))], sep)
		if at < 0 {
			return elementRun{first: -1, next: -1}
		}
		start = lo + at
	}
	r := elementRun{first: start, next: -1}
	for start < hi {
		if data[start] != '{' && data[start] != '[' {
			return r
		}
		end, scanned := scanValue(data, start)
		if scanned != scanOK || end >= len(data) || data[end] != ',' {
			return r
		}
		r.spans = append(r.spans, windowSpan{start, end})
		start = end + 1
	}
	r.next = start
	return r
}
