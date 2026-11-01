package contextindex

import (
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/runtimeenv"
)

// TCP-V0-650..652 (V1-3711): the opt-in fuzzy lexical fallback, set by
// `CORVINT_CONTEXT_FUZZY=on`. A task term the body vocabulary does not carry,
// or carries in at most contextFuzzyRarePostings sources while one term at
// edit distance one is contextFuzzyMinRatio times as common, also credits that
// neighbour at contextFuzzyGain of its weight; a task term that is a short
// stem of a path token (`narrowing` and `narrow`), or two task terms whose
// concatenation is a path token (`unplanned` and `read`, `unplannedread`),
// credit that path token; a hyphenated task word (`merge-base`) is matched
// verbatim like a TCP-V0-022 anchor; and a task word for configuration
// credits configuration files. Every credit is
// labelled `fuzzy` on its row, and a record-data source whose path names two
// task terms is ranked as code.
const (
	contextFuzzyMinTerm      = 5
	contextFuzzyRarePostings = 2
	contextFuzzyMinRatio     = 8
	contextFuzzyGain         = 0.75
	contextFuzzyStemSlack    = 3
)

// contextFuzzyCompound is a hyphenated command or flag word (`merge-base`),
// which the term tokeniser splits; it is matched verbatim like an anchor.
var contextFuzzyCompound = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)+`)

// contextConfigWords are the task words that name a configuration file.
var contextConfigWords = stringSetOf([]string{"config", "configs", "configuration", "settings"})

// fuzzyMatch is one disclosed fuzzy credit: the task's term and what it
// matched instead.
type fuzzyMatch struct{ term, matched string }

func configureContextFuzzy(compiler *taskContextCompiler) *taskContextCompiler {
	// The fallback needs the plain task terms; the identifier-weighted term
	// selection weighs only the words it selected, so it keeps the fallback off.
	compiler.fuzzy = runtimeenv.Value("CONTEXT_FUZZY") == "on" && compiler.selectedTerms == nil
	return compiler
}

// fuzzyNeighbour is the most common body term at edit distance one from term
// when term is absent or rare and the neighbour is contextFuzzyMinRatio times
// as common; ties break to the smaller key.
func fuzzyNeighbour(postings *termPostings, term string) (string, int, int, bool) {
	if len(term) < contextFuzzyMinTerm {
		return "", 0, 0, false
	}
	own := 0
	if low, high, ok := postings.find(term); ok {
		own = high - low
	}
	if own > contextFuzzyRarePostings {
		return "", 0, 0, false
	}
	best, bestLow, bestHigh := "", 0, 0
	for index := range postings.keyCount() {
		key := postings.key(index)
		if len(key) < len(term)-1 || len(key) > len(term)+1 || key == term || !editDistanceOne(term, key) {
			continue
		}
		low, high := int(postings.Offsets[index]), int(postings.Offsets[index+1])
		if high-low > bestHigh-bestLow {
			best, bestLow, bestHigh = key, low, high
		}
	}
	if best == "" || bestHigh-bestLow < contextFuzzyMinRatio*max(own, 1) {
		return "", 0, 0, false
	}
	return best, bestLow, bestHigh, true
}

// fuzzyPathStems are the path tokens term is a stem of, or that are a stem of
// term, by at most contextFuzzyStemSlack trailing bytes over a shared prefix
// of at least contextFuzzyMinTerm bytes; the exact token is not a stem.
func fuzzyPathStems(postings *termPostings, term string) []int {
	if len(term) < contextFuzzyMinTerm {
		return nil
	}
	var keys []int
	for index := range postings.keyCount() {
		key := postings.key(index)
		if key == term || len(key) < contextFuzzyMinTerm {
			continue
		}
		short, long := key, term
		if len(short) > len(long) {
			short, long = long, short
		}
		if len(long)-len(short) <= contextFuzzyStemSlack && strings.HasPrefix(long, short) {
			keys = append(keys, index)
		}
	}
	return keys
}

// editDistanceOne reports whether one insertion, deletion, substitution or
// transposition of adjacent bytes turns left into right.
func editDistanceOne(left, right string) bool {
	if len(left) > len(right) {
		left, right = right, left
	}
	switch len(right) - len(left) {
	case 0:
		first := -1
		for index := range len(left) {
			if left[index] == right[index] {
				continue
			}
			if first >= 0 {
				return index == first+1 && left[first] == right[index] && left[index] == right[first] &&
					left[index+1:] == right[index+1:]
			}
			first = index
		}
		return first >= 0
	case 1:
		index := 0
		for index < len(left) && left[index] == right[index] {
			index++
		}
		return left[index:] == right[index+1:]
	}
	return false
}

// isConfigurationPath reports a configuration file: a structured settings or
// shell file under a dot directory or named as a dotfile.
func isConfigurationPath(candidate string) bool {
	switch strings.ToLower(path.Ext(candidate)) {
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf", ".sh":
	default:
		return false
	}
	return strings.HasPrefix(candidate, ".") || strings.Contains(candidate, "/.")
}

// fuzzyReason prefixes the evidence line with every fuzzy credit of the row.
func fuzzyReason(matches []fuzzyMatch) string {
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		if match.matched == "" {
			parts = append(parts, fmt.Sprintf("`%s` verbatim", match.term))
			continue
		}
		parts = append(parts, fmt.Sprintf("`%s`~`%s`", match.term, match.matched))
	}
	return "fuzzy " + strings.Join(parts, ", ") + "; "
}

// fuzzyCredits runs the TCP-V0-650..652 walks after the exact term walk, with
// the same BM25 fields: a neighbour as a body and path term, a stem and a
// configuration word as path terms, each at contextFuzzyGain. It returns the
// disclosed credits per source and counts path matches in pathNamed.
func (compiler *taskContextCompiler) fuzzyCredits(table *TermTable, idfOf func(int) float64, lengths []uint32,
	average float64, occurrences, pathNamed []int, credit func(int, uint32, string, float64, float64)) map[uint32][]fuzzyMatch {
	const k1, b = 1.2, 0.3
	hits := map[uint32][]fuzzyMatch{}
	note := func(source uint32, term, matched string) {
		for _, seen := range hits[source] {
			if seen.term == term && seen.matched == matched {
				return
			}
		}
		hits[source] = append(hits[source], fuzzyMatch{term: term, matched: matched})
	}
	pathCredit := func(term, matched string, low, high int) {
		idf := idfOf(high - low)
		for index := low; index < high; index++ {
			source := table.PathTerms.Sources[index]
			credit(1, source, matched, idf, idf*contextFuzzyGain)
			pathNamed[source]++
			note(source, term, matched)
		}
	}
	for _, term := range compiler.terms {
		if compiler.stopped("lexical-fuzzy") {
			return hits
		}
		if neighbour, low, high, ok := fuzzyNeighbour(&table.Terms, term); ok {
			idf := idfOf(high - low)
			for index := low; index < high; index++ {
				source := table.Terms.Sources[index]
				tf := float64(table.Terms.Counts[index])
				norm := k1 * (1 - b + b*float64(lengths[source])/average)
				occurrences[source] += int(table.Terms.Counts[index])
				credit(0, source, neighbour, idf, idf*tf*(k1+1)/(tf+norm)*contextFuzzyGain)
				note(source, term, neighbour)
			}
			if low, high, ok := table.PathTerms.find(neighbour); ok {
				pathCredit(term, neighbour, low, high)
			}
		}
		for _, key := range fuzzyPathStems(&table.PathTerms, term) {
			if len(table.PathTerms.key(key)) < contextFuzzyMinTerm {
				continue
			}
			pathCredit(term, table.PathTerms.key(key), int(table.PathTerms.Offsets[key]), int(table.PathTerms.Offsets[key+1]))
		}
		if _, ok := contextConfigWords[term]; ok {
			var sources []uint32
			for source, candidate := range table.Paths {
				if isConfigurationPath(candidate) {
					sources = append(sources, uint32(source))
				}
			}
			idf := idfOf(len(sources))
			for _, source := range sources {
				credit(1, source, "configuration file", idf, idf*contextFuzzyGain)
				note(source, term, "configuration file")
			}
		}
	}
	// Two task terms, or their neighbours, whose concatenation is a path
	// token: a directory or file named by joined words.
	words := make([]string, 0, len(compiler.terms))
	for _, term := range compiler.terms {
		words = append(words, term)
		if neighbour, _, _, ok := fuzzyNeighbour(&table.Terms, term); ok {
			words = append(words, neighbour)
		}
	}
	for _, left := range words {
		for _, right := range words {
			if left == right {
				continue
			}
			joined := left + right
			if low, high, ok := table.PathTerms.find(joined); ok {
				pathCredit(left+" "+right, joined, low, high)
			}
		}
	}
	for _, literal := range contextFuzzyCompound.FindAllString(compiler.task, contextAnchorCap) {
		literal = strings.ToLower(literal)
		if len(literal) < contextAnchorMinBytes || compiler.anchored(literal) {
			continue
		}
		sources, counts := compiler.anchorOccurrences(table, literal)
		idf := idfOf(len(sources))
		for index, source := range sources {
			tf := float64(counts[index])
			norm := k1 * (1 - b + b*float64(lengths[source])/average)
			credit(3, source, literal, idf, idf*tf*(k1+1)/(tf+norm)*contextFuzzyGain)
			note(source, literal, "")
		}
	}
	return hits
}

// anchored reports a literal the TCP-V0-022 anchor field already credits.
func (compiler *taskContextCompiler) anchored(literal string) bool {
	for _, anchor := range compiler.anchors {
		if strings.EqualFold(anchor.literal, literal) {
			return true
		}
	}
	return false
}
