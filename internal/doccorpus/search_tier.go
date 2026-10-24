package doccorpus

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// The docs_search ranking tiers, in rank order (DCP-V1-541 to DCP-V1-544).
// A hit of an earlier tier orders before every hit of a later one, whatever
// the scores; within a tier hits order by score (DCP-V1-402, DCP-V1-477).
const (
	tierExact = iota
	tierIdentifier
	tierPathPhrase
	tierPhrase
	tierIDPhrase
	tierNamed
	tierTerms
)

// tierNames are the served tier values (DCP-V1-544).
var tierNames = [...]string{"exact", "identifier", "path-phrase", "phrase", "id-phrase", "named", "terms"}

// queryGroups groups a query's evidence terms by the query word that yields
// them, so "conditions" and its singular "condition" are one group and the
// word counts once (DCP-V1-540). Each term joins the first word that yields
// it; a term no single word yields, such as an identifier's compound, is a
// group of its own. Terms within a group and the groups by their first term
// are sorted, so the score sums in a fixed order.
func queryGroups(query string, terms []string) [][]string {
	open := make(map[string]bool, len(terms))
	for _, term := range terms {
		open[term] = true
	}
	var groups [][]string
	for _, word := range identifierWords(query) {
		var group []string
		for term := range contextindex.EvidenceTerms(word) {
			if open[term] {
				group = append(group, term)
				delete(open, term)
			}
		}
		if len(group) > 0 {
			slices.Sort(group)
			groups = append(groups, group)
		}
	}
	for _, term := range terms {
		if open[term] {
			groups = append(groups, []string{term})
		}
	}
	slices.SortFunc(groups, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
	return groups
}

// phraseTier is the phrase tier of a subject for the query phrase
// (DCP-V1-475, DCP-V1-542): tierPathPhrase when a module or file subject's
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
// (DCP-V1-541). A token is a run of letters, digits, '_', '-' and '.', so the
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

// questionWords open a question-shaped query (DCP-V1-543).
var questionWords = words("what how why when where which who whom whose does do did is are was were can could should would will has have")

// questionShaped reports whether a query is a question: it ends with '?' or
// its first whitespace-separated field is a question word, ignoring case
// (DCP-V1-543). The identifier is_admin is not a question.
func questionShaped(query string) bool {
	q := strings.TrimSpace(query)
	if strings.HasSuffix(q, "?") {
		return true
	}
	fields := strings.Fields(q)
	return len(fields) > 0 && questionWords[strings.ToLower(fields[0])]
}

// namedInQuery reports whether a subject's name of at least two words, at
// least one of them an evidence term, appears as a contiguous run of the
// question's words (DCP-V1-543): the flow "Reservation Cancellation" is named
// in "What breaks in Reservation Cancellation, and under what conditions?". A
// prose name never is. folded is the lower-cased query, which an
// allocation-free test reads before the words are compared.
func namedInQuery(s Subject, queryWords []string, folded string) bool {
	if !mayBeNamedIn(s.Name, folded) {
		return false
	}
	name := identifierWords(s.Name)
	if len(name) < 2 || !containsPhrase(queryWords, name) {
		return false
	}
	terms := contextindex.EvidenceTerms(s.Name)
	return len(terms) > 0 && (pathSubject(s) || len(terms) <= proseNameTerms)
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
