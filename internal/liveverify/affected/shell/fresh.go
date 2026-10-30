package shell

import (
	"strings"
)

// freshDirectorySource reports a `source` or `.` command, parsed.commands[at],
// whose target reads a runtime file inside a directory `mktemp -d` created in
// the same straight-line block (TSP-V0-004). Such a directory is new and
// empty when created, so what the command reads is what the script itself
// put there at run time, never a repository file. The target is `$NAME/REST`
// or `${NAME}/REST` with REST literal and free of `..`; NAME is bound exactly
// once in the file's text, by a command that is that assignment alone with the
// value one `$(mktemp ... -d ...)` substitution (no dry run, no pipeline or
// list inside); that command runs unconditionally before the source in one
// block (not after `&&`, `||` or `|`, not before `|` or `&`), with no source,
// load or eval between them; every other occurrence of NAME in the text is
// a `$NAME` or `${NAME...}` expansion that cannot assign. Every command
// between the assignment and the source passes freshAllowed, and nothing in
// the file can redirect `mktemp` or an allowed command (freshShadowed); when
// the file assigns PATH, each external command runs as `command -p NAME`.
// Anything else stays a dynamic source.
func freshDirectorySource(r *resolver, parsed script, text string, at int, target string) bool {
	if at >= len(parsed.commands) || parsed.commands[at].substituted {
		return false
	}
	match := variable.FindStringSubmatch(target)
	if match == nil {
		return false
	}
	name, rest := match[1]+match[2], target[len(match[0]):]
	if !strings.HasPrefix(rest, "/") || len(rest) == 1 || dynamic(rest) || nameContinues(rest, match[0]) {
		return false
	}
	for _, segment := range strings.Split(rest[1:], "/") {
		if segment == ".." {
			return false
		}
	}
	pathOverridden := !onlyRead(text, "PATH")
	assigned, ok := r.plainAt[name]
	if !ok || r.counts[name] != 1 || assigned >= at || !makesDirectory(r.values[name], pathOverridden) || freshShadowed(parsed, text, at) {
		return false
	}
	assignment, source := parsed.commands[assigned], parsed.commands[at]
	if assignment.substituted || assignment.block != source.block || assignment.block < 0 {
		return false
	}
	switch assignment.lead {
	case "&&", "||", "|", "&":
		return false
	}
	switch assignment.trail {
	case "|", "&":
		return false
	}
	check := freshCheck{parsed: parsed, block: assignment.block, pathOverridden: pathOverridden}
	for index := assigned + 1; index < at; index++ {
		// A function defined in the assignment's block between the two is
		// inert there; a call to it is checked where it is made.
		if last, ok := check.definedAt(index); ok {
			index = last
			continue
		}
		between := parsed.commands[index]
		if !between.substituted && between.block != assignment.block || !check.allowed(index, !between.substituted) {
			return false
		}
	}
	return onlyExpanded(text, name)
}

// freshCheck holds what freshAllowed needs about the file and the block
// holding the assignment.
type freshCheck struct {
	parsed         script
	block          int
	pathOverridden bool
}

// freshExternal are the external commands the rule trusts: `mktemp` in the
// assignment and the others between it and the source. A PATH assignment in
// the file could resolve them to a repository file, so they must then run as
// `command -p NAME`, which uses the default PATH.
var freshExternal = map[string]bool{"mktemp": true, "tar": true, "base64": true, "hostname": true}

// freshBuiltins are the builtins allowed between the assignment and the
// source; none writes a file or changes the working directory.
var freshBuiltins = map[string]bool{":": true, "true": true, "false": true, "echo": true, "printf": true, "exit": true, "return": true, "trap": true}

// freshShadowed reports a file in which mktemp or an allowed command may not
// be what it names: a function of that name or of `command`, an `alias`,
// `hash` or `enable` command, an assignment to TAPE or TAR_OPTIONS (which
// redirect tar), or a `source`, `.`, `load` or `eval` before the fresh
// source, whose file could define any of them.
func freshShadowed(parsed script, text string, at int) bool {
	for name := range parsed.functions {
		if name == "command" || freshExternal[name] || freshBuiltins[name] {
			return true
		}
	}
	for index, entry := range parsed.commands {
		word, _ := commandWord(plainWords(entry.words))
		switch word {
		case "alias", "hash", "enable":
			return true
		case "source", ".", "load", "eval":
			if index < at {
				return true
			}
		}
	}
	return !onlyRead(text, "TAPE") || !onlyRead(text, "TAR_OPTIONS")
}

// definedAt reports the definition of a function, in the assignment's block,
// whose body starts at command index, with the body's last command index.
func (f freshCheck) definedAt(index int) (int, bool) {
	for name, defined := range f.parsed.functions {
		if defined != index {
			continue
		}
		if first, last, ok := f.span(name); ok && first == index && f.parsed.bodies[name].parent == f.block {
			return last, true
		}
	}
	return 0, false
}

// span returns the command indexes of a function body defined once: from its
// definition to its last command, every one in the body's own block or a
// substitution within it. A nested block or an empty body fails.
func (f freshCheck) span(name string) (int, int, bool) {
	entry, ok := f.parsed.bodies[name]
	if !ok || f.parsed.definitions[name] != 1 {
		return 0, 0, false
	}
	first, last := f.parsed.functions[name], -1
	for index := first; index < len(f.parsed.commands); index++ {
		if f.parsed.commands[index].block == entry.block && !f.parsed.commands[index].substituted {
			last = index
		}
	}
	if last < 0 {
		return 0, 0, false
	}
	for _, inner := range f.parsed.commands[first : last+1] {
		if !inner.substituted && inner.block != entry.block {
			return 0, 0, false
		}
	}
	return first, last, true
}

// allowed reports a command that cannot make the fresh directory, or what the
// source reads in it, a repository file: no backquote or process
// substitution, redirections only to /dev/null or a file descriptor, no
// assignment or prefix other than `command -p` for an external command, and
// the command one of `:`, `true`, `false`, `echo`, `printf` without `-v`,
// `exit`, `return`, `hostname` without operands, `base64` with only decode
// options, a tar extraction of standard input (tarExtracts), a `trap` in the
// assignment's block whose handler for a signal other than EXIT is `exit`
// (trapAllowed), or, where calls is set, a call to a function defined once
// earlier in the assignment's block whose body's commands are all allowed
// without calls.
func (f freshCheck) allowed(index int, calls bool) bool {
	entry := f.parsed.commands[index]
	for at, word := range entry.words {
		if strings.Contains(word, "`") || strings.Contains(word, "<(") || strings.Contains(word, ">(") {
			return false
		}
		if word == redirect || word == writeRedirect {
			if at+1 >= len(entry.words) {
				return false
			}
			if target := unquote(entry.words[at+1]); target != "/dev/null" && target != "-" && strings.Trim(target, "0123456789") != "" {
				return false
			}
		}
	}
	plain := plainWords(entry.words)
	if len(plain) == 0 || assignment.MatchString(plain[0]) {
		return false
	}
	viaDefault := len(plain) > 2 && unquote(plain[0]) == "command" && unquote(plain[1]) == "-p"
	if viaDefault {
		plain = plain[2:]
	}
	word, args := unquote(plain[0]), plain[1:]
	if freshExternal[word] {
		if f.pathOverridden && !viaDefault {
			return false
		}
	} else if viaDefault {
		return false
	}
	switch word {
	case ":", "true", "false", "echo", "exit", "return":
		return true
	case "printf":
		for _, arg := range args {
			if strings.HasPrefix(unquote(arg), "-v") {
				return false
			}
		}
		return true
	case "hostname":
		return len(args) == 0
	case "base64":
		for _, arg := range args {
			switch unquote(arg) {
			case "-d", "-D", "--decode":
			default:
				return false
			}
		}
		return true
	case "tar":
		return tarExtracts(args)
	case "trap":
		return calls && entry.block == f.block && trapAllowed(args)
	}
	if !calls || !shadowed(f.parsed, word, index) || f.parsed.bodies[word].parent != f.block {
		return false
	}
	first, last, ok := f.span(word)
	if !ok {
		return false
	}
	for body := first; body <= last; body++ {
		if !f.allowed(body, false) {
			return false
		}
	}
	return true
}

// tarExtracts reports tar operands that extract standard input and name no
// member: `-x` or `--extract`, `-C`/`--directory` with its directory, `-f -`
// or `--file=-`, and short option groups of x, v, z, j, J, p, o and m.
func tarExtracts(args []string) bool {
	extract := false
	for index := 0; index < len(args); index++ {
		arg := unquote(args[index])
		switch {
		case arg == "-x" || arg == "--extract":
			extract = true
		case arg == "-C" || arg == "--directory":
			index++
			if index >= len(args) {
				return false
			}
		case strings.HasPrefix(arg, "--directory="), arg == "--file=-":
		case arg == "-f" || arg == "--file":
			index++
			if index >= len(args) || unquote(args[index]) != "-" {
				return false
			}
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-' && strings.Trim(arg[1:], "xvzjJpom") == "":
			extract = extract || strings.ContainsRune(arg, 'x')
		default:
			return false
		}
	}
	return extract
}

// trapAllowed reports trap operands that set a handler for EXIT, which runs
// only when the shell exits, or for INT, TERM, HUP or QUIT with the handler
// `exit` or `exit N`. DEBUG, ERR, RETURN and any other operand fail.
func trapAllowed(args []string) bool {
	if len(args) < 2 || strings.HasPrefix(unquote(args[0]), "-") {
		return false
	}
	handler := strings.Fields(unquote(args[0]))
	exits := len(handler) == 1 && handler[0] == "exit" || len(handler) == 2 && handler[0] == "exit" && strings.Trim(handler[1], "0123456789") == ""
	for _, raw := range args[1:] {
		switch signal := strings.TrimPrefix(unquote(raw), "SIG"); signal {
		case "EXIT", "0":
		case "INT", "TERM", "HUP", "QUIT", "1", "2", "3", "15":
			if !exits {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// makesDirectory reports a value that is one `$(mktemp ...)` or, always
// accepted and required when pathOverridden, `$(command -p mktemp ...)`
// substitution creating a directory: a `-d` short option or `--directory`,
// and neither the dry-run option (`-u`, `--dry-run`) nor a list, pipeline or
// redirection.
func makesDirectory(value string, pathOverridden bool) bool {
	inner, rest, ok := substitution(value)
	if !ok || rest != "" || !strings.HasPrefix(value, "$(") || strings.ContainsAny(inner, ";&|<>`\n") {
		return false
	}
	words := fieldsTop(inner)
	if len(words) > 2 && words[0] == "command" && words[1] == "-p" {
		words = words[2:]
	} else if pathOverridden {
		return false
	}
	if len(words) < 2 || words[0] != "mktemp" {
		return false
	}
	directory := false
	for _, word := range words[1:] {
		switch {
		case word == "--":
			return directory
		case word == "--directory":
			directory = true
		case word == "--dry-run" || strings.HasPrefix(word, "--dry"):
			return false
		case strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "--"):
			if strings.ContainsRune(word, 'u') {
				return false
			}
			if strings.ContainsRune(word, 'd') {
				directory = true
			}
		}
	}
	return directory
}

// onlyExpanded reports text in which name occurs, as a whole word, exactly
// once as an assignment (`name=` at a word start) and otherwise only as an
// expansion `$name` or `${name...}` other than `${name=...}` and
// `${name:=...}`. Any other occurrence, a `read`, `for`, `local`, `unset`,
// `printf -v`, `export` operand or prose included, refuses.
func onlyExpanded(text, name string) bool {
	assignments, ok := occurrences(text, name)
	return ok && assignments == 1
}

// onlyRead reports text in which name occurs, as a whole word, only as an
// expansion that cannot assign: the file never sets it.
func onlyRead(text, name string) bool {
	assignments, ok := occurrences(text, name)
	return ok && assignments == 0
}

// occurrences counts name's whole-word `name=` assignments in text and
// reports whether every other occurrence is a non-assigning expansion.
func occurrences(text, name string) (int, bool) {
	assignments := 0
	for from := 0; ; {
		offset := strings.Index(text[from:], name)
		if offset < 0 {
			break
		}
		at := from + offset
		end := at + len(name)
		from = end
		if at > 0 && nameByte(text[at-1]) || end < len(text) && nameByte(text[end]) {
			continue
		}
		after := text[end:]
		switch {
		case at >= 2 && text[at-2:at] == "${":
			if strings.HasPrefix(after, "=") || strings.HasPrefix(after, ":=") {
				return assignments, false
			}
		case at >= 1 && text[at-1] == '$':
		case strings.HasPrefix(after, "=") && (at == 0 || strings.IndexByte(" \t\n;", text[at-1]) >= 0):
			assignments++
		default:
			return assignments, false
		}
	}
	return assignments, true
}

func nameByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
