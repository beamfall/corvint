package obligation

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/secretscreen"
)

// MaxExcerptBytes bounds one quoted failure or log line.
const MaxExcerptBytes = 240

// ExcerptWithheld replaces an excerpt that matches the secret screen.
const ExcerptWithheld = "line withheld: it matches the secret screen"

var (
	expectedFailWords = regexp.MustCompile(`(?i)(^|[^a-z])expected[-_ ]?(to[-_ ])?fail`)
	defectToken       = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,15}-[1-9][0-9]{0,8}$`)
	ansiEscape        = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]")
	actionableLine    = regexp.MustCompile(`(?i)(^|[^a-z])(error|errors|fail|failed|failure|failures|mismatch|violation|violations|invalid|does not match)([^a-z]|$)|✘|✗|×`)
)

// SaysExpectedFail reports whether a test or step title declares an
// expected failure ("expected-fail", "expected fail", "expected to fail").
func SaysExpectedFail(title string) bool { return expectedFailWords.MatchString(title) }

// DefectIDs returns, in order of first occurrence, the distinct whole tokens
// of s shaped like a defect id (`[A-Z][A-Z0-9]{0,15}-N`) whose prefix is not
// the ledger prefix (TOL-V0-022).
func DefectIDs(s, prefix string) []string {
	var out []string
	seen := map[string]bool{}
	for i := 0; i < len(s); {
		if !idByte(s[i]) {
			i++
			continue
		}
		j := i
		for j < len(s) && idByte(s[j]) {
			j++
		}
		tok := s[i:j]
		i = j
		if !defectToken.MatchString(tok) || strings.HasPrefix(tok, prefix+"-") || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// Excerpt is the first non-blank line of s with terminal escapes and control
// characters removed, bounded to MaxExcerptBytes, or ExcerptWithheld when it
// matches the secret screen. It is "" when s has no non-blank line.
func Excerpt(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if e := cleanLine(line); e != "" {
			return screened(e)
		}
	}
	return ""
}

// FirstActionableLine is the excerpt of the first non-blank line of log that
// names an error, failure, mismatch or violation, else of its first
// non-blank line (TOL-V0-024, TOL-V0-027).
func FirstActionableLine(log string) string {
	first := ""
	for _, line := range strings.Split(log, "\n") {
		e := cleanLine(line)
		if e == "" {
			continue
		}
		if actionableLine.MatchString(e) {
			return screened(e)
		}
		if first == "" {
			first = e
		}
	}
	return screened(first)
}

// FailureTail is the excerpts of the last n non-blank lines of log, oldest
// first, each cleaned and bounded as Excerpt is, and the excerpt of the last
// of them that names an error, failure, mismatch or violation, else of the
// last non-blank line (TCD-V0-165). A line is ExcerptWithheld when its
// cleaned or raw text matches the secret screen, or when it overlaps a
// secret-screen match over the raw text of the whole log, so a match that
// spans lines, the excerpt bound or the tail boundary is withheld too. A
// private-key BEGIN marker with no END marker after it withholds every line
// to the end of the log. Callers bound log (the gate output bound), and the
// screen covers all of it, so a key block whose BEGIN line lies before the
// tail never leaks its body.
func FailureTail(log string, n int) (tail []string, failing string) {
	return FailureTailMatching(log, n, nil)
}

// makeWrapperLine is a make driver's own report of a failed recipe or a
// directory change ("make: *** [gate] Error 1", "make[2]: *** [Makefile:9:
// test] Error 2", "make[1]: Leaving directory ..."). It names the target,
// never the failing step, so the default choice of failing skips it
// (TCD-V0-800, V1-3940).
var makeWrapperLine = regexp.MustCompile(`^g?make(\[[0-9]+\])?: (\*\*\* |Entering directory|Leaving directory)`)

// FailureTailMatching is FailureTail with a policy gate's failurePattern
// (TCD-V0-801): when pattern is non-nil, failing is the last tail line it
// matches that is not withheld; when it matches none, or pattern is nil,
// failing is the last actionable tail line that is not a make wrapper line,
// else the last tail line.
func FailureTailMatching(log string, n int, pattern *regexp.Regexp) (tail []string, failing string) {
	type picked struct {
		start, end int
		text       string
	}
	var picks []picked
	for rest := len(log); len(picks) < n && rest > 0; {
		start := strings.LastIndexByte(log[:rest], '\n') + 1
		if e := cleanLine(log[start:rest]); e != "" {
			picks = append(picks, picked{start, rest, e})
		}
		rest = start - 1
	}
	if len(picks) == 0 {
		return nil, ""
	}
	matched := ""
	spans := secretscreen.MatchIndexes(log)
	for i, sp := range spans {
		if unterminatedKeyBlock(log[sp[0]:sp[1]]) {
			spans[i][1] = len(log)
		}
	}
	tail = make([]string, len(picks))
	for i, p := range picks {
		text := screened(p.text)
		if text != ExcerptWithheld && MatchesSecretScreen(ansiEscape.ReplaceAllString(log[p.start:p.end], "")) {
			text = ExcerptWithheld
		}
		for _, sp := range spans {
			if sp[0] < p.end && sp[1] > p.start {
				text = ExcerptWithheld
			}
		}
		if failing == "" && actionableLine.MatchString(p.text) && !makeWrapperLine.MatchString(p.text) {
			failing = text
		}
		if pattern != nil && matched == "" && text != ExcerptWithheld && pattern.MatchString(p.text) {
			matched = text
		}
		tail[len(picks)-1-i] = text
	}
	if matched != "" {
		failing = matched
	}
	if failing == "" {
		failing = tail[len(tail)-1]
	}
	return tail, failing
}

// unterminatedKeyBlock reports whether a secret-screen match is a bare
// private-key BEGIN marker: the screen matches the whole block only when its
// END marker follows, so a bare marker means the key body runs on unmatched.
func unterminatedKeyBlock(match string) bool {
	upper := strings.ToUpper(match)
	return strings.HasPrefix(upper, "-----BEGIN ") && !strings.Contains(upper, "-----END ")
}

func cleanLine(line string) string {
	line = ansiEscape.ReplaceAllString(line, "")
	line = strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if unicode.IsControl(r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, line)
	line = strings.TrimSpace(line)
	if len(line) > MaxExcerptBytes {
		cut := MaxExcerptBytes
		for cut > 0 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		line = line[:cut]
	}
	return line
}

func screened(s string) string {
	if s != "" && secretscreen.MatchString(s) {
		return ExcerptWithheld
	}
	return s
}

func sortedUnique(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for i, s := range in {
		if i == 0 || s != in[i-1] {
			out = append(out, s)
		}
	}
	return out
}

// MatchesSecretScreen reports whether s matches the shared secret screen. It
// keeps the decision 0397 #712 import edge in this one file while the
// TDR-V0-006 defect register reuses the screen.
func MatchesSecretScreen(s string) bool { return s != "" && secretscreen.MatchString(s) }

// SecretScreenSpans returns the byte spans of s the shared secret screen
// matches, through the same import edge as MatchesSecretScreen.
func SecretScreenSpans(s string) [][]int { return secretscreen.MatchIndexes(s) }
