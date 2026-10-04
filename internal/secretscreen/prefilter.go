package secretscreen

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
)

// alternative is one top-level branch of Pattern with a necessary condition.
// need reports false only when no substring of the screened text can match
// expr; it receives the text with ASCII letters lowered. A branch that can
// match nowhere contributes nothing to a leftmost-first search, so dropping
// it leaves every match unchanged (LTA-V0-014).
type alternative struct {
	expr string
	need func(lower []byte) bool
}

// writerAlternatives is Pattern in precedence order; see the comments on the
// expression constants for why each branch sits where it does.
var writerAlternatives = func() []alternative {
	secret := secretAlternatives(writerAssignmentFields, writerCredentialedURLAlt)
	// secretPattern has always carried its own flag group; keeping it here
	// leaves Pattern's source text unchanged.
	secret[0] = `(?i)` + secret[0]
	secretNeeds := []func([]byte) bool{
		assignmentCandidate,
		contains(`-----begin `), contains(`-----begin `), contains(`-----begin `), contains(`-----begin `),
		all(contains(`://`), contains(`@`)),
		contains(`ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`), contains(`github_pat_`),
		contains(`glpat-`), slackTokenPrefix,
		contains(`akia`, `asia`), contains(`aiza`), contains(`npm_`),
		contains(`sk-`), contains(`_live_`, `_test_`),
		contains(`pypi-ageichlwasi5vcmc`),
		contains(`sg.`), literalThenHex(`sk`, 32),
		contains(`bearer`),
		dottedTriple,
	}
	alternatives := []alternative{
		{writerQuotedAssignmentAlt, assignmentCandidate},
		{awsAccessKeyIDWithSecretAlt, contains(`akia`, `asia`)},
		{authorizationSchemeAlt, contains(`authorization`)},
		{writerURLTokenUserinfoAlt, all(contains(`://`), contains(`@`))},
	}
	for i, expr := range secret {
		alternatives = append(alternatives, alternative{expr, secretNeeds[i]})
	}
	alternatives = append(alternatives, alternative{
		`"` + writerAssignmentFields + `"[` + pythonWhitespace + `]*:[` + pythonWhitespace + `]*(?:"(?:[^"\\]|\\[\s\S])*(?:"|\\?\z)|[^` + pythonWhitespace + `]+)`,
		assignmentCandidate,
	})
	vendorNeeds := []func([]byte) bool{
		contains(`whsec_`), contains(`hf_`), contains(`dop_v1_`), contains(`xapp-`),
		contains(`ya29.`), contains(`gocspx-`), contains(`shpat_`, `shpss_`, `shpca_`), contains(`glrt-`, `gldt-`),
		contains(`hooks.slack.com/services/`),
	}
	for i, expr := range writerOnlyVendorAlts {
		alternatives = append(alternatives, alternative{expr, vendorNeeds[i]})
	}
	flagNeeds := []func([]byte) bool{
		flagCandidate,
		all(contains(`login`, `-u`), contains(`-p`)),
		contains(`curl`),
	}
	for i, expr := range writerCredentialFlagAlts {
		alternatives = append(alternatives, alternative{expr, flagNeeds[i]})
	}
	return alternatives
}()

func writerExpression(alternatives []alternative) string {
	exprs := make([]string, len(alternatives))
	for i, a := range alternatives {
		exprs[i] = a.expr
	}
	return `(?i)` + strings.Join(exprs, `|`)
}

func contains(literals ...string) func([]byte) bool {
	wanted := make([][]byte, len(literals))
	for i, literal := range literals {
		wanted[i] = []byte(literal)
	}
	return func(lower []byte) bool {
		for _, literal := range wanted {
			if bytes.Contains(lower, literal) {
				return true
			}
		}
		return false
	}
}

func all(needs ...func([]byte) bool) func([]byte) bool {
	return func(lower []byte) bool {
		for _, need := range needs {
			if !need(lower) {
				return false
			}
		}
		return true
	}
}

// literalThenHex reports whether literal is somewhere followed directly by n
// hexadecimal digits.
func literalThenHex(literal string, n int) func([]byte) bool {
	return func(lower []byte) bool {
		for rest := lower; ; {
			at := bytes.Index(rest, []byte(literal))
			if at < 0 {
				return false
			}
			after := rest[at+len(literal):]
			run := 0
			for run < len(after) && run < n && (after[run] >= '0' && after[run] <= '9' || after[run] >= 'a' && after[run] <= 'f') {
				run++
			}
			if run == n {
				return true
			}
			rest = rest[at+1:]
		}
	}
}

// slackTokenPrefix reports whether `xox`, one more byte and a dash occur
// together; base64 text holds a bare `xox` often enough to matter.
func slackTokenPrefix(lower []byte) bool {
	for rest := lower; ; {
		at := bytes.Index(rest, []byte(`xox`))
		if at < 0 {
			return false
		}
		// Advance one byte, not past the literal: occurrences can overlap.
		rest = rest[at+1:]
		if len(rest) > 3 && rest[3] == '-' {
			return true
		}
	}
}

// credentialStems covers writerAssignmentNames: every name there contains one
// of these, and may be followed by further field characters. Bare pass must
// end the field, so it is checked without a suffix.
var credentialStems = []string{"key", "authorization", "credential", "passphrase", "passwd", "password", "secret", "token"}

func fieldByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

// afterCredentialName calls next with the text following each place a
// credential field name can end, and reports whether any call returned true.
func afterCredentialName(lower []byte, next func(rest []byte) bool) bool {
	scan := func(stem string, suffix bool) bool {
		for rest := lower; ; {
			at := bytes.Index(rest, []byte(stem))
			if at < 0 {
				return false
			}
			after := rest[at+len(stem):]
			end := 0
			for suffix && end < len(after) && fieldByte(after[end]) {
				end++
			}
			if next(after[end:]) {
				return true
			}
			rest = rest[at+1:]
		}
	}
	for _, stem := range credentialStems {
		if scan(stem, true) {
			return true
		}
	}
	return scan("pass", false)
}

// assignmentCandidate is the necessary condition shared by the three
// assignment branches: a credential field name, an optional closing quote
// (the JSON property form), optional whitespace and then `:` or `=`. Any
// non-ASCII byte in the whitespace position counts as whitespace, which only
// widens the condition.
func assignmentCandidate(lower []byte) bool {
	return afterCredentialName(lower, func(rest []byte) bool {
		if len(rest) > 0 && rest[0] == '"' {
			rest = rest[1:]
		}
		for len(rest) > 0 {
			switch c := rest[0]; {
			case c == ':' || c == '=' || c >= 0x80:
				return true
			case c >= '\t' && c <= '\r' || c >= 0x1c && c <= 0x20:
				rest = rest[1:]
			default:
				return false
			}
		}
		return false
	})
}

// flagCandidate is the necessary condition for a credential flag with a
// separate argument: a dash somewhere, and a credential name followed by a
// space or tab.
func flagCandidate(lower []byte) bool {
	return bytes.IndexByte(lower, '-') >= 0 && afterCredentialName(lower, func(rest []byte) bool {
		return len(rest) > 0 && (rest[0] == ' ' || rest[0] == '\t')
	})
}

func tokenByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// dottedTriple reports whether the text holds three runs of at least ten
// token characters joined by single dots, the JWT-shaped branch's whole shape.
func dottedTriple(lower []byte) bool {
	const floor = 10
	run, parts := 0, 0
	for _, c := range lower {
		switch {
		case tokenByte(c):
			run++
			if run >= floor && parts == 2 {
				return true
			}
		case c == '.' && run >= floor:
			parts++
			if parts > 2 {
				parts = 2
			}
			run = 0
		default:
			run, parts = 0, 0
		}
	}
	return false
}

// maxLivePatterns bounds the compiled-subset cache. Past it, texts with a
// subset not yet compiled are screened with Pattern itself.
const maxLivePatterns = 128

var livePatterns struct {
	sync.Mutex
	bySubset map[uint64]*regexp.Regexp
}

// livePattern returns a detector equivalent to Pattern on text: Pattern
// restricted to the branches whose necessary condition holds, or nil when
// none does.
func livePattern(text string) *regexp.Regexp {
	// Under (?i), `k` also matches U+212A and `s` also matches U+017F. Those
	// are the only non-ASCII runes that fold to an ASCII letter, so a text
	// without them is compared after lowering ASCII letters alone.
	if strings.Contains(text, "\u212a") || strings.Contains(text, "\u017f") {
		return Pattern
	}
	lower := []byte(text)
	for i, c := range lower {
		if c >= 'A' && c <= 'Z' {
			lower[i] = c + ('a' - 'A')
		}
	}
	var subset uint64
	for i, a := range writerAlternatives {
		if a.need(lower) {
			subset |= 1 << i
		}
	}
	if subset == 0 {
		return nil
	}
	if subset == 1<<len(writerAlternatives)-1 {
		return Pattern
	}
	livePatterns.Lock()
	defer livePatterns.Unlock()
	if pattern, ok := livePatterns.bySubset[subset]; ok {
		return pattern
	}
	if len(livePatterns.bySubset) >= maxLivePatterns {
		return Pattern
	}
	var live []alternative
	for i, a := range writerAlternatives {
		if subset&(1<<i) != 0 {
			live = append(live, a)
		}
	}
	pattern := regexp.MustCompile(writerExpression(live))
	if livePatterns.bySubset == nil {
		livePatterns.bySubset = map[uint64]*regexp.Regexp{}
	}
	livePatterns.bySubset[subset] = pattern
	return pattern
}
