package testsource

import (
	"regexp"
	"strings"
)

// The glob dialect the runner selection reading accepts (TSA-V0-142): `**`, `*`, `?`, `[...]`,
// `{a,b}` and the extglobs `?(…)`, `*(…)`, `+(…)` and `@(…)`. A negated extglob `!(…)`, a negated
// whole pattern and anything the dialect cannot place return ok=false, so the runner abstains
// rather than guess.

// globRegexp compiles one glob to an anchored regular expression over a slash-separated path.
func globRegexp(glob string) (*regexp.Regexp, bool) {
	if glob == "" || strings.HasPrefix(glob, "!") {
		return nil, false
	}
	body, rest, ok := globPart(glob, "")
	if !ok || rest != "" {
		return nil, false
	}
	compiled, err := regexp.Compile("^" + body + "$")
	return compiled, err == nil
}

// globPart translates glob up to the first unnested byte in stops and returns the translation and
// the remaining input, which begins with that stop byte (or is empty).
func globPart(glob, stops string) (string, string, bool) {
	var b strings.Builder
	for glob != "" {
		c := glob[0]
		if strings.IndexByte(stops, c) >= 0 {
			return b.String(), glob, true
		}
		if len(glob) > 1 && glob[1] == '(' && strings.IndexByte("?*+@!", c) >= 0 {
			if c == '!' {
				return "", "", false
			}
			inner, rest, ok := globAlternatives(glob[2:], '|', ')')
			if !ok {
				return "", "", false
			}
			b.WriteString("(?:" + inner + ")" + map[byte]string{'?': "?", '*': "*", '+': "+", '@': ""}[c])
			glob = rest
			continue
		}
		switch c {
		case '*':
			if strings.HasPrefix(glob, "**") {
				switch {
				case strings.HasPrefix(glob, "**/"):
					b.WriteString("(?:.*/)?")
					glob = glob[3:]
				default:
					b.WriteString(".*")
					glob = glob[2:]
				}
				continue
			}
			b.WriteString("[^/]*")
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
			glob = glob[end+2:]
			continue
		case '{':
			inner, rest, ok := globAlternatives(glob[1:], ',', '}')
			if !ok {
				return "", "", false
			}
			b.WriteString("(?:" + inner + ")")
			glob = rest
			continue
		case '\\', '(', ')', '|', '}', ']':
			return "", "", false
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
		glob = glob[1:]
	}
	return b.String(), "", true
}

// globAlternatives translates separator-delimited alternatives up to the closing byte.
func globAlternatives(glob string, separator, closing byte) (string, string, bool) {
	parts := []string{}
	for {
		part, rest, ok := globPart(glob, string([]byte{separator, closing}))
		if !ok || rest == "" {
			return "", "", false
		}
		parts = append(parts, part)
		if rest[0] == closing {
			return strings.Join(parts, "|"), rest[1:], true
		}
		glob = rest[1:]
	}
}
