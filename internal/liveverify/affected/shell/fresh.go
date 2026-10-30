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
// a `$NAME` or `${NAME...}` expansion that cannot assign. Anything else stays
// a dynamic source.
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
	assigned, ok := r.plainAt[name]
	if !ok || r.counts[name] != 1 || assigned >= at || !makesDirectory(r.values[name]) {
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
	for _, between := range parsed.commands[assigned+1 : at] {
		word, _ := commandWord(plainWords(between.words))
		if word == "source" || word == "." || word == "load" || word == "eval" {
			return false
		}
	}
	return onlyExpanded(text, name)
}

// makesDirectory reports a value that is one `$(mktemp ...)` substitution
// creating a directory: a `-d` short option or `--directory`, and neither the
// dry-run option (`-u`, `--dry-run`) nor a list, pipeline or redirection.
func makesDirectory(value string) bool {
	inner, rest, ok := substitution(value)
	if !ok || rest != "" || !strings.HasPrefix(value, "$(") || strings.ContainsAny(inner, ";&|<>`\n") {
		return false
	}
	words := fieldsTop(inner)
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
				return false
			}
		case at >= 1 && text[at-1] == '$':
		case strings.HasPrefix(after, "=") && (at == 0 || strings.IndexByte(" \t\n;", text[at-1]) >= 0):
			assignments++
		default:
			return false
		}
	}
	return assignments == 1
}

func nameByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
