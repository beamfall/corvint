package lrfrepo

import (
	"bytes"
	"encoding/json"
	"regexp"
	"sort"
)

// Per-blob Go derivation, computed ONCE.
//
// The Go claim rules ask a blob three questions: which test function encloses
// an offset, how many table cases in the blob derive a given selector, and how
// many `func NAME(` sites a name has. Answering each from scratch per claim is
// cubic in claims per blob -- N claims, each re-deriving the parent of all N
// cases, each derivation a whole-blob scan -- so a LEGAL artifact (maxClaims
// claims in one maxOCMBlob blob) runs hours past gitrun.DefaultTotalBudget and
// returns git-timeout instead of a verdict. This index answers all three from
// one pass, in log or constant time, with byte-identical results.

// goTableCaseTail is the goTableCase key, anchored at end of line: what sits
// immediately before a table case's opening quote. Compiled once, not per claim.
var goTableCaseTail = regexp.MustCompile(`\b(?:(?:name|testName|test_name)\s*:|[A-Za-z_][A-Za-z0-9_]*\.Run\()\s*$`)

// goFuncSite is one `\bfunc\s+` occurrence whose name position begins "Test".
// start is the `func` offset, which FindAll's non-overlap rule is stated in;
// name is the first offset past the whitespace run, where a name must begin.
type goFuncSite struct {
	start int
	name  int
}

// goTestSite is one match of goTestFunction: `\bfunc\s+(Test[A-Z]\w*)\s*\(`.
type goTestSite struct {
	nameStart int
	nameEnd   int
	end       int
	name      string
}

// goCaseSite is one match of goTableCase whose quoted name decodes, carrying
// the selector that match derives. start/end span the quoted content.
type goCaseSite struct {
	start    int
	end      int
	parent   string
	selector string
}

type goBlobIndex struct {
	clean      []byte
	tests      []goTestSite
	cases      []goCaseSite
	sites      []goFuncSite
	nameCounts map[string]int
	caseCounts map[string]int
	oddCounts  map[string]int
}

func newGoBlobIndex(blob []byte) *goBlobIndex {
	index := &goBlobIndex{
		clean:      maskComments(blob, ".go"),
		nameCounts: map[string]int{},
		caseCounts: map[string]int{},
		oddCounts:  map[string]int{},
	}
	index.indexFunctions()
	index.indexCases()
	return index
}

// indexFunctions walks every `\bfunc\s+` site once and records the two forms
// the Go rules query: the goTestFunction matches (with FindAll's non-overlap
// rule applied at the same offsets the regexp would), and every Test-prefixed
// site, which is where a name that is not a plain identifier can still match.
func (index *goBlobIndex) indexFunctions() {
	clean := index.clean
	resume := 0
	for at := 0; at+4 <= len(clean); {
		offset := bytes.Index(clean[at:], []byte("func"))
		if offset < 0 {
			return
		}
		start := at + offset
		at = start + 1
		if start > 0 && isWordByte(clean[start-1]) {
			continue
		}
		name := start + 4
		if name >= len(clean) || !isSpaceByte(clean[name]) {
			continue
		}
		for name < len(clean) && isSpaceByte(clean[name]) {
			name++
		}
		if !bytes.HasPrefix(clean[name:], []byte("Test")) {
			continue
		}
		index.sites = append(index.sites, goFuncSite{start: start, name: name})
		site, matched := index.testSiteAt(name)
		if !matched || start < resume {
			continue
		}
		index.tests = append(index.tests, site)
		index.nameCounts[site.name]++
		resume = site.end
	}
}

// testSiteAt reports the goTestFunction match that begins its name at offset,
// if any. The name is the maximal identifier run: the regexp's greedy
// [A-Za-z0-9_]* can never backtrack to a shorter name, because a shorter name
// is followed by a word byte, which is neither \s nor '('.
func (index *goBlobIndex) testSiteAt(name int) (goTestSite, bool) {
	clean := index.clean
	end := name + 4
	if end >= len(clean) || clean[end] < 'A' || clean[end] > 'Z' {
		return goTestSite{}, false
	}
	for end++; end < len(clean) && isWordByte(clean[end]); end++ {
	}
	open := skipSpace(clean, end)
	if open >= len(clean) || clean[open] != '(' {
		return goTestSite{}, false
	}
	return goTestSite{nameStart: name, nameEnd: end, end: open + 1, name: string(clean[name:end])}, true
}

func (index *goBlobIndex) indexCases() {
	for _, match := range goTableCase.FindAllSubmatchIndex(index.clean, -1) {
		var claimText string
		if err := json.Unmarshal(append(append([]byte{'"'}, index.clean[match[2]:match[3]]...), '"'), &claimText); err != nil {
			continue
		}
		parent := index.nearestTest(match[0])
		selector := "test:" + parent + "/case:" + selectorFragment(claimText)
		index.cases = append(index.cases, goCaseSite{start: match[2], end: match[3], parent: parent, selector: selector})
		index.caseCounts[selector]++
	}
}

// nearestTest names the last goTestFunction match lying wholly before offset,
// which is what running the regexp over clean[:offset] and taking the last
// match returns: a match inside a prefix is a match of the whole blob at the
// same offsets, and the pattern admits no interior `\bfunc\s+` site, so the
// prefix's non-overlapping scan cannot diverge from the whole blob's.
func (index *goBlobIndex) nearestTest(offset int) string {
	position := sort.Search(len(index.tests), func(candidate int) bool {
		return index.tests[candidate].end > offset
	})
	if position == 0 {
		return ""
	}
	return index.tests[position-1].name
}

func (index *goBlobIndex) caseCount(selector string) int {
	return index.caseCounts[selector]
}

// funcCount counts `\bfunc\s+` + regexp.QuoteMeta(name) + `\s*\(` matches.
// Callers have already established that name begins Test + an upper-case byte,
// so every match starts its name at a recorded site. A plain identifier name
// is answered from the goTestFunction tally; any other name -- `TestA.B`, say,
// which the anchor rules admit but the identifier pattern does not -- is
// counted over the Test-prefixed sites and memoized.
func (index *goBlobIndex) funcCount(name string) int {
	if plainGoTestName(name) {
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

// plainGoTestName reports whether name is exactly what goTestFunction captures:
// Test, an upper-case byte, then identifier bytes.
func plainGoTestName(name string) bool {
	if len(name) < 5 || name[:4] != "Test" || name[4] < 'A' || name[4] > 'Z' {
		return false
	}
	for index := 5; index < len(name); index++ {
		if !isWordByte(name[index]) {
			return false
		}
	}
	return true
}
