// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"regexp"
	"slices"
	"strings"
)

// Name-listing reads (AFP-V0-9961). A listing or walking command whose
// output is a function of entry names and kinds alone cannot observe an edit
// of an existing regular file, so its reads are recorded apart from content
// reads and the planner narrows them by change status (AFP-V0-9962,
// AFP-V0-9963). The admitted forms are a closed list, and every uncertainty
// keeps the content read:
//
//   - `ls` with no flags or only -1, -a, -A, and the same with -R;
//   - `find` with an empty expression or only -name, -iname, -path, -type,
//     -maxdepth, -mindepth, -print and -print0 with literal arguments;
//   - `tree` with no flags.
//
// Every path operand must be a bare directory name (listedDirectories) or,
// for a recursive form, `.` or `./`; a computed or slashed operand keeps the
// content read. Every later stage of the command's pipeline must be a
// stdin-only filter (nameFilter), and every other command of the file must
// read only what a literal token bounds (nameListingAdmitted, plainCommand):
// otherwise a name the listing prints, or a computed read anywhere, may
// reach a file's content by a name no token holds, and today the listing's
// own token, or the walker's unbounded reads, select the test for it.

// The option and count shapes the closed list admits.
var (
	nameListingLs = regexp.MustCompile(`^-[1aAR]+$`)
	wcFlags       = regexp.MustCompile(`^-[lcwmL]+$`)
	sortFlags     = regexp.MustCompile(`^-[rnuf]+$`)
	lineCount     = regexp.MustCompile(`^-n?[0-9]+$`)
	digits        = regexp.MustCompile(`^[0-9]+$`)
	// environmentName is a name by the environment convention, which a
	// program may read; environmentDefault is an expansion that assigns one.
	environmentName    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)
	environmentDefault = regexp.MustCompile(`\$\{[A-Z_][A-Z0-9_]*:?=`)
	grepFlags          = regexp.MustCompile(`^-[vcqiEFxwn]+$`)
)

// controlPrefixes are the leading words that only make a command
// conditional or negate its status. Every other commandPrefixes word
// (`run`, `env`, `exec`, `sudo`, `timeout` and the rest) can capture the
// output, change the directory or run the command elsewhere, so it is not
// skipped: it stays the command word, which no closed-list form matches.
var controlPrefixes = map[string]bool{
	"!": true, "if": true, "then": true, "else": true, "elif": true, "do": true,
	"while": true, "until": true,
}

// directCommandWord is commandWord skipping only assignments and
// controlPrefixes before the command word.
func directCommandWord(words []string) (string, []string) {
	for index, raw := range words {
		if assignment.MatchString(raw) {
			continue
		}
		if word := unquote(raw); !controlPrefixes[word] {
			return word, words[index+1:]
		}
	}
	return "", nil
}

// literalArgument is an expression argument with no expansion or command
// substitution in it.
func literalArgument(raw string) bool {
	return raw != "" && !strings.ContainsAny(raw, "$`")
}

// nameListingForm reports whether word with args is an admitted name-only
// form whose every path operand is a bare directory or, for a recursive
// form, the working directory.
func nameListingForm(word string, args []string) bool {
	if slices.Contains(args, redirect) || slices.Contains(args, writeRedirect) {
		return false
	}
	var operands []string
	recursive := false
	switch word {
	case "ls":
		for _, arg := range args {
			switch value := unquote(arg); {
			case arg == value && nameListingLs.MatchString(value):
				recursive = recursive || strings.Contains(value, "R")
			case strings.HasPrefix(value, "-"):
				return false
			default:
				operands = append(operands, arg)
			}
		}
	case "find":
		index := 0
		for ; index < len(args); index++ {
			if value := unquote(args[index]); strings.HasPrefix(value, "-") || value == "(" || value == "!" || value == "\\(" {
				break
			}
			operands = append(operands, args[index])
		}
		if !findNameExpression(args[index:]) {
			return false
		}
		recursive = true
	case "tree":
		for _, arg := range args {
			if strings.HasPrefix(unquote(arg), "-") {
				return false
			}
			operands = append(operands, arg)
		}
		recursive = true
	default:
		return false
	}
	for _, raw := range operands {
		value := unquote(raw)
		if recursive && (value == "." || value == "./") {
			continue
		}
		if !slices.Contains(listedDirectories(word, []string{raw}), value) {
			return false
		}
	}
	return true
}

// findNameExpression reports a find expression made only of the admitted
// primaries, each with its literal argument: no operator, no action other
// than printing, no test of anything but names, kinds and depth.
func findNameExpression(words []string) bool {
	for index := 0; index < len(words); index++ {
		switch unquote(words[index]) {
		case "-print", "-print0":
		case "-name", "-iname", "-path", "-type":
			if index+1 >= len(words) || !literalArgument(words[index+1]) {
				return false
			}
			index++
		case "-maxdepth", "-mindepth":
			if index+1 >= len(words) || strings.Trim(unquote(words[index+1]), "0123456789") != "" || unquote(words[index+1]) == "" {
				return false
			}
			index++
		default:
			return false
		}
	}
	return true
}

// nameListingCommand reports whether parsed.commands[index] is an admitted
// name-only form whose pipeline's later stages are all filters. Only a
// file's own commands are admitted: a command string's commands keep the
// content read.
func nameListingCommand(parsed script, index int) bool {
	_, ok := nameListingPipeline(parsed, index)
	return ok
}

// nameListingPipeline is nameListingCommand with the index of the last
// stage of the command's pipeline.
func nameListingPipeline(parsed script, index int) (int, bool) {
	commands := parsed.commands
	if index < 0 || index >= len(commands) {
		return 0, false
	}
	word, args := directCommandWord(literalWords(commands[index].words))
	if !nameListingForm(word, args) {
		return 0, false
	}
	last := index
	for commands[last].trail == "|" {
		last++
		if last >= len(commands) || commands[last].lead != "|" || !nameFilter(commands[last].words) {
			return 0, false
		}
	}
	return last, true
}

// nameListingAdmitted reports whether a file may hold name-listing reads at
// all (AFP-V0-9961): a test that assigns no environment-style (upper-case)
// name, since PATH may put a stub ahead of a listing utility or a filter and
// a variable such as CLICOLOR_FORCE or GREP_OPTIONS makes one read more than
// names, and whose every command outside an admitted name-listing pipeline
// is one plainCommand admits. A program, a
// helper, a sourced file, a function call, a directory change or a computed
// read anywhere in the file may read below a listed directory, or below the
// tree a name-only walker lists, by a name no token holds, so such a file
// keeps every listing a content read. Where a listing's names go then does
// not matter: no remaining command can open a file by a computed name.
func nameListingAdmitted(parsed script, test bool) bool {
	if !test {
		return false
	}
	covered := map[int]bool{}
	for index := range parsed.commands {
		if last, ok := nameListingPipeline(parsed, index); ok {
			for stage := index; stage <= last; stage++ {
				covered[stage] = true
			}
		}
	}
	for index, cmd := range parsed.commands {
		for _, raw := range cmd.words {
			if match := assignment.FindStringSubmatch(raw); match != nil && environmentName.MatchString(match[1]) {
				return false
			}
			if environmentDefault.MatchString(raw) {
				return false
			}
		}
		if !covered[index] && !plainCommand(cmd.words) {
			return false
		}
	}
	return true
}

// outputBuiltins are the commands that print, test values or set shell
// state and open no file but a redirection target. `export` is not one: it
// changes what a later program reads from its environment.
var outputBuiltins = map[string]bool{
	"echo": true, "printf": true, "true": true, "false": true, ":": true, "exit": true,
	"return": true, "set": true, "shift": true, "local": true, "readonly": true,
	"unset": true, "read": true, "for": true, "fi": true, "done": true,
	"esac": true,
}

// assignsEnvironment reports an output builtin that sets an
// environment-style name without an assignment word: `read NAME`,
// `for NAME in`, `printf -v NAME`.
func assignsEnvironment(word string, args []string) bool {
	switch word {
	case "read":
		for _, arg := range args {
			if environmentName.MatchString(unquote(arg)) {
				return true
			}
		}
	case "for":
		return len(args) > 0 && environmentName.MatchString(unquote(args[0]))
	case "printf":
		for index, arg := range args {
			if unquote(arg) == "-v" {
				return index+1 >= len(args) || environmentName.MatchString(unquote(args[index+1]))
			}
		}
	}
	return false
}

// testBuiltins are the conditional commands; testOperators are the operators
// they may use, which compare values or read an entry's existence or kind
// only. A size, time, permission or ownership test can change with an edit.
var (
	testBuiltins  = map[string]bool{"test": true, "[": true, "[[": true}
	testOperators = map[string]bool{
		"-e": true, "-f": true, "-d": true, "-h": true, "-L": true, "-z": true, "-n": true,
		"-eq": true, "-ne": true, "-lt": true, "-le": true, "-gt": true, "-ge": true,
		"-a": true, "-o": true,
	}
)

// operandReaders are the utilities that read only their file operands, or
// standard input without one, and fail on a directory operand;
// patternReaders take a pattern first. `diff` is not one, since a directory
// operand makes it read the files below, and neither is a checksum tool,
// whose check mode reads the files its operand names.
var (
	operandReaders = map[string]bool{
		"cat": true, "head": true, "tail": true, "wc": true, "sort": true, "cmp": true,
	}
	patternReaders = map[string]bool{"grep": true, "egrep": true, "fgrep": true}
)

// plainCommand reports a command whose every read a literal token bounds or
// that reads no file at all: an output builtin or an existence or kind test,
// whose redirections only write or duplicate a descriptor, or an operand
// reader whose operands and input redirections are literal path-shaped words
// (each a token of its own) and whose options are short letters with no
// recursion. A bare control word or assignment list is plain too. Anything
// else, a computed operand, a program, a function call, a directory change
// or a long option included, is not.
func plainCommand(words []string) bool {
	plain := literalWords(words)
	for index := 0; index < len(plain); index++ {
		if plain[index] != redirect && plain[index] != writeRedirect {
			continue
		}
		index++
		if index >= len(plain) || plain[index-1] == writeRedirect {
			continue
		}
		switch target := unquote(plain[index]); target {
		case "1", "2", "&1", "&2", "-", "/dev/null":
		default:
			// An input redirection reads its target, which must then be a
			// literal path-shaped word.
			if !literalPathWord(plain[index]) {
				return false
			}
		}
	}
	word, args := directCommandWord(plainWords(plain))
	if word == "" {
		return true
	}
	switch {
	case outputBuiltins[word]:
		return !assignsEnvironment(word, args)
	case testBuiltins[word]:
		for _, arg := range args {
			if value := unquote(arg); arg == value && strings.HasPrefix(value, "-") && !testOperators[value] {
				return false
			}
		}
		return true
	case operandReaders[word], patternReaders[word]:
		return plainOperands(word, args)
	}
	return false
}

// plainOperands reports operand-reader arguments plainCommand admits: short
// options with no recursion (`grep -r`) and no other pattern
// source (`grep -e`, `-f`), a digit count after `head` or `tail` -n or -c,
// a grep's pattern, and literal path-shaped file operands.
func plainOperands(word string, args []string) bool {
	pattern := patternReaders[word]
	count := false
	for _, arg := range args {
		value := unquote(arg)
		switch {
		case count:
			if !digits.MatchString(value) {
				return false
			}
			count = false
		case arg == value && strings.HasPrefix(value, "-") && len(value) > 1:
			switch {
			case strings.HasPrefix(value, "--"):
				return false
			case patternReaders[word] && strings.ContainsAny(value, "rRdef"):
				return false
			case (word == "head" || word == "tail") && (value == "-n" || value == "-c"):
				count = true
			}
		case pattern:
			pattern = false
		case !literalPathWord(arg):
			return false
		}
	}
	return !count
}

// literalPathWord reports a word with no expansion, substitution or glob
// whose unquoted value is path-shaped, so it is a path token of its own.
func literalPathWord(raw string) bool {
	value := unquote(raw)
	return value != "" && !strings.ContainsAny(raw, "$`*?[{~") && pathShaped(value) && pathToken.FindString(value) == value
}

// literalWords drops only a write redirection to /dev/null and its target,
// which hides output and opens nothing; every other redirection stays, so
// nameListingForm and the filters refuse it.
func literalWords(words []string) []string {
	kept := make([]string, 0, len(words))
	for index := 0; index < len(words); index++ {
		if words[index] == writeRedirect && index+1 < len(words) && unquote(words[index+1]) == "/dev/null" {
			index++
			continue
		}
		kept = append(kept, words[index])
	}
	return kept
}

// nameFilter reports a pipeline stage that reads names only as text from its
// standard input and opens no file a name could reach: `wc` with count
// flags, `sort` with ordering flags, `head` or `tail` with a line count, and
// a non-recursive `grep` given one pattern. Anything else, `xargs`, `while`,
// `tee`, `sed` and every long option included, is not one.
func nameFilter(words []string) bool {
	plain := literalWords(words)
	if slices.Contains(plain, redirect) || slices.Contains(plain, writeRedirect) {
		return false
	}
	word, args := directCommandWord(plain)
	values := make([]string, len(args))
	for index, arg := range args {
		values[index] = unquote(arg)
	}
	switch word {
	case "wc":
		return allMatch(values, wcFlags)
	case "sort":
		return allMatch(values, sortFlags)
	case "head", "tail":
		switch len(values) {
		case 0:
			return true
		case 1:
			return lineCount.MatchString(values[0])
		case 2:
			return values[0] == "-n" && digits.MatchString(values[1])
		}
		return false
	case "grep", "egrep", "fgrep":
		pattern := 0
		for index, value := range values {
			switch {
			case args[index] == value && grepFlags.MatchString(value):
			case strings.HasPrefix(value, "-"):
				return false
			default:
				pattern++
			}
		}
		return pattern == 1
	}
	return false
}

// allMatch reports whether every value matches expression.
func allMatch(values []string, expression *regexp.Regexp) bool {
	for _, value := range values {
		if !expression.MatchString(value) {
			return false
		}
	}
	return true
}
