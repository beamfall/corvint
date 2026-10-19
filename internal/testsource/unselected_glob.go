package testsource

import (
	"regexp"
	"strconv"
	"strings"
)

// The glob dialect the runner selection reading accepts (TSA-V0-142): braces `{a,b}` and numeric
// ranges `{1..3}` expand first, as minimatch does; then `**` as a whole path segment, `*`, `?`,
// `[...]` and the extglobs `?(…)`, `*(…)`, `+(…)` and `@(…)`. A `**` that is not a whole segment
// is a `*`. A negated extglob `!(…)`, a negated whole pattern, a `**` inside an extglob, another
// brace range and anything the dialect cannot place return ok=false, so the runner abstains
// rather than guess.

// maxBraceExpansions bounds the patterns one glob expands to.
const maxBraceExpansions = 256

// globRegexp compiles one glob to an anchored regular expression over a slash-separated path.
func globRegexp(glob string) (*regexp.Regexp, bool) {
	return globsRegexp([]string{glob}, false)
}

// globsRegexp compiles globs to one anchored regular expression matching any of them; nocase
// matches case-insensitively, as Playwright does.
func globsRegexp(globs []string, nocase bool) (*regexp.Regexp, bool) {
	bodies := []string{}
	for _, glob := range globs {
		if glob == "" || strings.HasPrefix(glob, "!") {
			return nil, false
		}
		expanded, ok := expandBraces(glob)
		if !ok {
			return nil, false
		}
		for _, one := range expanded {
			body, rest, ok := globPart(one, "", true)
			if !ok || rest != "" {
				return nil, false
			}
			bodies = append(bodies, body)
		}
	}
	flags := ""
	if nocase {
		flags = "(?i)"
	}
	compiled, err := regexp.Compile(flags + "^(?:" + strings.Join(bodies, "|") + ")$")
	return compiled, err == nil
}

// expandBraces expands the first brace group of glob, then the rest, recursively.
func expandBraces(glob string) ([]string, bool) {
	open := strings.IndexByte(glob, '{')
	if open < 0 {
		return []string{glob}, !strings.ContainsRune(glob, '}')
	}
	if strings.IndexByte(glob[:open], '}') >= 0 {
		return nil, false
	}
	braces, parens, closing, cuts := 0, 0, -1, []int{open}
	for i := open; i < len(glob) && closing < 0; i++ {
		switch glob[i] {
		case '{':
			braces++
		case '}':
			braces--
			if braces == 0 {
				closing = i
			}
		case '(':
			parens++
		case ')':
			parens--
		case ',':
			if braces == 1 && parens == 0 {
				cuts = append(cuts, i)
			}
		}
	}
	if closing < 0 {
		return nil, false
	}
	cuts = append(cuts, closing)
	alternatives := []string{}
	if len(cuts) == 2 {
		values, ok := braceRange(glob[open+1 : closing])
		if !ok {
			return nil, false
		}
		alternatives = values
	} else {
		for i := 0; i+1 < len(cuts); i++ {
			alternatives = append(alternatives, glob[cuts[i]+1:cuts[i+1]])
		}
	}
	out := []string{}
	for _, alternative := range alternatives {
		expanded, ok := expandBraces(glob[:open] + alternative + glob[closing+1:])
		if !ok || len(out)+len(expanded) > maxBraceExpansions {
			return nil, false
		}
		out = append(out, expanded...)
	}
	return out, true
}

// braceRange expands a numeric range such as `1..3`; any other brace without a comma (a letter
// range, a stepped or zero-padded range, or a lone `{a}`) is outside the dialect.
func braceRange(text string) ([]string, bool) {
	low, high, ok := strings.Cut(text, "..")
	if !ok {
		return nil, false
	}
	from, errFrom := strconv.Atoi(low)
	to, errTo := strconv.Atoi(high)
	if errFrom != nil || errTo != nil || strconv.Itoa(from) != low || strconv.Itoa(to) != high {
		return nil, false
	}
	step := 1
	if to < from {
		step = -1
	}
	if (to-from)*step >= maxBraceExpansions {
		return nil, false
	}
	out := []string{}
	for n := from; ; n += step {
		out = append(out, strconv.Itoa(n))
		if n == to {
			return out, true
		}
	}
}

// globPart translates glob up to the first unnested byte in stops and returns the translation and
// the remaining input, which begins with that stop byte (or is empty). top is false inside an
// extglob, where a `**` is outside the dialect.
func globPart(glob, stops string, top bool) (string, string, bool) {
	var b strings.Builder
	segmentStart := top
	for glob != "" {
		c := glob[0]
		if strings.IndexByte(stops, c) >= 0 {
			return b.String(), glob, true
		}
		if len(glob) > 1 && glob[1] == '(' && strings.IndexByte("?*+@!", c) >= 0 {
			if c == '!' {
				return "", "", false
			}
			inner, rest, ok := globAlternatives(glob[2:])
			if !ok {
				return "", "", false
			}
			b.WriteString("(?:" + inner + ")" + map[byte]string{'?': "?", '*': "*", '+': "+", '@': ""}[c])
			glob, segmentStart = rest, false
			continue
		}
		switch c {
		case '*':
			stars := len(glob) - len(strings.TrimLeft(glob, "*"))
			if stars > 1 && !top {
				return "", "", false
			}
			rest := glob[stars:]
			if strings.HasPrefix(rest, "(") {
				stars, rest = stars-1, glob[stars-1:] // the last `*` opens an extglob
			}
			switch {
			case stars == 2 && segmentStart && strings.HasPrefix(rest, "/"):
				b.WriteString("(?:.*/)?")
				glob = rest[1:]
				continue // still at a segment start
			case stars == 2 && segmentStart && rest == "":
				b.WriteString(".*")
			default:
				// A `*` or a `**` that is not a whole path segment stays within one segment.
				b.WriteString("[^/]*")
			}
			glob, segmentStart = rest, false
			continue
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(glob[1:], ']')
			if end < 1 {
				return "", "", false
			}
			class := glob[1 : end+1]
			if class[0] == '!' {
				class = "^" + class[1:]
			}
			if strings.ContainsAny(class, "[\\") {
				return "", "", false
			}
			b.WriteString("[" + class + "]")
			glob, segmentStart = glob[end+2:], false
			continue
		case '\\', '(', ')', '|', '{', '}', ']':
			return "", "", false
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
		segmentStart = c == '/'
		glob = glob[1:]
	}
	return b.String(), "", true
}

// globAlternatives translates `|`-delimited extglob alternatives up to the closing `)`.
func globAlternatives(glob string) (string, string, bool) {
	parts := []string{}
	for {
		part, rest, ok := globPart(glob, "|)", false)
		if !ok || rest == "" {
			return "", "", false
		}
		parts = append(parts, part)
		if rest[0] == ')' {
			return strings.Join(parts, "|"), rest[1:], true
		}
		glob = rest[1:]
	}
}
