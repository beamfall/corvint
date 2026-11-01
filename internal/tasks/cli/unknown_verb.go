package cli

import (
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// unknownVerb is the CAP-V0-820 refusal of an unknown word after the verb
// group (empty for the first word): outcome ERROR, code MALFORMED, a command
// that names the group and the word when the word is an identifier, and a
// warning that quotes the word and routes it (CAP-V0-060) or suggests the
// nearest implemented verb.
func unknownVerb(group []string, word string) *wire.Result {
	cmd := append([]string{}, group...)
	quoted := ""
	if _, err := wire.ParseIdentifier("verb", word); err == nil && word != "" && !strings.ContainsAny(word, " `") {
		cmd, quoted = append(cmd, word), " `"+word+"`"
	}
	if len(cmd) == 0 {
		cmd = []string{"help"}
	}
	label := "unknown verb"
	if len(group) > 0 {
		label = "unknown " + strings.Join(group, " ") + " verb"
	}
	var matches []string
	if len(group) == 0 {
		matches = lastWordRoutes(word)
	}
	if len(matches) == 0 {
		for _, verb := range nearestVerbs(group, word) {
			matches = append(matches, "`corvint-tasks "+verb+" --help`")
		}
	}
	msg := label + quoted + "; run `corvint-tasks help`"
	if len(matches) > 0 {
		msg = label + quoted + "; did you mean " + strings.Join(matches, " or ") + "? run `corvint-tasks help` for every verb"
	}
	return &wire.Result{Command: cmd, Outcome: wire.OutcomeError, Codes: []string{wire.CodeMalformed}, Warnings: []string{prose(msg)}}
}

// groupMiss is a verb group's refusal of its first word: a missing word or
// a flag keeps the group's usage message, and any other word is an
// unknown verb of that group (CAP-V0-820).
func groupMiss(group, rest []string, msg string) *wire.Result {
	if len(rest) == 0 || rest[0] == "" || strings.HasPrefix(rest[0], "-") {
		return usage(group, msg)
	}
	return unknownVerb(group, rest[0])
}

// lastWordRoutes names the implemented verbs whose last word is the unknown
// first word, so a guessed `heartbeat --help` routes to `attempt heartbeat`
// (CAP-V0-060).
func lastWordRoutes(word string) []string {
	var matches []string
	for _, verb := range ReadVerbs {
		if i := strings.LastIndexByte(verb, ' '); i >= 0 && verb[i+1:] == word {
			matches = append(matches, "`corvint-tasks "+verb+" --help`")
		}
	}
	return matches
}

// nearestVerbs is every implemented verb under group whose next word is
// closest to word by edit distance, within 2 or a third of the word's
// length when longer; ties are all named, in ReadVerbs order.
func nearestVerbs(group []string, word string) []string {
	if word == "" || len(word) > 64 {
		return nil
	}
	limit := max(2, len(word)/3)
	prefix := strings.Join(group, " ")
	best, seen := limit+1, map[string]bool{}
	var out []string
	for _, verb := range ReadVerbs {
		words := strings.Fields(verb)
		if len(words) <= len(group) || strings.Join(words[:len(group)], " ") != prefix {
			continue
		}
		next := strings.Join(words[:len(group)+1], " ")
		if seen[next] {
			continue
		}
		seen[next] = true
		d := editDistance(word, words[len(group)])
		switch {
		case d < best:
			best, out = d, []string{next}
		case d == best && d <= limit:
			out = append(out, next)
		}
	}
	return out
}

// editDistance is the Levenshtein distance between a and b in bytes.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
