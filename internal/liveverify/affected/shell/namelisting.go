// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"path"
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
// stdin-only filter (nameFilter), and the whole file must pass a closed
// grammar (nameListingAdmitted): no construct that writes a variable, changes
// the directory, runs a program or opens a file by a name no literal token
// holds. Anything the grammar does not recognise keeps the content read:
// otherwise a name the listing prints, or a computed read anywhere, may reach
// a file's content, and today the listing's own token, or the walker's
// unbounded reads, select the test for it.

// The option and count shapes the closed list admits.
var (
	nameListingLs = regexp.MustCompile(`^-[1aAR]+$`)
	wcFlags       = regexp.MustCompile(`^-[lcwmL]+$`)
	sortFlags     = regexp.MustCompile(`^-[rnuf]+$`)
	lineCount     = regexp.MustCompile(`^-n?[0-9]+$`)
	digits        = regexp.MustCompile(`^[0-9]+$`)
	grepFlags     = regexp.MustCompile(`^-[vcqiEFxwn]+$`)
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
// all (AFP-V0-9961). The rule is a closed grammar: it states what is
// admitted, and every construct it does not recognise keeps every listing a
// content read. A test file is admitted only when
//
//   - its first line is no `#!` line, or one shellInterpreter admits: the
//     lexer reads `#!` as a comment, and another interpreter would run the
//     file under a grammar the lexer does not model;
//   - it defines no function and holds no here-document, `case` statement or
//     `((` arithmetic command, which the lexer folds away without commands
//     whose words the rules below could see;
//   - every word of every command passes gateWord, so no word assigns a
//     variable or holds an expansion that can write one; and
//   - every command outside an admitted name-listing pipeline has a shape
//     gateCommand admits.
//
// No admitted construct can write a variable, change the working directory,
// run a program other than the named utilities, or open a file by a name no
// literal token holds, so where a listing's names go does not matter and no
// write needs to be judged environment-style. The environment the runner
// exports before the unit starts is outside the unit's words and is not
// modelled.
func nameListingAdmitted(parsed script, firstLine string, test bool) bool {
	if !test || !shellInterpreter(firstLine) || len(parsed.functions) != 0 || len(parsed.data) != 0 || parsed.cases != 0 || parsed.arithmetic != 0 {
		return false
	}
	covered, listed := map[int]bool{}, map[string]bool{}
	for index, cmd := range parsed.commands {
		word, args := directCommandWord(plainWords(cmd.words))
		for _, directory := range listedDirectories(word, args) {
			listed[directory] = true
		}
		if last, ok := nameListingPipeline(parsed, index); ok {
			for stage := index; stage <= last; stage++ {
				covered[stage] = true
			}
		}
	}
	for index, cmd := range parsed.commands {
		if slices.ContainsFunc(cmd.words, func(raw string) bool { return !gateWord(raw) }) {
			return false
		}
		if !covered[index] && !gateCommand(cmd.words, listed) {
			return false
		}
	}
	return true
}

// lexedShells are the interpreters whose grammar the lexer reads.
var lexedShells = map[string]bool{"sh": true, "bash": true, "dash": true, "zsh": true, "ksh": true}

// shellInterpreter reports a first line that is no `#!` line, or one that
// runs a shell the lexer reads: `#!/path/<shell>`, or `#!/usr/bin/env
// <shell>` with no option and no NAME=value field. Every other `#!` line,
// with an argument, another program or another env form, is refused.
func shellInterpreter(firstLine string) bool {
	rest, ok := strings.CutPrefix(strings.TrimSuffix(firstLine, "\r"), "#!")
	if !ok {
		return true
	}
	fields := strings.Fields(rest)
	switch {
	case len(fields) == 1 && strings.HasPrefix(fields[0], "/"):
		return lexedShells[path.Base(fields[0])]
	case len(fields) == 2 && fields[0] == "/usr/bin/env":
		return lexedShells[fields[1]]
	}
	return false
}

// braceName is a `{name}` word, which before a redirection operator makes
// bash allocate a descriptor and assign its number to name.
var braceName = regexp.MustCompile(`^\{[A-Za-z_][A-Za-z0-9_]*\}$`)

// gateWord reports a word that writes no variable and holds only read-only
// expansions: not an assignment word (`x=1`, `x+=1`, `x[0]=1`) in any
// position or case, not a `{name}` redirection word, no backtick or process
// substitution, no arithmetic expansion, and every `$` one readExpansion
// admits or a `$(…)` substitution, whose inner commands are commands of the
// file held to the same gate.
func gateWord(raw string) bool {
	switch {
	case assignment.MatchString(raw), braceName.MatchString(raw):
		return false
	case strings.Contains(raw, "`"), strings.Contains(raw, "<("), strings.Contains(raw, ">("):
		return false
	case strings.Contains(raw, "$(("):
		// Refused wherever it stands, quoted or not: scanExpansions would
		// read it as a command substitution.
		return false
	}
	end, ok := scanExpansions(raw, 0, false)
	return ok && end == len(raw)
}

// readExpansion is the text after a `$` that only reads: a name, `{name}`,
// `{name:-text}` or `{name-text}` with literal text, `#`, `?` or a single
// digit. `${name=text}`, `${name:=text}`, `${!name}`, `${name[i]}` (an
// arithmetic subscript), `$'…'`, `$"…"` and every other form are not.
var readExpansion = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*|[#?0-9]|\{[A-Za-z_][A-Za-z0-9_]*(?::?-[^$` + "`" + `}\\'"]*)?\})`)

// scanExpansions scans raw from at, tracking quotes, and reports whether
// every `$` it meets is a readExpansion or a `$(…)` substitution. nested
// scans a substitution's text and returns the index past its closing
// parenthesis; the top level returns len(raw) when the word is complete.
func scanExpansions(raw string, at int, nested bool) (int, bool) {
	double, depth := false, 0
	for at < len(raw) {
		switch c := raw[at]; {
		case c == '\\':
			at += 2
		case c == '\'' && !double:
			end := strings.IndexByte(raw[at+1:], '\'')
			if end < 0 {
				return at, false
			}
			at += end + 2
		case c == '"':
			double = !double
			at++
		case c == '$' && strings.HasPrefix(raw[at:], "$("):
			end, ok := scanExpansions(raw, at+2, true)
			if !ok {
				return at, false
			}
			at = end
		case c == '$':
			match := readExpansion.FindString(raw[at+1:])
			if match == "" {
				return at, false
			}
			at += 1 + len(match)
		case nested && !double && c == '(':
			depth++
			at++
		case nested && !double && c == ')':
			if depth == 0 {
				return at + 1, true
			}
			depth--
			at++
		default:
			at++
		}
	}
	return at, !nested && !double
}

// expanding reports a word with an expansion or substitution in it.
func expanding(raw string) bool {
	return strings.ContainsAny(raw, "$`")
}

// fixedWord reports a word that expands to itself alone: no expansion, glob,
// brace or tilde character, quoted or not.
func fixedWord(raw string) bool {
	return !strings.ContainsAny(raw, "$`*?[{~")
}

// singleField reports a word that expands to exactly one field: every
// expansion and glob character in it is inside double quotes.
func singleField(raw string) bool {
	double := false
	for at := 0; at < len(raw); at++ {
		switch c := raw[at]; {
		case c == '\\':
			at++
		case c == '\'' && !double:
			end := strings.IndexByte(raw[at+1:], '\'')
			if end < 0 {
				return false
			}
			at += end + 1
		case c == '"':
			double = !double
		case double && strings.HasPrefix(raw[at:], "$("):
			end, ok := scanExpansions(raw, at+2, true)
			if !ok {
				return false
			}
			at = end - 1
		case !double && strings.IndexByte("$`*?[{~", c) >= 0:
			return false
		}
	}
	return !double
}

// gateCommand reports a command shape the closed grammar admits, outside an
// admitted name-listing pipeline:
//
//   - control words and redirections alone (`then`, `else`, `fi`, `done`,
//     whose only arguments can be redirections);
//   - `echo`, `true`, `false`, `:` or `exit` with any arguments;
//   - `printf` whose first argument is fixed, does not begin with `-`, and
//     holds no conversion or escape that writes a variable
//     (printfFormatWrites), since bash's printf reads options only before
//     its first operand;
//   - `test`, `[` or `[[` as testArguments and conditionalArguments admit;
//   - an operand reader (`cat`, `head`, `tail`, `wc`, `sort`, `cmp`) or a
//     non-recursive grep with no expansion in any argument, as
//     plainOperands admits.
//
// Every redirection target is a descriptor, `-`, `/dev/null` or a literal
// operand (literalOperand). An expansion therefore stands only in an argument
// of an output or test builtin. Everything else, `set`, `read`, `for`, `cd`,
// `export`, `eval`, `exec`, `source`, `diff`, a checksum tool, a function call
// or any other program included, is not admitted.
func gateCommand(words []string, listed map[string]bool) bool {
	plain := literalWords(words)
	for index := 0; index < len(plain); index++ {
		if plain[index] != redirect && plain[index] != writeRedirect {
			continue
		}
		index++
		if index >= len(plain) {
			return false
		}
		switch target := plain[index]; target {
		case "1", "2", "&1", "&2", "-", "/dev/null":
		default:
			if !literalOperand(target, listed) {
				return false
			}
		}
	}
	word, args := directCommandWord(plainWords(plain))
	switch word {
	case "", "fi", "done", "echo", "true", "false", ":", "exit":
		return true
	case "printf":
		return len(args) > 0 && fixedWord(args[0]) && !strings.HasPrefix(unquote(args[0]), "-") &&
			!printfFormatWrites(args[0])
	case "test", "[":
		return testArguments(word, args)
	case "[[":
		return conditionalArguments(args)
	}
	if operandReaders[word] || patternReaders[word] {
		return !slices.ContainsFunc(args, expanding) && plainOperands(word, args, listed)
	}
	return false
}

// printfFormatWrites reports a printf format that can assign a variable.
// Bash's printf stores the count of bytes written so far into the variable
// its `%n` conversion names, so `printf '%n' PATH` sets PATH to 0. A numeric
// escape (`\NNN`, `\xHH`, `\uHHHH`, `\UHHHHHHHH`) is refused too, since
// printf expands one inside its format before reading conversions. Quote
// characters are dropped first, so a split spelling such as `%"n"` is caught,
// and a conversion ending in a backslash is refused. The check is
// conservative: it can refuse a format that writes nothing, never the reverse.
func printfFormatWrites(raw string) bool {
	format := strings.NewReplacer(`'`, "", `"`, "").Replace(raw)
	for index := 0; index < len(format); index++ {
		switch format[index] {
		case '\\':
			if index+1 < len(format) && strings.IndexByte("01234567xuU", format[index+1]) >= 0 {
				return true
			}
			index++
		case '%':
			index++
			for index < len(format) && strings.IndexByte("-+ #0123456789.*", format[index]) >= 0 {
				index++
			}
			if index < len(format) && (format[index] == 'n' || format[index] == '\\') {
				return true
			}
		}
	}
	return false
}

// testOperators are the operators a test builtin may use, which compare
// values or read an entry's existence or kind only: a size, time, permission
// or ownership test can change with an edit, and `-v` reads a variable.
// valueTests and valueOperators are the unary and binary operators an
// expanded argument may stand beside; fileOperators read an entry.
var (
	testOperators = map[string]bool{
		"-e": true, "-f": true, "-d": true, "-h": true, "-L": true, "-z": true, "-n": true,
		"-eq": true, "-ne": true, "-lt": true, "-le": true, "-gt": true, "-ge": true,
		"-a": true, "-o": true,
	}
	valueTests     = map[string]bool{"!": true, "-z": true, "-n": true}
	valueOperators = map[string]bool{
		"=": true, "==": true, "!=": true,
		"-eq": true, "-ne": true, "-lt": true, "-le": true, "-gt": true, "-ge": true,
	}
	fileOperators = map[string]bool{"-e": true, "-f": true, "-d": true, "-h": true, "-L": true, "-a": true}
)

// testArguments reports `test` or `[` arguments the gate admits. With every
// argument fixed, each one whose value begins with `-` is a testOperators
// operator. An argument with an expansion must be a double-quoted single
// field and stand where test reads it as an operand, never as an operator
// or a file name: `[ X ]`, `[ ! X ]`, `[ -z X ]`, `[ -n X ]`, or `[ X OP Y ]`
// with a valueOperators OP, which test, given three arguments, applies
// before any other reading. test compares integers without arithmetic
// evaluation, so `-eq` writes nothing.
func testArguments(word string, args []string) bool {
	if word == "[" {
		if len(args) == 0 || unquote(args[len(args)-1]) != "]" {
			return false
		}
		args = args[:len(args)-1]
	}
	if !slices.ContainsFunc(args, func(raw string) bool { return !fixedWord(raw) }) {
		for _, arg := range args {
			if value := unquote(arg); strings.HasPrefix(value, "-") && len(value) > 1 && !testOperators[value] {
				return false
			}
		}
		return true
	}
	for _, arg := range args {
		if !fixedWord(arg) && !singleField(arg) {
			return false
		}
	}
	switch len(args) {
	case 1:
		return true
	case 2:
		return fixedWord(args[0]) && valueTests[unquote(args[0])]
	case 3:
		return fixedWord(args[1]) && valueOperators[unquote(args[1])]
	}
	return false
}

// conditionalArguments reports `[[ … ]]` arguments the gate admits. `[[`
// fixes its operators before expansion and splits no field, so an expanded
// argument is always an operand; it is admitted only when no fileOperators
// operator could make it a file name no token holds. The operators are
// testOperators less the integer ones, whose operands `[[` evaluates
// arithmetically (`[[ x++ -eq 0 ]]` assigns), and `=~`, which sets
// BASH_REMATCH, is refused.
func conditionalArguments(args []string) bool {
	if len(args) == 0 || args[len(args)-1] != "]]" {
		return false
	}
	args = args[:len(args)-1]
	computed, file := false, false
	for _, arg := range args {
		value := unquote(arg)
		switch {
		case value == "=~":
			// Judged before fixedWord, which its `~` would fail.
			return false
		case !fixedWord(arg):
			computed = true
		case strings.HasPrefix(value, "-") && len(value) > 1:
			if !testOperators[value] || valueOperators[value] {
				return false
			}
			file = file || fileOperators[value]
		}
	}
	return !(computed && file)
}

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

// plainOperands reports operand-reader arguments the gate admits: short
// options, judged by their unquoted value, with no recursion (`grep -r`) and
// no other pattern source (`grep -e`, `-f`), a digit count after `head` or
// `tail` -n or -c, a grep's pattern as one field with no glob a shell could
// expand into operands, and literal file operands (literalOperand).
func plainOperands(word string, args []string, listed map[string]bool) bool {
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
		case strings.HasPrefix(value, "-") && len(value) > 1:
			switch {
			case strings.HasPrefix(value, "--"):
				return false
			case patternReaders[word] && strings.ContainsAny(value, "rRdef"):
				return false
			case (word == "head" || word == "tail") && (value == "-n" || value == "-c"):
				count = true
			}
		case pattern:
			if !singleField(arg) {
				return false
			}
			pattern = false
		case !literalOperand(arg, listed):
			return false
		}
	}
	return !count
}

// literalOperand reports a literal path-shaped word (literalPathWord) a
// reader may open: one that does not end in `/` or `/.` and does not name a
// directory a listing of the file names, so no admitted reader is handed a
// listed directory. A reader handed another directory fails on it and reads
// nothing below it.
func literalOperand(raw string, listed map[string]bool) bool {
	if !literalPathWord(raw) {
		return false
	}
	value := unquote(raw)
	if strings.HasSuffix(value, "/") || strings.HasSuffix(value, "/.") {
		return false
	}
	return !listed[path.Clean(value)]
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
// a non-recursive `grep` given one literal pattern. Anything else, `xargs`, `while`,
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
			case expanding(args[index]) || !singleField(args[index]):
				// A computed pattern, or a glob the shell could expand into
				// file operands, is not a stdin-only filter.
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
