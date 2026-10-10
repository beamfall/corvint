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
