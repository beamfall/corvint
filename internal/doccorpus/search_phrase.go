package doccorpus

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// identifierWords splits text into lower-case words at every rune that is
// not a letter or digit and at case changes inside an identifier:
// "LogoutRedirectURL", "logout_redirect_url" and "logout redirect url" all
// give logout, redirect, url (DCX-V1-475). Stop words are kept, so the
// phrase "on deck" matches the segment on_deck. It is separate from the
// frozen evidence tokenizer and never changes the indexed terms.
func identifierWords(text string) []string {
	var words []string
	var word []rune
	flush := func() {
		if len(word) > 0 {
			words = append(words, strings.ToLower(string(word)))
			word = word[:0]
		}
	}
	runes := []rune(text)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(word) > 0 && unicode.IsUpper(r) {
			prev := word[len(word)-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || unicode.IsUpper(prev) && nextLower {
				flush()
			}
		}
		word = append(word, r)
	}
	flush()
	return words
}

// containsPhrase reports whether words holds phrase as a contiguous run of
// whole words.
func containsPhrase(words, phrase []string) bool {
	for i := 0; i+len(phrase) <= len(words); i++ {
		if slices.Equal(words[i:i+len(phrase)], phrase) {
			return true
		}
	}
	return false
}

// queryPhrase returns the words of a query that can match as a phrase, at
// least two of them, and whether the query is one identifier, such as
// logout_redirect_url, rather than space-separated words (DCX-V1-475).
func queryPhrase(query string) ([]string, bool) {
	words := identifierWords(query)
	if len(words) < 2 {
		return nil, false
	}
	return words, len(strings.Fields(query)) == 1
}

// phraseMatch reports whether a subject's name field holds the query phrase
// as whole words in order: its id, its name unless the name is prose, and a
// module or file subject's anchor paths (DCS-V1-400, DCS-V1-401). A query
// that is one identifier also matches inside a prose name, since an
// identifier quoted in a paragraph names the same thing (DCX-V1-475). Which
// of these holds it orders the match (phraseTier, DCS-V1-542).
func phraseMatch(s Subject, phrase []string, identifier bool) bool {
	return phraseTier(s, phrase, identifier) != tierTerms
}

// mayHoldPhrase is phraseMatch's allocation-free prefilter: a subject can
// hold the phrase only if its id, name or (for a module or file subject) an
// anchor path holds the phrase's longest word, ignoring ASCII case. A word
// with a non-ASCII rune is not filtered.
func mayHoldPhrase(s Subject, phrase []string) bool {
	word := ""
	for _, w := range phrase {
		if len(w) > len(word) {
			word = w
		}
	}
	for i := 0; i < len(word); i++ {
		if word[i] >= utf8.RuneSelf {
			return true
		}
	}
	if containsFoldASCII(s.ID, word) || containsFoldASCII(s.Name, word) {
		return true
	}
	if pathSubject(s) {
		for _, anchor := range s.Evidence.Anchors {
			if containsFoldASCII(anchor.Path, word) {
				return true
			}
		}
	}
	return false
}

// containsFoldASCII reports whether text holds the ASCII word, ignoring ASCII
// case on both sides, without allocating.
func containsFoldASCII(text, word string) bool {
	for i := 0; i+len(word) <= len(text); i++ {
		j := 0
		for ; j < len(word); j++ {
			c := text[i+j]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			w := word[j]
			if 'A' <= w && w <= 'Z' {
				w += 'a' - 'A'
			}
			if c != w {
				break
			}
		}
		if j == len(word) {
			return true
		}
	}
	return false
}

// claimPostings returns the claims whose ID or text holds term or a regular
// inflection of it, in claim order (DCX-V1-476). The index is never
// modified.
func claimPostings(x *QueryIndex, term string) []int {
	held := x.ClaimTerms[term]
	var merged []int
	for _, form := range inflections(term) {
		if more := x.ClaimTerms[form]; len(more) > 0 {
			if merged == nil {
				merged = slices.Clone(held)
			}
			merged = append(merged, more...)
		}
	}
	if merged == nil {
		return held
	}
	slices.Sort(merged)
	return slices.Compact(merged)
}

// inflections lists the regular English inflections of a lower-case
// alphabetic term of at least three letters: -s, -es, -ed and -ing, the
// doubled final consonant (set, setting) and the dropped final e (revoke,
// revoking, revoked). It is a fixed suffix list, not a stemmer, and only
// widens a query term (DCX-V1-476).
func inflections(term string) []string {
	if len(term) < 3 || strings.IndexFunc(term, func(r rune) bool { return r < 'a' || r > 'z' }) >= 0 {
		return nil
	}
	forms := []string{term + "s", term + "es", term + "ed", term + "ing"}
	last, before := term[len(term)-1], term[len(term)-2]
	vowel := func(b byte) bool { return strings.IndexByte("aeiou", b) >= 0 }
	if !vowel(last) && vowel(before) && strings.IndexByte("wxy", last) < 0 {
		forms = append(forms, term+string(last)+"ed", term+string(last)+"ing")
	}
	if last == 'e' {
		forms = append(forms, term+"d", term[:len(term)-1]+"ing")
	}
	return forms
}

// naturalLess orders record IDs with every run of digits compared by value,
// so claim:209 orders before claim:2004 (DCX-V1-477). Equal values fall back
// to the plain string order, keeping the order total.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			ei, ej := digitsEnd(a, i), digitsEnd(b, j)
			da, db := strings.TrimLeft(a[i:ei], "0"), strings.TrimLeft(b[j:ej], "0")
			if len(da) != len(db) {
				return len(da) < len(db)
			}
			if da != db {
				return da < db
			}
			i, j = ei, ej
			continue
		}
		ra, na := utf8.DecodeRuneInString(a[i:])
		rb, nb := utf8.DecodeRuneInString(b[j:])
		if ra != rb {
			return ra < rb
		}
		i, j = i+na, j+nb
	}
	if len(a)-i != len(b)-j {
		return len(a)-i < len(b)-j
	}
	return a < b
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func digitsEnd(s string, i int) int {
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// specificity is the tie-break length of a record's ranked text: a subject's
// name or a claim's text in runes. Of two records with the same score, the
// shorter holds the matched terms more densely and orders first (DCX-V1-477).
func specificity(text string) int { return utf8.RuneCountInString(text) }
