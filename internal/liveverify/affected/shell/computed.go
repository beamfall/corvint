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

// variableValues returns every distinct value the file gives each name: each
// assignment's value, as written, and each word of a `for NAME in` list. A
// value repeated verbatim (the same word list in several loops) expands to
// the same values, so it counts once toward maxNameValues (AFP-V0-3148).
func variableValues(commands []command) map[string][]string {
	values := map[string][]string{}
	add := func(name, value string) {
		if len(values[name]) < maxNameValues+1 && !slices.Contains(values[name], value) {
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
				if repositoryRootCommand(inner) {
					appendAll([]string{"."})
					index = end + 1
					continue
				}
				// A leading `$(dirname "$0")/` keeps its directory form so the
				// resolver can place the value (AFP-V0-553).
				form, own := ownDirectory(inner)
				if !own || index != 0 || end+1 >= len(word) || word[end+1] != '/' {
					form = wildcard
				}
				appendAll([]string{form})
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

// ownDirectory reports a substitution that names the running script's
// directory, `dirname "$0"` or `dirname "${BASH_SOURCE[0]}"`, and the
// directory form the resolver places the same way.
func ownDirectory(inner string) (string, bool) {
	words := dropFlags(fieldsTop(redirection.ReplaceAllString(inner, "")))
	if len(words) != 2 || words[0] != "dirname" || !scriptPaths[words[1]] {
		return "", false
	}
	switch {
	case words[1] == "$0" || words[1] == "${0}":
		return "${0%/*}", true
	case strings.Contains(words[1], "BATS_TEST_FILENAME"):
		return "${BATS_TEST_FILENAME%/*}", true
	}
	return "${BASH_SOURCE%/*}", true
}

// braced expands `${...}`: a plain name, or a name with a `-`, `:-`, `=` or
// `:=` default, takes the name's values and the default's. A `+` or `:+`
// alternative takes the empty value and its alternative's first field, the
// word a command runs (AFP-V0-553). Any other operator transforms the value
// in a way the model does not follow.
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
				return truncated(cut)
			}
			alternatives := append(named(name, values, depth, cut), expand(unquote(rest[len(operator):]), values, depth+1, cut)...)
			if len(alternatives) > maxValueCount {
				alternatives, *cut = alternatives[:maxValueCount], true
			}
			return alternatives
		}
	}
	for _, operator := range []string{":+", "+"} {
		if strings.HasPrefix(rest, operator) {
			fields := fieldsTop(rest[len(operator):])
			if len(fields) == 0 {
				return []string{wildcard}
			}
			if depth >= maxValueDepth {
				return truncated(cut)
			}
			return append([]string{""}, expand(unquote(fields[0]), values, depth+1, cut)...)
		}
	}
	return []string{wildcard}
}

// named expands one variable: its in-file values, or wildcard for a name the
// file never sets. A name it sets more often than maxNameValues, or one that
// recurses past maxValueDepth, is a truncated expansion: the values the file
// does give are dropped, so it sets *cut as the count bound does
// (AFP-V0-3146, AFP-V0-3147).
func named(name string, values map[string][]string, depth int, cut *bool) []string {
	given := values[name]
	if len(given) == 0 {
		return []string{wildcard}
	}
	if len(given) > maxNameValues || depth >= maxValueDepth {
		return truncated(cut)
	}
	var out []string
	for _, value := range given {
		// An unrolled value expands at this depth: its reference takes values
		// that no longer refer to name, so it adds no level to a chain and
		// cannot recur on itself (AFP-V0-3149).
		expanded, next := values, depth+1
		if others, ok := unrolled(name, value, values, depth); ok {
			expanded, next = others, depth
		} else if at, _ := selfReference(name, value); at >= 0 {
			// An uncertified self-reference may transform values the file
			// supplies even if braced cannot follow it to a depth bound.
			*cut = true
		}
		out = append(out, expand(value, expanded, next, cut)...)
	}
	if len(out) > maxValueCount {
		out, *cut = out[:maxValueCount], true
	}
	return out
}

// unrolled substitutes a closed set of seeds only when every assignment to
// name is certified. Defaults contribute their literal operands; dropping them
// would lose composed targets. Other self-reference forms retain the cut in
// named, even when an unsupported operator does not recurse (AFP-V0-3149).
func unrolled(name, value string, values map[string][]string, depth int) (map[string][]string, bool) {
	if at, _ := selfReference(name, value); at < 0 {
		return nil, false
	}
	others := []string{wildcard}
	for _, other := range values[name] {
		seed, ok := unrollSeed(name, other, values, depth)
		if !ok {
			return nil, false
		}
		if !slices.Contains(others, seed) {
			others = append(others, seed)
		}
	}
	copied := make(map[string][]string, len(values))
	for key, list := range values {
		copied[key] = list
	}
	copied[name] = others
	return copied, true
}

// unrollSeed admits non-self values, plain identities, standalone defaults
// with literal operands, and plain references normalized by exactly $PWD/ or
// ${PWD}/, with no suffix and no in-file directory component in that prefix.
// Everything else fails closed; a braced expression's enclosing span alone
// never proves it is an identity.
func unrollSeed(name, value string, values map[string][]string, depth int) (string, bool) {
	at, end := selfReference(name, value)
	if at < 0 {
		return value, true
	}
	ref := value[at:end]
	plain := ref == "$"+name || ref == "${"+name+"}"
	if at == 0 && end == len(value) {
		if plain {
			return wildcard, true
		}
		for _, operator := range []string{":-", ":=", "-", "="} {
			prefix := "${" + name + operator
			if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, "}") {
				operand := value[len(prefix) : len(value)-1]
				if !strings.ContainsAny(operand, "$`") {
					return unquote(operand), true
				}
			}
		}
		return "", false
	}
	if !plain || at == 0 || end != len(value) || !topLevel(value[:at]) {
		return "", false
	}
	// Only working-directory normalization preserves the seed's relative path.
	// An arbitrary unknown prefix can add directories ("$PWD/script/$x")
	// or transform it opaquely. constructed cannot resolve the resulting
	// wildcard-prefixed extensionless target, so those forms retain the cut.
	prefix := value[:at]
	if prefix != "$PWD/" && prefix != "${PWD}/" {
		return "", false
	}
	var cut bool
	prefixes := expand(prefix, values, depth+1, &cut)
	if cut || len(prefixes) != 1 || prefixes[0] != wildcard+"/" {
		return "", false
	}
	return wildcard, true
}

// selfReference locates a reference the value expander follows. Command
// substitutions and backquotes produce opaque results: their argument references
// do not recursively expand values and must not create a false cycle.
func selfReference(name, value string) (int, int) {
	for at := 0; at < len(value); at++ {
		if value[at] == '`' {
			end := strings.IndexByte(value[at+1:], '`')
			if end < 0 {
				break
			}
			at += end + 1
			continue
		}
		if value[at] != '$' {
			continue
		}
		rest := value[at+1:]
		switch {
		case strings.HasPrefix(rest, "("):
			end := closing(value, at+1)
			if end < 0 {
				return -1, -1
			}
			at = end
		case strings.HasPrefix(rest, "{"+name) && !identifierByte(rest, len(name)+1):
			if end := closing(value, at+1); end >= 0 {
				return at, end + 1
			}
			return at, len(value)
		case strings.HasPrefix(rest, name) && !identifierByte(rest, len(name)):
			return at, at + 1 + len(name)
		}
	}
	return -1, -1
}

// topLevel reports whether prefix closes every `${`, `$(` and backquote it
// opens, so a reference right after it is not nested in another expansion
// (`${y:-abc$x}`), whose unrollings could be literal.
func topLevel(prefix string) bool {
	for index := 0; index < len(prefix); index++ {
		switch {
		case prefix[index] == '`':
			end := strings.IndexByte(prefix[index+1:], '`')
			if end < 0 {
				return false
			}
			index += end + 1
		case prefix[index] == '$' && index+1 < len(prefix) && (prefix[index+1] == '{' || prefix[index+1] == '('):
			end := closing(prefix, index+1)
			if end < 0 {
				return false
			}
			index = end
		}
	}
	return true
}

// identifierByte reports whether text has a name character at index.
func identifierByte(text string, index int) bool {
	if index >= len(text) {
		return false
	}
	c := text[index]
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// truncated is the wildcard a value bound stands in for values the file
// gives: it records the cut, so the caller keeps the unbounded-reader
// protection instead of reading the wildcard as an environment input that
// names no owned script (AFP-V0-3147). A wildcard for a part no text fixes
// (an unset name, `$1`, a command substitution, an operator the model does
// not follow) is no cut unless it discards a self-referential assignment
// rejected by the closed allowlist in unrolled (AFP-V0-3149).
func truncated(cut *bool) []string {
	*cut = true
	return []string{wildcard}
}

// constructed links each fully literal value of a computed target and
// collects the script names among its other values. It reports whether some
// value is a script name the resolver could not link, and whether the value
// bound dropped values, which may be scripts it links to no edge.
func constructed(target string, resolve *resolver, values map[string][]string, known, edges map[string]bool, names *computedNames) (unresolved, cut bool) {
	if !strings.ContainsAny(target, "$`") {
		return false, false
	}
	// A target whose file name is literal, carries no shell suffix and is the
	// name of no owned extensionless script (`"$sdk/go"`, a binary in a
	// computed directory) names no script this plugin owns under any
	// expansion, so no dropped value could be one: it stays outside the model
	// without expanding (TSP-V0-960, AFP-V0-3144). A name such a script has
	// (`"$dir/check"` beside script/check) is expanded, so its values link.
	if base := path.Base(target); !strings.ContainsAny(base, "$`{}") && !hasAnySuffix(base, Suffixes) && !ownedExtensionless(known, base) {
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

// ownedExtensionless reports whether an owned file in known has the file name
// base, which carries no shell suffix: an ExtensionlessScript (AFP-V0-2205,
// AFP-V0-3144). A target value links only a known path, and that path's file
// name is the target's literal one, so a name no known file has links nothing.
func ownedExtensionless(known map[string]bool, base string) bool {
	for relative := range known {
		if path.Base(relative) == base {
			return true
		}
	}
	return false
}
