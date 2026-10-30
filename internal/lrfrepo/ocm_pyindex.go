package lrfrepo

import (
	"bytes"
	"regexp"
	"sort"
)

// Per-blob Python derivation, computed ONCE.
//
// The Python claim rules ask a blob three questions -- which test def encloses
// an offset, how many defs a name has, and which signature line precedes an
// anchor -- and each answer was a fresh whole-blob regexp scan, plus a fresh
// maskComments copy, per claim. At the legal ceiling that is minutes of
// scanning inside a 20-second Git budget.
//
// The shape here is deliberately NOT a straight port of goBlobIndex, because
// the Python patterns behave differently under truncation. Go's declaration
// pattern is bounded by the `(` it ends on, so a prefix scan and a whole-blob
// scan agree offset for offset. Python's signature pattern ends in a greedy
// `\s*(?:\r?\n)`, and \s includes \n, so over the whole blob it swallows
// every blank line after a signature and stops later than the same scan cut
// short at a claim would -- see truncatedEnd, which reconstructs the cut end
// rather than pretending the two agree. So only the def-site walk, which has to
// answer per-name questions, is derived by hand; the signature pattern is run
// once per blob and located by binary search, then corrected.
var (
	// pythonTestDef is what nearestPythonTest captured.
	pythonTestDef = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+(test_[A-Za-z0-9_]+)\s*\(`)
	// pythonTestSignature is what pythonFirstStatement scanned for.
	pythonTestSignature = regexp.MustCompile(`(?m)^\s*(?:async\s+)?def\s+test_[A-Za-z0-9_]+\s*\([^\n]*\)\s*(?:->[^:\n]+)?\s*:\s*(?:\r?\n)`)
)

// pythonSite records one place `^\s*(?:async\s+)?def\s+` can end. start is the
// `def` keyword, ordered and compared for FindAll's non-overlap rule; name is
// the offset just past the whitespace run, where a def name must begin.
type pythonSite struct {
	start int
	name  int
}

// pythonTestSite is one match of pythonTestDef.
type pythonTestSite struct {
	end  int
	name string
}

type pythonBlobIndex struct {
	clean      []byte
	sites      []pythonSite
	tests      []pythonTestSite
	signatures [][]int
	nameCounts map[string]int
	oddCounts  map[string]int
}

func newPythonBlobIndex(blob []byte) *pythonBlobIndex {
	index := &pythonBlobIndex{
		clean:      maskComments(blob, ".py"),
		nameCounts: map[string]int{},
		oddCounts:  map[string]int{},
	}
	index.indexDefs()
	index.signatures = pythonTestSignature.FindAllIndex(index.clean, -1)
	return index
}

// indexDefs records every def site, and among them the pythonTestDef matches
// with FindAll's non-overlap rule applied at the offsets the regexp would.
//
// The sites are walked by hand rather than scanned with pythonTestDef's own
// `^\s*(?:async\s+)?def\s+` prefix, because that prefix's trailing \s+ can
// swallow a line break and, with it, the NEXT line's `def` -- FindAll then
// resumes past a real declaration and loses it.
func (index *pythonBlobIndex) indexDefs() {
	clean := index.clean
	resume := 0
	for at := 0; at+3 <= len(clean); {
		offset := bytes.Index(clean[at:], []byte("def"))
		if offset < 0 {
			return
		}
		keyword := at + offset
		at = keyword + 1
		if !index.defReachesLineStart(keyword) {
			continue
		}
		name := keyword + 3
		if name >= len(clean) || !isSpaceByte(clean[name]) {
			continue
		}
		name = skipSpace(clean, name)
		site := pythonSite{start: keyword, name: name}
		index.sites = append(index.sites, site)
		end, testName, matched := index.testDefAt(name)
		if !matched || site.start < resume {
			continue
		}
		index.tests = append(index.tests, pythonTestSite{end: end, name: testName})
		index.nameCounts[testName]++
		resume = end
	}
}

// defReachesLineStart reports whether `^\s*(?:async\s+)?` can end exactly at
// the def keyword: either only whitespace separates it from a line start, or
// only whitespace separates it from an `async` that is itself only whitespace
// from a line start.
func (index *pythonBlobIndex) defReachesLineStart(keyword int) bool {
	clean := index.clean
	blank := backSpace(clean, keyword)
	if spaceRunOpensALine(clean, blank, keyword) {
		return true
	}
	if blank == keyword || blank < 5 || string(clean[blank-5:blank]) != "async" {
		return false
	}
	return spaceRunOpensALine(clean, backSpace(clean, blank-5), blank-5)
}

// spaceRunOpensALine reports whether the whitespace run [blank, end) contains a
// position multiline ^ matches, so that `^\s*` can end exactly at end. \s
// includes \n, so a run's own line break is what supplies that position --
// testing whether the RUN START is a line start instead would reject every def
// whose preceding line ends in code.
func spaceRunOpensALine(clean []byte, blank, end int) bool {
	return blank == 0 || bytes.IndexByte(clean[blank:end], '\n') >= 0
}

// backSpace walks back over regexp \s to the first offset whose predecessor is
// not whitespace.
func backSpace(clean []byte, at int) int {
	for at > 0 && isSpaceByte(clean[at-1]) {
		at--
	}
	return at
}

// testDefAt reports the pythonTestDef match whose name begins at offset. The
// name is the maximal identifier run: the regexp's greedy [A-Za-z0-9_]+ can
// never backtrack to a shorter name, because a shorter name is followed by a
// word byte, which is neither \s nor '('.
func (index *pythonBlobIndex) testDefAt(name int) (int, string, bool) {
	clean := index.clean
	if !bytes.HasPrefix(clean[name:], []byte("test_")) {
		return 0, "", false
	}
	end := name + 5
	if end >= len(clean) || !isWordByte(clean[end]) {
		return 0, "", false
	}
	for end++; end < len(clean) && isWordByte(clean[end]); end++ {
	}
	open := skipSpace(clean, end)
	if open >= len(clean) || clean[open] != '(' {
		return 0, "", false
	}
	return open + 1, string(clean[name:end]), true
}

// nearestTest names the last pythonTestDef match lying wholly before offset,
// which is what running the regexp over clean[:offset] and taking the last
// match returns: a match inside a prefix is a match of the whole blob at the
// same offsets, and the pattern admits no interior line-start `def`, so the
// prefix's non-overlapping scan cannot diverge from the whole blob's.
func (index *pythonBlobIndex) nearestTest(offset int) string {
	position := sort.Search(len(index.tests), func(candidate int) bool {
		return index.tests[candidate].end > offset
	})
	if position == 0 {
		return ""
	}
	return index.tests[position-1].name
}

// testCount counts `(?m)^\s*(?:async\s+)?def\s+` + regexp.QuoteMeta(name) +
// `\s*\(` matches. Callers have already established that name begins "test_",
// so every match starts its name at a recorded site. A plain identifier name is
// answered from the pythonTestDef tally; any other name -- `test_a.b`, say,
// which the def-prefix rule admits but the identifier pattern does not -- is
// counted over the sites and memoized.
func (index *pythonBlobIndex) testCount(name string) int {
	if plainPythonTestName(name) {
		return index.nameCounts[name]
	}
	if count, found := index.oddCounts[name]; found {
		return count
	}
	count, resume := 0, 0
	for _, site := range index.sites {
		if site.start < resume || !bytes.HasPrefix(index.clean[site.name:], []byte(name)) {
			continue
		}
		open := skipSpace(index.clean, site.name+len(name))
		if open >= len(index.clean) || index.clean[open] != '(' {
			continue
		}
		count++
		resume = open + 1
	}
	index.oddCounts[name] = count
	return count
}

// firstStatement is pythonFirstStatement with the whole-blob signature scan in
// place of the per-claim one over clean[:start]. Everything after the lookup is
// the original test, unchanged.
func (index *pythonBlobIndex) firstStatement(start int) bool {
	clean := index.clean
	position := sort.Search(len(index.signatures), func(candidate int) bool {
		return index.signatures[candidate][0] >= start
	})
	for candidate := position - 1; candidate >= 0; candidate-- {
		end, complete := index.truncatedEnd(index.signatures[candidate], start)
		if !complete {
			continue
		}
		definitionIndent := leadingIndent(clean[index.signatures[candidate][0]:])
		anchorLineStart := bytes.LastIndexByte(clean[:start], '\n') + 1
		if leadingIndent(clean[anchorLineStart:]) <= definitionIndent {
			return false
		}
		return blankBetween(clean[end:start])
	}
	return false
}

// truncatedEnd reports where the original scan over clean[:start] would have
// ended this signature match.
//
// The match START is truncation-stable -- it is fixed by the text BEFORE the
// def -- but the END is not: the pattern's trailing `\s*(?:\r?\n)` is greedy
// and \s includes \n, so over the whole blob it swallows every blank line
// after the signature and stops at the LAST of them, while a scan cut at start
// stops at the last line break before the cut. When the greedy end overshoots
// the cut, the truncated scan re-picks that earlier line break; when no line
// break is left before the cut, the match does not complete there at all and
// the previous match is the last one.
//
// Pinning the pattern's tail instead is NOT equivalent: FindAll resumes at each
// match's end, so a shorter tail moves the NEXT match's start, and that start is
// what the indent test reads.
func (index *pythonBlobIndex) truncatedEnd(match []int, start int) (int, bool) {
	if match[0] >= start {
		return 0, false
	}
	if match[1] <= start {
		return match[1], true
	}
	tail := backSpace(index.clean, match[1]-1)
	if tail > start {
		return 0, false
	}
	offset := bytes.LastIndexByte(index.clean[tail:start], '\n')
	if offset < 0 {
		return 0, false
	}
	return tail + offset + 1, true
}

// blankBetween is the original's per-line emptiness test without its Split
// allocation: splitting on '\n' and asking whether every piece TrimSpaces to
// empty is asking whether the whole region TrimSpaces to empty, because '\n' is
// itself trimmed.
func blankBetween(between []byte) bool {
	return len(bytes.TrimSpace(between)) == 0
}

// plainPythonTestName reports whether name is exactly what pythonTestDef
// captures: "test_" followed by at least one identifier byte, and nothing else.
func plainPythonTestName(name string) bool {
	if len(name) < 6 || name[:5] != "test_" {
		return false
	}
	for index := 5; index < len(name); index++ {
		if !isWordByte(name[index]) {
			return false
		}
	}
	return true
}
