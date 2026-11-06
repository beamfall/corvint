// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"bytes"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// ForeignCallerReference compiles the literal and constructed reference check
// for a declared callee. The repository gate applies it to every tracked
// executable-source candidate, including files outside the plugin's walk.
func ForeignCallerReference(script string) func([]byte) bool {
	base := []byte(path.Base(script))
	constructed := constructedForeignMatcher(script)
	return func(body []byte) bool { return bytes.Contains(body, base) || constructed(body) }
}

// constructedForeignMatcher is deliberately conservative. In addition to
// shell command expansion, a foreign caller can assemble a callee with a
// directory concatenation or a path-join call. Every script below that
// directory is a possible target; the project must list that caller for each.
// AZU-V0-004, AZU-V0-8901. This is a completeness guard, not an edge inference.
func constructedForeignMatcher(script string) func([]byte) bool {
	directory := path.Dir(script)
	dir := regexp.QuoteMeta(directory)
	patterns := []string{
		// The separator may be in the next operand: "run" + "/" + name.
		`["'\x60](?:\./)?` + dir + `/?["'\x60]\s*\+`,
		`(?:[Jj]oin|joinpath)\s*\(\s*["'\x60](?:\./)?` + dir + `/?["'\x60]\s*,`,
	}
	if directory == "." {
		// A computed basename needs no directory literal. Without one we
		// cannot exclude a root callee from joins, concatenations or formats;
		// conservatively require these possible callers to be declared.
		patterns = []string{
			`(?:[Jj]oin|joinpath)\s*\(`,
			`["'\x60][^"'\x60\n]*["'\x60]\s*\+|\+\s*["'\x60]`,
		}
	}
	// Keep the substitution vocabulary shared for root and nested scripts.
	// Root callees have no directory literal with which to exclude a basename.
	prefix := ""
	backtickPrefix := `[^"'\x60\n]*`
	if directory != "." {
		prefix = `(?:\./)?` + dir + `/`
		backtickPrefix = prefix
	}
	patterns = append(patterns,
		`["'\x60]`+prefix+`[^"'\x60\n]*[$%{]`,
		// Stop at whitespace after a nested directory so backticks in later
		// prose are not part of its path. A raw-string closing delimiter
		// alone is not a substitution.
		`["']`+backtickPrefix+`[^"'\s\x60]*\x60`,
	)
	if directory != "." {
		// A quoted command can precede the path ("sh run/$(name).sh").
		// Bound this extension to command words and a single path token;
		// markers in later arguments or diagnostic prose are not basenames.
		commandPrefix := `(?:[A-Za-z0-9_./=-]+[ \t]+)+` + prefix
		patterns = append(patterns,
			`["'\x60]`+commandPrefix+`[^"'\x60\s]*[$%{]`,
			`["']`+commandPrefix+`[^"'\s\x60]*\x60`,
		)
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(pattern))
	}
	return func(body []byte) bool {
		for _, pattern := range compiled {
			if pattern.Match(body) {
				return true
			}
		}
		return normalizedForeignJoin(body, script)
	}
}

var foreignJoinStart = regexp.MustCompile(`\b(?:[Jj]oin|joinpath)\s*\(`)
var foreignJoinLiteral = regexp.MustCompile("^\\s*(\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`[^`]*`)\\s*([,)])")

// AZU-V0-8901: consume whole literal operands before deciding which directory
// a join can reach. An unresolved intermediate operand may traverse out of
// any literal prefix. Fully literal joins match the callee directory,
// so results remain invariant across the directory cache in callers.go.
func normalizedForeignJoin(body []byte, script string) bool {
	directory := path.Dir(script)
	for _, call := range foreignJoinStart.FindAllIndex(body, -1) {
		args, prefix, seen := body[call[1]:], "", false
		for {
			literal := foreignJoinLiteral.FindSubmatch(args)
			if literal == nil {
				if seen && (foreignJoinMayTraverse(args) || prefix == "." || prefix == directory || strings.HasPrefix(directory, prefix+"/")) {
					return true
				}
				break
			}
			quoted := string(literal[1])
			value, ok := foreignJoinString(quoted)
			if !ok {
				return true // Unsupported string escapes cannot prove exclusion.
			}
			prefix, seen = path.Clean(path.Join(prefix, value)), true
			if literal[2][0] == ')' {
				if path.Dir(prefix) == directory {
					return true
				}
				break
			}
			args = args[len(literal[0]):]
		}
	}
	return false
}

// A dynamic directory operand can be "..", cancelling an unrelated prefix
// before later operands supply the callee directory. Inspect only the current
// call: commas inside strings or nested expressions are not operand boundaries.
// A suffixed dynamic final operand traverses too (foreignJoinSuffixedDynamic).
func foreignJoinMayTraverse(args []byte) bool {
	depth, quote, last, more := 0, byte(0), 0, false
	for i := 0; i < len(args); i++ {
		c := args[i]
		if quote != 0 {
			if c == '\\' && quote != '`' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '\'', '"', '`':
			quote = c
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return (more || foreignJoinSuffixedDynamic(args[last:i])) && !foreignJoinNonShellTail(args[last:i])
			}
			depth--
		case ',':
			if depth == 0 {
				if bytes.HasPrefix(bytes.TrimSpace(args[i+1:]), []byte(")")) {
					return (more || foreignJoinSuffixedDynamic(args[last:i])) && !foreignJoinNonShellTail(args[last:i])
				}
				more, last = true, i+1
			}
		}
	}
	return more // An incomplete call cannot prove exclusion.
}

var foreignJoinWholeLiteral = regexp.MustCompile("^\\s*(?:\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`[^`]*`)\\s*$")

// A dynamic final operand concatenated with a literal suffix, such as
// name+".sh", can itself carry ".." (name="../run/run"). A bare identifier
// keeps the terminal-basename bound; a non-shell suffix is excluded later.
func foreignJoinSuffixedDynamic(operand []byte) bool {
	operand = foreignJoinUnparen(operand)
	return !foreignJoinWholeLiteral.Match(operand) && foreignJoinTail.Match(operand)
}

// foreignJoinUnparen strips grouping parentheses that enclose a whole operand,
// so (name+".sh") is judged like name+".sh".
func foreignJoinUnparen(operand []byte) []byte {
	for {
		operand = bytes.TrimSpace(operand)
		if len(operand) < 2 || operand[0] != '(' || operand[len(operand)-1] != ')' {
			return operand
		}
		depth := 0
		for i, c := range operand {
			if c == '(' {
				depth++
			} else if c == ')' {
				depth--
				if depth == 0 && i != len(operand)-1 {
					return operand
				}
			}
		}
		operand = operand[1 : len(operand)-1]
	}
}

var foreignJoinTail = regexp.MustCompile("(?:^|\\+)\\s*(\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`[^`]*`)\\s*$")

// A fixed final suffix such as .json cannot become a shell filename through
// an earlier traversal. Check every supported shell suffix, keeping this proof
// independent of the callee basename for the directory-level match cache.
func foreignJoinNonShellTail(operand []byte) bool {
	literal := foreignJoinTail.FindSubmatch(operand)
	if literal == nil {
		return false
	}
	value, ok := foreignJoinString(string(literal[1]))
	// A foreign caller may run on Windows, where a backslash is a separator.
	value = strings.ReplaceAll(value, `\`, "/")
	if !ok || value == "" || strings.HasSuffix(value, "/") || strings.ContainsAny(value, "$%{}:") {
		return false
	}
	tail := path.Base(value)
	if tail == "." || tail == ".." {
		return false // Cleaning may remove this operand and expose an earlier one.
	}
	// Windows may ignore case and trailing dots/spaces in a filename.
	tail = strings.ToLower(strings.TrimRight(tail, " ."))
	if tail == "" {
		return false
	}
	for _, suffix := range Suffixes {
		if strings.HasSuffix(tail, suffix) || strings.HasSuffix(suffix, tail) {
			return false
		}
	}
	return true
}

// Single quotes delimit strings in other source languages, not just Go runes.
// Decode them a character at a time; their common short NUL spelling is also
// a string-join separator, and cannot name a file in the repository.
func foreignJoinString(quoted string) (string, bool) {
	if quoted[0] == '`' {
		value, err := strconv.Unquote(quoted)
		return value, err == nil
	}
	var value strings.Builder
	for rest := quoted[1 : len(quoted)-1]; rest != ""; {
		if strings.HasPrefix(rest, `\0`) && (len(rest) == 2 || rest[2] < '0' || rest[2] > '7') {
			value.WriteByte(0)
			rest = rest[2:]
			continue
		}
		char, multibyte, tail, err := strconv.UnquoteChar(rest, quoted[0])
		if err != nil {
			return "", false
		}
		if multibyte {
			value.WriteRune(char)
		} else {
			value.WriteByte(byte(char))
		}
		rest = tail
	}
	return value.String(), true
}

func constructedForeignReference(body []byte, script string) bool {
	return constructedForeignMatcher(script)(body)
}
