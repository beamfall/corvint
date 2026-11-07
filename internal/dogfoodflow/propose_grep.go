package dogfoodflow

import (
	"strconv"
	"strings"
)

// anchorGrepFastPath turns onlyMatching's listing-then-match path on; a test
// turns it off to compare it with the single git grep -o (DWC-V0-754).
var anchorGrepFastPath = true

// anchorGrepFastPatterns is the pattern count from which the listing pays for
// its second Git call: Git's cost grows with the patterns per line, and a
// search of a few patterns stays one git grep (DWC-V0-754).
var anchorGrepFastPatterns = 32

// onlyMatching returns what `git <arguments>` prints, where arguments is the
// anchor search's `grep -z -n -o -F -I -e P... BASE -- PATHSPEC...`, without
// asking Git for the line matches: Git checks every line against every -e
// pattern, which took 104 s for 496 patterns where -l over the same files took
// 1.4 s (DWC-V0-754). It lists the matching files with -l, reads each through
// the batch reader and finds the matches itself, as git grep -o does: on each
// line the leftmost match of any pattern, the longest at that start, then the
// next one from its end. The records are byte-identical to Git's; any read it
// cannot complete runs the single git grep instead.
func (p *proposal) onlyMatching(arguments []string, patterns []string) (string, int, bool) {
	if !anchorGrepFastPath || len(patterns) < anchorGrepFastPatterns || len(arguments) < 6 || strings.Join(arguments[:6], " ") != "grep -z -n -o -F -I" {
		return p.boundedGit(arguments...)
	}
	listing := append([]string{"grep", "-z", "-l", "-F", "-I"}, arguments[6:]...)
	out, status, bounded := p.boundedGit(listing...)
	if status == 1 && bounded && out == "" {
		return "", 1, true
	}
	if status != 0 || !bounded {
		return p.boundedGit(arguments...)
	}
	limit := p.gitOutputLimit
	if limit <= 0 {
		limit = maxGitOutputBytes
	}
	matcher := newFixedMatcher(patterns)
	var records strings.Builder
	prefix := p.base + ":"
	for _, listed := range strings.Split(strings.TrimSuffix(out, "\x00"), "\x00") {
		p.checkpoint()
		name, found := strings.CutPrefix(listed, prefix)
		if !found {
			return p.boundedGit(arguments...)
		}
		// The proposal's batch reader (CKC-V0-015) answers false past the
		// flow's output bound, for a non-blob entry, or when it is off.
		data, present, ok := p.batchBlob(p.base, name)
		if !present || !ok {
			return p.boundedGit(arguments...)
		}
		if !matcher.records(&records, prefix+name, data, limit, p.checkpoint) {
			return "", 0, false
		}
	}
	if records.Len() == 0 {
		return "", 1, true
	}
	return records.String(), 0, true
}

// fixedMatcher is an Aho-Corasick automaton over fixed byte patterns.
type fixedMatcher struct {
	longest int
	next    []map[byte]int32
	fail    []int32
	lengths [][]int // lengths of the patterns ending at each state, own and by suffix
}

func newFixedMatcher(patterns []string) *fixedMatcher {
	m := &fixedMatcher{next: []map[byte]int32{{}}, fail: []int32{0}, lengths: [][]int{nil}}
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		m.longest = max(m.longest, len(pattern))
		state := int32(0)
		for i := 0; i < len(pattern); i++ {
			child, ok := m.next[state][pattern[i]]
			if !ok {
				child = int32(len(m.next))
				m.next = append(m.next, map[byte]int32{})
				m.fail = append(m.fail, 0)
				m.lengths = append(m.lengths, nil)
				m.next[state][pattern[i]] = child
			}
			state = child
		}
		m.lengths[state] = append(m.lengths[state], len(pattern))
	}
	// Breadth-first failure links; each state also reports its suffixes'
	// patterns.
	queue := []int32{}
	for _, child := range m.next[0] {
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		for symbol, child := range m.next[state] {
			fail := m.fail[state]
			for fail != 0 {
				if _, ok := m.next[fail][symbol]; ok {
					break
				}
				fail = m.fail[fail]
			}
			if target, ok := m.next[fail][symbol]; ok && target != child {
				m.fail[child] = target
			}
			m.lengths[child] = append(m.lengths[child], m.lengths[m.fail[child]]...)
			queue = append(queue, child)
		}
	}
	return m
}

// occurrence is one pattern match [start, end) within a line.
type occurrence struct{ start, end int }

// records appends git grep -z -n -o records for data to out, as Git prints
// them for the file name: `name NUL line NUL match LF` per match. It reports
// false once out would pass limit, as Git's output would. A match is written
// as soon as no later match can start before it or at its start (none is
// longer than the longest pattern), so it keeps only the matches still open
// in that window, never a whole line's, and checks limit as it writes; tick
// runs every 64 KiB of data, so a deadline or an interrupt stops a long scan
// (DWC-V0-754).
func (m *fixedMatcher) records(out *strings.Builder, name string, data []byte, limit int, tick func()) bool {
	state := int32(0)
	line, lineStart, at := 1, 0, 0
	number := "1"
	pending := []occurrence{}
	// emit writes the leftmost pending match, the longest at its start, and
	// drops the matches it covers.
	emit := func() bool {
		best := 0
		for index, match := range pending {
			if match.start < pending[best].start || match.start == pending[best].start && match.end > pending[best].end {
				best = index
			}
		}
		match := pending[best]
		if out.Len()+len(name)+len(number)+match.end-match.start+3 > limit {
			return false
		}
		out.WriteString(name)
		out.WriteByte(0)
		out.WriteString(number)
		out.WriteByte(0)
		out.Write(data[match.start:match.end])
		out.WriteByte('\n')
		at = match.end
		kept := pending[:0]
		for _, open := range pending {
			if open.start >= at {
				kept = append(kept, open)
			}
		}
		pending = kept
		return true
	}
	for i, symbol := range data {
		if i&0xffff == 0 && tick != nil {
			tick()
		}
		if symbol == '\n' {
			for len(pending) > 0 {
				if !emit() {
					return false
				}
			}
			line, lineStart = line+1, i+1
			at, number, state = lineStart, strconv.Itoa(line), 0
			continue
		}
		for {
			if child, ok := m.next[state][symbol]; ok {
				state = child
				break
			}
			if state == 0 {
				break
			}
			state = m.fail[state]
		}
		for _, length := range m.lengths[state] {
			if start := i + 1 - length; start >= at {
				pending = append(pending, occurrence{start, i + 1})
			}
		}
		// A later match ends after i and starts at or after i+2-longest.
		for len(pending) > 0 && m.settled(pending, i) {
			if !emit() {
				return false
			}
		}
	}
	// A last line without LF is still a line.
	for len(pending) > 0 {
		if !emit() {
			return false
		}
	}
	return true
}

// settled reports whether no match ending after position i can start at or
// before the leftmost pending start.
func (m *fixedMatcher) settled(pending []occurrence, i int) bool {
	leftmost := pending[0].start
	for _, match := range pending[1:] {
		leftmost = min(leftmost, match.start)
	}
	return i+2-m.longest > leftmost
}
