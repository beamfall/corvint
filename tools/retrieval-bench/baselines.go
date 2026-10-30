package main

// The baseline ladder beside the bench's grep control: grep-ident scores
// whole-word identifier hits the way an agent types them into ripgrep, and
// bm25 scores whole files with untuned BM25 over either every bench term or
// only the identifier-derived terms. Both read the same materialized copy
// the other arms see and neither indexes, learns, or abstains beyond an
// empty match. runGrep stays untouched so old reports remain comparable.

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

const (
	minIdentifierChars = 4
	bm25K1             = 1.2
	bm25B              = 0.75
)

var identifierPattern = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
var camelBoundaryPattern = regexp.MustCompile(`[a-z][A-Z]`)
var codeStemPattern = regexp.MustCompile(`([A-Za-z0-9_\-]+)\.(?:go|py|rs|ts|tsx|js|java|kt|rb|cs|cpp|c|h)\b`)

// queryIdentifiers is the frozen identifier rule: from the query's string
// values only (never its keys), every ASCII identifier of at least four
// characters with an inner underscore or a camel-case boundary, plus the
// stem of every code file name, distinct and sorted.
func queryIdentifiers(query map[string]any) []string {
	seen := map[string]bool{}
	var walk func(value any)
	walk = func(value any) {
		switch typed := value.(type) {
		case map[string]any:
			for _, element := range typed {
				walk(element)
			}
		case []any:
			for _, element := range typed {
				walk(element)
			}
		case string:
			collectIdentifiers(typed, seen)
		}
	}
	walk(query)
	return sortedKeys(seen)
}

func collectIdentifiers(text string, seen map[string]bool) {
	for _, token := range identifierPattern.FindAllString(text, -1) {
		if len(token) < minIdentifierChars {
			continue
		}
		if !strings.Contains(strings.Trim(token, "_"), "_") && !camelBoundaryPattern.MatchString(token) {
			continue
		}
		seen[token] = true
	}
	for _, match := range codeStemPattern.FindAllStringSubmatch(text, -1) {
		if len(match[1]) >= minIdentifierChars {
			seen[match[1]] = true
		}
	}
}

// identifierTerms is the bm25 `ident` term set: the bench tokens of every
// identifier, distinct and sorted, with no length floor.
func identifierTerms(identifiers []string) []string {
	seen := map[string]bool{}
	for _, identifier := range identifiers {
		for _, token := range tokenize(identifier) {
			seen[token] = true
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// corpusIndex is one materialized copy read once: every text file under the
// grep size bound with its bench tokens counted. Given files stay in the
// statistics and are excluded at ranking time, as the probe did.
type corpusIndex struct {
	root              string
	files             []corpusFile
	documentFrequency map[string]int
	meanLength        float64
}

type corpusFile struct {
	path   string
	text   string
	counts map[string]int
	length int
}

var cachedCorpus *corpusIndex

// corpusFor indexes root once and keeps the latest index: samples arrive
// grouped by snapshot, so one cached copy serves a run.
func corpusFor(root string) *corpusIndex {
	if cachedCorpus != nil && cachedCorpus.root == root {
		return cachedCorpus
	}
	cachedCorpus = indexCorpus(root)
	return cachedCorpus
}

func indexCorpus(root string) *corpusIndex {
	index := &corpusIndex{root: root, documentFrequency: map[string]int{}}
	totalLength := 0
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".context-corvint" || entry.Name() == ".corvint" {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		text, readable := readTextFile(path)
		if !readable {
			return nil
		}
		relative, _ := filepath.Rel(root, path)
		file := corpusFile{path: filepath.ToSlash(relative), text: text, counts: map[string]int{}}
		for _, token := range tokenize(text) {
			file.counts[token]++
			file.length++
		}
		for token := range file.counts {
			index.documentFrequency[token]++
		}
		totalLength += file.length
		index.files = append(index.files, file)
		return nil
	})
	sort.Slice(index.files, func(left, right int) bool { return index.files[left].path < index.files[right].path })
	index.meanLength = float64(totalLength) / float64(max(1, len(index.files)))
	return index
}

func readTextFile(path string) (string, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxGrepFileBytes {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil || isBinary(data) {
		return "", false
	}
	return string(data), true
}

// runGrepIdent ranks files by the distinct identifiers they contain as whole
// words, case-sensitive, plus one for every identifier the lowercased path
// contains; ties break by occurrences then path. No identifiers, or no file
// holding one, is an abstention.
func runGrepIdent(index *corpusIndex, identifiers, given []string, limit int) arm {
	excluded := set(given)
	hits := make([]grepHit, 0, 256)
	for _, file := range index.files {
		if excluded[file.path] {
			continue
		}
		hit := grepHit{path: file.path}
		lowerPath := strings.ToLower(file.path)
		for _, identifier := range identifiers {
			count := wholeWordCount(file.text, identifier)
			if count > 0 {
				hit.distinct++
				hit.occurrences += count
			}
			if strings.Contains(lowerPath, strings.ToLower(identifier)) {
				hit.distinct++
			}
		}
		if hit.distinct > 0 {
			hits = append(hits, hit)
		}
	}
	return rankGrepHits(hits, limit)
}

func rankGrepHits(hits []grepHit, limit int) arm {
	sort.Slice(hits, func(left, right int) bool {
		if hits[left].distinct != hits[right].distinct {
			return hits[left].distinct > hits[right].distinct
		}
		if hits[left].occurrences != hits[right].occurrences {
			return hits[left].occurrences > hits[right].occurrences
		}
		return hits[left].path < hits[right].path
	})
	ranked := make([]string, 0, limit)
	for index := 0; index < len(hits) && index < limit; index++ {
		ranked = append(ranked, hits[index].path)
	}
	top := 0.0
	if len(hits) > 0 {
		top = float64(hits[0].distinct)
	}
	return arm{Ranked: ranked, Abstained: len(ranked) == 0, TopScore: top}
}

// wholeWordCount counts non-overlapping occurrences of word in text that no
// identifier byte touches on either side.
func wholeWordCount(text, word string) int {
	count, offset := 0, 0
	for {
		found := strings.Index(text[offset:], word)
		if found < 0 {
			return count
		}
		start := offset + found
		end := start + len(word)
		bounded := (start == 0 || !isWordByte(text[start-1])) && (end == len(text) || !isWordByte(text[end]))
		if bounded {
			count++
			offset = end
			continue
		}
		offset = start + 1
	}
}

func isWordByte(value byte) bool {
	return value == '_' || (value >= '0' && value <= '9') || (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

type bm25Hit struct {
	path  string
	score float64
}

type weightedTerm struct {
	term   string
	weight float64
}

// runBM25 ranks files by whole-file BM25 (k1 1.2, b 0.75, never tuned) with
// the inverse document frequency ln(1 + (N - n + 0.5) / (n + 0.5)) over the
// query terms the corpus holds, summed in term order; ties break by path. No
// file scoring above zero is an abstention.
func runBM25(index *corpusIndex, queryTerms, given []string, limit int) arm {
	documents := float64(len(index.files))
	weighted := make([]weightedTerm, 0, len(queryTerms))
	for _, term := range queryTerms {
		frequency := index.documentFrequency[term]
		if frequency == 0 {
			continue
		}
		weight := math.Log(1 + (documents-float64(frequency)+0.5)/(float64(frequency)+0.5))
		weighted = append(weighted, weightedTerm{term: term, weight: weight})
	}
	excluded := set(given)
	hits := make([]bm25Hit, 0, 256)
	for _, file := range index.files {
		if excluded[file.path] {
			continue
		}
		score := 0.0
		for _, entry := range weighted {
			occurrences := float64(file.counts[entry.term])
			if occurrences == 0 {
				continue
			}
			score += entry.weight * occurrences * (bm25K1 + 1) / (occurrences + bm25K1*(1-bm25B+bm25B*float64(file.length)/index.meanLength))
		}
		if score > 0 {
			hits = append(hits, bm25Hit{path: file.path, score: score})
		}
	}
	sort.Slice(hits, func(left, right int) bool {
		if hits[left].score != hits[right].score {
			return hits[left].score > hits[right].score
		}
		return hits[left].path < hits[right].path
	})
	ranked := make([]string, 0, limit)
	for index := 0; index < len(hits) && index < limit; index++ {
		ranked = append(ranked, hits[index].path)
	}
	top := 0.0
	if len(hits) > 0 {
		top = round(hits[0].score)
	}
	return arm{Ranked: ranked, Abstained: len(ranked) == 0, TopScore: top}
}

// statIndex is the cache-state observation: the copy's snapshot store.
func statIndex(root string) (os.FileInfo, error) {
	return os.Stat(contextindex.SnapshotDirectory(root))
}
