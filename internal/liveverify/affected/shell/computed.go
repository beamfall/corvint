package shell

import (
	"path"
	"slices"
	"strings"
)

// This file models a computed command word or interpreter operand
// (TSP-V0-960). A word such as "$heavy" or "$dir/$name.sh" names no literal
// path, so the resolver alone gives it no edge. Its value is expanded through
// the file's own assignments, `${NAME-default}` defaults and `for NAME in`
// word lists: each fully literal value is linked like a literal target, and
// each value whose file name is still computed but ends in an owned shell
// suffix is a constructed script name. A constructed name keeps the caller's
// FrontierForeignInvocation and counts, in the foreign-caller completeness
// scan, as naming every script it may be. A value with no literal shell
// suffix (a binary in a variable, "$@", an environment input) is not a script
// the file builds and stays outside the model.

const (
	// wildcard marks the unknown part of an expanded value.
	wildcard      = "\x00"
	maxValueDepth = 4
	maxValueCount = 64
	maxNameValues = 16
)

// computedNames are the file-name patterns of the scripts one file builds,
// sorted and unique: a literal file name it spells only in parts, or a name
// with wildcard for each computed part.
type computedNames []string

// names reports whether the file may run the script whose file name is base:
// some pattern matches it, each wildcard standing for any text.
func (names computedNames) names(base string) bool {
	for _, pattern := range names {
		if matchesPattern(pattern, base) {
			return true
		}
	}
	return false
}

func matchesPattern(pattern, base string) bool {
	parts := strings.Split(pattern, wildcard)
	if len(parts) == 1 {
		return pattern == base
	}
	if !strings.HasPrefix(base, parts[0]) {
		return false
	}
	rest := base[len(parts[0]):]
	for _, part := range parts[1 : len(parts)-1] {
		at := strings.Index(rest, part)
		if at < 0 {
			return false
		}
		rest = rest[at+len(part):]
	}
	return strings.HasSuffix(rest, parts[len(parts)-1])
}

func (names computedNames) empty() bool { return len(names) == 0 }

// variableValues returns every value the file gives each name: each
// assignment's value, as written, and each word of a `for NAME in` list.
func variableValues(commands []command) map[string][]string {
	values := map[string][]string{}
	add := func(name, value string) {
		if len(values[name]) < maxNameValues+1 {
			values[name] = append(values[name], value)
		}
	}
	for _, cmd := range commands {
		words := plainWords(cmd.words)
		if len(words) >= 3 && words[0] == "for" && words[2] == "in" {
			for _, word := range words[3:] {
				add(words[1], unquote(word))
			}
			continue
		}
		for index, raw := range words {
			if index == 0 && declarers[unquote(raw)] {
				continue
			}
			match := assignment.FindStringSubmatch(raw)
			if match == nil {
				if index > 0 && declarers[unquote(words[0])] && strings.HasPrefix(raw, "-") {
					continue
				}
				break
			}
			add(match[1], unquote(raw[len(match[0]):]))
		}
	}
	return values
}

// expand lists the values word may take, with wildcard for each part the
// file does not fix, at most maxValueCount of them. It sets *cut when the
// bound drops a value, so the caller can record that the list is incomplete.
func expand(word string, values map[string][]string, depth int, cut *bool) []string {
	out := []string{""}
	appendAll := func(alternatives []string) {
		next := make([]string, 0, len(out)*len(alternatives))
		for _, prefix := range out {
			for _, alternative := range alternatives {
				if len(next) == maxValueCount {
					*cut = true
					break
				}
				next = append(next, prefix+alternative)
			}
		}
		out = next
	}
	for index := 0; index < len(word); {
		switch {
		case word[index] == '`':
			end := strings.IndexByte(word[index+1:], '`')
			if end < 0 {
				return []string{wildcard}
			}
			appendAll([]string{wildcard})
			index += end + 2
		case word[index] == '$' && index+1 < len(word) && (word[index+1] == '{' || word[index+1] == '('):
			end := closing(word, index+1)
			if end < 0 {
				return []string{wildcard}
			}
			inner := word[index+2 : end]
			if word[index+1] == '(' {
				appendAll([]string{wildcard})
			} else {
				appendAll(braced(inner, values, depth, cut))
			}
			index = end + 1
		case word[index] == '$':
			end := index + 1
			for end < len(word) && (word[end] == '_' || word[end] >= 'a' && word[end] <= 'z' || word[end] >= 'A' && word[end] <= 'Z' || end > index+1 && word[end] >= '0' && word[end] <= '9') {
				end++
			}
			if end == index+1 {
				// $1, $@, $#, $? and a lone $ are fixed by no text here.
				appendAll([]string{wildcard})
				if end < len(word) {
					end++
				}
			} else {
				appendAll(named(word[index+1:end], values, depth, cut))
			}
			index = end
		default:
			end := index + 1
			for end < len(word) && word[end] != '$' && word[end] != '`' {
				end++
			}
			appendAll([]string{word[index:end]})
			index = end
		}
	}
	return out
}

// closing returns the index of the bracket that closes the one at open.
func closing(word string, open int) int {
	closer := byte('}')
	if word[open] == '(' {
		closer = ')'
	}
	depth := 0
	for index := open; index < len(word); index++ {
		switch word[index] {
		case word[open]:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

// braced expands `${...}`: a plain name, or a name with a `-`, `:-`, `=` or
// `:=` default, takes the name's values and the default's; any other operator
// transforms the value in a way the model does not follow.
func braced(inner string, values map[string][]string, depth int, cut *bool) []string {
	end := 0
	for end < len(inner) && (inner[end] == '_' || inner[end] >= 'a' && inner[end] <= 'z' || inner[end] >= 'A' && inner[end] <= 'Z' || end > 0 && inner[end] >= '0' && inner[end] <= '9') {
		end++
	}
	if end == 0 {
		return []string{wildcard}
	}
	name, rest := inner[:end], inner[end:]
	if rest == "" {
		return named(name, values, depth, cut)
	}
	for _, operator := range []string{":-", ":=", "-", "="} {
		if strings.HasPrefix(rest, operator) {
			if depth >= maxValueDepth {
				return []string{wildcard}
			}
			alternatives := append(named(name, values, depth, cut), expand(unquote(rest[len(operator):]), values, depth+1, cut)...)
			if len(alternatives) > maxValueCount {
				alternatives, *cut = alternatives[:maxValueCount], true
			}
			return alternatives
		}
	}
	return []string{wildcard}
}

// named expands one variable: its in-file values, or wildcard for a name the
// file never sets, sets more often than the bound, or that recurses too deep.
func named(name string, values map[string][]string, depth int, cut *bool) []string {
	given := values[name]
	if len(given) == 0 || len(given) > maxNameValues || depth >= maxValueDepth {
		return []string{wildcard}
	}
	var out []string
	for _, value := range given {
		out = append(out, expand(value, values, depth+1, cut)...)
	}
	if len(out) > maxValueCount {
		out, *cut = out[:maxValueCount], true
	}
	return out
}

// constructed links each fully literal value of a computed target and
// collects the script names among its other values. It reports whether some
// value is a script name the resolver could not link, and whether the value
// bound dropped values, which may be scripts it links to no edge.
func constructed(target string, resolve *resolver, values map[string][]string, known, edges map[string]bool, names *computedNames) (unresolved, cut bool) {
	if !strings.ContainsAny(target, "$`") {
		return false, false
	}
	for _, value := range expand(target, values, 0, &cut) {
		if !strings.Contains(value, wildcard) {
			if resolve.link(value, nil, known, edges) {
				continue
			}
		}
		base := path.Base(value)
		if !hasAnySuffix(base[strings.LastIndex(base, wildcard)+1:], Suffixes) {
			continue
		}
		unresolved = true
		if !slices.Contains(*names, base) {
			*names = append(*names, base)
			slices.Sort(*names)
		}
	}
	return unresolved, cut
}
