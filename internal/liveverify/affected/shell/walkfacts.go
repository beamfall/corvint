// SPDX-License-Identifier: Apache-2.0
package shell

import (
	"path"
	"slices"
	"strings"
)

// Walk facts: the directories a unit's own commands list or walk, and the
// first construct whose listing, walk or run the plugin cannot resolve
// (AFP-V0-9990). The planner reads them only to decide which units may have
// observed a deleted shell script it admits as a deletion (AFP-V0-9992,
// AFP-V0-9993); no other rule uses them.
//
// The recognised walkers are a closed list: ls, find, du, tree, the
// recursive forms of grep, egrep, fgrep and zgrep, rg, ag and ack
// (walkerPaths), the git listers of AFP-V0-9973, and a shell glob
// (literalPathGlob). A literal operand of one of them names a walk
// directory. Every construct below names an opacity reason instead, because
// what it lists or runs is not a function of the unit's words.
const (
	// OpaqueComputedWalkOperand is a walker path operand that holds an
	// expansion and is not below a private temporary root.
	OpaqueComputedWalkOperand = "computed-walk-operand"
	// OpaqueClimbingWalkOperand is a walker path operand that is `/`,
	// starts with `~` or holds a `..` component: it may walk above the
	// directory the plan resolves operands against.
	OpaqueClimbingWalkOperand = "climbing-walk-operand"
	// OpaqueTreeWalk is a walk from the working directory (AFP-V0-9134,
	// AFP-V0-9973), which may list any path. A test that walks is an
	// unbounded reader too; a helper is not (V1-4283).
	OpaqueTreeWalk = "tree-walk"
	// OpaqueForwardedCommand is a command word that is a positional
	// expansion (`"$@"`, `$*`, `$1`): the caller's words choose the command,
	// so a walker passed as arguments is not seen as one.
	OpaqueForwardedCommand = "forwarded-command"
	// OpaqueComputedCommand is any other command word that holds an
	// expansion and links no repository script.
	OpaqueComputedCommand = "computed-command"
	// OpaqueEval is `eval`, which runs text as commands.
	OpaqueEval = "eval"
	// OpaqueXargs is `xargs`, which runs a command on words read from its
	// input.
	OpaqueXargs = "xargs"
	// OpaqueInterpreter is a program interpreter (interpreters) not run as
	// `INTERPRETER FILE` with a literal file operand first: inline code, a
	// module, or a program read from standard input.
	OpaqueInterpreter = "interpreter"
	// OpaqueAwkCommand is an awk program that may run a command (`system`,
	// `getline` or a `|`).
	OpaqueAwkCommand = "awk-command"
	// OpaqueWorkingDirectoryListing is a listing of the working directory
	// (`ls` with no operand, a glob whose first component holds the
	// wildcard): the plan does not know which directory that is.
	OpaqueWorkingDirectoryListing = "working-directory-listing"
	// OpaqueComputedGlob is an unquoted glob after an expansion that no
	// private temporary root or AFP-V0-3333 root variable bounds.
	OpaqueComputedGlob = "computed-glob"
	// OpaqueTokenBound is a unit whose path tokens the plugin dropped at
	// its bound (PathTokensBounded), so its reads are unknown.
	OpaqueTokenBound = "token-bound"
	// OpaqueToolchain is a helper that runs a toolchain (Toolchains), whose
	// build or test may list repository files. A test that does is an
	// unbounded reader already.
	OpaqueToolchain = "toolchain"
)

// interpreters run a program that may list any directory. The value reports
// whether a planned language plugin observes the program file: then a
// literal program file operand is that plugin's unit, judged by its own
// reads, and only another form is opaque. Every form of an interpreter whose
// programs no plugin observes is opaque (OpaqueInterpreter).
var interpreters = map[string]bool{
	"python": true, "python2": true, "python3": true, "ruby": true,
	"node": true, "nodejs": true, "deno": true, "bun": true,
	"perl": false, "php": false, "lua": false, "luajit": false, "Rscript": false,
	"tclsh": false, "wish": false, "pwsh": false, "osascript": false,
}

// walkFacts accumulates one unit's walk directories and its first opacity
// reason.
type walkFacts struct {
	directories map[string]bool
	opaque      string
}

func (facts *walkFacts) hold(reason string) {
	if facts.opaque == "" {
		facts.opaque = reason
	}
}

// observe records what one command run (a command word and its operands,
// after any wrapper) lists, walks or runs. temp names the file's private
// temporary roots, whose operands name no repository path. linked reports
// that a computed command word linked a repository script.
func (facts *walkFacts) observe(word string, args []string, temp map[string]bool, linked bool) {
	base := path.Base(word)
	switch {
	case positional(word):
		facts.hold(OpaqueForwardedCommand)
	case strings.ContainsAny(word, "$`") && !linked:
		facts.hold(OpaqueComputedCommand)
	case word == "eval":
		facts.hold(OpaqueEval)
	case word == "xargs":
		facts.hold(OpaqueXargs)
	case interpreterRunsUnobserved(base, args):
		facts.hold(OpaqueInterpreter)
	case awkCommands[base] && slices.ContainsFunc(args, awkRunsCommand):
		facts.hold(OpaqueAwkCommand)
	}
	paths, recursive, ok := walkerPaths(word, args, false)
	if !ok {
		return
	}
	if len(paths) == 0 && !recursive {
		facts.hold(OpaqueWorkingDirectoryListing)
	}
	for _, value := range paths {
		facts.operand(value, temp)
	}
}

// operand classifies one walker path operand (unquoted).
func (facts *walkFacts) operand(value string, temp map[string]bool) {
	switch {
	case value == "" || value == "." || value == "./":
		// A walk of the working directory is OpaqueTreeWalk, recorded
		// by the caller from walksTree.
		return
	case strings.ContainsAny(value, "$`"):
		if !tempRootedWord(value, temp) && !underTempRoot(value, temp) {
			facts.hold(OpaqueComputedWalkOperand)
		}
		return
	case value == "/" || strings.HasPrefix(value, "~") || slices.Contains(strings.Split(value, "/"), ".."):
		facts.hold(OpaqueClimbingWalkOperand)
		return
	case strings.HasPrefix(value, "/"):
		// An absolute literal names no repository path: the plan does not
		// know where the checkout lies.
		return
	}
	if directory, ok := walkDirectory(value); ok {
		facts.directories[directory] = true
	}
}

// literalWalkDirectories returns the directories the literal path operands of
// a listing or walking command name (walkDirectory), skipping a computed
// operand, the working directory, an absolute operand and one that climbs
// (AFP-V0-9991). A glob operand names its literal prefix.
func literalWalkDirectories(word string, args []string) []string {
	paths, _, ok := walkerPaths(word, args, false)
	if !ok {
		return nil
	}
	var directories []string
	for _, value := range paths {
		if value == "" || strings.ContainsAny(value, "$`~\\") || strings.HasPrefix(value, "/") || slices.Contains(strings.Split(value, "/"), "..") {
			continue
		}
		if directory, ok := walkDirectory(value); ok {
			directories = append(directories, directory)
		}
	}
	return directories
}

// walkDirectory normalises a literal walker operand to the directory it
// names, relative and without `./` or a trailing `/`. A glob operand names
// the literal directory before its first wildcard component; one whose first
// component holds the wildcard names none (it is a walk of the working
// directory's entries, held by the caller).
func walkDirectory(value string) (string, bool) {
	for strings.HasPrefix(value, "./") {
		value = strings.TrimLeft(value[2:], "/")
	}
	value = strings.TrimSuffix(strings.TrimRight(value, "/"), "/.")
	value = strings.TrimRight(value, "/")
	parts := strings.Split(value, "/")
	kept := parts[:0]
	for _, part := range parts {
		if strings.ContainsAny(part, "*?[") {
			break
		}
		if part == "" || part == "." {
			continue
		}
		kept = append(kept, part)
	}
	if len(kept) == 0 {
		return "", false
	}
	return strings.Join(kept, "/"), true
}

// positional reports a command word that is a positional or special
// parameter expansion, alone.
func positional(word string) bool {
	switch word {
	case "$@", "$*", "${@}", "${*}":
		return true
	}
	name := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(word, "${"), "$"), "}")
	return strings.HasPrefix(word, "$") && len(name) == 1 && name[0] >= '0' && name[0] <= '9'
}

// interpreterRunsUnobserved reports an interpreter run whose program no
// plugin observes: any run of an interpreter without a planned plugin, and a
// run of one with a plugin that is not `INTERPRETER FILE`.
func interpreterRunsUnobserved(base string, args []string) bool {
	observed, known := interpreters[base]
	return known && (!observed || !programOperand(args))
}

// programOperand reports an interpreter run as `INTERPRETER FILE ...`: its
// first operand is a literal file name with a suffix, and no option comes
// before it.
func programOperand(args []string) bool {
	if len(args) == 0 {
		return false
	}
	value := unquote(args[0])
	if strings.HasPrefix(value, "-") || strings.ContainsAny(value, "$`*?[~") || strings.HasPrefix(value, "/dev/") {
		return false
	}
	return path.Ext(value) != ""
}

// awkRunsCommand reports an awk argument that may run a command.
func awkRunsCommand(raw string) bool {
	value := unquote(raw)
	return strings.Contains(value, "system") || strings.Contains(value, "getline") || strings.Contains(value, "|")
}

// words classifies the unquoted globs among one command's words: a glob after
// an expansion that literalPathGlob does not bound and that no private
// temporary root holds is OpaqueComputedGlob, and a literal glob whose first
// component holds the wildcard is OpaqueWorkingDirectoryListing. The
// operands of `[[` are not globbed by the shell; those of `[` and `test` are.
func (facts *walkFacts) words(words []string, temp map[string]bool) {
	if word, _ := commandWord(plainWords(words)); word == "[[" {
		return
	}
	for _, raw := range words {
		if raw == "[" || raw == "[[" || raw == "]" || raw == "]]" {
			continue
		}
		glob, expands, at := unquotedGlob(raw)
		if !glob {
			continue
		}
		if _, ok := literalPathGlob(raw); ok || tempRootedWord(raw, temp) {
			continue
		}
		switch {
		case expands:
			facts.hold(OpaqueComputedGlob)
		case !strings.Contains(unquote(raw)[:min(at, len(unquote(raw)))], "/"):
			facts.hold(OpaqueWorkingDirectoryListing)
		}
	}
}

// unquotedGlob reports whether raw holds a `*`, `?` or `[` outside quotes,
// parameter braces, command and arithmetic substitutions, whether it holds an
// expansion, and the offset of the first wildcard in the unquoted text. An
// assignment word's value is not globbed by the shell unless it is an array,
// which the lexer does not split, so an assignment word reports no glob.
func unquotedGlob(raw string) (glob, expands bool, at int) {
	if assignment.MatchString(raw) {
		return false, strings.ContainsAny(raw, "$`"), 0
	}
	quote := byte(0)
	plain := 0
	for index := 0; index < len(raw); index++ {
		c := raw[index]
		switch {
		case c == '\\' && quote != '\'':
			index++
			plain++
		case quote != 0:
			if c == quote {
				quote = 0
			} else if c == '$' && quote == '"' {
				expands = true
				index = skipExpansion(raw, index)
			} else if c == '`' {
				expands = true
			}
			if quote != 0 || c != '"' && c != '\'' {
				plain++
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '`':
			expands = true
			plain++
		case c == '$':
			expands = true
			index = skipExpansion(raw, index)
			plain++
		case c == '*' || c == '?' || c == '[':
			if !glob {
				glob, at = true, plain
			}
			plain++
		default:
			plain++
		}
	}
	return glob, expands, at
}

// skipExpansion returns the index of the last byte of the expansion that
// starts with the `$` at index: a balanced `${...}`, `$(...)` or `$((...))`,
// or a name or special parameter.
func skipExpansion(raw string, index int) int {
	if index+1 >= len(raw) {
		return index
	}
	open := raw[index+1]
	if open == '{' || open == '(' {
		closing := byte('}')
		if open == '(' {
			closing = ')'
		}
		depth := 0
		for at := index + 1; at < len(raw); at++ {
			switch raw[at] {
			case open:
				depth++
			case closing:
				depth--
				if depth == 0 {
					return at
				}
			}
		}
		return len(raw) - 1
	}
	at := index + 1
	if !identifierByte(raw, at) {
		return at
	}
	for identifierByte(raw, at+1) {
		at++
	}
	return at
}
