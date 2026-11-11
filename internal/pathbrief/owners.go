package pathbrief

import (
	"regexp"
	"strings"
)

// ownersFiles are the places a CODEOWNERS file is read from, first match wins.
var ownersFiles = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// ownerRule is one CODEOWNERS line that names a pattern.
type ownerRule struct {
	line    int
	pattern string
	owners  []string
	match   *regexp.Regexp
}

// parseOwners reads the rules of a CODEOWNERS file. A line whose pattern this reader cannot
// translate is kept with a matcher that matches nothing, so line numbers stay true.
func parseOwners(text string) []ownerRule {
	var rules []ownerRule
	for index, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		owners := []string{}
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "#") {
				break
			}
			owners = append(owners, field)
		}
		rules = append(rules, ownerRule{line: index + 1, pattern: fields[0], owners: owners, match: ownerPattern(fields[0])})
	}
	return rules
}

// ownerPattern translates one CODEOWNERS pattern. A pattern with a slash before its end is
// anchored at the repository top; any other matches at every depth. A pattern that names a
// directory covers everything beneath it, except that `dir/*` covers only the files directly in
// `dir`. Negation and character ranges are not CODEOWNERS syntax and match nothing.
func ownerPattern(pattern string) *regexp.Regexp {
	if strings.HasPrefix(pattern, "!") || strings.ContainsAny(pattern, "[]") {
		return nil
	}
	directory := strings.HasSuffix(pattern, "/")
	body := strings.TrimSuffix(pattern, "/")
	anchored := strings.Contains(body, "/")
	body = strings.TrimPrefix(body, "/")
	if body == "" {
		return nil
	}
	var out strings.Builder
	out.WriteString("^")
	if !anchored {
		out.WriteString("(?:.*/)?")
	}
	for index := 0; index < len(body); {
		switch {
		case strings.HasPrefix(body[index:], "**/"):
			out.WriteString("(?:.*/)?")
			index += 3
		case strings.HasPrefix(body[index:], "**"):
			out.WriteString(".*")
			index += 2
		case body[index] == '*':
			out.WriteString("[^/]*")
			index++
		case body[index] == '?':
			out.WriteString("[^/]")
			index++
		default:
			out.WriteString(regexp.QuoteMeta(body[index : index+1]))
			index++
		}
	}
	switch {
	case directory:
		out.WriteString("/.*$")
	case strings.HasSuffix(body, "/*"):
		out.WriteString("$")
	default:
		out.WriteString("(?:/.*)?$")
	}
	compiled, err := regexp.Compile(out.String())
	if err != nil {
		return nil
	}
	return compiled
}

// ownerOf returns the index of the last rule that matches the name, or -1. A directory is asked
// as its path with a trailing slash.
func ownerOf(rules []ownerRule, name string) int {
	for index := len(rules) - 1; index >= 0; index-- {
		if rules[index].match != nil && rules[index].match.MatchString(name) {
			return index
		}
	}
	return -1
}
