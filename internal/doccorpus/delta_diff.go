package doccorpus

// DeltaHunk is one changed line range between two blobs (DCX-V1-363): 1-based
// starts and counts on each side. A pure insertion has OldCount 0 and OldStart
// at the old line it precedes; a pure deletion has NewCount 0. Coarse marks a
// hunk that stands for every line between the common prefix and suffix because
// the edit distance passed the diff bound.
type DeltaHunk struct {
	OldStart int  `json:"old_start"`
	OldCount int  `json:"old_count"`
	NewStart int  `json:"new_start"`
	NewCount int  `json:"new_count"`
	Coarse   bool `json:"coarse,omitempty"`
}

// deltaMaxEdits bounds the Myers edit distance one line diff explores; past it
// the diff is one coarse hunk, never a guessed alignment.
const deltaMaxEdits = 1024

// splitLines returns the lines of data, each with its newline, with no empty
// trailing line for a final newline: the line numbering anchors use.
func splitLines(data []byte) []string {
	lines := []string{}
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i+1]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// lineDiff is a bounded Myers O(ND) diff of two line sequences. It trims the
// common prefix and suffix, and returns one coarse hunk when the remaining edit
// distance exceeds maxEdits.
func lineDiff(a, b []string, maxEdits int) []DeltaHunk {
	prefix := 0
	for prefix < len(a) && prefix < len(b) && a[prefix] == b[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(a)-prefix && suffix < len(b)-prefix && a[len(a)-1-suffix] == b[len(b)-1-suffix] {
		suffix++
	}
	a, b = a[prefix:len(a)-suffix], b[prefix:len(b)-suffix]
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	if len(a) == 0 || len(b) == 0 {
		return []DeltaHunk{{OldStart: prefix + 1, OldCount: len(a), NewStart: prefix + 1, NewCount: len(b)}}
	}
	ids := map[string]int32{}
	intern := func(lines []string) []int32 {
		out := make([]int32, len(lines))
		for i, l := range lines {
			id, ok := ids[l]
			if !ok {
				id = int32(len(ids))
				ids[l] = id
			}
			out[i] = id
		}
		return out
	}
	x, y := intern(a), intern(b)
	ops, ok := myers(x, y, maxEdits)
	if !ok {
		return []DeltaHunk{{OldStart: prefix + 1, OldCount: len(a), NewStart: prefix + 1, NewCount: len(b), Coarse: true}}
	}
	hunks := []DeltaHunk{}
	i, j := 0, 0
	var open *DeltaHunk
	for _, op := range ops {
		switch op {
		case '=':
			if open != nil {
				hunks = append(hunks, *open)
				open = nil
			}
			i++
			j++
			continue
		}
		if open == nil {
			open = &DeltaHunk{OldStart: prefix + i + 1, NewStart: prefix + j + 1}
		}
		if op == '-' {
			open.OldCount++
			i++
		} else {
			open.NewCount++
			j++
		}
	}
	if open != nil {
		hunks = append(hunks, *open)
	}
	return hunks
}

// myers returns the edit script ('=', '-', '+') of a shortest edit from a to
// b, or false when it needs more than maxEdits edits. Each round keeps only
// its diagonal band, so memory is O(D^2) for D <= maxEdits.
func myers(a, b []int32, maxEdits int) ([]byte, bool) {
	n, m := len(a), len(b)
	limit := min(n+m, maxEdits)
	offset := limit + 1
	v := make([]int32, 2*limit+3)
	trace := [][]int32{}
	for d := 0; d <= limit; d++ {
		for k := -d; k <= d; k += 2 {
			var px int32
			if k == -d || k != d && v[offset+k-1] < v[offset+k+1] {
				px = v[offset+k+1]
			} else {
				px = v[offset+k-1] + 1
			}
			py := px - int32(k)
			for int(px) < n && int(py) < m && a[px] == b[py] {
				px++
				py++
			}
			v[offset+k] = px
			if int(px) >= n && int(py) >= m {
				trace = append(trace, append([]int32(nil), v[offset-d:offset+d+1]...))
				return backtrack(trace, n, m), true
			}
		}
		trace = append(trace, append([]int32(nil), v[offset-d:offset+d+1]...))
	}
	return nil, false
}

// backtrack walks the per-round bands from the end point back to the origin.
func backtrack(trace [][]int32, n, m int) []byte {
	ops := []byte{}
	x, y := n, m
	at := func(d, k int) int { return int(trace[d][k+d]) }
	for d := len(trace) - 1; d > 0; d-- {
		k := x - y
		var prevK int
		if k == -d || k != d && at(d-1, k-1) < at(d-1, k+1) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := at(d-1, prevK)
		prevY := prevX - prevK
		midX := prevX
		if prevK == k-1 {
			midX = prevX + 1
		}
		for x > midX {
			ops = append(ops, '=')
			x--
			y--
		}
		if prevK == k+1 {
			ops = append(ops, '+')
		} else {
			ops = append(ops, '-')
		}
		x, y = prevX, prevY
	}
	for x > 0 && y > 0 {
		ops = append(ops, '=')
		x--
		y--
	}
	for l, r := 0, len(ops)-1; l < r; l, r = l+1, r-1 {
		ops[l], ops[r] = ops[r], ops[l]
	}
	return ops
}

// mapLine maps an old line through hunks to the new numbering. A line inside a
// hunk maps to that hunk's new range edge (first for start, last for end), so
// the mapping is an estimate there; ok is false when the hunk deleted it with
// no replacement.
func mapLine(hunks []DeltaHunk, line int, end bool) (int, bool) {
	shift := 0
	for _, h := range hunks {
		if line < h.OldStart {
			break
		}
		if line < h.OldStart+h.OldCount {
			if h.NewCount == 0 {
				if end {
					return h.NewStart - 1, false
				}
				return h.NewStart, false
			}
			if end {
				return h.NewStart + h.NewCount - 1, true
			}
			return h.NewStart, true
		}
		shift += h.NewCount - h.OldCount
	}
	return line + shift, true
}

// overlappingHunks returns the hunks that touch old lines start..end: a change
// inside the span or an insertion strictly within it.
func overlappingHunks(hunks []DeltaHunk, start, end, bound int) ([]DeltaHunk, bool) {
	out := []DeltaHunk{}
	for _, h := range hunks {
		touches := h.OldCount > 0 && h.OldStart <= end && h.OldStart+h.OldCount-1 >= start || h.OldCount == 0 && h.OldStart > start && h.OldStart <= end
		if !touches {
			continue
		}
		if len(out) == bound {
			return out, true
		}
		out = append(out, h)
	}
	return out, false
}
