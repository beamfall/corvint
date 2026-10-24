package doccorpus

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// The docs_search ranking tiers, in rank order (DCS-V1-541 to DCS-V1-544).
// A hit of an earlier tier orders before every hit of a later one, whatever
// the scores; within a tier hits order by score (DCS-V1-402, DCX-V1-477).
const (
	tierExact = iota
	tierIdentifier
	tierPathPhrase
	tierPhrase
	tierIDPhrase
	tierNamed
	tierTerms
)

// tierNames are the served tier values (DCS-V1-544).
var tierNames = [...]string{"exact", "identifier", "path-phrase", "phrase", "id-phrase", "named", "terms"}

// queryGroups groups a query's evidence terms by the query words that yield
// them, so "conditions" and its singular "condition" are one group and the
// word counts once (DCS-V1-540). Terms that one word yields join one group,
// and groups sharing a term merge, so the grouping does not depend on word
// order: "condition conditions" and "conditions condition" both give one
// group. A term no word yields, such as an identifier's compound, is a group
// of its own. Terms within a group and the groups by their first term are
// sorted, so the score sums in a fixed order.
func queryGroups(query string, terms []string) [][]string {
	parent := make(map[string]string, len(terms))
	for _, term := range terms {
		parent[term] = term
	}
	root := func(term string) string {
		for parent[term] != term {
			term = parent[term]
		}
		return term
	}
	for _, word := range identifierWords(query) {
		first := ""
		for term := range contextindex.EvidenceTerms(word) {
			if _, ok := parent[term]; !ok {
				continue
			}
			if first == "" {
				first = root(term)
				continue
			}
			if r := root(term); r != first {
				// Link the larger root under the smaller, so the forest's
				// shape does not depend on map iteration order.
				lo, hi := min(r, first), max(r, first)
				parent[hi] = lo
				first = lo
			}
		}
	}
	byRoot := map[string][]string{}
	for _, term := range terms {
		r := root(term)
		byRoot[r] = append(byRoot[r], term)
	}
	groups := make([][]string, 0, len(byRoot))
	for _, group := range byRoot {
		slices.Sort(group)
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
	return groups
}

// phraseTier is the phrase tier of a subject for the query phrase
// (DCX-V1-475, DCS-V1-542): tierPathPhrase when a module or file subject's
// name or anchor path holds it, tierPhrase when another subject's name does
// (unless the name is prose and the query is not one identifier), tierIDPhrase
// when only the subject's ID does, and tierTerms otherwise.
func phraseTier(s Subject, phrase []string, identifier bool) int {
	if pathSubject(s) {
		if containsPhrase(identifierWords(s.Name), phrase) {
			return tierPathPhrase
		}
		for _, p := range extraPaths(s) {
			if containsPhrase(identifierWords(p), phrase) {
				return tierPathPhrase
			}
		}
	} else if identifier || len(contextindex.EvidenceTerms(s.Name)) <= proseNameTerms {
		if containsPhrase(identifierWords(s.Name), phrase) {
			return tierPhrase
		}
	}
	if containsPhrase(identifierWords(s.ID), phrase) {
		return tierIDPhrase
	}
	return tierTerms
}

// identifierToken reports whether one token of the subject's name, or of a
// module or file subject's anchor path, spells the query identifier: its
// words, or its words without the token's last dot suffix, equal the query's
// (DCS-V1-541). A token is a run of letters, digits, '_', '-' and '.', so the
// path src/app/services/pos_sync.rb holds the token pos_sync.rb, which spells
// pos_sync, PosSync and pos_sync.rb, while the name "POS sync dashboard"
// spells none of them. The subject's ID is not read.
func identifierToken(s Subject, phrase []string) bool {
	if spellsIdentifier(s.Name, phrase) {
		return true
	}
	if pathSubject(s) {
		for _, p := range extraPaths(s) {
			if spellsIdentifier(p, phrase) {
				return true
			}
		}
	}
	return false
}

func spellsIdentifier(text string, phrase []string) bool {
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.'
	})
	for _, token := range tokens {
		token = strings.Trim(token, ".")
		if slices.Equal(identifierWords(token), phrase) {
			return true
		}
		if dot := strings.LastIndexByte(token, '.'); dot > 0 && slices.Equal(identifierWords(token[:dot]), phrase) {
			return true
		}
	}
	return false
}

// questionWords open a question-shaped query (DCS-V1-543).
var questionWords = words("what how why when where which who whom whose does do did is are was were can could should would will has have")

// questionShaped reports whether a query is a question: it ends with '?' or
// its first whitespace-separated field is a question word, ignoring case
// (DCS-V1-543). The identifier is_admin is not a question.
func questionShaped(query string) bool {
	q := strings.TrimSpace(query)
	if strings.HasSuffix(q, "?") {
		return true
	}
	fields := strings.Fields(q)
	return len(fields) > 0 && questionWords[strings.ToLower(fields[0])]
}

// namedInQuery reports whether a question names a subject (DCS-V1-543): its
// name of at least two words, at least two of them content words, appears as
// a contiguous run of the question's words (DCS-V1-567), so the flow
// "Reservation Cancellation" is named in "What breaks in Reservation
// Cancellation, and under what conditions?" while "In the source" is not. A
// one-word name is named only by a capitalized query word that no other
// capitalized word adjoins (DCS-V1-548, DCS-V1-567): "Waitlist" in "How does
// Waitlist work?", but not "Cancellation" in "Where is Reservation
// Cancellation implemented?". A prose name never is. A subject is also named
// by its ID slug (DCS-V1-566). folded is the lower-cased query, which an
// allocation-free test reads before the words are compared.
func namedInQuery(s Subject, queryWords []string, folded string, capitalized map[string]bool) bool {
	return nameNamed(s, queryWords, folded, capitalized) || slugNamed(s.ID, queryWords, folded)
}

func nameNamed(s Subject, queryWords []string, folded string, capitalized map[string]bool) bool {
	if !mayBeNamedIn(s.Name, folded) {
		return false
	}
	name := identifierWords(s.Name)
	switch {
	case len(name) == 1:
		if !capitalized[name[0]] {
			return false
		}
	case len(name) < 2 || contentWords(name) < 2 || !containsPhrase(queryWords, name):
		return false
	}
	terms := contextindex.EvidenceTerms(s.Name)
	return len(terms) > 0 && (pathSubject(s) || len(terms) <= proseNameTerms)
}

// slugNamed reports whether a subject's ID slug, the ID after its last '/' or
// ':' without its last dot suffix, holds at least two words, two of them
// content words, that appear as a contiguous run of the question's words
// (DCS-V1-566): adapter:flow:flows/reservation-cancellation and
// docs/reservation-cancellation.md are named in "Where in the source is
// Reservation Cancellation implemented?".
func slugNamed(id string, queryWords []string, folded string) bool {
	slug := id[strings.LastIndexAny(id, "/:")+1:]
	if dot := strings.LastIndexByte(slug, '.'); dot > 0 {
		slug = slug[:dot]
	}
	if !mayBeNamedIn(slug, folded) {
		return false
	}
	words := identifierWords(slug)
	return len(words) >= 2 && contentWords(words) >= 2 && containsPhrase(queryWords, words)
}

// frameWords say where or how a question asks about its subject rather than
// what the subject is (DCS-V1-565): "Where in the source is X implemented?"
// and "How does X work?" are about X.
var frameWords = words("source sources codebase repo repository implemented implement implements implementation implementations defined define defines definition declared declare declares located location live lives handled handle handles work works happen happens")

// contentWord reports whether a question word says what the question is
// about: it yields an evidence term and is neither a frame word nor a
// question word (DCS-V1-565).
func contentWord(word string) bool {
	return !frameWords[word] && !questionWords[word] && len(contextindex.EvidenceTerms(word)) > 0
}

func contentWords(words []string) int {
	n := 0
	for _, w := range words {
		if contentWord(w) {
			n++
		}
	}
	return n
}

// coveredGroups reports, for each query term group, whether it counts as a
// covered query word (DCS-V1-547, DCS-V1-565). In a question a group every
// term of which only frame and question words yield is not covered, so a
// result holding "source" and "implemented" holds no more of "Where in the
// source is Player Types implemented?" than one holding neither. A term no
// word yields, such as an identifier's compound, is content. In a statement
// every group is covered.
func coveredGroups(query string, groups [][]string) []bool {
	covered := make([]bool, len(groups))
	for i := range covered {
		covered[i] = true
	}
	if !questionShaped(query) {
		return covered
	}
	frame, content := map[string]bool{}, map[string]bool{}
	for _, word := range identifierWords(query) {
		set := content
		if !contentWord(word) {
			set = frame
		}
		for term := range contextindex.EvidenceTerms(word) {
			set[term] = true
		}
	}
	for i, group := range groups {
		covered[i] = slices.ContainsFunc(group, func(term string) bool { return content[term] || !frame[term] })
	}
	return covered
}

// capitalizedWords are the words of a question that may name a one-word
// subject (DCS-V1-548): a field of letters and digits, other than the first,
// whose first letter is upper case, that is one identifier word, an evidence
// term and not a question word. A capitalized field that another capitalized
// field other than the first adjoins is part of a longer name and yields
// nothing (DCS-V1-567). "How does Waitlist work?" yields waitlist; the
// sentence-initial "What", the compound "WaitlistNotifier", the lower case
// "waitlist" and the "Cancellation" of "Reservation Cancellation" yield
// nothing.
func capitalizedWords(query string) map[string]bool {
	fields := strings.FieldsFunc(query, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	capital := func(n int) bool {
		if n < 1 || n >= len(fields) {
			return false
		}
		first, _ := utf8.DecodeRuneInString(fields[n])
		return unicode.IsUpper(first)
	}
	words := map[string]bool{}
	for n, field := range fields {
		if !capital(n) || capital(n-1) || capital(n+1) {
			continue
		}
		parts := identifierWords(field)
		if len(parts) != 1 || questionWords[parts[0]] || len(contextindex.EvidenceTerms(parts[0])) == 0 {
			continue
		}
		words[parts[0]] = true
	}
	return words
}

// mayBeNamedIn is namedInQuery's prefilter: every ASCII letter-and-digit
// piece of the name, split also before each upper-case letter, occurs in the
// lower-cased query, ignoring ASCII case. Each such piece lies inside one of
// the name's words, so a name whose words the query holds always passes. A
// name with a non-ASCII byte is not filtered.
func mayBeNamedIn(name, folded string) bool {
	start := -1
	for i := 0; i <= len(name); i++ {
		c := byte(' ')
		if i < len(name) {
			c = name[i]
		}
		if c >= utf8.RuneSelf {
			return true
		}
		alnum := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
		if start >= 0 && (!alnum || 'A' <= c && c <= 'Z') {
			if !containsFoldASCII(folded, name[start:i]) {
				return false
			}
			start = -1
		}
		if alnum && start < 0 {
			start = i
		}
	}
	return true
}
